package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"backend/internal/application"
	"backend/internal/domain"
	"backend/internal/infrastructure/messaging"
	"backend/internal/infrastructure/repository"
	"backend/internal/infrastructure/storage"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type S3Listener struct {
	gameRepo      repository.GameRepository
	natsClient    repository.MessagingClient
	validationSvc *application.ValidationService
	storage       *storage.MinIOAdapter
	backendURL    string
	appEnv        string
	logger        *zap.SugaredLogger
}

func NewS3Listener(
	gameRepo repository.GameRepository,
	natsClient repository.MessagingClient,
	validationSvc *application.ValidationService,
	storage *storage.MinIOAdapter,
	backendURL string,
	appEnv string,
	logger *zap.SugaredLogger,
) *S3Listener {
	return &S3Listener{
		gameRepo:      gameRepo,
		natsClient:    natsClient,
		validationSvc: validationSvc,
		storage:       storage,
		backendURL:    backendURL,
		appEnv:        appEnv,
		logger:        logger,
	}
}

func (l *S3Listener) Start() {
	// Subscribe to S3 stored events
	subj := messaging.GetS3StoredSubject()
	_, err := l.natsClient.Subscribe(subj, func(msg *nats.Msg) {
		l.logger.Debugw("Received S3 message", "data", string(msg.Data))
		var payload domain.S3StoredEvent

		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			l.logger.Errorw("Failed to unmarshal S3 message", "error", err)
			return
		}

		if payload.Status == "success" {
			l.logger.Infow("S3 Upload successful, starting validation", "game_id", payload.GameID)

			// 1. Get Game Record
			game, err := l.gameRepo.GetGame(context.Background(), strconv.Itoa(payload.GameID))
			if err != nil {
				l.logger.Errorw("Failed to find game", "game_id", payload.GameID, "error", err)
				return
			}

			// 2. Download to Temp Location (Direct Access from MinIO)
			tempZip := filepath.Join("/tmp", fmt.Sprintf("val_%d.zip", payload.GameID))
			tempDir := filepath.Join("/tmp", fmt.Sprintf("val_ext_%d", payload.GameID))
			defer os.RemoveAll(tempZip)
			defer os.RemoveAll(tempDir)

			// Parsing bucket and key from ARN: arn:aws:s3:::bucket/key
			// Expected format: arn:aws:s3:::gamelift_games/uploads/games/11/game.zip
			arnParts := strings.Split(payload.StorageARN, ":::")
			if len(arnParts) < 2 {
				l.logger.Errorw("Invalid StorageARN", "arn", payload.StorageARN)
				return
			}
			pathParts := strings.SplitN(arnParts[1], "/", 2)
			if len(pathParts) < 2 {
				l.logger.Errorw("Could not parse bucket/key from ARN", "arn", payload.StorageARN)
				return
			}
			bucket, key := pathParts[0], pathParts[1]

			l.logger.Infow("Directly downloading from MinIO", "bucket", bucket, "key", key)
			if err := l.storage.DownloadFile(context.Background(), bucket, key, tempZip); err != nil {
				l.logger.Errorw("Direct download failed", "error", err)
				l.gameRepo.UpdateGameStatus(context.Background(), strconv.Itoa(payload.GameID), domain.GameStatusFailed)
				return
			}

			// 3. Validate
			var manifest domain.GameManifest
			json.Unmarshal([]byte(game.Manifest), &manifest)

			if err := l.validationSvc.Unzip(tempZip, tempDir); err != nil {
				l.logger.Errorw("Unzip failed", "error", err)
				l.gameRepo.UpdateGameStatus(context.Background(), strconv.Itoa(payload.GameID), domain.GameStatusFailed)
				return
			}

			if err := l.validationSvc.ValidateStructure(tempDir, manifest); err != nil {
				l.logger.Errorw("Validation failed", "game_id", payload.GameID, "error", err)
				l.gameRepo.UpdateGameStatus(context.Background(), strconv.Itoa(payload.GameID), domain.GameStatusFailed)
				return
			}

			// 4. Finalize
			l.logger.Infow("Validation passed! Finalizing game", "game_id", payload.GameID)
			err = l.gameRepo.UpdateGameStatus(context.Background(), strconv.Itoa(payload.GameID), domain.GameStatusStored)
			if err != nil {
				l.logger.Errorw("Failed to update status", "error", err)
				return
			}

			// 5. Trigger Specialized EC2 Service for VM Commissioning
			ec2Subj := messaging.GetEC2ProvisionSubject()
			ec2Payload := domain.EC2ProvisionRequest{
				Profile: "gamelift",
				Specs: map[string]int{
					"cpu": 2,
					"ram": 4096,
				},
				Parameters: map[string]string{
					"game_id":      strconv.Itoa(payload.GameID),
					"storage_arn":  payload.StorageARN,
					"headless_bin": manifest.HeadlessBin,
					"game_name":    game.Name,
					"backend_url":  l.backendURL,
				},
				UserID: game.UserID,
			}
			ec2Data, _ := json.Marshal(ec2Payload)
			l.natsClient.Publish(ec2Subj, ec2Data)
			l.logger.Infow("Triggered EC2 Provisioning", "game_id", payload.GameID, "game_name", game.Name)
		}
	})
	if err != nil {
		l.logger.Fatalf("Failed to subscribe to NATS", "error", err)
	}

	// Subscribe to EC2 instance lifecycle events
	lifecycleSubj := messaging.GetEC2InstanceLifecycleSubject()
	_, err = l.natsClient.Subscribe(lifecycleSubj, func(msg *nats.Msg) {
		l.handleInstanceLifecycle(msg)
	})
	if err != nil {
		l.logger.Fatalf("Failed to subscribe to lifecycle topic", "error", err)
	}
}

func (l *S3Listener) downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (l *S3Listener) handleInstanceLifecycle(msg *nats.Msg) {
	l.logger.Debugw("Received lifecycle message", "raw", string(msg.Data))

	var payload struct {
		InstanceID string `json:"instance_id"`
		EventType  string `json:"event_type"`
		Stage      string `json:"stage"`
		Message    string `json:"message"`
		Timestamp  string `json:"timestamp"`
	}

	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		l.logger.Errorw("Failed to unmarshal lifecycle event", "error", err)
		return
	}

	l.logger.Infow("EC2 Lifecycle Event",
		"instance_id", payload.InstanceID,
		"event_type", payload.EventType,
		"stage", payload.Stage,
		"message", payload.Message,
		"timestamp", payload.Timestamp,
	)
}