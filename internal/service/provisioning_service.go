package service

import (
	"encoding/json"
	"fmt"
	"log"

	"backend/internal/domain"
	"backend/internal/interfaces"
	"backend/internal/messaging"
)

type ProvisioningService struct {
	gameRepo   interfaces.GameRepository
	natsClient *messaging.NatsClient
}

func NewProvisioningService(gameRepo interfaces.GameRepository, natsClient *messaging.NatsClient) *ProvisioningService {
	return &ProvisioningService{
		gameRepo:   gameRepo,
		natsClient: natsClient,
	}
}

// ProvisionGame triggers the on-demand startup of a stored game.
func (s *ProvisioningService) ProvisionGame(gameID int) error {
	game, err := s.gameRepo.GetGame(gameID)
	if err != nil {
		return err
	}

	if game.Status != domain.GameStatusStored {
		return fmt.Errorf("game %d is not in stored state (current: %s)", gameID, game.Status)
	}

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
		GameID     int    `json:"game_id"`
		StorageARN string `json:"storage_arn"`
		TargetNode string `json:"target_node"`
	}{
		GameID:     game.ID,
		StorageARN: game.StorageARN,
		TargetNode: targetNode,
	}

	data, _ := json.Marshal(payload)
	log.Printf("[ProvisioningService] Requesting startup for game %d on node %s", gameID, targetNode)
	
	return s.natsClient.Publish(subj, data)
}
