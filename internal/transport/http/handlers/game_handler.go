package handlers

import (
	"backend/internal/application"
	"backend/internal/domain"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type GameHandler struct {
	svc application.GameService
	log *zap.SugaredLogger
}

func NewGameHandler(svc application.GameService, log *zap.SugaredLogger) *GameHandler {
	return &GameHandler{svc: svc, log: log}
}

func (h *GameHandler) ListGames(c *gin.Context) {
	games, err := h.svc.ListGames(c.Request.Context())
	if err != nil {
		h.log.Errorw("ListGames", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list games"})
		return
	}
	c.JSON(http.StatusOK, games)
}

func (h *GameHandler) GetGame(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	game, err := h.svc.GetGame(c.Request.Context(), strconv.Itoa(int(id)))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
		return
	}
	c.JSON(http.StatusOK, game)
}

func (h *GameHandler) CreateGame(c *gin.Context) {
	var req application.CreateGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	game, err := h.svc.CreateGame(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create game"})
		return
	}
	c.JSON(http.StatusCreated, game)
}

func (h *GameHandler) UpdateGame(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req application.UpdateGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	game, err := h.svc.UpdateGame(c.Request.Context(), strconv.Itoa(int(id)), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update game"})
		return
	}
	c.JSON(http.StatusOK, game)
}

func (h *GameHandler) DeleteGame(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.svc.DeleteGame(c.Request.Context(), strconv.Itoa(int(id))); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete game"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *GameHandler) GetManifest(c *gin.Context) {
	id, _ := parseID(c)
	manifest, err := h.svc.GetManifest(c.Request.Context(), strconv.Itoa(int(id)))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "manifest not found"})
		return
	}
	c.JSON(http.StatusOK, manifest)
}

func (h *GameHandler) DownloadPackage(c *gin.Context) {
	id, _ := parseID(c)
	url, err := h.svc.GetDownloadURL(c.Request.Context(), strconv.Itoa(int(id)))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *GameHandler) InitUpload(c *gin.Context) {
	h.log.Info("InitUpload")

	var req application.InitUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		h.log.Error("InitUpload", "error", err)
		return
	}
	h.log.Info("InitUpload 1", "req", req)
	result, err := h.svc.InitUpload(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to init upload"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GameHandler) PlayGame(c *gin.Context) {
	var req application.PlayGameRequest
	userID := c.GetString("userID")
	req.UserID= userID
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.svc.PlayGame(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start game"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GameHandler) CreateSession(c *gin.Context) {

	gameId := c.Param("id")
	userID := c.GetString("userID")
	
	// userId := c.GetString(string(domain.UserIDKey))
	var req domain.CreateSessionRequest
	req.UserID = userID

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error-": err.Error()})
		return
	}
	session, err := h.svc.CreateSession(c.Request.Context(), gameId, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	c.JSON(http.StatusCreated, session)
}

func (h *GameHandler) GetSessionStatus(c *gin.Context) {
	id, _ := parseID(c)
	session, err := h.svc.GetSessionStatus(c.Request.Context(), strconv.Itoa(int(id)))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	c.JSON(http.StatusOK, session)
}

// ── helper ────────────────────────────────────────────────────────────────────

func parseID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	return uint(id), err
}