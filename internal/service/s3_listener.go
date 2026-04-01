package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"backend/internal/domain"
	"backend/internal/infrastructure/storage"
	"backend/internal/interfaces"
	"backend/internal/messaging"

	"github.com/nats-io/nats.go"
)

type S3Listener struct {
	gameRepo      interfaces.GameRepository
	natsClient    *messaging.NatsClient
	validationSvc *ValidationService
	storage       *storage.MinIOAdapter
	backendURL    string
}

func NewS3Listener(
	gameRepo interfaces.GameRepository, 
	natsClient *messaging.NatsClient, 
	validationSvc *ValidationService,
	storage *storage.MinIOAdapter,
	backendURL string,
) *S3Listener {
	return &S3Listener{
		gameRepo:      gameRepo,
		natsClient:    natsClient,
		validationSvc: validationSvc,
		storage:       storage,
		backendURL:    backendURL,
	}
}

func (l *S3Listener) Start() {
	subj := messaging.Subject{
		Env:        "dev",
		Service:    "s3",
		Version:    "v1",
		Domain:     "game",
		ActionType: "stored",
	}

	_, err := l.natsClient.Subscribe(subj, func(msg *nats.Msg) {
		log.Printf("[S3Listener] Received message from NATS: %s", string(msg.Data))
		var payload struct {
			GameID      int    `json:"game_id"`
			StorageARN  string `json:"s3_arn"`
			DownloadURL string `json:"download_url"`
			Status      string `json:"status"`
		}

		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Printf("[S3Listener] Failed to unmarshal message: %v", err)
			return
		}

		if payload.Status == "success" {
			log.Printf("[S3Listener] S3 Upload successful for Game %d. Starting Async Validation...", payload.GameID)

			// 1. Get Game Record
			game, err := l.gameRepo.GetGame(payload.GameID)
			if err != nil {
				log.Printf("[S3Listener] Failed to find game %d: %v", payload.GameID, err)
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
				log.Printf("[S3Listener] Invalid StorageARN: %s", payload.StorageARN)
				return
			}
			pathParts := strings.SplitN(arnParts[1], "/", 2)
			if len(pathParts) < 2 {
				log.Printf("[S3Listener] Could not parse bucket/key from ARN: %s", payload.StorageARN)
				return
			}
			bucket, key := pathParts[0], pathParts[1]

			log.Printf("[S3Listener] Directly downloading %s/%s from MinIO...", bucket, key)
			if err := l.storage.DownloadFile(context.Background(), bucket, key, tempZip); err != nil {
				log.Printf("[S3Listener] Direct download failed: %v", err)
				l.gameRepo.UpdateGameStatus(payload.GameID, "failed", "")
				return
			}

			// 4. Validate
			var manifest domain.GameManifest
			json.Unmarshal([]byte(game.Manifest), &manifest)

			if err := l.validationSvc.Unzip(tempZip, tempDir); err != nil {
				log.Printf("[S3Listener] Unzip failed: %v", err)
				l.gameRepo.UpdateGameStatus(payload.GameID, "failed", "")
				return
			}

			if err := l.validationSvc.ValidateStructure(tempDir, manifest); err != nil {
				log.Printf("[S3Listener] Validation failed for Game %d: %v", payload.GameID, err)
				l.gameRepo.UpdateGameStatus(payload.GameID, "failed", "")
				return
			}

			// 5. Finalize
			log.Printf("[S3Listener] Validation passed! Finalizing game %d", payload.GameID)
			err = l.gameRepo.UpdateGameStatus(payload.GameID, domain.GameStatusStored, payload.StorageARN)
			if err != nil {
				log.Printf("[S3Listener] Failed to update status: %v", err)
				return
			}

			// 6. Trigger Specialized EC2 Service for VM Commissioning
			ec2Subj := messaging.Subject{
				Env:        "dev",
				Service:    "ec2",
				Version:    "v1",
				Domain:     "vm",
				ActionType: "provision",
			}
			ec2Payload := map[string]interface{}{
				"profile": "gamelift",
				"specs": map[string]int{
					"cpu": 2,
					"ram": 4096,
				},
				"parameters": map[string]string{
					"game_id":      strconv.Itoa(payload.GameID),
					"storage_arn":  payload.StorageARN,
					"headless_bin": manifest.HeadlessBin,
					"game_name":    game.Name,
					"backend_url":  l.backendURL,
				},
				"user_id": game.UserID,
			}
			ec2Data, _ := json.Marshal(ec2Payload)
			l.natsClient.Publish(ec2Subj, ec2Data)
			log.Printf("[S3Listener] Triggered EC2 Provisioning for Game %d (%s)", payload.GameID, game.Name)
		}
	})

	if err != nil {
		log.Fatalf("[S3Listener] Failed to subscribe to NATS: %v", err)
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
