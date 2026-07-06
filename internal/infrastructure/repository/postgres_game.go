package repository

import (
	"context"
	"fmt"

	"backend/internal/domain"
	"backend/pkg/database"

	"go.uber.org/zap"
)

// ── Interface ─────────────────────────────────────────────────────────────────

type GameRepository interface {
	ListGames(ctx context.Context, log *zap.SugaredLogger) ([]domain.Game, error)
	GetGame(ctx context.Context, id string, log *zap.SugaredLogger) (*domain.Game, error)
	CreateGame(ctx context.Context, game *domain.Game, log *zap.SugaredLogger) error
	UpdateGame(ctx context.Context, game *domain.Game, log *zap.SugaredLogger) error
	DeleteGame(ctx context.Context, id string, log *zap.SugaredLogger) error
	GetGameByName(ctx context.Context, name string, log *zap.SugaredLogger) (*domain.Game, error)
	UpdateGameStatus(ctx context.Context, id string, status domain.GameStatus, log *zap.SugaredLogger) error
	GetManifest(ctx context.Context, id string, log *zap.SugaredLogger) (map[string]any, error)
	SetManifest(ctx context.Context, id string, manifest domain.GameManifest, log *zap.SugaredLogger) error
	GetGameByVMID(ctx context.Context, vmid string, log *zap.SugaredLogger) (*domain.Game, error)
	UpdateStatusByVMID(ctx context.Context, vmid string, status domain.GameStatus, log *zap.SugaredLogger) error
	CreateSession(ctx context.Context, session *domain.GameSession, log *zap.SugaredLogger) error
	GetSession(ctx context.Context, gameID string, log *zap.SugaredLogger) (*domain.GameSession, error)
	UpdateSessionStatus(ctx context.Context, sessionID string, status string, log *zap.SugaredLogger) error
}

type SessionRepository interface {
	Create(ctx context.Context, session *domain.GameSession) error
	GetActiveSession(ctx context.Context, gameID, userID string) (*domain.GameSession, error)
	MarkReady(ctx context.Context, sessionID, agentWSURL, nodeID string) error
	CloseSession(ctx context.Context, sessionID string) error
	UpdateStatus(ctx context.Context, sessionID, status string) error
}

// ── Struct + constructor ──────────────────────────────────────────────────────

type postgresGameRepository struct {
	db  *database.DB
	log *zap.SugaredLogger
}

type postgresSessionRepository struct {
	db  *database.DB
	log *zap.SugaredLogger
}

func NewPostgresSessionRepository(db *database.DB, log *zap.SugaredLogger) SessionRepository {
	return &postgresSessionRepository{db: db, log: log}
}

// ── SessionRepository Implementation ─────────────────────────────────────────

func (r *postgresSessionRepository) Create(ctx context.Context, session *domain.GameSession) error {
	r.log.Infow("SESSION_CREATE", "session_id", session.ID, "game_id", session.GameID, "user_id", session.UserID)

	if err := r.db.GORM.WithContext(ctx).Create(session).Error; err != nil {
		r.log.Errorw("SESSION_CREATE_FAILED", "session_id", session.ID, "game_id", session.GameID, "user_id", session.UserID, "error", err)
		return err
	}

	r.log.Infow("SESSION_CREATE_SUCCESS", "session_id", session.ID, "game_id", session.GameID, "user_id", session.UserID)
	return nil
}

func (r *postgresSessionRepository) GetActiveSession(ctx context.Context, gameID, userID string) (*domain.GameSession, error) {
	r.log.Infow("SESSION_GET_ACTIVE", "game_id", gameID, "user_id", userID)

	var session domain.GameSession
	if err := r.db.GORM.WithContext(ctx).Where("game_id = ? AND user_id = ? AND status = ?", gameID, userID, "active").First(&session).Error; err != nil {
		r.log.Warnw("SESSION_GET_ACTIVE_NOT_FOUND", "game_id", gameID, "user_id", userID, "error", err)
		return nil, fmt.Errorf("session for game %s and user %s not found: %w", gameID, userID, err)
	}

	r.log.Infow("SESSION_GET_ACTIVE_FOUND", "session_id", session.ID, "game_id", gameID, "user_id", userID, "status", session.Status)
	return &session, nil
}

