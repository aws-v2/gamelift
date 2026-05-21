package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"backend/internal/domain"
	"backend/internal/infrastructure/messaging"
	"backend/internal/infrastructure/repository"
	"backend/internal/infrastructure/storage"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type NodeAgent struct {
	NodeID     string
	gameRepo   repository.GameRepository
	natsClient repository.MessagingClient
	storage    *storage.MinIOAdapter
	debug      bool
	godotPath  string
	appEnv     string
	logger     *zap.SugaredLogger
}

func NewNodeAgent(
	nodeID string,
	gameRepo repository.GameRepository,
	natsClient repository.MessagingClient,
	storage *storage.MinIOAdapter,
	debug bool,
	godotPath string,
	appEnv string,
	logger *zap.SugaredLogger,
) *NodeAgent {
	return &NodeAgent{
		NodeID:     nodeID,
		gameRepo:   gameRepo,
		natsClient: natsClient,
		storage:    storage,
		debug:      debug,
		godotPath:  godotPath,
		appEnv:     appEnv,
		logger:     logger,
	}
}

func (a *NodeAgent) Start() {
	a.logger.Infow("NODE_AGENT_STARTING", "node_id", a.NodeID, "debug", a.debug, "env", a.appEnv)

	// 1. Provision requests
	provisionSubj := messaging.GetProvisionGameSubject()
	a.logger.Infow("NODE_AGENT_SUBSCRIBING", "node_id", a.NodeID, "subject", provisionSubj)

	_, err := a.natsClient.Subscribe(provisionSubj, func(msg *nats.Msg) {
		a.handleProvision(msg)
	})
	if err != nil {
		a.logger.Errorw("NODE_AGENT_SUBSCRIBE_FAILED", "node_id", a.NodeID, "subject", provisionSubj, "error", err)
		return
	}
	a.logger.Infow("NODE_AGENT_SUBSCRIBED", "node_id", a.NodeID, "subject", provisionSubj)

	// 2. Finished S3 upload events
	uploadSubj := messaging.GetFinishedS3UploadSubject()
	a.logger.Infow("NODE_AGENT_SUBSCRIBING", "node_id", a.NodeID, "subject", uploadSubj)

	_, err = a.natsClient.Subscribe(uploadSubj, func(msg *nats.Msg) {
		a.handleFinishedUpload(msg)
	})
	if err != nil {
		a.logger.Errorw("NODE_AGENT_SUBSCRIBE_FAILED", "node_id", a.NodeID, "subject", uploadSubj, "error", err)
		return
	}
	a.logger.Infow("NODE_AGENT_SUBSCRIBED", "node_id", a.NodeID, "subject", uploadSubj)

	a.logger.Infow("NODE_AGENT_STARTED", "node_id", a.NodeID)
}

func (a *NodeAgent) handleProvision(msg *nats.Msg) {
	a.logger.Debugw("NODE_AGENT_PROVISION_MESSAGE_RECEIVED", "node_id", a.NodeID, "bytes", len(msg.Data))

	var payload domain.ProvisionGameRequest
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		a.logger.Errorw("NODE_AGENT_PROVISION_UNMARSHAL_FAILED", "node_id", a.NodeID, "bytes", len(msg.Data), "error", err)
		return
	}

	if payload.TargetNode != a.NodeID {
		a.logger.Debugw("NODE_AGENT_PROVISION_SKIPPED",
			"node_id", a.NodeID,
			"target_node", payload.TargetNode,
			"game_id", payload.GameID,
		)
		return
	}

	a.logger.Infow("NODE_AGENT_PROVISION_ACCEPTED",
		"node_id", a.NodeID,
		"game_id", payload.GameID,
		"storage_arn", payload.StorageARN,
		"streaming_mode", payload.StreamingMode,
		"debug", a.debug,
	)

	if a.debug {
		go a.initializeGameDebug(payload.GameID, payload.StorageARN, payload.StreamingMode)
	} else {
		go a.initializeGame(payload.GameID, payload.StorageARN, payload.StreamingMode)
	}
}

func (a *NodeAgent) handleFinishedUpload(msg *nats.Msg) {
	a.logger.Debugw("NODE_AGENT_UPLOAD_MESSAGE_RECEIVED", "node_id", a.NodeID, "bytes", len(msg.Data))

	var payload struct {
		GameID     int    `json:"game_id"`
		StorageARN string `json:"storage_arn"`
	}

	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		a.logger.Errorw("NODE_AGENT_UPLOAD_UNMARSHAL_FAILED", "node_id", a.NodeID, "error", err)
		return
	}

	a.logger.Infow("NODE_AGENT_UPLOAD_COMPLETED",
		"node_id", a.NodeID,
		"game_id", payload.GameID,
		"storage_arn", payload.StorageARN,
	)

	provision := domain.ProvisionGameRequest{
		GameID:        payload.GameID,
		StorageARN:    payload.StorageARN,
		TargetNode:    a.NodeID,
		StreamingMode: "default",
	}

	data, err := json.Marshal(provision)
	if err != nil {
		a.logger.Errorw("NODE_AGENT_UPLOAD_MARSHAL_FAILED", "node_id", a.NodeID, "game_id", payload.GameID, "error", err)
		return
	}

	a.logger.Infow("NODE_AGENT_UPLOAD_FORWARDING_TO_PROVISION", "node_id", a.NodeID, "game_id", payload.GameID)
	a.handleProvision(&nats.Msg{Data: data})
}

