package response

import (
	"errors"
	"net/http"

	"backend/internal/domain"

	"github.com/gin-gonic/gin"
)

// JSONResponse defines the standard response structure for all HTTP requests.
type JSONResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SendSuccess sends a standardized success response.
func SendSuccess(c *gin.Context, code int, message string, data interface{}) {
	if message == "" {
		message = "success"
	}
	c.JSON(code, JSONResponse{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// SendError sends a standardized error response.
func SendError(c *gin.Context, code int, message string) {
	c.JSON(code, JSONResponse{
		Code:    code,
		Message: message,
	})
}

// SendAppError maps a domain error or AppError to a standardized response.
func SendAppError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		SendError(c, appErr.Code, appErr.Message)
		return
	}

	// Default mapping for core domain errors
	code := http.StatusInternalServerError
	msg := "internal server error"

	switch {
	case errors.Is(err, domain.ErrGameNotFound):
		code = http.StatusNotFound
		msg = domain.ErrGameNotFound.Error()
	case errors.Is(err, domain.ErrInactiveGame):
		code = http.StatusForbidden
		msg = domain.ErrInactiveGame.Error()
	case errors.Is(err, domain.ErrInvalidCredentials):
		code = http.StatusUnauthorized
		msg = domain.ErrInvalidCredentials.Error()
	case errors.Is(err, domain.ErrUnauthorized):
		code = http.StatusUnauthorized
		msg = domain.ErrUnauthorized.Error()
	case errors.Is(err, domain.ErrAlreadyExists):
		code = http.StatusConflict
		msg = domain.ErrAlreadyExists.Error()
	case errors.Is(err, domain.ErrValidationFailed):
		code = http.StatusBadRequest
		msg = domain.ErrValidationFailed.Error()
	case errors.Is(err, domain.ErrProvisioningFailed):
		code = http.StatusServiceUnavailable
		msg = domain.ErrProvisioningFailed.Error()
	}

	SendError(c, code, msg)
}
