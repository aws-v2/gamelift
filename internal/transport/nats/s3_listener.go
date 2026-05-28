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
	sseRegistry   *application.SSERegistry
}

func NewS3Listener(
	gameRepo repository.GameRepository,
	natsClient repository.MessagingClient,
	validationSvc *application.ValidationService,
	storage *storage.MinIOAdapter,
	backendURL string,
	appEnv string,
	logger *zap.SugaredLogger,
	sseRegistry *application.SSERegistry,
) *S3Listener {
	return &S3Listener{
		gameRepo:      gameRepo,
		natsClient:    natsClient,
		validationSvc: validationSvc,
		storage:       storage,
		backendURL:    backendURL,
		appEnv:        appEnv,
		logger:        logger,
		sseRegistry:   sseRegistry,
	}
}

const (
	EventInstanceStarted = "INSTANCE_STARTED"
	EventInstanceStopped = "INSTANCE_STOPPED"
	EventInstanceError   = "INSTANCE_ERROR"

	EventHealthUpdate         = "HEALTH_UPDATE"
	EventProvisioningProgress = "PROVISIONING_PROGRESS"
	EventInstanceProvisioned  = "INSTANCE_PROVISIONED"
)

type InstanceLifecycleEvent struct {
	CorrelationID string                   `json:"correlation_id"`
	InstanceID    string                   `json:"instance_id"`
	EventType     string                   `json:"event_type"`
	Timestamp     string                   `json:"timestamp"`
	Payload       InstanceLifecyclePayload `json:"payload"`
	AgentURL      string                   `json:"agent_url,omitempty"`
}

type InstanceLifecyclePayload struct {
	IPAddress   string           `json:"ip_address"`
	VPCID       string           `json:"vpc_id"`
	ServicePort int              `json:"service_port"`
	Metadata    InstanceMetadata `json:"metadata"`
	AgentWS     string           `json:"agent_ws,omitempty"`
}

type InstanceMetadata struct {
	InstanceType string `json:"instance_type"`
	AMIID        string `json:"ami_id"`
}

func (l *S3Listener) Start(ctx context.Context) {
	// userId := ctx.("user_id").(string)
	// ── S3 stored events ──────────────────────────────────────────────────────
	s3Subj := messaging.GetS3StoredSubject()
	l.logger.Infow("S3_LISTENER_STARTING", "subject", s3Subj, "env", l.appEnv)

	_, err := l.natsClient.Subscribe(s3Subj, func(msg *nats.Msg) {
		l.handleS3Stored(msg)
	})
	if err != nil {
		l.logger.Errorw("S3_LISTENER_SUBSCRIBE_FAILED", "subject", s3Subj, "error", err)
		return
	}
	l.logger.Infow("S3_LISTENER_SUBSCRIBED", "subject", s3Subj)

	// ── EC2 instance lifecycle events ─────────────────────────────────────────
	lifecycleSubj := messaging.GetEC2InstanceLifecycleSubject()
	l.logger.Infow("S3_LISTENER_LIFECYCLE_STARTING", "subject", lifecycleSubj, "env", l.appEnv)

	_, err = l.natsClient.Subscribe(lifecycleSubj, func(msg *nats.Msg) {
		l.handleInstanceLifecycle(msg)
	})
	if err != nil {
		l.logger.Errorw("S3_LISTENER_LIFECYCLE_SUBSCRIBE_FAILED", "subject", lifecycleSubj, "error", err)
		return
	}
	l.logger.Infow("S3_LISTENER_LIFECYCLE_SUBSCRIBED", "subject", lifecycleSubj)
}

