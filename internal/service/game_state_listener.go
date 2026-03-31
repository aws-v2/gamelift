package service

import (
	"log"

	"backend/internal/messaging"
	"backend/internal/transport/handler"

	"github.com/nats-io/nats.go"
)

type GameStateListener struct {
	natsClient *messaging.NatsClient
	hub        *handler.Hub
}

func NewGameStateListener(natsClient *messaging.NatsClient, hub *handler.Hub) *GameStateListener {
	return &GameStateListener{
		natsClient: natsClient,
		hub:        hub,
	}
}

func (l *GameStateListener) Start() {
	// 1. Listen for any game state updates from Godot instances (NATS)
	subj := messaging.Subject{
		Env:        "dev",
		Service:    "game",
		Version:    "v1",
		Domain:     "state",
		ActionType: "broadcast",
	}

	_, err := l.natsClient.Subscribe(subj, func(msg *nats.Msg) {
		log.Printf("[GameStateListener] Relaying Godot State: %d bytes", len(msg.Data))
		l.hub.Broadcast(msg.Data)
	})

	if err != nil {
		log.Printf("[GameStateListener] Failed to subscribe to NATS: %v", err)
	}

	// 2. Start Randomized Mock Data Generator (Hijacking the pipe)
	// go l.runMockGenerator()
}

func (l *GameStateListener) runMockGenerator() {
	log.Println("[GameStateListener][Mock] Randomized 3D Mock Data Generator is available but currently DISABLED.")
	// (Hidden logic)
}
