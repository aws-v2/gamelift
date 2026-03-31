package handler

import (
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var webrtcUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WebRTCSignalingHandler manages the exchange of SDP/ICE candidates between the client and the Godot instance.
type WebRTCSignalingHandler struct {
	// rooms maps a session ID to a list of participants (client and game server)
	rooms   map[string][]*websocket.Conn
	roomsMu sync.Mutex
}

func NewWebRTCSignalingHandler() *WebRTCSignalingHandler {
	return &WebRTCSignalingHandler{
		rooms: make(map[string][]*websocket.Conn),
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
		log.Printf("[WebRTC] Upgrade error: %v", err)
		return
	}
	defer conn.Close()

	h.roomsMu.Lock()
	h.rooms[sessionID] = append(h.rooms[sessionID], conn)
	roomSize := len(h.rooms[sessionID])
	h.roomsMu.Unlock()

	log.Printf("[WebRTC] Session %s: Participant joined. Total: %d", sessionID, roomSize)

	// In a simple signaling server, we just relay any received message to the other participant in the same room.
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WebRTC] Session %s error: %v", sessionID, err)
			break
		}

		h.roomsMu.Lock()
		participants := h.rooms[sessionID]
		for _, p := range participants {
			if p != conn {
				if err := p.WriteMessage(messageType, message); err != nil {
					log.Printf("[WebRTC] Relay error: %v", err)
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
