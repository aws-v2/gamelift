package websocket

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.Mutex
	logger     *zap.SugaredLogger
}

func NewHub(logger *zap.SugaredLogger) *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		logger:     logger,
	}
}

func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.mu.Unlock()
			h.logger.Infow("New client connected", "total_clients", len(h.clients))
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.logger.Infow("Client disconnected", "total_clients", len(h.clients))
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

type WebSocketHandler struct {
	hub *Hub
}

func NewWebSocketHandler(hub *Hub) *WebSocketHandler {
	return &WebSocketHandler{hub: hub}
}

func (h *WebSocketHandler) Handle(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.hub.logger.Errorw("WebSocket upgrade failed", "error", err)
		return
	}

	client := &Client{conn: conn, send: make(chan []byte, 256)}
	h.hub.register <- client

	go h.writePump(client)
	h.readPump(client)
}

func (h *WebSocketHandler) readPump(c *Client) {
	defer func() {
		h.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			h.hub.logger.Debugw("Error reading from websocket", "client", fmt.Sprintf("%p", c), "error", err)
			break
		}
		
		// Log every message received (Keyboard, Mouse, or Godot State)
		h.hub.logger.Debugw("Incoming message", "client", fmt.Sprintf("%p", c), "msg", string(message))
		
		// Log the data being sent to the hub for transparency
		h.hub.logger.Debugw("Broadcasting message", "client", fmt.Sprintf("%p", c), "msg", string(message))
		h.hub.broadcast <- message
	}
}

func (h *WebSocketHandler) writePump(c *Client) {
	defer func() {
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}
