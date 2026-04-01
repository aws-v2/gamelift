package interfaces

import (
	"time"

	"backend/internal/messaging"

	"github.com/nats-io/nats.go"
)

// MessagingClient defines the interface for our messaging layer,
// allowing for easy mocking in unit tests.
type MessagingClient interface {
	Publish(subj messaging.Subject, data []byte) error
	Subscribe(subj messaging.Subject, cb nats.MsgHandler) (*nats.Subscription, error)
	Request(subj messaging.Subject, data []byte, timeout time.Duration) (*nats.Msg, error)
}
