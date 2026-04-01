package domain

import (
	"errors"
	"fmt"
)

// Typed Errors for Domain Logic
var (
	ErrGameNotFound          = errors.New("game not found")
	ErrInactiveGame          = errors.New("game is not in a playable state")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrUnauthorized          = errors.New("unauthorized access")
	ErrInternal              = errors.New("internal server error")
	ErrProvisioningFailed    = errors.New("failed to provision game instance")
	ErrValidationFailed      = errors.New("game validation failed")
	ErrStorageError          = errors.New("storage operation failed")
	ErrAlreadyExists         = errors.New("resource already exists")
)

// AppError is a custom error type that includes a status code and a message
type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func NewAppError(code int, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}
