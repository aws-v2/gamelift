package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"backend/internal/domain"
	"backend/internal/infrastructure/storage"
	"backend/internal/interfaces"
	"backend/internal/messaging"
	"backend/internal/transport/response"
	"io"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type GameHandler struct {
	gameSvc         interfaces.GameRepository
	natsClient      *messaging.NatsClient
	provisioningSvc interfaces.ProvisioningService
	storage         *storage.MinIOAdapter
}

func NewGameHandler(
	gameSvc interfaces.GameRepository, 
	natsClient *messaging.NatsClient, 
	provisioningSvc interfaces.ProvisioningService,
	storage *storage.MinIOAdapter,
) *GameHandler {
	return &GameHandler{
		gameSvc:         gameSvc,
		natsClient:      natsClient,
		provisioningSvc: provisioningSvc,
		storage:         storage,
	}
}

func (h *GameHandler) ListGames(c *gin.Context) {
	games, err := h.gameSvc.ListGames()
	if err != nil {
		response.SendError(c, http.StatusInternalServerError, "internal error")
		return
	}

	response.SendSuccess(c, http.StatusOK, "Games retrieved successfully", games)
}

func (h *GameHandler) InitUpload(c *gin.Context) {
	userID := c.GetString(string(domain.UserIDKey))

	var req struct {
		GameName string              `json:"game_name" binding:"required"`
		VMID     string              `json:"vm_id" binding:"required"`
		Manifest domain.GameManifest `json:"manifest" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.SendError(c, http.StatusBadRequest, "invalid request: missing game_name, vm_id, or manifest")
		return
	}

	// 1. Basic Manifest Validation (Schema only)
	if req.Manifest.HeadlessBin == "" || req.Manifest.MainScene == "" {
		response.SendError(c, http.StatusBadRequest, "invalid manifest: missing headless_bin or main_scene")
		return
	}

	// 2. Database Initialization (PENDING record)
	game, err := h.gameSvc.InitUpload(req.GameName, req.VMID, userID)
	if err != nil {
		response.SendError(c, http.StatusInternalServerError, "failed to initialize game record")
		return
	}

	// Store manifest early
	manifestJSON, _ := json.Marshal(req.Manifest)
	h.gameSvc.UpdateGameManifest(game.ID, string(manifestJSON))

	// 3. Request Presigned S3 URL via NATS
	subj := messaging.Subject{
		Env:        "dev",
		Service:    "api",
		Version:    "v1",
		Domain:     "s3",
		ActionType: "create_presigned_url",
	}
	payload := map[string]interface{}{
		"game_id":   game.ID,
		"user_id":   userID,
		"arn":       game.ARN,
		"extension": ".zip",
	}
	data, _ := json.Marshal(payload)
	msg, err := h.natsClient.Request(subj, data, 2*time.Second)
	if err != nil {
		response.SendError(c, http.StatusServiceUnavailable, "s3 service unavailable (nats timeout)")
		return
	}

	var natsResp struct {
		UploadURL string `json:"upload_url"`
	}
	if err := json.Unmarshal(msg.Data, &natsResp); err != nil || natsResp.UploadURL == "" {
		response.SendError(c, http.StatusInternalServerError, "invalid response from s3 service")
		return
	}

	log.Printf("[GameHandler] User %s initiated upload for game '%s' -> Returning Presigned URL  with this url: '%s'", userID, req.GameName,natsResp.UploadURL)

	response.SendSuccess(c, http.StatusOK, "Upload initialized. Please upload your ZIP to S3.", gin.H{
		"game_id":    game.ID,
		"upload_url": natsResp.UploadURL,
		"arn":        game.ARN,
	})
}



func (h *GameHandler) GetManifest(c *gin.Context) {
	idStr := c.Param("id")
	var id int
	fmt.Sscanf(idStr, "%d", &id)

	game, err := h.gameSvc.GetGame(id)
	if err != nil || game.Manifest == "" {
		response.SendError(c, http.StatusNotFound, "manifest not found")
		return
	}

	var manifest interface{}
	json.Unmarshal([]byte(game.Manifest), &manifest)
	response.SendSuccess(c, http.StatusOK, "Manifest retrieved successfully", manifest)
}

// PlayGame handles the on-demand provisioning request.
func (h *GameHandler) PlayGame(c *gin.Context) {
	var req struct {
		GameID int                  `json:"game_id" binding:"required"`
		Mode   domain.StreamingMode `json:"streaming_mode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.SendError(c, http.StatusBadRequest, "missing game_id")
		return
	}

	// Default to state_sync if not provided
	if req.Mode == "" {
		req.Mode = domain.StreamingModeState
	}

	err := h.provisioningSvc.ProvisionGame(req.GameID, req.Mode)
	if err != nil {
		log.Printf("[GameHandler] Provisioning failed for game %d: %v", req.GameID, err)
		response.SendError(c, http.StatusConflict, err.Error())
		return
	}

	response.SendSuccess(c, http.StatusAccepted, "Game provisioning started", gin.H{
		"game_id": req.GameID,
		"status":  "provisioning",
	})
}

// DownloadPackage streams the game ZIP directly from MinIO to the requester (e.g. EC2 VM)
func (h *GameHandler) DownloadPackage(c *gin.Context) {
	gameIDStr := c.Param("id")
	gameID, err := strconv.Atoi(gameIDStr)
	if err != nil {
		response.SendError(c, http.StatusBadRequest, "invalid game id")
		return
	}

	game, err := h.gameSvc.GetGame(gameID)
	if err != nil {
		response.SendError(c, http.StatusNotFound, "game not found")
		return
	}

	// Parsing bucket and key from ARN: arn:aws:s3:::bucket/key
	arnParts := strings.Split(game.ARN, ":::")
	if len(arnParts) < 2 {
		response.SendError(c, http.StatusInternalServerError, "invalid storage configuration for game")
		return
	}
	pathParts := strings.SplitN(arnParts[1], "/", 2)
	if len(pathParts) < 2 {
		response.SendError(c, http.StatusInternalServerError, "invalid path in game storage")
		return
	}
	bucket, key := pathParts[0], pathParts[1]

	log.Printf("[GameHandler] Internal download request for Game %d (%s/%s)", gameID, bucket, key)

	// Get Object Info for Content-Length
	info, err := h.storage.GetObjectInfo(c.Request.Context(), bucket, key)
	if err != nil {
		response.SendError(c, http.StatusInternalServerError, fmt.Sprintf("failed to get package info: %v", err))
		return
	}

	// Get Stream
	stream, err := h.storage.GetObjectStream(c.Request.Context(), bucket, key)
	if err != nil {
		response.SendError(c, http.StatusInternalServerError, fmt.Sprintf("failed to open package stream: %v", err))
		return
	}
	defer stream.Close()

	// Set Headers
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"game_%d.zip\"", gameID))
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Length", strconv.FormatInt(info.Size, 10))

	// Stream directly to response writer
	_, err = io.Copy(c.Writer, stream)
	if err != nil {
		log.Printf("[GameHandler] Stream error for Game %d: %v", gameID, err)
	}
}
