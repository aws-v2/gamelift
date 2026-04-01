package logger

import "go.uber.org/zap"

// New creates a new production Zap logger
func New() *zap.Logger {
	logger, _ := zap.NewProduction()
	return logger
}

// NewSugared creates a new production Zap sugared logger
func NewSugared() *zap.SugaredLogger {
	logger, _ := zap.NewProduction()
	return logger.Sugar()
}
