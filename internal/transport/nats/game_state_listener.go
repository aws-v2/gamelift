package nats

import (
	"backend/internal/interfaces"
	"backend/internal/messaging"
	"backend/internal/transport/websocket"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type GameStateListener struct {
	natsClient interfaces.MessagingClient
	hub        *websocket.Hub
	appEnv     string
	logger     *zap.SugaredLogger
}

func NewGameStateListener(natsClient interfaces.MessagingClient, hub *websocket.Hub, appEnv string, logger *zap.SugaredLogger) *GameStateListener {
	return &GameStateListener{
		natsClient: natsClient,
		hub:        hub,
		appEnv:     appEnv,
		logger:     logger,
	}
}

func (l *GameStateListener) Start() {
	// 1. Listen for any game state updates from Godot instances (NATS)
	subj := messaging.GetGameStateBroadcastSubject()

	_, err := l.natsClient.Subscribe(subj, func(msg *nats.Msg) {
		l.logger.Debugw("Relaying Godot State", "bytes", len(msg.Data))
		l.hub.Broadcast(msg.Data)
	})

	if err != nil {
		l.logger.Errorw("Failed to subscribe to NATS", "error", err)
	}

	// 2. Start Randomized Mock Data Generator (Hijacking the pipe)
	// go l.runMockGenerator()
}

func (l *GameStateListener) runMockGenerator() {
	l.logger.Infow("Randomized 3D Mock Data Generator is available but currently DISABLED")
	// (Hidden logic)
}
