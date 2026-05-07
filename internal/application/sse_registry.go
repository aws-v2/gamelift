package application

import (
	"backend/internal/domain"
	"sync"
)

// SSERegistry manages multiple channels of SSE events, indexed by SessionID or GameID.
type SSERegistry struct {
	mu       sync.RWMutex
	channels map[string][]chan domain.GameSessionEvent
}

// NewSSERegistry creates a new thread-safe registry.
func NewSSERegistry() *SSERegistry {
	return &SSERegistry{
		channels: make(map[string][]chan domain.GameSessionEvent),
	}
}

// Register creates and returns a new channel for a specific ID.
func (r *SSERegistry) Register(id string) chan domain.GameSessionEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan domain.GameSessionEvent, 1) // Buffered to prevent blocking publishers
	r.channels[id] = append(r.channels[id], ch)
	return ch
}

// Unregister removes and closes a channel for a specific ID.
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

	if len(r.channels[id]) == 0 {
		delete(r.channels, id)
	}
}

// Notify sends an event to all channels registered for a specific ID.
func (r *SSERegistry) Notify(id string, event domain.GameSessionEvent) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if channels, ok := r.channels[id]; ok {
		for _, ch := range channels {
			// Non-blocking send to avoid hanging the publisher if a consumer is slow
			select {
			case ch <- event:
			default:
				// Channel is full or blocked, skip or handle as needed
			}
		}
	}
}
