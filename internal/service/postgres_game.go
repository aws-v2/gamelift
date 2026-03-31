package service

import (
	"fmt"

	"backend/internal/domain"
	"backend/pkg/database"

)

type PostgresGameRepository struct {
	db *database.DB
}

func NewPostgresGameRepository(db *database.DB) *PostgresGameRepository {
	return &PostgresGameRepository{db: db}
}

func (r *PostgresGameRepository) ListGames() ([]domain.Game, error) {
	var games []domain.Game
	// List games that are either running (active) or ready to be provisioned (stored)
	err := r.db.GORM.Where("status IN ?", []domain.GameStatus{domain.GameStatusActive, domain.GameStatusStored}).Find(&games).Error
	return games, err
}

func (r *PostgresGameRepository) GetGame(id int) (*domain.Game, error) {
	var game domain.Game
	if err := r.db.GORM.First(&game, id).Error; err != nil {
		return nil, fmt.Errorf("game %d not found: %w", id, err)
	}
	return &game, nil
}

func (r *PostgresGameRepository) RegisterGame(name, vmID, accountID string) (*domain.Game, error) {
	// Note: We don't have an ID yet, GORM will generate it
	arn := "pending" // Will update after save
	g := domain.Game{
		Name:   name,
		VMID:   vmID,
		ARN:    arn,
		Status: domain.GameStatusActive,
	}

	if err := r.db.GORM.Create(&g).Error; err != nil {
		return nil, err
	}

	// Update ARN with the real ID
	g.ARN = fmt.Sprintf("arn:serw:game:eu-north-1:%s:game/%d", accountID, g.ID)
	g.FolderLocation = fmt.Sprintf("./uploads/games/%d", g.ID)
	
	if err := r.db.GORM.Save(&g).Error; err != nil {
		return nil, err
	}

	return &g, nil
}

func (r *PostgresGameRepository) InitUpload(name, vmID, accountID string) (*domain.Game, error) {
	g := domain.Game{
		Name:   name,
		VMID:   vmID,
		Status: domain.GameStatusPending,
	}

	if err := r.db.GORM.Create(&g).Error; err != nil {
		return nil, err
	}

	// Generate the official ARN
	g.ARN = fmt.Sprintf("arn:serw:game:eu-north-1:%s:game/%d", accountID, g.ID)
	if err := r.db.GORM.Save(&g).Error; err != nil {
		return nil, err
	}

	return &g, nil
}

func (r *PostgresGameRepository) UpdateGameStatus(id int, status domain.GameStatus, storageARN string) error {
	return r.db.GORM.Model(&domain.Game{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      status,
		"storage_arn": storageARN,
	}).Error
}
func (r *PostgresGameRepository) UpdateGameManifest(id int, manifest string) error {
	return r.db.GORM.Model(&domain.Game{}).Where("id = ?", id).Update("manifest", manifest).Error
}
