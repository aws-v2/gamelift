package nats

import (
	"backend/internal/infrastructure/messaging"
	"backend/internal/infrastructure/repository"
	"backend/internal/transport/websocket"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type GameStateListener struct {
	natsClient repository.MessagingClient
	hub        *websocket.Hub
	appEnv     string
	logger     *zap.SugaredLogger
}

func NewGameStateListener(natsClient repository.MessagingClient, hub *websocket.Hub, appEnv string, logger *zap.SugaredLogger) *GameStateListener {
	return &GameStateListener{
		natsClient: natsClient,
		hub:        hub,
		appEnv:     appEnv,
		logger:     logger,
	}
}

func (l *GameStateListener) Start() {
	subj := messaging.GetGameStateBroadcastSubject()

	l.logger.Infow("GAME_STATE_LISTENER_STARTING", "subject", subj, "env", l.appEnv)

	_, err := l.natsClient.Subscribe(subj, func(msg *nats.Msg) {
		l.logger.Debugw("GAME_STATE_MESSAGE_RECEIVED",
			"subject", msg.Subject,
			"bytes", len(msg.Data),
		)

		l.hub.Broadcast(msg.Data)

		l.logger.Debugw("GAME_STATE_MESSAGE_BROADCASTED",
			"subject", msg.Subject,
			"bytes", len(msg.Data),
		)
	})

	if err != nil {
		l.logger.Errorw("GAME_STATE_LISTENER_SUBSCRIBE_FAILED", "subject", subj, "env", l.appEnv, "error", err)
		return
	}

	l.logger.Infow("GAME_STATE_LISTENER_SUBSCRIBED", "subject", subj, "env", l.appEnv)
}

func (l *GameStateListener) runMockGenerator() {
	l.logger.Infow("GAME_STATE_MOCK_GENERATOR_DISABLED", "note", "mock generator is available but currently disabled")
}