func (a *NodeAgent) initializeGameDebug(gameID int, storageARN string, mode domain.StreamingMode) {
	a.logger.Infow("NODE_AGENT_DEBUG_INIT_STARTING", "node_id", a.NodeID, "game_id", gameID, "mode", mode)

	tempDir := filepath.Join("/tmp", fmt.Sprintf("game_%d", gameID))
	tempZip := filepath.Join("/tmp", fmt.Sprintf("game_%d.zip", gameID))

	os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		a.logger.Errorw("NODE_AGENT_DEBUG_MKDIR_FAILED", "node_id", a.NodeID, "game_id", gameID, "path", tempDir, "error", err)
		return
	}
	defer os.Remove(tempZip)

	// Parse ARN → bucket + key
	arnParts := strings.Split(storageARN, ":::")
	if len(arnParts) < 2 {
		a.logger.Errorw("NODE_AGENT_DEBUG_INVALID_ARN", "node_id", a.NodeID, "game_id", gameID, "arn", storageARN)
		return
	}
	pathParts := strings.SplitN(arnParts[1], "/", 2)
	if len(pathParts) < 2 {
		a.logger.Errorw("NODE_AGENT_DEBUG_ARN_PARSE_FAILED", "node_id", a.NodeID, "game_id", gameID, "arn", storageARN)
		return
	}
	bucket, key := pathParts[0], pathParts[1]

	a.logger.Infow("NODE_AGENT_DEBUG_DOWNLOADING",
		"node_id", a.NodeID,
		"game_id", gameID,
		"bucket", bucket,
		"key", key,
		"dest", tempZip,
	)

	if err := a.storage.DownloadFile(context.Background(), bucket, key, tempZip); err != nil {
		a.logger.Errorw("NODE_AGENT_DEBUG_DOWNLOAD_FAILED", "node_id", a.NodeID, "game_id", gameID, "bucket", bucket, "key", key, "error", err)
		return
	}

	a.logger.Infow("NODE_AGENT_DEBUG_DOWNLOAD_SUCCESS", "node_id", a.NodeID, "game_id", gameID, "dest", tempZip)

	unzipper := NewValidationService(a.logger)
	if err := unzipper.Unzip(tempZip, tempDir); err != nil {
		a.logger.Errorw("NODE_AGENT_DEBUG_UNZIP_FAILED", "node_id", a.NodeID, "game_id", gameID, "error", err)
		return
	}

	a.logger.Infow("NODE_AGENT_DEBUG_UNZIP_SUCCESS", "node_id", a.NodeID, "game_id", gameID, "dest", tempDir)

	game, err := a.gameRepo.GetGame(context.Background(), strconv.Itoa(gameID), a.logger)
	if err != nil {
		a.logger.Errorw("NODE_AGENT_DEBUG_GAME_FETCH_FAILED", "node_id", a.NodeID, "game_id", gameID, "error", err)
		return
	}

	var manifest domain.GameManifest
	if err := json.Unmarshal([]byte(game.Manifest), &manifest); err != nil {
		a.logger.Errorw("NODE_AGENT_DEBUG_MANIFEST_PARSE_FAILED", "node_id", a.NodeID, "game_id", gameID, "error", err)
		return
	}

	binPath := filepath.Join(tempDir, manifest.HeadlessBin)

	a.logger.Infow("NODE_AGENT_DEBUG_BINARY_RESOLVED", "node_id", a.NodeID, "game_id", gameID, "bin", binPath)

	if err := os.Chmod(binPath, 0755); err != nil {
		a.logger.Warnw("NODE_AGENT_DEBUG_CHMOD_FAILED", "node_id", a.NodeID, "game_id", gameID, "bin", binPath, "error", err)
	}

	args := []string{"--headless"}
	if mode == domain.StreamingModeVideo {
		args = append(args, "--mode=webrtc")
	} else {
		args = append(args, "--mode=state_sync")
	}

	terminalCmd := fmt.Sprintf("cd %s && ./%s %s; read -p 'Press enter to close...'",
		tempDir, filepath.Base(binPath), strings.Join(args, " "))

	a.logger.Infow("NODE_AGENT_DEBUG_LAUNCHING",
		"node_id", a.NodeID,
		"game_id", gameID,
		"mode", mode,
		"args", args,
	)

	cmd := exec.Command("gnome-terminal", "--", "bash", "-c", terminalCmd)
	if err := cmd.Start(); err != nil {
		a.logger.Warnw("NODE_AGENT_DEBUG_GNOME_TERMINAL_FAILED",
			"node_id", a.NodeID,
			"game_id", gameID,
			"error", err,
			"fallback", "background exec",
		)
		cmd = exec.Command(binPath, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			a.logger.Errorw("NODE_AGENT_DEBUG_FALLBACK_EXEC_FAILED", "node_id", a.NodeID, "game_id", gameID, "bin", binPath, "error", err)
			return
		}
	}

	a.logger.Infow("NODE_AGENT_DEBUG_PROCESS_STARTED", "node_id", a.NodeID, "game_id", gameID, "pid", cmd.Process.Pid)

	if err := a.gameRepo.UpdateGameStatus(context.Background(), strconv.Itoa(gameID), domain.GameStatusActive, a.logger); err != nil {
		a.logger.Errorw("NODE_AGENT_DEBUG_STATUS_UPDATE_FAILED", "node_id", a.NodeID, "game_id", gameID, "error", err)
		return
	}

	a.logger.Infow("NODE_AGENT_DEBUG_STATUS_UPDATED", "node_id", a.NodeID, "game_id", gameID, "new_status", domain.GameStatusActive)

	a.notifyReady(gameID, 8091)

	go func() {
		if err := cmd.Wait(); err != nil {
			a.logger.Warnw("NODE_AGENT_DEBUG_PROCESS_EXITED_ERROR", "node_id", a.NodeID, "game_id", gameID, "error", err)
		} else {
			a.logger.Infow("NODE_AGENT_DEBUG_PROCESS_EXITED", "node_id", a.NodeID, "game_id", gameID)
		}

		if err := a.gameRepo.UpdateGameStatus(context.Background(), strconv.Itoa(gameID), domain.GameStatusStored, a.logger); err != nil {
			a.logger.Errorw("NODE_AGENT_DEBUG_POST_EXIT_STATUS_UPDATE_FAILED", "node_id", a.NodeID, "game_id", gameID, "error", err)
		} else {
			a.logger.Infow("NODE_AGENT_DEBUG_POST_EXIT_STATUS_UPDATED", "node_id", a.NodeID, "game_id", gameID, "new_status", domain.GameStatusStored)
		}
	}()
}