func (l *S3Listener) handleS3Stored(msg *nats.Msg) {


	
	l.logger.Debugw("S3_STORED_MESSAGE_RECEIVED", "bytes", len(msg.Data))

	var payload domain.S3StoredEvent
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		l.logger.Errorw("S3_STORED_UNMARSHAL_FAILED", "bytes", len(msg.Data), "error", err)
		return
	}

	if payload.Status != "success" {
		l.logger.Warnw("S3_STORED_NON_SUCCESS_STATUS", "game_id", payload.GameID, "status", payload.Status)
		return
	}

	l.logger.Infow("S3_STORED_UPLOAD_SUCCESS", "game_id", payload.GameID, "arn", payload.StorageARN)

	// 1. Fetch game record
	game, err := l.gameRepo.GetGame(context.Background(), strconv.Itoa(payload.GameID), l.logger)
	if err != nil {
		l.logger.Errorw("S3_STORED_GAME_NOT_FOUND", "game_id", payload.GameID, "error", err)
		return
	}
	l.logger.Infow("S3_STORED_GAME_FETCHED", "game_id", payload.GameID, "name", game.Name)

	// 2. Resolve temp paths
	tempZip := filepath.Join("/tmp", fmt.Sprintf("val_%d.zip", payload.GameID))
	tempDir := filepath.Join("/tmp", fmt.Sprintf("val_ext_%d", payload.GameID))
	defer os.RemoveAll(tempZip)
	defer os.RemoveAll(tempDir)

	// 3. Parse ARN → bucket + key
	// Expected: arn:aws:s3:::gamelift_games/uploads/games/11/game.zip
	arnParts := strings.Split(payload.StorageARN, ":::")
	if len(arnParts) < 2 {
		l.logger.Errorw("S3_STORED_INVALID_ARN", "game_id", payload.GameID, "arn", payload.StorageARN)
		return
	}
	pathParts := strings.SplitN(arnParts[1], "/", 2)
	if len(pathParts) < 2 {
		l.logger.Errorw("S3_STORED_ARN_PARSE_FAILED", "game_id", payload.GameID, "arn", payload.StorageARN)
		return
	}
	bucket, key := pathParts[0], pathParts[1]

	// 4. Download from MinIO
	l.logger.Infow("S3_STORED_DOWNLOAD_STARTING", "game_id", payload.GameID, "bucket", bucket, "key", key)
	if err := l.storage.DownloadFile(context.Background(), bucket, key, tempZip); err != nil {
		l.logger.Errorw("S3_STORED_DOWNLOAD_FAILED", "game_id", payload.GameID, "bucket", bucket, "key", key, "error", err)
		l.gameRepo.UpdateGameStatus(context.Background(), strconv.Itoa(payload.GameID), domain.GameStatusFailed, l.logger)
		return
	}
	l.logger.Infow("S3_STORED_DOWNLOAD_SUCCESS", "game_id", payload.GameID, "dest", tempZip)

	// 5. Parse manifest
	var manifest domain.GameManifest
	if err := json.Unmarshal([]byte(game.Manifest), &manifest); err != nil {
		l.logger.Warnw("S3_STORED_MANIFEST_PARSE_FAILED", "game_id", payload.GameID, "error", err)
	}

	// 6. Unzip
	l.logger.Infow("S3_STORED_UNZIP_STARTING", "game_id", payload.GameID, "src", tempZip, "dest", tempDir)
	if err := l.validationSvc.Unzip(tempZip, tempDir); err != nil {
		l.logger.Errorw("S3_STORED_UNZIP_FAILED", "game_id", payload.GameID, "error", err)
		l.gameRepo.UpdateGameStatus(context.Background(), strconv.Itoa(payload.GameID), domain.GameStatusFailed, l.logger)
		return
	}
	l.logger.Infow("S3_STORED_UNZIP_SUCCESS", "game_id", payload.GameID, "dest", tempDir)

	// 7. Validate structure
	l.logger.Infow("S3_STORED_VALIDATION_STARTING", "game_id", payload.GameID)
	if err := l.validationSvc.ValidateStructure(tempDir, manifest); err != nil {
		l.logger.Errorw("S3_STORED_VALIDATION_FAILED", "game_id", payload.GameID, "error", err)
		l.gameRepo.UpdateGameStatus(context.Background(), strconv.Itoa(payload.GameID), domain.GameStatusFailed, l.logger)
		return
	}
	l.logger.Infow("S3_STORED_VALIDATION_PASSED", "game_id", payload.GameID)

	// 8. Mark game as stored
	if err := l.gameRepo.UpdateGameStatus(context.Background(), strconv.Itoa(payload.GameID), domain.GameStatusStored, l.logger); err != nil {
		l.logger.Errorw("S3_STORED_STATUS_UPDATE_FAILED", "game_id", payload.GameID, "target_status", domain.GameStatusStored, "error", err)
		return
	}
	l.logger.Infow("S3_STORED_STATUS_UPDATED", "game_id", payload.GameID, "new_status", domain.GameStatusStored)

	// 9. Trigger EC2 provisioning
	ec2Subj := messaging.GetEC2ProvisionSubject()
	ec2Payload := domain.EC2ProvisionRequest{
		Profile: "gamelift",
		Specs:   map[string]int{"cpu": 2, "ram": 4096},
		Parameters: map[string]string{
			"game_id":      strconv.Itoa(payload.GameID),
			"storage_arn":  payload.StorageARN,
			"headless_bin": manifest.HeadlessBin,
			"game_name":    game.Name,
			"backend_url":  l.backendURL,
		},
		UserID:     game.UserID,
		StorageARN: game.StorageARN,
	}

	ec2Data, err := json.Marshal(ec2Payload)
	if err != nil {
		l.logger.Errorw("S3_STORED_EC2_PAYLOAD_MARSHAL_FAILED", "game_id", payload.GameID, "error", err)
		return
	}

	l.logger.Infow("S3_STORED_EC2_PROVISION_TRIGGERED", "game_id", payload.GameID, "game_name", game.Name, "subject", ec2Subj)
	l.natsClient.Publish(ec2Subj, ec2Data)
}

