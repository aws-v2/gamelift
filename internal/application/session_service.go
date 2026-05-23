package application

import (
	"backend/internal/domain"
	"backend/internal/infrastructure/messaging"
	"backend/internal/infrastructure/repository"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type Service struct {
	repo            repository.SessionRepository
	provisioningSvc *ProvisioningService
	logger          *zap.SugaredLogger
	debug           bool
	natsClient      *messaging.NatsClient
}

func NewSessionService(
	sessionRepo repository.SessionRepository,
	provisioningSvc *ProvisioningService,
	logger *zap.SugaredLogger,
	debug bool,
	natsClient *messaging.NatsClient,
) *Service {
	return &Service{
		repo:            sessionRepo,
		provisioningSvc: provisioningSvc,
		logger:          logger,
		debug:           debug,
		natsClient:      natsClient,
	}
}

const (
	StatusProvisioning = "provisioning"
	StatusReady        = "ready"
	StatusClosed       = "closed"
	StatusFailed       = "failed"
)

func (s *Service) CreateSession(ctx context.Context, req domain.CreateSessionRequest) (*domain.GameSession, error) {
	s.logger.Infow("SESSION_CREATE", "game_id", req.GameID, "user_id", req.UserID)

	// Reuse an existing active session if one exists
	existing, err := s.repo.GetActiveSession(ctx, req.GameID, req.UserID)
	if err == nil && existing != nil {
		s.logger.Infow("SESSION_CREATE_REUSING_EXISTING",
			"session_id", existing.ID,
			"game_id", req.GameID,
			"user_id", req.UserID,
			"status", existing.Status,
		)
		return existing, nil
	}

	token, err := generateToken()
	if err != nil {
		s.logger.Errorw("SESSION_CREATE_TOKEN_FAILED", "game_id", req.GameID, "user_id", req.UserID, "error", err)
		return nil, fmt.Errorf("generate token: %w", err)
	}

	session := &domain.GameSession{
		ID:        generateID(),
		GameID:    req.GameID,
		UserID:    req.UserID,
		Status:    StatusProvisioning,
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(2 * time.Hour),
	}

	s.logger.Infow("SESSION_CREATE_PERSISTING",
		"session_id", session.ID,
		"game_id", session.GameID,
		"user_id", session.UserID,
		"expires_at", session.ExpiresAt,
	)

	if err := s.repo.Create(ctx, session); err != nil {
		s.logger.Errorw("SESSION_CREATE_PERSIST_FAILED",
			"session_id", session.ID,
			"game_id", session.GameID,
			"user_id", session.UserID,
			"error", err,
		)
		return nil, fmt.Errorf("persist session: %w", err)
	}

	s.logger.Infow("SESSION_CREATE_PERSISTED", "session_id", session.ID, "game_id", session.GameID)

	// if s.debug {
	// 	s.logger.Warnw("SESSION_CREATE_DEBUG_MODE",
	// 		"session_id", session.ID,
	// 		"note", "provisioning skipped in debug mode",
	// 	)
	// 	return session, nil
	// }

	s.logger.Infow("SESSION_CREATE_PROVISIONING",
		"session_id", session.ID,
		"game_id", session.GameID,
	)

	if err := s.provisioningSvc.ProvisionGame(session.GameID, domain.StreamingModeState, session.ID,req.AssetURL,req.Sha256); err != nil {
		s.logger.Errorw("SESSION_CREATE_PROVISION_FAILED",
			"session_id", session.ID,
			"game_id", session.GameID,
			"error", err,
		)
		_ = s.repo.UpdateStatus(ctx, session.ID, StatusFailed)
		return nil, fmt.Errorf("provision game: %w", err)
	}

	s.logger.Infow("SESSION_CREATE_SUCCESS",
		"session_id", session.ID,
		"game_id", session.GameID,
		"user_id", session.UserID,
		"status", session.Status,
	)

	return session, nil
}

func (s *Service) GetActiveSession(ctx context.Context, gameID, userID string) (*domain.GameSession, error) {
	s.logger.Infow("SESSION_GET_ACTIVE", "game_id", gameID, "user_id", userID)

	session, err := s.repo.GetActiveSession(ctx, gameID, userID)
	if err != nil {
		s.logger.Warnw("SESSION_GET_ACTIVE_NOT_FOUND", "game_id", gameID, "user_id", userID, "error", err)
		return nil, fmt.Errorf("get session: %w", err)
	}

	s.logger.Infow("SESSION_GET_ACTIVE_FOUND", "session_id", session.ID, "game_id", gameID, "status", session.Status)
	return session, nil
}

func (s *Service) MarkReady(ctx context.Context, sessionID, agentWSURL, nodeID string) error {
	s.logger.Infow("SESSION_MARK_READY", "session_id", sessionID, "node_id", nodeID, "agent_ws_url", agentWSURL)

	if err := s.repo.MarkReady(ctx, sessionID, agentWSURL, nodeID); err != nil {
		s.logger.Errorw("SESSION_MARK_READY_FAILED",
			"session_id", sessionID,
			"node_id", nodeID,
			"agent_ws_url", agentWSURL,
			"error", err,
		)
		return fmt.Errorf("mark ready: %w", err)
	}

	s.logger.Infow("SESSION_MARK_READY_SUCCESS", "session_id", sessionID, "node_id", nodeID, "agent_ws_url", agentWSURL)
	return nil
}

func (s *Service) CloseSession(ctx context.Context, sessionID string) error {
	s.logger.Infow("SESSION_CLOSE", "session_id", sessionID)

	if err := s.repo.UpdateStatus(ctx, sessionID, StatusClosed); err != nil {
		s.logger.Errorw("SESSION_CLOSE_FAILED", "session_id", sessionID, "error", err)
		return fmt.Errorf("close session: %w", err)
	}

	s.logger.Infow("SESSION_CLOSE_SUCCESS", "session_id", sessionID)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}