package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func NewNodeAgent(nodeID string, gameRepo repository.GameRepository, natsClient repository.MessagingClient, storage *storage.MinIOAdapter, debug bool, godotPath string, appEnv string, logger *zap.SugaredLogger) *NodeAgent {
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
	fmt.Println("Node Agent started")

	// ================================
	// 1. LISTEN FOR PROVISION REQUESTS
	// ================================
	provisionSubj := messaging.GetProvisionGameSubject()

	_, err := a.natsClient.Subscribe(provisionSubj, func(msg *nats.Msg) {
		a.handleProvision(msg)
	})
	if err != nil {
		a.logger.Fatalf("Failed to subscribe to provisioning topic", "error", err)
	}

	// ======================================
	// 2. LISTEN FOR FINISHED S3 UPLOAD EVENTS
	// ======================================
	uploadSubj := messaging.GetFinishedS3UploadSubject()

	_, err = a.natsClient.Subscribe(uploadSubj, func(msg *nats.Msg) {
		a.handleFinishedUpload(msg)
	})
	if err != nil {
		a.logger.Fatalf("Failed to subscribe to finished upload topic", "error", err)
	}
}

func (a *NodeAgent) handleProvision(msg *nats.Msg) {
	var payload domain.ProvisionGameRequest

	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		a.logger.Errorw("Failed to unmarshal provisioning request", "error", err)
		return
	}

	// Only handle requests for this node
	if payload.TargetNode != a.NodeID {
		return
	}

	a.logger.Infow("Provisioning request received",
		"node_id", a.NodeID,
		"game_id", payload.GameID,
	)

	if a.debug {
		go a.initializeGameDebug(payload.GameID, payload.StorageARN, payload.StreamingMode)
	} else {
		go a.initializeGame(payload.GameID, payload.StorageARN, payload.StreamingMode)
	}
}

func (a *NodeAgent) handleFinishedUpload(msg *nats.Msg) {
	var payload struct {
		GameID     int    `json:"game_id"`
		StorageARN string `json:"storage_arn"`
	}

	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		a.logger.Errorw("Failed to unmarshal finished upload event", "error", err)
		return
	}

	a.logger.Infow("S3 upload completed, triggering provisioning",
		"game_id", payload.GameID,
	)

	// Build provisioning request
	provision := domain.ProvisionGameRequest{
		GameID:        payload.GameID,
		StorageARN:    payload.StorageARN,
		TargetNode:    a.NodeID,
		StreamingMode: "default",
	}

	// Reuse same flow
	data, _ := json.Marshal(provision)

	a.handleProvision(&nats.Msg{
		Data: data,
	})
}
func (a *NodeAgent) initializeGameDebug(gameID int, storageARN string, mode domain.StreamingMode) {
	a.logger.Infow("Starting debug local execution", "node_id", a.NodeID, "game_id", gameID)

	// 1. Prepare Paths
	tempDir := filepath.Join("/tmp", fmt.Sprintf("game_%d", gameID))
	tempZip := filepath.Join("/tmp", fmt.Sprintf("game_%d.zip", gameID))
	os.RemoveAll(tempDir)
	os.MkdirAll(tempDir, os.ModePerm)
	defer os.Remove(tempZip)

	// 2. Parse ARN: arn:aws:s3:::bucket/key
	arnParts := strings.Split(storageARN, ":::")
	if len(arnParts) < 2 {
		a.logger.Errorw("Invalid StorageARN", "arn", storageARN)
		return
	}
	pathParts := strings.SplitN(arnParts[1], "/", 2)
	bucket, key := pathParts[0], pathParts[1]

	// 3. Download from MinIO
	a.logger.Debugw("Downloading game files", "node_id", a.NodeID, "bucket", bucket, "key", key)
	if err := a.storage.DownloadFile(context.Background(), bucket, key, tempZip); err != nil {
		a.logger.Errorw("Download failed", "node_id", a.NodeID, "error", err)
		return
	}

	// 4. Unzip
	unzipper := NewValidationService()
	if err := unzipper.Unzip(tempZip, tempDir); err != nil {
		a.logger.Errorw("Unzip failed", "node_id", a.NodeID, "error", err)
		return
	}

	// 5. Get Manifest to find HeadlessBin
	game, _ := a.gameRepo.GetGame(context.Background(), uint(gameID))
	var manifest domain.GameManifest
	json.Unmarshal([]byte(game.Manifest), &manifest)

	binPath := filepath.Join(tempDir, manifest.HeadlessBin)
	a.logger.Infow("Launching binary", "node_id", a.NodeID, "bin", binPath)

	// Make binary executable
	os.Chmod(binPath, 0755)

	// 6. Execute!
	args := []string{"--headless"}
	if mode == domain.StreamingModeVideo {
		args = append(args, "--mode=webrtc")
	} else {
		args = append(args, "--mode=state_sync")
	}

	// Launch in a new terminal for visibility in debug mode
	terminalCmd := fmt.Sprintf("cd %s && ./%s %s; read -p 'Press enter to close...'", 
		tempDir, filepath.Base(binPath), strings.Join(args, " "))
	cmd := exec.Command("gnome-terminal", "--", "bash", "-c", terminalCmd)
	
	if err := cmd.Start(); err != nil {
		a.logger.Warnw("Failed to start gnome-terminal", "node_id", a.NodeID, "error", err)
		cmd = exec.Command(binPath, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			a.logger.Errorw("Failed to start Godot", "node_id", a.NodeID, "error", err)
			return
		}
	}

	a.logger.Infow("Process started", "node_id", a.NodeID, "pid", cmd.Process.Pid)

	// 7. Update status to Active
	err := a.gameRepo.UpdateGameStatus(context.Background(), uint(gameID), domain.GameStatusActive)
	if err != nil {
		a.logger.Errorw("Failed to finalize status", "node_id", a.NodeID, "error", err)
		return
	}

	// 8. Notify "Game Ready"
	a.notifyReady(gameID, 8091)

	// Keep process running in background
	go func() {
		cmd.Wait()
		a.logger.Infow("Game process exited", "node_id", a.NodeID, "game_id", gameID)
		a.gameRepo.UpdateGameStatus(context.Background(), uint(gameID), domain.GameStatusStored)
	}()
}

func (a *NodeAgent) notifyReady(gameID int, port int) {
	readySubj := messaging.GetGameReadySubject()

	readyPayload := domain.GameReadyEvent{
		GameID: gameID,
		NodeID: a.NodeID,
		Port:   port,
	}

	data, _ := json.Marshal(readyPayload)
	a.natsClient.Publish(readySubj, data)
	a.logger.Infow("Game is LIVE and READY", "node_id", a.NodeID, "game_id", gameID)
}

func (a *NodeAgent) initializeGame(gameID int, storageARN string, mode domain.StreamingMode) {
	a.logger.Infow("Starting initialization", "node_id", a.NodeID, "game_id", gameID)
	
	// Simulation of cold start delay
	time.Sleep(3 * time.Second) 

	a.logger.Infow("Process started for game", "node_id", a.NodeID, "game_id", gameID, "port", 8091)

	// 4. Update status to Active
	err := a.gameRepo.UpdateGameStatus(context.Background(), uint(gameID), domain.GameStatusActive)
	if err != nil {
		a.logger.Errorw("Failed to finalize status", "node_id", a.NodeID, "game_id", gameID, "error", err)
		return
	}

	// 5. Notify "Game Ready"
	a.notifyReady(gameID, 8091)
}
