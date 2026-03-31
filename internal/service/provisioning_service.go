package service

import (
	"encoding/json"
	"fmt"
	"log"

	"backend/internal/domain"
	"backend/internal/interfaces"
	"backend/internal/infrastructure/storage"
	"backend/internal/messaging"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ProvisioningService struct {
	gameRepo   interfaces.GameRepository
	natsClient *messaging.NatsClient
	storage    *storage.MinIOAdapter
	debug      bool
	godotPath  string
}

func NewProvisioningService(
	gameRepo interfaces.GameRepository,
	natsClient *messaging.NatsClient,
	storage *storage.MinIOAdapter,
	debug bool,
	godotPath string,
) *ProvisioningService {
	return &ProvisioningService{
		gameRepo:   gameRepo,
		natsClient: natsClient,
		storage:    storage,
		debug:      debug,
		godotPath:  godotPath,
	}
}

// ProvisionGame triggers the on-demand startup of a stored game.
func (s *ProvisioningService) ProvisionGame(gameID int, mode domain.StreamingMode) error {
	game, err := s.gameRepo.GetGame(gameID)
	if err != nil {
		return err
	}

	// Allow re-provisioning if in debug mode or if specifically requested
	// if game.Status != domain.GameStatusStored && (!s.debug || game.Status != domain.GameStatusActive) {
	// 	return fmt.Errorf("game %d is not in stored state (current: %s)", gameID, game.Status)
	// }

	// 1. Update status to Provisioning to prevent duplicate requests
	err = s.gameRepo.UpdateGameStatus(gameID, domain.GameStatusProvisioning, game.StorageARN)
	if err != nil {
		return err
	}

	// 2. Select a node (Mock: using the VMID from the game record for now)
	targetNode := game.VMID
	if targetNode == "" {
		targetNode = "default-worker-node"
	}

	// 3. Publish Provisioning Event
	subj := messaging.Subject{
		Env:        "dev",
		Service:    "provisioning",
		Version:    "v1",
		Domain:     "game",
		ActionType: "provision",
	}

	payload := struct {
		GameID        int                  `json:"game_id"`
		StorageARN    string               `json:"storage_arn"`
		TargetNode    string               `json:"target_node"`
		StreamingMode domain.StreamingMode `json:"streaming_mode"`
	}{
		GameID:        game.ID,
		StorageARN:    game.StorageARN,
		TargetNode:    targetNode,
		StreamingMode: mode,
	}

	data, _ := json.Marshal(payload)
	log.Printf("[ProvisioningService] Requesting startup for game %d on node %s", gameID, targetNode)

	err = s.natsClient.Publish(subj, data)
	if err != nil {
		return err
	}
	// if s.debug {
		go s.launchLocalDebug(game, mode)
	// }

	return nil
}

func (s *ProvisioningService) launchLocalDebug(game *domain.Game, mode domain.StreamingMode) {
	log.Printf("[ProvisioningService][DEBUG] STARTING LOCAL EXECUTION for Game %d...", game.ID)

	// Prepare Paths
	tempDir := filepath.Join("/tmp", fmt.Sprintf("game_%d", game.ID))
	tempZip := filepath.Join("/tmp", fmt.Sprintf("game_%d.zip", game.ID))
	os.RemoveAll(tempDir)
	os.MkdirAll(tempDir, os.ModePerm)

	// S3 Download
	arnParts := strings.Split(game.StorageARN, ":::")
	if len(arnParts) < 2 {
		return
	}
	pathParts := strings.SplitN(arnParts[1], "/", 2)
	bucket, key := pathParts[0], pathParts[1]

	if err := s.storage.DownloadFile(context.Background(), bucket, key, tempZip); err != nil {
		log.Printf("[ProvisioningService][DEBUG] Download failed: %v", err)
		return
	}

	// Unzip
	exec.Command("unzip", "-o", tempZip, "-d", tempDir).Run()

	// Parse Manifest
	var manifest domain.GameManifest
	json.Unmarshal([]byte(game.Manifest), &manifest)
	binPath := filepath.Join(tempDir, manifest.HeadlessBin)
	os.Chmod(binPath, 0755)

	// Execute in Terminal
	args := []string{"--headless"}
	if mode == domain.StreamingModeVideo {
		args = append(args, "--mode=webrtc")
	} else {
		args = append(args, "--mode=state_sync")
	}

	// Correctly resolve binary path relative to tempDir
	terminalCmd := fmt.Sprintf("cd %s && ./%s %s; read -p 'Press enter to close...'",
		tempDir, manifest.HeadlessBin, strings.Join(args, " "))
	cmd := exec.Command("gnome-terminal", "--", "bash", "-c", terminalCmd)

	if err := cmd.Start(); err != nil {
		log.Printf("[ProvisioningService][DEBUG] Failed to start gnome-terminal, falling back to background exec: %v", err)
		exec.Command(binPath, args...).Start()
	}

	// Finalize Status
	s.gameRepo.UpdateGameStatus(game.ID, domain.GameStatusActive, game.StorageARN)
}
