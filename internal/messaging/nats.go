package messaging

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Subject represents the strict naming scheme: <env>.<service>.<version>.<domain>.<action_type>
type Subject struct {
	Env        string // e.g., dev, staging, prod
	Service    string // e.g., auth, s3, iam, lambda, backend
	Version    string // e.g., v1, v2
	Domain     string // e.g., user, game, token
	ActionType string // e.g., created, updated, join
}

// String builds the formatted NATS subject string.
func (s Subject) String() string {
	return fmt.Sprintf("%s.%s.%s.%s.%s", s.Env, s.Service, s.Version, s.Domain, s.ActionType)
}

// NatsClient wrapper for structured messaging.
type NatsClient struct {
	nc     *nats.Conn
	logger *zap.SugaredLogger
}

// NewNatsClient creates a new NATS client with an established connection and logger.
func NewNatsClient(nc *nats.Conn, logger *zap.SugaredLogger) *NatsClient {
	return &NatsClient{
		nc:     nc,
		logger: logger,
	}
}

// Publish enforces the structured subject scheme for publishing messages.
func (c *NatsClient) Publish(subject Subject, data []byte) error {
	subjStr := subject.String()
	c.logger.Infow("NATS Publish", "subject", subjStr, "bytes", len(data))
	if c.nc != nil {
		return c.nc.Publish(subjStr, data)
	}
	return nil
}

// Subscribe enforces the structured subject scheme for consuming messages.
func (c *NatsClient) Subscribe(subject Subject, handler nats.MsgHandler) (*nats.Subscription, error) {
	subjStr := subject.String()
	c.logger.Infow("NATS Subscribe", "subject", subjStr)
	if c.nc != nil {
		return c.nc.Subscribe(subjStr, handler)
	}
	return nil, nil
}

// Request enforces the structured subject scheme for request-reply patterns.
func (c *NatsClient) Request(subject Subject, data []byte, timeout time.Duration) (*nats.Msg, error) {
	subjStr := subject.String()
	c.logger.Infow("NATS Request", "subject", subjStr, "bytes", len(data))
	if c.nc != nil {
		return c.nc.Request(subjStr, data, timeout)
	}
	return nil, fmt.Errorf("nats connection is nil")
}
