package handlers

import (
	"net/http"

	"backend/internal/domain"
	"backend/internal/infrastructure/repository"
	"backend/internal/transport/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuthHandler struct {
	authSvc repository.AuthService
	logger  *zap.SugaredLogger
}

func NewAuthHandler(authSvc repository.AuthService, logger *zap.SugaredLogger) *AuthHandler {
	return &AuthHandler{
		authSvc: authSvc,
		logger:  logger,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" {
		response.SendAppError(c, domain.ErrValidationFailed)
		return
	}

	h.logger.Infow("Login request received", "username", req.Username)

	token, err := h.authSvc.GenerateToken(req.Username)
	if err != nil {
		h.logger.Errorw("Token generation failed", "error", err)
		response.SendAppError(c, err)
		return
	}

	response.SendSuccess(c, http.StatusOK, "Login successful", domain.LoginResponse{Token: token})
}
