package websocket

import (
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
	conn       *websocket.Conn
	send       chan []byte
	remoteAddr string
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
	h.logger.Infow("WS_HUB_STARTED")

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			count := len(h.clients)
			h.mu.Unlock()

			h.logger.Infow("WS_HUB_CLIENT_REGISTERED",
				"remote_addr", client.remoteAddr,
				"total_clients", count,
			)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				count := len(h.clients)
				h.mu.Unlock()

				h.logger.Infow("WS_HUB_CLIENT_UNREGISTERED",
					"remote_addr", client.remoteAddr,
					"total_clients", count,
				)
			} else {
				h.mu.Unlock()
			}

		case message := <-h.broadcast:
			h.mu.Lock()
			total := len(h.clients)
			dropped := 0
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
					dropped++
				}
			}
			h.mu.Unlock()

			h.logger.Debugw("WS_HUB_BROADCAST",
				"bytes", len(message),
				"recipients", total-dropped,
				"dropped_clients", dropped,
			)
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
	remoteAddr := c.Request.RemoteAddr

	h.hub.logger.Infow("WS_UPGRADE_REQUESTED", "remote_addr", remoteAddr, "path", c.FullPath())

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.hub.logger.Errorw("WS_UPGRADE_FAILED", "remote_addr", remoteAddr, "error", err)
		return
	}

	client := &Client{conn: conn, send: make(chan []byte, 256), remoteAddr: remoteAddr}
	h.hub.register <- client

	h.hub.logger.Infow("WS_CLIENT_CONNECTED", "remote_addr", remoteAddr)

	go h.writePump(client)
	h.readPump(client)
}

func (h *WebSocketHandler) readPump(c *Client) {
	defer func() {
		h.hub.logger.Infow("WS_READ_PUMP_CLOSING", "remote_addr", c.remoteAddr)
		h.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.hub.logger.Errorw("WS_READ_ERROR", "remote_addr", c.remoteAddr, "error", err)
			} else {
				h.hub.logger.Infow("WS_CLIENT_DISCONNECTED", "remote_addr", c.remoteAddr)
			}
			break
		}

		h.hub.logger.Debugw("WS_MESSAGE_RECEIVED",
			"remote_addr", c.remoteAddr,
			"bytes", len(message),
		)

		h.hub.broadcast <- message
	}
}

func (h *WebSocketHandler) writePump(c *Client) {
	defer func() {
		h.hub.logger.Infow("WS_WRITE_PUMP_CLOSING", "remote_addr", c.remoteAddr)
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				h.hub.logger.Infow("WS_SEND_CHANNEL_CLOSED", "remote_addr", c.remoteAddr)
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				h.hub.logger.Errorw("WS_WRITE_FAILED", "remote_addr", c.remoteAddr, "bytes", len(message), "error", err)
				return
			}

			h.hub.logger.Debugw("WS_MESSAGE_SENT", "remote_addr", c.remoteAddr, "bytes", len(message))
		}
	}
}