package handlers

import (
	"backend/internal/application"
	"backend/internal/domain"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type GameHandler struct {
	svc         application.GameService
	log         *zap.SugaredLogger
	sseRegistry *application.SSERegistry
}

func NewGameHandler(svc application.GameService, log *zap.SugaredLogger, sseRegistry *application.SSERegistry) *GameHandler {
	return &GameHandler{svc: svc, log: log, sseRegistry: sseRegistry}
}

func (h *GameHandler) ListGames(c *gin.Context) {
	h.log.Infow("HANDLER_LIST_GAMES")

	games, err := h.svc.ListGames(c.Request.Context())
	if err != nil {
		h.log.Errorw("HANDLER_LIST_GAMES_FAILED", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list games"})
		return
	}

	h.log.Infow("HANDLER_LIST_GAMES_SUCCESS", "count", len(games))
	c.JSON(http.StatusOK, games)
}

func (h *GameHandler) GetGame(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		h.log.Warnw("HANDLER_GET_GAME_INVALID_ID", "raw_id", c.Param("id"), "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	h.log.Infow("HANDLER_GET_GAME", "game_id", id)

	game, err := h.svc.GetGame(c.Request.Context(), strconv.Itoa(int(id)))
	if err != nil {
		h.log.Warnw("HANDLER_GET_GAME_NOT_FOUND", "game_id", id, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
		return
	}

	h.log.Infow("HANDLER_GET_GAME_SUCCESS", "game_id", id, "name", game.Name, "status", game.Status)
	c.JSON(http.StatusOK, game)
}

func (h *GameHandler) CreateGame(c *gin.Context) {
	var req application.CreateGameRequest
	req.UserID = c.GetString("userID")

	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warnw("HANDLER_CREATE_GAME_BAD_REQUEST", "user_id", req.UserID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.log.Infow("HANDLER_CREATE_GAME", "user_id", req.UserID, "name", req.Name)

	game, err := h.svc.CreateGame(c.Request.Context(), req)
	if err != nil {
		h.log.Errorw("HANDLER_CREATE_GAME_FAILED", "user_id", req.UserID, "name", req.Name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create game"})
		return
	}

	h.log.Infow("HANDLER_CREATE_GAME_SUCCESS", "game_id", game.ID, "name", game.Name, "user_id", req.UserID)
	c.JSON(http.StatusCreated, game)
}

func (h *GameHandler) UpdateGame(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		h.log.Warnw("HANDLER_UPDATE_GAME_INVALID_ID", "raw_id", c.Param("id"), "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req application.UpdateGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warnw("HANDLER_UPDATE_GAME_BAD_REQUEST", "game_id", id, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.log.Infow("HANDLER_UPDATE_GAME", "game_id", id)

	game, err := h.svc.UpdateGame(c.Request.Context(), strconv.Itoa(int(id)), req)
	if err != nil {
		h.log.Errorw("HANDLER_UPDATE_GAME_FAILED", "game_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update game"})
		return
	}

	h.log.Infow("HANDLER_UPDATE_GAME_SUCCESS", "game_id", id, "name", game.Name, "status", game.Status)
	c.JSON(http.StatusOK, game)
}

func (h *GameHandler) DeleteGame(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		h.log.Warnw("HANDLER_DELETE_GAME_INVALID_ID", "raw_id", c.Param("id"), "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	h.log.Infow("HANDLER_DELETE_GAME", "game_id", id)

	if err := h.svc.DeleteGame(c.Request.Context(), strconv.Itoa(int(id))); err != nil {
		h.log.Errorw("HANDLER_DELETE_GAME_FAILED", "game_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete game"})
		return
	}

	h.log.Infow("HANDLER_DELETE_GAME_SUCCESS", "game_id", id)
	c.Status(http.StatusNoContent)
}

func (h *GameHandler) StreamSessionEvents(c *gin.Context) {
	sessionID := c.Param("instanceId")
	w := c.Writer
	r := c.Request

	h.log.Infow("HANDLER_SSE_CONNECT", "session_id", sessionID, "remote_addr", r.RemoteAddr)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.log.Errorw("HANDLER_SSE_NOT_SUPPORTED", "session_id", sessionID)
		c.String(http.StatusInternalServerError, "streaming not supported")
		return
	}

	ch := h.sseRegistry.Register(sessionID)
	defer h.sseRegistry.Unregister(sessionID, ch)

	for {
		select {
		case payload, ok := <-ch:
			if !ok {
				h.log.Infow("HANDLER_SSE_CHANNEL_CLOSED", "session_id", sessionID)
				return
			}

			data, _ := json.Marshal(payload)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			h.log.Infow("HANDLER_SSE_EVENT_SENT", "session_id", sessionID, "agent_url", payload.AgentURL)

			if payload.AgentURL != "" {
				h.log.Infow("HANDLER_SSE_PROVISIONED", "session_id", sessionID, "agent_url", payload.AgentURL)
				return
			}

		case <-time.After(5 * time.Minute):
			h.log.Warnw("HANDLER_SSE_TIMEOUT", "session_id", sessionID)
			fmt.Fprintf(w, "data: {\"error\": \"provisioning timed out\"}\n\n")
			flusher.Flush()
			return

		case <-r.Context().Done():
			h.log.Infow("HANDLER_SSE_DISCONNECT", "session_id", sessionID, "reason", r.Context().Err())
			return
		}
	}
}

func (h *GameHandler) GetManifest(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		h.log.Warnw("HANDLER_GET_MANIFEST_INVALID_ID", "raw_id", c.Param("id"), "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	h.log.Infow("HANDLER_GET_MANIFEST", "game_id", id)

	manifest, err := h.svc.GetManifest(c.Request.Context(), strconv.Itoa(int(id)))
	if err != nil {
		h.log.Warnw("HANDLER_GET_MANIFEST_NOT_FOUND", "game_id", id, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "manifest not found"})
		return
	}

	h.log.Infow("HANDLER_GET_MANIFEST_SUCCESS", "game_id", id)
	c.JSON(http.StatusOK, manifest)
}

func (h *GameHandler) DownloadPackage(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		h.log.Warnw("HANDLER_DOWNLOAD_PACKAGE_INVALID_ID", "raw_id", c.Param("id"), "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	h.log.Infow("HANDLER_DOWNLOAD_PACKAGE", "game_id", id)

	url, err := h.svc.GetDownloadURL(c.Request.Context(), strconv.Itoa(int(id)))
	if err != nil {
		h.log.Warnw("HANDLER_DOWNLOAD_PACKAGE_NOT_FOUND", "game_id", id, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
		return
	}

	h.log.Infow("HANDLER_DOWNLOAD_PACKAGE_REDIRECT", "game_id", id, "url", url)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// computeSHA256 calculates the SHA256 checksum of the provided reader
func computeSHA256(file io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}


func (h *GameHandler) InitUpload(c *gin.Context) {
	reqID := uuid.New().String()

	log := h.log.With(
		"layer", "handler",
		"method", "InitUpload",
		"request_id", reqID,
	)

	log.Infow("HANDLER_INIT_UPLOAD_RECEIVED")

	// 1. Get metadata from form fields
	gameName := c.PostForm("game_name")
	clientSHA := c.PostForm("sha256") // The SHA256 sent from the frontend
	userID := c.GetString("userID")

	// 2. Get the file from the request
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		log.Warnw("HANDLER_INIT_UPLOAD_MISSING_FILE", "user_id", userID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required for upload protocol"})
		return
	}
	defer file.Close()

	// 3. Calculate SHA256 on the backend
	backendSHA, err := computeSHA256(file)
	if err != nil {
		log.Errorw("HANDLER_SHA_CALCULATION_FAILED", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify file integrity"})
		return
	}

	// 4. Integrity Verification: Reject if they don't match
	if clientSHA != "" && clientSHA != backendSHA {
		log.Errorw("HANDLER_INTEGRITY_MISMATCH", 
			"user_id", userID, 
			"client_sha", clientSHA, 
			"backend_sha", backendSHA,
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "integrity check failed: local hash does not match server-calculated hash",
		})
		return
	}

	// 5. Construct the request for the service layer
	req := application.InitUploadRequest{
		UserID:  userID,
		Name:    gameName,
		SHA256:  backendSHA,
		Version: c.DefaultPostForm("version", "latest"),
	}

	log.Infow("HANDLER_INIT_UPLOAD_PARSED", "game_name", req.Name, "user_id", req.UserID)

	result, err := h.svc.InitUpload(c.Request.Context(), req, log)
	if err != nil {
		log.Errorw("HANDLER_INIT_UPLOAD_FAILED", "game_name", req.Name, "user_id", req.UserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to init upload: " + err.Error()})
		return
	}

	log.Infow("HANDLER_INIT_UPLOAD_SUCCESS", "game_id", result.GameID, "user_id", req.UserID)
	c.PureJSON(http.StatusOK, result)
}


func (h *GameHandler) PlayGame(c *gin.Context) {
	var req application.PlayGameRequest
	req.UserID = c.GetString("userID")

	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warnw("HANDLER_PLAY_GAME_BAD_REQUEST", "user_id", req.UserID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.log.Infow("HANDLER_PLAY_GAME", "user_id", req.UserID, "game_id", req.GameID)

	result, err := h.svc.PlayGame(c.Request.Context(), req)
	if err != nil {
		h.log.Errorw("HANDLER_PLAY_GAME_FAILED", "user_id", req.UserID, "game_id", req.GameID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start game"})
		return
	}

	h.log.Infow("HANDLER_PLAY_GAME_SUCCESS", "user_id", req.UserID, "game_id", req.GameID, "session_id", result.SessionID)
	c.JSON(http.StatusOK, result)
}

func (h *GameHandler) CreateSession(c *gin.Context) {
	gameID := c.Param("id")
	userID := c.GetString("userID")

	var req domain.CreateSessionRequest
	req.UserID = userID

	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warnw("HANDLER_CREATE_SESSION_BAD_REQUEST", "game_id", gameID, "user_id", userID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.log.Infow("HANDLER_CREATE_SESSION", "game_id", gameID, "user_id", userID)

	session, err := h.svc.CreateSession(c.Request.Context(), gameID, req)
	if err != nil {
		h.log.Errorw("HANDLER_CREATE_SESSION_FAILED", "game_id", gameID, "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.log.Infow("HANDLER_CREATE_SESSION_SUCCESS", "session_id", session.ID, "game_id", gameID, "user_id", userID)
	c.JSON(http.StatusCreated, session)
}

func (h *GameHandler) GetSessionStatus(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		h.log.Warnw("HANDLER_GET_SESSION_STATUS_INVALID_ID", "raw_id", c.Param("id"), "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	h.log.Infow("HANDLER_GET_SESSION_STATUS", "session_id", id)

	session, err := h.svc.GetSessionStatus(c.Request.Context(), strconv.Itoa(int(id)))
	if err != nil {
		h.log.Warnw("HANDLER_GET_SESSION_STATUS_NOT_FOUND", "session_id", id, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	h.log.Infow("HANDLER_GET_SESSION_STATUS_SUCCESS", "session_id", id, "status", session.Status)
	c.JSON(http.StatusOK, session)
}

// ── helper ────────────────────────────────────────────────────────────────────

func parseID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	return uint(id), err
}

// func (h *GameHandler) StreamInstanceEvents(c *gin.Context) {
// 	instanceID := c.Param("instanceId")

// 	// ch := h.sseBroker.Register(instanceID)
// 	// defer h.sseBroker.Unregister(instanceID)


// 	ch := h.sseRegistry.Register(sessionID)
// 	defer h.sseRegistry.Unregister(sessionID, ch)

// 	c.Writer.Header().Set("Content-Type", "text/event-stream")
// 	c.Writer.Header().Set("Cache-Control", "no-cache")
// 	c.Writer.Header().Set("Connection", "keep-alive")

// 	select {
// 	case payload := <-ch:
// 		fmt.Fprintf(c.Writer, "sse-data: %s\n\n", payload)
// 		c.Writer.Flush()
// 	case <-time.After(5 * time.Minute):
// 		fmt.Fprintf(c.Writer, "sse-data: {\"error\":\"timeout\"}\n\n")
// 		c.Writer.Flush()
// 	case <-c.Request.Context().Done():
// 		return
// 	}
// }
