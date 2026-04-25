package handlers

import (
	"encoding/json"
	"fmt"
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
	"go.uber.org/zap"
)

type GameHandler struct {
	gameSvc         interfaces.GameRepository
	natsClient      interfaces.MessagingClient
	provisioningSvc interfaces.ProvisioningService
	storage         *storage.MinIOAdapter
	logger          *zap.SugaredLogger
	natsPrefix		string
}

func NewGameHandler(
	gameSvc interfaces.GameRepository,
	natsClient interfaces.MessagingClient,
	provisioningSvc interfaces.ProvisioningService,
	storage *storage.MinIOAdapter,
	logger *zap.SugaredLogger,
	natsPrefix string,
) *GameHandler {
	return &GameHandler{
		gameSvc:         gameSvc,
		natsClient:      natsClient,
		provisioningSvc: provisioningSvc,
		storage:         storage,
		logger:          logger,
		natsPrefix: natsPrefix,
	}
}

func (h *GameHandler) ListGames(c *gin.Context) {
	games, err := h.gameSvc.ListGames()
	if err != nil {
		response.SendAppError(c, err)
		return
	}

	response.SendSuccess(c, http.StatusOK, "Games retrieved successfully", games)
}

func (h *GameHandler) InitUpload(c *gin.Context) {
	userID := c.GetString(string(domain.UserIDKey))

	var req domain.GameInitUploadRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.SendAppError(c, domain.ErrValidationFailed)
		return
	}

	// 1. Basic Manifest Validation (Schema only)
	if req.Manifest.HeadlessBin == "" || req.Manifest.MainScene == "" {
		response.SendAppError(c, domain.ErrValidationFailed)
		return
	}

	// 2. Database Initialization (PENDING record)
	game, err := h.gameSvc.InitUpload(req.GameName, req.VMID, userID)
	fmt.Println("game----------------------1", game)
	if err != nil {
		response.SendAppError(c, err)
		return
	}
	fmt.Println("game----------------------12")

	// Store manifest early
	manifestJSON, _ := json.Marshal(req.Manifest)
	h.gameSvc.UpdateGameManifest(game.ID, string(manifestJSON))

	// 3. Request Presigned S3 URL via NATS
	subj := messaging.Subject{
		Service:    "s3",
		Domain:     "bucket",
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
		response.SendAppError(c, domain.ErrProvisioningFailed)
		return
	}

	var natsResp domain.S3PresignedURLResponse
	if err := json.Unmarshal(msg.Data, &natsResp); err != nil || natsResp.UploadURL == "" {
		response.SendAppError(c, domain.ErrInternal)
		return
	}

	h.logger.Infow("User initiated upload", "user_id", userID, "game_name", req.GameName, "upload_url", natsResp.UploadURL)




	
	response.SendSuccess(c, http.StatusOK, "Upload initialized. Please upload your ZIP to S3.", domain.InitUploadResponse{
		GameID:    game.ID,
		UploadURL: "http://localhost:8080"+natsResp.UploadURL,
		ARN:        game.ARN,
	})
}



func (h *GameHandler) GetManifest(c *gin.Context) {
	idStr := c.Param("id")
	var id int
	fmt.Sscanf(idStr, "%d", &id)

	game, err := h.gameSvc.GetGame(id)
	if err != nil || game.Manifest == "" {
		response.SendAppError(c, domain.ErrGameNotFound)
		return
	}

	var manifest interface{}
	json.Unmarshal([]byte(game.Manifest), &manifest)
	response.SendSuccess(c, http.StatusOK, "Manifest retrieved successfully", manifest)
}

// PlayGame handles the on-demand provisioning request.
func (h *GameHandler) PlayGame(c *gin.Context) {
	var req domain.PlayGameRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.SendAppError(c, domain.ErrValidationFailed)
		return
	}

	// Default to state_sync if not provided
	if req.Mode == "" {
		req.Mode = domain.StreamingModeState
	}

	err := h.provisioningSvc.ProvisionGame(req.GameID, req.Mode)
	if err != nil {
		h.logger.Errorw("Provisioning failed", "game_id", req.GameID, "error", err)
		response.SendAppError(c, err)
		return
	}

	response.SendSuccess(c, http.StatusAccepted, "Game provisioning started", domain.PlayGameResponse{
		GameID: req.GameID,
		Status:  "provisioning",
	})
}

// DownloadPackage streams the game ZIP directly from MinIO to the requester (e.g. EC2 VM)
func (h *GameHandler) DownloadPackage(c *gin.Context) {
	gameIDStr := c.Param("id")
	gameID, err := strconv.Atoi(gameIDStr)
	if err != nil {
		response.SendAppError(c, domain.ErrValidationFailed)
		return
	}

	game, err := h.gameSvc.GetGame(gameID)
	if err != nil {
		response.SendAppError(c, err)
		return
	}

	// Parsing bucket and key from ARN: arn:aws:s3:::bucket/key
	arnParts := strings.Split(game.ARN, ":::")
	if len(arnParts) < 2 {
		response.SendAppError(c, domain.ErrStorageError)
		return
	}
	pathParts := strings.SplitN(arnParts[1], "/", 2)
	if len(pathParts) < 2 {
		response.SendAppError(c, domain.ErrStorageError)
		return
	}
	bucket, key := pathParts[0], pathParts[1]

	h.logger.Infow("Internal download request", "game_id", gameID, "bucket", bucket, "key", key)

	// Get Object Info for Content-Length
	info, err := h.storage.GetObjectInfo(c.Request.Context(), bucket, key)
	if err != nil {
		response.SendAppError(c, domain.ErrStorageError)
		return
	}

	// Get Stream
	stream, err := h.storage.GetObjectStream(c.Request.Context(), bucket, key)
	if err != nil {
		response.SendAppError(c, domain.ErrStorageError)
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
		h.logger.Errorw("Stream error", "game_id", gameID, "error", err)
	}
}
