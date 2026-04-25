package messaging

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Subject represents the strict naming scheme: <env>.<service>.<version>.<domain>.<action_type>

type Subject struct {
	Service    string // s3, auth, iam
	Domain     string // bucket, user, token
	ActionType string // create_presigned_url, get_download_url
}

func (s Subject) String() string {
	return fmt.Sprintf("%s.%s.%s", s.Service, s.Domain, s.ActionType)
}
// NatsClient wrapper for structured messaging.
type NatsClient struct {
	nc     *nats.Conn
	logger *zap.SugaredLogger
	prefix string
}

// NewNatsClient creates a new NATS client with an established connection, logger and subject prefix.
func NewNatsClient(nc *nats.Conn, logger *zap.SugaredLogger, prefix string) *NatsClient {
	return &NatsClient{
		nc:     nc,
		logger: logger,
		prefix: prefix,
	}
}

func (c *NatsClient) formatSubject(subject Subject) string {
	if c.prefix != "" {
		return fmt.Sprintf("%s.%s", c.prefix, subject.String())
	}
	return subject.String()
}

// Publish enforces the structured subject scheme for publishing messages.
func (c *NatsClient) Publish(subject Subject, data []byte) error {
	subjStr := c.formatSubject(subject)
	c.logger.Infow("NATS Publish", "subject", subjStr, "bytes", len(data))
	if c.nc != nil {
		return c.nc.Publish(subjStr, data)
	}
	return nil
}

// Subscribe enforces the structured subject scheme for consuming messages.
func (c *NatsClient) Subscribe(subject Subject, handler nats.MsgHandler) (*nats.Subscription, error) {
	subjStr := c.formatSubject(subject)
	c.logger.Infow("NATS Subscribe", "subject", subjStr)
	if c.nc != nil {
		return c.nc.Subscribe(subjStr, handler)
	}
	return nil, nil
}

// Request enforces the structured subject scheme for request-reply patterns.
func (c *NatsClient) Request(subject Subject, data []byte, timeout time.Duration) (*nats.Msg, error) {
	subjStr := c.formatSubject(subject)
	c.logger.Infow("NATS Request", "subject", subjStr, "bytes", len(data))
	if c.nc != nil {
		return c.nc.Request(subjStr, data, timeout)
	}
	return nil, fmt.Errorf("nats connection is nil")
}
