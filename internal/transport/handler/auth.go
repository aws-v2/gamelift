package handler

import (
	"log"
	"net/http"

	"backend/internal/domain"
	"backend/internal/interfaces"
	"backend/internal/transport/response"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authSvc interfaces.AuthService
}

func NewAuthHandler(authSvc interfaces.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" {
		response.SendError(c, http.StatusBadRequest, "invalid request")
		return
	}

	log.Printf("[AUTH] Login request from user: %s", req.Username)

	token, err := h.authSvc.GenerateToken(req.Username)
	if err != nil {
		log.Printf("[AUTH] Token generation failed: %v", err)
		response.SendError(c, http.StatusInternalServerError, "internal error")
		return
	}

	response.SendSuccess(c, http.StatusOK, "Login successful", domain.LoginResponse{Token: token})
}
