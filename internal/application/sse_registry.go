package application

import (
	"backend/internal/domain"
	"log"
	"sync"
)

type SSERegistry struct {
    mu       sync.RWMutex
    channels map[string][]chan domain.GameSessionEvent
    pending  map[string]domain.GameSessionEvent // 👈
}

func NewSSERegistry() *SSERegistry {
    return &SSERegistry{
        channels: make(map[string][]chan domain.GameSessionEvent),
        pending:  make(map[string]domain.GameSessionEvent),
    }
}

func (r *SSERegistry) Register(id string) chan domain.GameSessionEvent {
    r.mu.Lock()
    defer r.mu.Unlock()

    ch := make(chan domain.GameSessionEvent, 1)

    // deliver buffered event immediately if notify already fired
    if event, ok := r.pending[id]; ok {
        log.Printf("[SSERegistry] Register — found pending event for id=%s", id)
        ch <- event
        delete(r.pending, id)
    }

    r.channels[id] = append(r.channels[id], ch)
    log.Printf("[SSERegistry] Register — channel registered id=%s", id)
    return ch
}

func (r *SSERegistry) Notify(id string, event domain.GameSessionEvent) {
    r.mu.Lock()
    defer r.mu.Unlock()

    channels, ok := r.channels[id]
    if !ok {
        log.Printf("[SSERegistry] Notify MISS — buffering event for id=%s", id)
        r.pending[id] = event
        return
    }

    log.Printf("[SSERegistry] Notify HIT — sending to id=%s", id)

    for _, ch := range channels {
        select {
        case ch <- event:
        default:
        }
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

    if len(r.channels[id]) == 0 {
        delete(r.channels, id)
    }
    log.Printf("[SSERegistry] Unregister — id=%s", id)
}
