package repository

import (
	"backend/internal/domain"
	"backend/internal/infrastructure/messaging"
	"context"
	"time"

	"github.com/nats-io/nats.go"
)

// ProvisioningService defines the contract for on-demand game startup.
type ProvisioningService interface {
	ProvisionGame(gameID int, mode domain.StreamingMode) error
	
}
// MessagingClient defines the interface for our messaging layer,
// allowing for easy mocking in unit tests.
type MessagingClient interface {
	Publish(subj messaging.Subject, data []byte) error
	Subscribe(subj messaging.Subject, cb nats.MsgHandler) (*nats.Subscription, error)
	Request(subj messaging.Subject, data []byte, timeout time.Duration) (*nats.Msg, error)
}



type SessionService interface {
	CreateSession(ctx context.Context, req domain.CreateSessionRequest) (*domain.GameSession, error)
	GetActiveSession(ctx context.Context, gameID, userID string) (*domain.GameSession, error)
}

// AuthService defines the contract for authentication operations.
type AuthService interface {
	GenerateToken(username string) (string, error)
	ValidateToken(tokenStr string) (map[string]interface{}, error)
}