func (r *postgresSessionRepository) MarkReady(ctx context.Context, sessionID, agentWSURL, nodeID string) error {
	r.log.Infow("SESSION_MARK_READY", "session_id", sessionID, "node_id", nodeID, "agent_ws_url", agentWSURL)

	if err := r.db.GORM.WithContext(ctx).Model(&domain.GameSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]any{"agent_ws_url": agentWSURL, "node_id": nodeID, "status": "ready"}).Error; err != nil {
		r.log.Errorw("SESSION_MARK_READY_FAILED", "session_id", sessionID, "node_id", nodeID, "error", err)
		return err
	}

	r.log.Infow("SESSION_MARK_READY_SUCCESS", "session_id", sessionID, "node_id", nodeID)
	return nil
}

func (r *postgresSessionRepository) CloseSession(ctx context.Context, sessionID string) error {
	r.log.Infow("SESSION_CLOSE", "session_id", sessionID)

	if err := r.db.GORM.WithContext(ctx).Model(&domain.GameSession{}).
		Where("id = ?", sessionID).
		Update("status", "closed").Error; err != nil {
		r.log.Errorw("SESSION_CLOSE_FAILED", "session_id", sessionID, "error", err)
		return err
	}

	r.log.Infow("SESSION_CLOSE_SUCCESS", "session_id", sessionID)
	return nil
}

func (r *postgresSessionRepository) UpdateStatus(ctx context.Context, sessionID, status string) error {
	r.log.Infow("SESSION_UPDATE_STATUS", "session_id", sessionID, "new_status", status)

	if err := r.db.GORM.WithContext(ctx).Model(&domain.GameSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]any{"status": status}).Error; err != nil {
		r.log.Errorw("SESSION_UPDATE_STATUS_FAILED", "session_id", sessionID, "new_status", status, "error", err)
		return err
	}

	r.log.Infow("SESSION_UPDATE_STATUS_SUCCESS", "session_id", sessionID, "new_status", status)
	return nil
}

func NewPostgresGameRepository(db *database.DB, log *zap.SugaredLogger) GameRepository {
	return &postgresGameRepository{db: db, log: log}
}

// ── GameRepository Implementation ─────────────────────────────────────────────

func (r *postgresGameRepository) ListGames(ctx context.Context, log *zap.SugaredLogger) ([]domain.Game, error) {
	log.Infow("GAME_LIST")

	var games []domain.Game
	if err := r.db.GORM.WithContext(ctx).Find(&games).Error; err != nil {
		log.Errorw("GAME_LIST_FAILED", "error", err)
		return nil, err
	}

	log.Infow("GAME_LIST_SUCCESS", "count", len(games))
	return games, nil
}

func (r *postgresGameRepository) GetGame(ctx context.Context, id string, log *zap.SugaredLogger) (*domain.Game, error) {
	log.Infow("GAME_GET", "game_id", id)

	var game domain.Game
	if err := r.db.GORM.WithContext(ctx).First(&game, "id = ?", id).Error; err != nil {
		log.Warnw("GAME_GET_NOT_FOUND", "game_id", id, "error", err)
		return nil, fmt.Errorf("game %s not found: %w", id, err)
	}

	log.Infow("GAME_GET_FOUND", "game_id", id, "name", game.Name, "status", game.Status)
	return &game, nil
}