func (l *S3Listener) handleInstanceLifecycle(msg *nats.Msg) {
	l.logger.Debugw("LIFECYCLE_MESSAGE_RECEIVED", "bytes", len(msg.Data))

	var event struct {
		CorrelationID string `json:"correlation_id"`
		InstanceID    string `json:"instance_id"`
		EventType     string `json:"event_type"`
		Timestamp     string `json:"timestamp"`
		SessionID     string `json:"session_id"`
		Payload       struct {
			IPAddress   string `json:"ip_address"`
			VPCID       string `json:"vpc_id"`
			ServicePort int    `json:"service_port"`
			AgentWS     string `json:"agent_ws"`
		} `json:"payload"`
	}

	if err := json.Unmarshal(msg.Data, &event); err != nil {
		l.logger.Errorw("LIFECYCLE_UNMARSHAL_FAILED", "bytes", len(msg.Data), "error", err)
		return
	}

	l.logger.Infow("LIFECYCLE_EVENT_RECEIVED",
		"event_type", event.EventType,
		"instance_id", event.InstanceID,
		"session_id", event.SessionID,
		"correlation_id", event.CorrelationID,
		"ip_address", event.Payload.IPAddress,
		"agent_ws", event.Payload.AgentWS,
		"timestamp", event.Timestamp,
	)

	switch event.EventType {
	case EventInstanceProvisioned:
		if event.Payload.AgentWS == "" {
			l.logger.Warnw("LIFECYCLE_PROVISIONED_MISSING_AGENT_WS",
				"instance_id", event.InstanceID,
				"session_id", event.SessionID,
			)
			return
		}
		l.logger.Infow("LIFECYCLE_PROVISIONED_NOTIFYING_SSE",
			"instance_id", event.InstanceID,
			"session_id", event.SessionID,
			"agent_ws", event.Payload.AgentWS,
		)
		l.sseRegistry.Notify(event.SessionID, domain.GameSessionEvent{
			AgentURL: event.Payload.AgentWS,
			VMID:     event.InstanceID,
			VMIP: event.Payload.IPAddress,
		})
		l.logger.Infow("LIFECYCLE_PROVISIONED_SSE_NOTIFIED",
			"instance_id", event.InstanceID,
			"session_id", event.SessionID,
		)

	case EventInstanceError:
		l.logger.Errorw("LIFECYCLE_INSTANCE_ERROR_NOTIFYING_SSE",
			"instance_id", event.InstanceID,
			"session_id", event.SessionID,
			"correlation_id", event.CorrelationID,
		)
		l.sseRegistry.Notify(event.SessionID, domain.GameSessionEvent{
			Error: "Instance error",
		})
		l.logger.Infow("LIFECYCLE_INSTANCE_ERROR_SSE_NOTIFIED",
			"instance_id", event.InstanceID,
			"session_id", event.SessionID,
		)
	}
}

func (l *S3Listener) downloadFile(url, dest string) error {
	l.logger.Infow("S3_LISTENER_HTTP_DOWNLOAD", "url", url, "dest", dest)

	resp, err := http.Get(url)
	if err != nil {
		l.logger.Errorw("S3_LISTENER_HTTP_DOWNLOAD_FAILED", "url", url, "error", err)
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(dest)
	if err != nil {
		l.logger.Errorw("S3_LISTENER_HTTP_DOWNLOAD_FILE_CREATE_FAILED", "dest", dest, "error", err)
		return err
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		l.logger.Errorw("S3_LISTENER_HTTP_DOWNLOAD_COPY_FAILED", "dest", dest, "error", err)
		return err
	}

	l.logger.Infow("S3_LISTENER_HTTP_DOWNLOAD_SUCCESS", "dest", dest, "bytes_written", written)
	return nil
}