package handlers

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var webrtcUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WebRTCSignalingHandler struct {
	// rooms maps a session ID to a list of participants (client and game server)
	rooms   map[string][]*websocket.Conn
	roomsMu sync.Mutex
	logger  *zap.SugaredLogger
}

func NewWebRTCSignalingHandler(logger *zap.SugaredLogger) *WebRTCSignalingHandler {
	return &WebRTCSignalingHandler{
		rooms:  make(map[string][]*websocket.Conn),
		logger: logger,
	}
}

func (h *WebRTCSignalingHandler) HandleSignaling(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	conn, err := webrtcUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Errorw("WebRTC upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	h.roomsMu.Lock()
	h.rooms[sessionID] = append(h.rooms[sessionID], conn)
	roomSize := len(h.rooms[sessionID])
	h.roomsMu.Unlock()

	h.logger.Infow("Participant joined WebRTC Hub", "session_id", sessionID, "total_participants", roomSize)

	// In a simple signaling server, we just relay any received message to the other participant in the same room.
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			h.logger.Errorw("WebRTC session error", "session_id", sessionID, "error", err)
			break
		}

		h.roomsMu.Lock()
		participants := h.rooms[sessionID]
		for _, p := range participants {
			if p != conn {
				if err := p.WriteMessage(messageType, message); err != nil {
					h.logger.Errorw("WebRTC relay error", "error", err)
				}
			}
		}
		h.roomsMu.Unlock()
	}

	// Cleanup
	h.roomsMu.Lock()
	participants := h.rooms[sessionID]
	for i, p := range participants {
		if p == conn {
			h.rooms[sessionID] = append(participants[:i], participants[i+1:]...)
			break
		}
	}
	if len(h.rooms[sessionID]) == 0 {
		delete(h.rooms, sessionID)
	}
	h.roomsMu.Unlock()
}