func (a *NodeAgent) initializeGame(gameID int, storageARN string, mode domain.StreamingMode) {
	a.logger.Infow("NODE_AGENT_INIT_STARTING", "node_id", a.NodeID, "game_id", gameID, "mode", mode, "storage_arn", storageARN)

	time.Sleep(3 * time.Second)

	a.logger.Infow("NODE_AGENT_INIT_COLD_START_DONE", "node_id", a.NodeID, "game_id", gameID)

	if err := a.gameRepo.UpdateGameStatus(context.Background(), strconv.Itoa(gameID), domain.GameStatusActive, a.logger); err != nil {
		a.logger.Errorw("NODE_AGENT_INIT_STATUS_UPDATE_FAILED", "node_id", a.NodeID, "game_id", gameID, "error", err)
		return
	}

	a.logger.Infow("NODE_AGENT_INIT_STATUS_UPDATED", "node_id", a.NodeID, "game_id", gameID, "new_status", domain.GameStatusActive)

	a.notifyReady(gameID, 8091)
}

func (a *NodeAgent) notifyReady(gameID int, port int) {
	a.logger.Infow("NODE_AGENT_NOTIFY_READY", "node_id", a.NodeID, "game_id", gameID, "port", port)

	readySubj := messaging.GetGameReadySubject()

	readyPayload := domain.GameReadyEvent{
		GameID: gameID,
		NodeID: a.NodeID,
		Port:   port,
	}

	data, err := json.Marshal(readyPayload)
	if err != nil {
		a.logger.Errorw("NODE_AGENT_NOTIFY_READY_MARSHAL_FAILED", "node_id", a.NodeID, "game_id", gameID, "error", err)
		return
	}

	if err := a.natsClient.Publish(readySubj, data); err != nil {
		a.logger.Errorw("NODE_AGENT_NOTIFY_READY_PUBLISH_FAILED", "node_id", a.NodeID, "game_id", gameID, "subject", readySubj, "error", err)
		return
	}

	a.logger.Infow("NODE_AGENT_NOTIFY_READY_SUCCESS", "node_id", a.NodeID, "game_id", gameID, "port", port, "subject", readySubj)
}