func (r *postgresGameRepository) GetGameByName(ctx context.Context, name string, log *zap.SugaredLogger) (*domain.Game, error) {
	log.Infow("GAME_GET_BY_NAME", "name", name)

	var game domain.Game
	if err := r.db.GORM.WithContext(ctx).Where("name = ?", name).First(&game).Error; err != nil {
		log.Warnw("GAME_GET_BY_NAME_NOT_FOUND", "name", name, "error", err)
		return nil, fmt.Errorf("game %s not found: %w", name, err)
	}

	log.Infow("GAME_GET_BY_NAME_FOUND", "game_id", game.ID, "name", name, "status", game.Status)
	return &game, nil
}

func (r *postgresGameRepository) GetGameByVMID(ctx context.Context, vmid string, log *zap.SugaredLogger) (*domain.Game, error) {
	log.Infow("GAME_GET_BY_VMID", "vmid", vmid)

	var game domain.Game
	if err := r.db.GORM.WithContext(ctx).First(&game, "vmid = ?", vmid).Error; err != nil {
		log.Warnw("GAME_GET_BY_VMID_NOT_FOUND", "vmid", vmid, "error", err)
		return nil, fmt.Errorf("game with vmid %s not found: %w", vmid, err)
	}

	log.Infow("GAME_GET_BY_VMID_FOUND", "game_id", game.ID, "vmid", vmid, "status", game.Status)
	return &game, nil
}

func (r *postgresGameRepository) CreateGame(ctx context.Context, game *domain.Game, log *zap.SugaredLogger) error {
	log.Infow("GAME_CREATE", "game_id", game.ID, "name", game.Name, "status", game.Status)

	if err := r.db.GORM.WithContext(ctx).Create(game).Error; err != nil {
		log.Errorw("GAME_CREATE_FAILED", "game_id", game.ID, "name", game.Name, "error", err)
		return err
	}

	log.Infow("GAME_CREATE_SUCCESS", "game_id", game.ID, "name", game.Name)
	return nil
}

func (r *postgresGameRepository) UpdateGame(ctx context.Context, game *domain.Game, log *zap.SugaredLogger) error {
	log.Infow("GAME_UPDATE", "game_id", game.ID, "name", game.Name, "status", game.Status)

	if err := r.db.GORM.WithContext(ctx).Save(game).Error; err != nil {
		log.Errorw("GAME_UPDATE_FAILED", "game_id", game.ID, "error", err)
		return err
	}

	log.Infow("GAME_UPDATE_SUCCESS", "game_id", game.ID, "name", game.Name, "status", game.Status)
	return nil
}

func (r *postgresGameRepository) DeleteGame(ctx context.Context, id string, log *zap.SugaredLogger) error {
	log.Infow("GAME_DELETE", "game_id", id)

	if err := r.db.GORM.WithContext(ctx).Delete(&domain.Game{}, id).Error; err != nil {
		log.Errorw("GAME_DELETE_FAILED", "game_id", id, "error", err)
		return err
	}

	log.Infow("GAME_DELETE_SUCCESS", "game_id", id)
	return nil
}

func (r *postgresGameRepository) UpdateGameStatus(ctx context.Context, id string, status domain.GameStatus, log *zap.SugaredLogger) error {
	log.Infow("GAME_UPDATE_STATUS", "game_id", id, "new_status", status)

	if err := r.db.GORM.WithContext(ctx).Model(&domain.Game{}).
		Where("id = ?", id).
		Update("status", status).Error; err != nil {
		log.Errorw("GAME_UPDATE_STATUS_FAILED", "game_id", id, "new_status", status, "error", err)
		return err
	}

	log.Infow("GAME_UPDATE_STATUS_SUCCESS", "game_id", id, "new_status", status)
	return nil
}

func (r *postgresGameRepository) UpdateStatusByVMID(ctx context.Context, vmid string, status domain.GameStatus, log *zap.SugaredLogger) error {
	log.Infow("GAME_UPDATE_STATUS_BY_VMID", "vmid", vmid, "new_status", status)

	if err := r.db.GORM.WithContext(ctx).Model(&domain.Game{}).
		Where("vmid = ?", vmid).
		Update("status", status).Error; err != nil {
		log.Errorw("GAME_UPDATE_STATUS_BY_VMID_FAILED", "vmid", vmid, "new_status", status, "error", err)
		return err
	}

	log.Infow("GAME_UPDATE_STATUS_BY_VMID_SUCCESS", "vmid", vmid, "new_status", status)
	return nil
}

