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

func (l *InstanceLifecycleListener) Start() {
	subj := messaging.GetInstanceLifecycleSubject()

	l.logger.Infow("INSTANCE_LIFECYCLE_LISTENER_STARTING", "subject", subj, "env", l.appEnv)

	_, err := l.natsClient.Subscribe(subj, func(msg *nats.Msg) {
		var event domain.InstanceLifecycleEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			l.logger.Errorw("INSTANCE_LIFECYCLE_UNMARSHAL_FAILED",
				"subject", msg.Subject,
				"bytes", len(msg.Data),
				"error", err,
			)
			return
		}

		l.logger.Infow("INSTANCE_LIFECYCLE_EVENT_RECEIVED",
			"event_type", event.EventType,
			"instance_id", event.InstanceID,
			"stage", event.Stage,
			"message", event.Message,
			"timestamp", event.Timestamp,
		)

		// 1. Resolve game by VMID
		var gameID string
		game, err := l.gameRepo.GetGameByVMID(context.Background(), event.InstanceID, l.logger)
		if err != nil {
			l.logger.Warnw("INSTANCE_LIFECYCLE_GAME_NOT_FOUND",
				"instance_id", event.InstanceID,
				"error", err,
			)
		} else {
			gameID = game.ID
			l.logger.Infow("INSTANCE_LIFECYCLE_GAME_RESOLVED",
				"instance_id", event.InstanceID,
				"game_id", gameID,
				"game_name", game.Name,
			)
		}

		// 2. Broadcast to WebSocket hub
		wsMsg := map[string]interface{}{
			"type":       "provisioning_progress",
			"gameId":     gameID,
			"instanceId": event.InstanceID,
			"stage":      event.Stage,
			"message":    event.Message,
			"timestamp":  event.Timestamp,
			"eventType":  event.EventType,
		}

		data, err := json.Marshal(wsMsg)
		if err != nil {
			l.logger.Errorw("INSTANCE_LIFECYCLE_WS_MARSHAL_FAILED",
				"instance_id", event.InstanceID,
				"game_id", gameID,
				"error", err,
			)
			return
		}

		l.hub.Broadcast(data)
		l.logger.Infow("INSTANCE_LIFECYCLE_WS_BROADCASTED",
			"instance_id", event.InstanceID,
			"game_id", gameID,
			"stage", event.Stage,
		)

		// 3. Update DB status on terminal stages
		switch {
		case event.Stage == "COMPLETED" || event.EventType == "INSTANCE_STARTED":
			l.logger.Infow("INSTANCE_LIFECYCLE_MARKING_ACTIVE",
				"instance_id", event.InstanceID,
				"game_id", gameID,
				"stage", event.Stage,
				"event_type", event.EventType,
			)
			if err := l.gameRepo.UpdateStatusByVMID(context.Background(), event.InstanceID, domain.GameStatusActive,l.logger); err != nil {
				l.logger.Errorw("INSTANCE_LIFECYCLE_STATUS_UPDATE_FAILED",
					"instance_id", event.InstanceID,
					"game_id", gameID,
					"target_status", domain.GameStatusActive,
					"error", err,
				)
			} else {
				l.logger.Infow("INSTANCE_LIFECYCLE_STATUS_UPDATED",
					"instance_id", event.InstanceID,
					"game_id", gameID,
					"new_status", domain.GameStatusActive,
				)
			}

		case event.Stage == "FAILED":
			l.logger.Errorw("INSTANCE_LIFECYCLE_FAILED",
				"instance_id", event.InstanceID,
				"game_id", gameID,
				"message", event.Message,
			)
			if err := l.gameRepo.UpdateStatusByVMID(context.Background(), event.InstanceID, domain.GameStatusFailed,l.logger); err != nil {
				l.logger.Errorw("INSTANCE_LIFECYCLE_STATUS_UPDATE_FAILED",
					"instance_id", event.InstanceID,
					"game_id", gameID,
					"target_status", domain.GameStatusFailed,
					"error", err,
				)
			} else {
				l.logger.Infow("INSTANCE_LIFECYCLE_STATUS_UPDATED",
					"instance_id", event.InstanceID,
					"game_id", gameID,
					"new_status", domain.GameStatusFailed,
				)
			}
		}
	})

	if err != nil {
		l.logger.Errorw("INSTANCE_LIFECYCLE_LISTENER_SUBSCRIBE_FAILED", "subject", subj, "env", l.appEnv, "error", err)
		return
	}

	l.logger.Infow("INSTANCE_LIFECYCLE_LISTENER_SUBSCRIBED", "subject", subj, "env", l.appEnv)
}