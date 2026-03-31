package response

import (
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
