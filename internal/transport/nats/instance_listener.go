package nats

import (
	"context"
	"encoding/json"

	"backend/internal/domain"
	"backend/internal/infrastructure/messaging"
	"backend/internal/infrastructure/repository"
	"backend/internal/transport/websocket"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type InstanceLifecycleListener struct {
	gameRepo   repository.GameRepository
	natsClient repository.MessagingClient
	hub        *websocket.Hub
	appEnv     string
	logger     *zap.SugaredLogger
}

func NewInstanceLifecycleListener(
	gameRepo repository.GameRepository,
	natsClient repository.MessagingClient,
	hub *websocket.Hub,
	appEnv string,
	logger *zap.SugaredLogger,
) *InstanceLifecycleListener {
	return &InstanceLifecycleListener{
		gameRepo:   gameRepo,
		natsClient: natsClient,
		hub:        hub,
		appEnv:     appEnv,
		logger:     logger,
	}
}

// (Event moved to domain.InstanceLifecycleEvent)

func (l *InstanceLifecycleListener) Start() {
	subj := messaging.GetInstanceLifecycleSubject()

	_, err := l.natsClient.Subscribe(subj, func(msg *nats.Msg) {
		var event domain.InstanceLifecycleEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			l.logger.Errorw("Failed to unmarshal instance event", "error", err)
			return
		}

		l.logger.Infow("Received Instance Lifecycle event", 
			"event_type", event.EventType, 
			"instance_id", event.InstanceID, 
			"stage", event.Stage, 
			"message", event.Message,
		)

		// 1. Find the Game by VMID (instance_id)
		var gameID int
		game, err := l.gameRepo.GetGameByVMID(context.Background(), event.InstanceID)
		if err == nil {
			gameID = game.ID
		}
		
		// 2. Broadcast to WebSocket Hub
		wsMsg := map[string]interface{}{
			"type":        "provisioning_progress",
			"gameId":      gameID,
			"instanceId":  event.InstanceID,
			"stage":       event.Stage,
			"message":     event.Message,
			"timestamp":   event.Timestamp,
			"eventType":   event.EventType,
		}

		data, _ := json.Marshal(wsMsg)
		l.hub.Broadcast(data)

		// 3. Update Database Status if completed or failed
		if event.Stage == "COMPLETED" || event.EventType == "INSTANCE_STARTED" {
			l.logger.Infow("Lifecycle completed", "instance_id", event.InstanceID, "status", "Active")
			l.gameRepo.UpdateStatusByVMID(context.Background(), event.InstanceID, domain.GameStatusActive)
		} else if event.Stage == "FAILED" {
			l.logger.Errorw("Lifecycle failed", "instance_id", event.InstanceID)
			l.gameRepo.UpdateStatusByVMID(context.Background(), event.InstanceID, "failed")
		}
	})

	if err != nil {
		l.logger.Fatalf("Failed to subscribe to Instance Lifecycle topic", "error", err)
	}
}
