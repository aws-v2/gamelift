package application

import (
	"backend/internal/domain"
	"sync"

	"go.uber.org/zap"
)

type SSERegistry struct {
	mu       sync.RWMutex
	channels map[string][]chan domain.GameSessionEvent
	pending  map[string]domain.GameSessionEvent
	logger   *zap.SugaredLogger
}

func NewSSERegistry(logger *zap.SugaredLogger) *SSERegistry {
	return &SSERegistry{
		channels: make(map[string][]chan domain.GameSessionEvent),
		pending:  make(map[string]domain.GameSessionEvent),
		logger:   logger,
	}
}

func (r *SSERegistry) Register(id string) chan domain.GameSessionEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan domain.GameSessionEvent, 1)

	if event, ok := r.pending[id]; ok {
		r.logger.Infow("SSE_REGISTRY_PENDING_EVENT_FOUND",
			"session_id", id,
			"agent_url", event.AgentURL,
		)
		ch <- event
		delete(r.pending, id)
	}

	r.channels[id] = append(r.channels[id], ch)

	r.logger.Infow("SSE_REGISTRY_REGISTERED",
		"session_id", id,
		"total_channels", len(r.channels[id]),
	)
	return ch
}

func (r *SSERegistry) Notify(id string, event domain.GameSessionEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	channels, ok := r.channels[id]
	if !ok {
		r.logger.Warnw("SSE_REGISTRY_NOTIFY_NO_LISTENER",
			"session_id", id,
			"agent_url", event.AgentURL,
			"action", "buffering",
		)
		r.pending[id] = event
		return
	}

	r.logger.Infow("SSE_REGISTRY_NOTIFY",
		"session_id", id,
		"agent_url", event.AgentURL,
		"total_channels", len(channels),
	)

	dropped := 0
	for _, ch := range channels {
		select {
		case ch <- event:
		default:
			dropped++
		}
	}

	if dropped > 0 {
		r.logger.Warnw("SSE_REGISTRY_NOTIFY_DROPPED",
			"session_id", id,
			"dropped", dropped,
			"total_channels", len(channels),
		)
	} else {
		r.logger.Infow("SSE_REGISTRY_NOTIFY_SUCCESS",
			"session_id", id,
			"notified", len(channels),
		)
	}
}

func (r *SSERegistry) Unregister(id string, ch chan domain.GameSessionEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	channels := r.channels[id]
	for i, c := range channels {
		if c == ch {
			r.channels[id] = append(channels[:i], channels[i+1:]...)
			close(ch)
			break
		}
	}

	remaining := len(r.channels[id])
	if remaining == 0 {
		delete(r.channels, id)
	}

	r.logger.Infow("SSE_REGISTRY_UNREGISTERED",
		"session_id", id,
		"remaining_channels", remaining,
	)
}