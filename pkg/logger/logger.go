package logger

import "go.uber.org/zap"

// New creates a new production Zap logger
func New() *zap.Logger {
	logger, _ := zap.NewProduction()
	return logger
}