func (r *postgresGameRepository) GetManifest(ctx context.Context, id string, log *zap.SugaredLogger) (map[string]any, error) {
	log.Infow("GAME_GET_MANIFEST", "game_id", id)

	var game domain.Game
	if err := r.db.GORM.WithContext(ctx).Select("id, name, arn, status").First(&game, "id = ?", id).Error; err != nil {
		log.Warnw("GAME_GET_MANIFEST_NOT_FOUND", "game_id", id, "error", err)
		return nil, fmt.Errorf("game %s not found: %w", id, err)
	}

	log.Infow("GAME_GET_MANIFEST_SUCCESS", "game_id", id, "name", game.Name, "arn", game.ARN)
	return map[string]any{
		"id":     game.ID,
		"name":   game.Name,
		"arn":    game.ARN,
		"status": game.Status,
	}, nil
}

func (r *postgresGameRepository) SetManifest(ctx context.Context, gameID string, manifest domain.GameManifest, log *zap.SugaredLogger) error {
	log.Infow("GAME_SET_MANIFEST", "game_id", gameID)

	if err := r.db.GORM.WithContext(ctx).Create(&manifest).Error; err != nil {
		log.Errorw("GAME_SET_MANIFEST_FAILED", "game_id", gameID, "error", err)
		return err
	}

	log.Infow("GAME_SET_MANIFEST_SUCCESS", "game_id", gameID)
	return nil
}

func (r *postgresGameRepository) CreateSession(ctx context.Context, session *domain.GameSession, log *zap.SugaredLogger) error {
	log.Infow("GAME_SESSION_CREATE", "session_id", session.ID, "game_id", session.GameID)

	if err := r.db.GORM.WithContext(ctx).Create(session).Error; err != nil {
		log.Errorw("GAME_SESSION_CREATE_FAILED", "session_id", session.ID, "game_id", session.GameID, "error", err)
		return err
	}

	log.Infow("GAME_SESSION_CREATE_SUCCESS", "session_id", session.ID, "game_id", session.GameID)
	return nil
}

func (r *postgresGameRepository) GetSession(ctx context.Context, gameID string, log *zap.SugaredLogger) (*domain.GameSession, error) {
	log.Infow("GAME_SESSION_GET_LATEST", "game_id", gameID)

	var session domain.GameSession
	if err := r.db.GORM.WithContext(ctx).Where("game_id = ?", gameID).Last(&session).Error; err != nil {
		log.Warnw("GAME_SESSION_GET_LATEST_NOT_FOUND", "game_id", gameID, "error", err)
		return nil, fmt.Errorf("session for game %s not found: %w", gameID, err)
	}

	log.Infow("GAME_SESSION_GET_LATEST_FOUND", "session_id", session.ID, "game_id", gameID, "status", session.Status)
	return &session, nil
}

func (r *postgresGameRepository) UpdateSessionStatus(ctx context.Context, sessionID string, status string, log *zap.SugaredLogger) error {
	log.Infow("GAME_SESSION_UPDATE_STATUS", "session_id", sessionID, "new_status", status)

	if err := r.db.GORM.WithContext(ctx).Model(&domain.GameSession{}).
		Where("id = ?", sessionID).
		Update("status", status).Error; err != nil {
		log.Errorw("GAME_SESSION_UPDATE_STATUS_FAILED", "session_id", sessionID, "new_status", status, "error", err)
		return err
	}

	log.Infow("GAME_SESSION_UPDATE_STATUS_SUCCESS", "session_id", sessionID, "new_status", status)
	return nil
}