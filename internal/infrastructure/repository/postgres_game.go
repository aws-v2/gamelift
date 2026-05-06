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
	ListGames(ctx context.Context) ([]domain.Game, error)
	GetGame(ctx context.Context, id string) (*domain.Game, error)
	CreateGame(ctx context.Context, game *domain.Game) error
	UpdateGame(ctx context.Context, game *domain.Game) error
	DeleteGame(ctx context.Context, id string) error
	GetGameByName(ctx context.Context, name string) (*domain.Game, error)
	UpdateGameStatus(ctx context.Context, id string, status domain.GameStatus) error
	GetManifest(ctx context.Context, id string) (map[string]any, error)
	SetManifest(ctx context.Context, id string, manifest map[string]any) error
	GetGameByVMID(ctx context.Context, vmid string) (*domain.Game, error)
	UpdateStatusByVMID(ctx context.Context, vmid string, status domain.GameStatus) error
	CreateSession(ctx context.Context, session *domain.GameSession) error
	GetSession(ctx context.Context, gameID string) (*domain.GameSession, error)
	UpdateSessionStatus(ctx context.Context, sessionID string, status string) error
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

func (r *postgresSessionRepository) Create(ctx context.Context, session *domain.GameSession) error {
	return r.db.GORM.WithContext(ctx).Create(session).Error
}

func (r *postgresSessionRepository) GetActiveSession(ctx context.Context, gameID, userID string) (*domain.GameSession, error) {
	var session domain.GameSession
	if err := r.db.GORM.WithContext(ctx).Where("game_id = ? AND user_id = ? AND status = ?", gameID, userID, "active").First(&session).Error; err != nil {
		return nil, fmt.Errorf("session for game %d and user %s not found: %w", gameID, userID, err)
	}
	return &session, nil
}

func (r *postgresSessionRepository) MarkReady(ctx context.Context, sessionID, agentWSURL, nodeID string) error {
	return r.db.GORM.WithContext(ctx).Model(&domain.GameSession{}).Where("id = ?", sessionID).Updates(map[string]any{"agent_ws_url": agentWSURL, "node_id": nodeID, "status": "ready"}).Error
}

func (r *postgresSessionRepository) CloseSession(ctx context.Context, sessionID string) error {
	return r.db.GORM.WithContext(ctx).Model(&domain.GameSession{}).Where("id = ?", sessionID).Update("status", "closed").Error
}
func (r *postgresSessionRepository) UpdateStatus(ctx context.Context, sessionID, status string) error {
	err := r.db.GORM.WithContext(ctx).Model(&domain.GameSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]any{"status": status}).Error
	if err != nil {
		r.log.Errorf("UpdateStatus failed: sessionID=%s status=%s err=%v", sessionID, status, err)
	}
	return err
}
func NewPostgresGameRepository(db *database.DB, log *zap.SugaredLogger) GameRepository {
	return &postgresGameRepository{db: db, log: log}
}

func (r *postgresGameRepository) GetGameByName(ctx context.Context, name string) (*domain.Game, error) {
	var game domain.Game
	if err := r.db.GORM.WithContext(ctx).Where("name = ?", name).First(&game).Error; err != nil {
		return nil, fmt.Errorf("game %s not found: %w", name, err)
	}
	return &game, nil
}

func (r *postgresGameRepository) UpdateStatusByVMID(ctx context.Context, vmid string, status domain.GameStatus) error {
	return r.db.GORM.WithContext(ctx).Model(&domain.Game{}).Where("vmid = ?", vmid).Update("status", status).Error
}

// ── Implementation ────────────────────────────────────────────────────────────

func (r *postgresGameRepository) ListGames(ctx context.Context) ([]domain.Game, error) {
	var games []domain.Game
	if err := r.db.GORM.WithContext(ctx).Find(&games).Error; err != nil {
		r.log.Errorw("ListGames failed", "error", err)
		return nil, err
	}
	return games, nil
}
func (r *postgresGameRepository) GetGame(ctx context.Context, id string) (*domain.Game, error) {
	var game domain.Game
	if err := r.db.GORM.WithContext(ctx).First(&game, id).Error; err != nil {
		return nil, fmt.Errorf("game %d not found: %w", id, err)
	}
	return &game, nil
}
func (r *postgresGameRepository) UpdateGameStatus(ctx context.Context, id string, status domain.GameStatus) error {
	return r.db.GORM.WithContext(ctx).Model(&domain.Game{}).Where("id = ?", id).Update("status", status).Error
}
func (r *postgresGameRepository) GetGameByVMID(ctx context.Context, vmid string) (*domain.Game, error) {
	var game domain.Game
	if err := r.db.GORM.WithContext(ctx).First(&game, vmid).Error; err != nil {
		return nil, fmt.Errorf("game %d not found: %w", vmid, err)
	}
	return &game, nil
}

func (r *postgresGameRepository) CreateGame(ctx context.Context, game *domain.Game) error {
	return r.db.GORM.WithContext(ctx).Create(game).Error
}

func (r *postgresGameRepository) UpdateGame(ctx context.Context, game *domain.Game) error {
	return r.db.GORM.WithContext(ctx).Save(game).Error
}

func (r *postgresGameRepository) DeleteGame(ctx context.Context, id string) error {
	return r.db.GORM.WithContext(ctx).Delete(&domain.Game{}, id).Error
}

func (r *postgresGameRepository) GetManifest(ctx context.Context, id string) (map[string]any, error) {
	var game domain.Game
	if err := r.db.GORM.WithContext(ctx).Select("id, name, arn, status").First(&game, id).Error; err != nil {
		return nil, fmt.Errorf("game %d not found: %w", id, err)
	}
	return map[string]any{
		"id":     game.ID,
		"name":   game.Name,
		"arn":    game.ARN,
		"status": game.Status,
	}, nil
}

func (r *postgresGameRepository) SetManifest(ctx context.Context, id string, manifest map[string]any) error {
	return r.db.GORM.WithContext(ctx).Model(&domain.Game{}).Where("id = ?", id).Updates(manifest).Error
}
func (r *postgresGameRepository) CreateSession(ctx context.Context, session *domain.GameSession) error {
	return r.db.GORM.WithContext(ctx).Create(session).Error
}
func (r *postgresGameRepository) GetSession(ctx context.Context, gameID string) (*domain.GameSession, error) {
	var session domain.GameSession
	if err := r.db.GORM.WithContext(ctx).Where("game_id = ?", gameID).Last(&session).Error; err != nil {
		return nil, fmt.Errorf("session for game %d not found: %w", gameID, err)
	}
	return &session, nil
}

func (r *postgresGameRepository) UpdateSessionStatus(ctx context.Context, sessionID string, status string) error {
	return r.db.GORM.WithContext(ctx).
		Model(&domain.Session{}).
		Where("id = ?", sessionID).
		Update("status", status).Error
}