package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"backend/internal/domain"
	"backend/internal/infrastructure/storage"
	"backend/internal/interfaces"
	"backend/internal/messaging"

	"github.com/nats-io/nats.go"
)

type NodeAgent struct {
	NodeID     string
	gameRepo   interfaces.GameRepository
	natsClient *messaging.NatsClient
	storage    *storage.MinIOAdapter
	debug      bool
	godotPath  string
}

func NewNodeAgent(nodeID string, gameRepo interfaces.GameRepository, natsClient *messaging.NatsClient, storage *storage.MinIOAdapter, debug bool, godotPath string) *NodeAgent {
	return &NodeAgent{
		NodeID:     nodeID,
		gameRepo:   gameRepo,
		natsClient: natsClient,
		storage:    storage,
		debug:      debug,
		godotPath:  godotPath,
	}
}

func (a *NodeAgent) Start() {
	subj := messaging.Subject{
		Env:        "dev",
		Service:    "provisioning",
		Version:    "v1",
		Domain:     "game",
		ActionType: "provision",
	}

	_, err := a.natsClient.Subscribe(subj, func(msg *nats.Msg) {
		var payload struct {
			GameID        int                  `json:"game_id"`
			StorageARN    string               `json:"storage_arn"`
			TargetNode    string               `json:"target_node"`
			StreamingMode domain.StreamingMode `json:"streaming_mode"`
		}

		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Printf("[NodeAgent] Failed to unmarshal provisioning request: %v", err)
			return
		}

		// Only handle requests for this specific node
		if payload.TargetNode != a.NodeID {
			return
		}

		log.Printf("[NodeAgent %s] Provisioning request received for game %d", a.NodeID, payload.GameID)

		// Simulating the Initialization Layer:
		// 1. Download artifact from S3 (StorageARN)
		// 2. Unzip & Setup Env
	// 3. Launch Process (Godot Headless)
	if a.debug {
		go a.initializeGameDebug(payload.GameID, payload.StorageARN, payload.StreamingMode)
	} else {
		go a.initializeGame(payload.GameID, payload.StorageARN, payload.StreamingMode)
	}
})

	if err != nil {
		log.Fatalf("[NodeAgent] Failed to subscribe to provisioning topic: %v", err)
	}
}

func (a *NodeAgent) initializeGameDebug(gameID int, storageARN string, mode domain.StreamingMode) {
	log.Printf("[NodeAgent %s][DEBUG] STARTING LOCAL EXECUTION for Game %d...", a.NodeID, gameID)

	// 1. Prepare Paths
	tempDir := filepath.Join("/tmp", fmt.Sprintf("game_%d", gameID))
	tempZip := filepath.Join("/tmp", fmt.Sprintf("game_%d.zip", gameID))
	os.RemoveAll(tempDir)
	os.MkdirAll(tempDir, os.ModePerm)
	defer os.Remove(tempZip)

	// 2. Parse ARN: arn:aws:s3:::bucket/key
	arnParts := strings.Split(storageARN, ":::")
	if len(arnParts) < 2 {
		log.Printf("[NodeAgent] Invalid StorageARN: %s", storageARN)
		return
	}
	pathParts := strings.SplitN(arnParts[1], "/", 2)
	bucket, key := pathParts[0], pathParts[1]

	// 3. Download from MinIO
	log.Printf("[NodeAgent %s][DEBUG] Downloading game files from %s/%s...", a.NodeID, bucket, key)
	if err := a.storage.DownloadFile(context.Background(), bucket, key, tempZip); err != nil {
		log.Printf("[NodeAgent %s][DEBUG] Download failed: %v", a.NodeID, err)
		return
	}

	// 4. Unzip
	unzipper := NewValidationService()
	if err := unzipper.Unzip(tempZip, tempDir); err != nil {
		log.Printf("[NodeAgent %s][DEBUG] Unzip failed: %v", a.NodeID, err)
		return
	}

	// 5. Get Manifest to find HeadlessBin
	game, _ := a.gameRepo.GetGame(gameID)
	var manifest domain.GameManifest
	json.Unmarshal([]byte(game.Manifest), &manifest)

	binPath := filepath.Join(tempDir, manifest.HeadlessBin)
	log.Printf("[NodeAgent %s][DEBUG] Launching binary: %s", a.NodeID, binPath)

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
		log.Printf("[NodeAgent %s][DEBUG] Failed to start gnome-terminal: %v (falling back to direct exec)", a.NodeID, err)
		cmd = exec.Command(binPath, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			log.Printf("[NodeAgent %s][DEBUG] Failed to start Godot: %v", a.NodeID, err)
			return
		}
	}

	log.Printf("[NodeAgent %s][DEBUG] Process started with PID %d", a.NodeID, cmd.Process.Pid)

	// 7. Update status to Active
	err := a.gameRepo.UpdateGameStatus(gameID, domain.GameStatusActive, storageARN)
	if err != nil {
		log.Printf("[NodeAgent %s] Failed to finalize status: %v", a.NodeID, err)
		return
	}

	// 8. Notify "Game Ready"
	a.notifyReady(gameID, 8080)

	// Keep process running in background
	go func() {
		cmd.Wait()
		log.Printf("[NodeAgent %s][DEBUG] Game %d process exited", a.NodeID, gameID)
		a.gameRepo.UpdateGameStatus(gameID, domain.GameStatusStored, storageARN)
	}()
}

func (a *NodeAgent) notifyReady(gameID int, port int) {
	readySubj := messaging.Subject{
		Env:        "dev",
		Service:    "provisioning",
		Version:    "v1",
		Domain:     "game",
		ActionType: "ready",
	}

	readyPayload := struct {
		GameID int    `json:"game_id"`
		NodeID string `json:"node_id"`
		Port   int    `json:"port"`
	}{
		GameID: gameID,
		NodeID: a.NodeID,
		Port:   port,
	}

	data, _ := json.Marshal(readyPayload)
	a.natsClient.Publish(readySubj, data)
	log.Printf("[NodeAgent %s] Game %d is now LIVE and READY!", a.NodeID, gameID)
}

func (a *NodeAgent) initializeGame(gameID int, storageARN string, mode domain.StreamingMode) {
	log.Printf("[NodeAgent %s] STARTING INITIALIZATION for Game %d...", a.NodeID, gameID)
	
	// Simulation of cold start delay
	time.Sleep(3 * time.Second) 

	log.Printf("[NodeAgent %s] Process started for Game %d. Listening on port 8080", a.NodeID, gameID)

	// 4. Update status to Active
	err := a.gameRepo.UpdateGameStatus(gameID, domain.GameStatusActive, storageARN)
	if err != nil {
		log.Printf("[NodeAgent %s] Failed to finalize status for game %d: %v", a.NodeID, gameID, err)
		return
	}

	// 5. Notify "Game Ready"
	a.notifyReady(gameID, 8080)
}
