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

func NewSessionService(sessionRepo repository.SessionRepository, provisioningSvc *ProvisioningService, logger *zap.SugaredLogger, debug bool, natsClient *messaging.NatsClient) *Service {
	return &Service{repo: sessionRepo, provisioningSvc: provisioningSvc, logger: logger, debug: debug, natsClient: natsClient}
}

const (
	StatusProvisioning = "provisioning"
	StatusReady        = "ready"
	StatusClosed       = "closed"
	StatusFailed       = "failed"
)

func (s *Service) CreateSession(ctx context.Context, req domain.CreateSessionRequest) (*domain.GameSession, error) {
	s.logger.Infow("the  gameid is : %s", req.GameID)
	s.logger.Infow("the  userid is : %s", req.UserID)
	existing, err := s.repo.GetActiveSession(ctx, req.GameID, req.UserID)
	if err == nil && existing != nil {
		s.logger.Infow("reusing existing session", "session_id", existing.ID, "status", existing.Status)
		return existing, nil
	}

	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	s.logger.Infow("the token is : ")
	s.logger.Infow(token)
	s.logger.Infow("the token is : ")

	session := &domain.GameSession{
		ID:        generateID(),
		GameID:    req.GameID,
		UserID:    req.UserID,
		Status:    StatusProvisioning,
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(2 * time.Hour),
	}

	s.logger.Infow("the session is : %s", session)

	if err := s.repo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("persist session: %w", err)
	}
	s.logger.Infow("the session is : %s", session.Token)
	// TODO: remove once real provisioning flow is wired up

	if s.debug {
	s.logger.Infow("------------------s------")
		
		session.Status = StatusReady
		session.AgentWSURL = "ws://localhost:9030/game"
		if err := s.repo.MarkReady(ctx, session.ID, session.AgentWSURL, "local"); err != nil {
			s.logger.Warnw("failed to persist debug ready status", "session_id", session.ID, "error", err)
		}
		s.logger.Infow("debug mode: session marked ready with local agent", "session_id", session.ID)
		return session, nil
	}else{
	s.logger.Infow("----------------f--------")

		gameIDInt :=session.GameID

		if err != nil {
	s.logger.Infow("-----------d-----f--------", "error", err)

			_ = s.repo.UpdateStatus(ctx, session.ID, StatusFailed)
			return nil, fmt.Errorf("invalid game_id: %w", err)
		}
	s.logger.Infow("------------k----f--------")

		s.provisioningSvc.ProvisionGame(gameIDInt, domain.StreamingModeState)
		// s.natsClient.Request(messaging.GetProvisionGameSubject(), []byte(session.ID), 10*time.Second)
	}

	gameIDInt := req.GameID
	if err != nil {
		_ = s.repo.UpdateStatus(ctx, session.ID, StatusFailed)
		return nil, fmt.Errorf("invalid game_id: %w", err)
	}

	if err := s.provisioningSvc.ProvisionGame(gameIDInt, domain.StreamingModeState); err != nil {
		_ = s.repo.UpdateStatus(ctx, session.ID, StatusFailed)
		return nil, fmt.Errorf("provision game: %w", err)
	}

	s.logger.Infow("session created, vm provisioning started",
		"session_id", session.ID,
		"game_id", req.GameID,
	)

	return session, nil
}

func (s *Service) GetActiveSession(ctx context.Context, gameID, userID string) (*domain.GameSession, error) {
	session, err := s.repo.GetActiveSession(ctx, gameID, userID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return session, nil
}

func (s *Service) MarkReady(ctx context.Context, sessionID, agentWSURL, nodeID string) error {
	if err := s.repo.MarkReady(ctx, sessionID, agentWSURL, nodeID); err != nil {
		return fmt.Errorf("mark ready: %w", err)
	}
	s.logger.Infow("session marked ready", "session_id", sessionID, "ws_url", agentWSURL)
	return nil
}

func (s *Service) CloseSession(ctx context.Context, sessionID string) error {
	if err := s.repo.UpdateStatus(ctx, sessionID, StatusClosed); err != nil {
		return fmt.Errorf("close session: %w", err)
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

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
