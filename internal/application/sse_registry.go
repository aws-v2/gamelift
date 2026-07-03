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

// The essence of this Register function is
// we check if the session_id for the current session is in the pending sessions map, 
// if its there(according to the map[key]) then get it and 
// create a new channel, and add this new channel to the channels map
// the defer keyword executes when this Register() finishes execution
// then return that channel we created 
func (r *SSERegistry) Register(session_id string) chan domain.GameSessionEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan domain.GameSessionEvent, 1)

	if event, ok := r.pending[session_id]; ok {
		r.logger.Infow("SSE_REGISTRY_PENDING_EVENT_FOUND",
			"session_id", session_id,
			"agent_url", event.AgentURL,
		)
		//the event in this case is an instance of domain.GameSessionEvent, 
		// this is what is added to the channel
		ch <- event
		delete(r.pending, session_id)
	}

	//if the session_id exists in the pending map then we create a new channel and add it
	// to the channels map
	r.channels[session_id] = append(r.channels[session_id], ch) 

	r.logger.Infow("SSE_REGISTRY_REGISTERED",
		"session_id", session_id,
		"total_channels", len(r.channels[session_id]),
	)
	return ch

	
}

// check if the the channel to this session id exists in the channels map
// (Not the pending map)
// if not exist then insert it into the pending and return , 

// loop over the items in the channels map
// in the case the channel exists in the channels  map then add 1 to the drooped variable
// the defer keyword executes when this Notify() finishes execution
// then return that channel we created 
func (r *SSERegistry) Notify(session_id string, event domain.GameSessionEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	channels, ok := r.channels[session_id]
	if !ok {
		r.logger.Warnw("SSE_REGISTRY_NOTIFY_NO_LISTENER",
			"session_id", session_id,
			"agent_url", event.AgentURL,
			"action", "buffering",
		)
		r.pending[session_id] = event
		return
	}

	r.logger.Infow("SSE_REGISTRY_NOTIFY",
		"session_id", session_id,
		"agent_url", event.AgentURL,
		"total_channels", len(channels),
	)

	dropped := 0
	// loop over the items in the channels map
	// in the case the channel exists in the channels  map then add 1 to the drooped variable
	for _, ch := range channels {
		select {
		case ch <- event:
		default:
			dropped++
		}
	}

	if dropped > 0 {
		r.logger.Warnw("SSE_REGISTRY_NOTIFY_DROPPED",
			"session_id", session_id,
			"dropped", dropped,
			"total_channels", len(channels),
		)
	} else {
		r.logger.Infow("SSE_REGISTRY_NOTIFY_SUCCESS",
			"session_id", session_id,
			"notified", len(channels),
		)
	}
}

// Gets all the channels created for this session_id
// TODO: Update this comment i should understand howthis slice was done r.channels[session_id] = append(channels[:i], channels[i+1:]...) 
// close the all the channels related to this session_id
// the defer keyword executes when this Unregister() finishes execution
// then return that channel we created 
func (r *SSERegistry) Unregister(session_id string, ch chan domain.GameSessionEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	channels := r.channels[session_id]
	for i, c := range channels {
		if c == ch {
			r.channels[session_id] = append(channels[:i], channels[i+1:]...)
			close(ch)
			break
		}
	}

	remaining := len(r.channels[session_id])
	if remaining == 0 {
		delete(r.channels, session_id)
	}

	r.logger.Infow("SSE_REGISTRY_UNREGISTERED",
		"session_id", session_id,
		"remaining_channels", remaining,
	)
}