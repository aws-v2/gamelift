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
	remoteAddr := c.Request.RemoteAddr

	if sessionID == "" {
		h.logger.Warnw("WEBRTC_SIGNALING_MISSING_SESSION_ID", "remote_addr", remoteAddr)
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	h.logger.Infow("WEBRTC_SIGNALING_UPGRADE", "session_id", sessionID, "remote_addr", remoteAddr)

	conn, err := webrtcUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Errorw("WEBRTC_SIGNALING_UPGRADE_FAILED", "session_id", sessionID, "remote_addr", remoteAddr, "error", err)
		return
	}
	defer conn.Close()

	h.roomsMu.Lock()
	h.rooms[sessionID] = append(h.rooms[sessionID], conn)
	roomSize := len(h.rooms[sessionID])
	h.roomsMu.Unlock()

	h.logger.Infow("WEBRTC_SIGNALING_PARTICIPANT_JOINED", "session_id", sessionID, "remote_addr", remoteAddr, "total_participants", roomSize)

	// relay loop
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.logger.Errorw("WEBRTC_SIGNALING_READ_ERROR", "session_id", sessionID, "remote_addr", remoteAddr, "error", err)
			} else {
				h.logger.Infow("WEBRTC_SIGNALING_CONNECTION_CLOSED", "session_id", sessionID, "remote_addr", remoteAddr)
			}
			break
		}

		h.logger.Infow("WEBRTC_SIGNALING_MESSAGE_RECEIVED",
			"session_id", sessionID,
			"remote_addr", remoteAddr,
			"message_type", messageType,
			"message_size_bytes", len(message),
		)

		h.roomsMu.Lock()
		participants := h.rooms[sessionID]
		relayCount := 0
		for _, p := range participants {
			if p != conn {
				if err := p.WriteMessage(messageType, message); err != nil {
					h.logger.Errorw("WEBRTC_SIGNALING_RELAY_FAILED",
						"session_id", sessionID,
						"remote_addr", remoteAddr,
						"error", err,
					)
				} else {
					relayCount++
				}
			}
		}
		h.roomsMu.Unlock()

		h.logger.Infow("WEBRTC_SIGNALING_MESSAGE_RELAYED",
			"session_id", sessionID,
			"remote_addr", remoteAddr,
			"relayed_to", relayCount,
		)
	}

	// cleanup
	h.roomsMu.Lock()
	participants := h.rooms[sessionID]
	for i, p := range participants {
		if p == conn {
			h.rooms[sessionID] = append(participants[:i], participants[i+1:]...)
			break
		}
	}
	remaining := len(h.rooms[sessionID])
	if remaining == 0 {
		delete(h.rooms, sessionID)
	}
	h.roomsMu.Unlock()

	h.logger.Infow("WEBRTC_SIGNALING_PARTICIPANT_LEFT",
		"session_id", sessionID,
		"remote_addr", remoteAddr,
		"remaining_participants", remaining,
	)

	if remaining == 0 {
		h.logger.Infow("WEBRTC_SIGNALING_ROOM_CLOSED", "session_id", sessionID)
	}
}