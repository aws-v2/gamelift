package interfaces

import "backend/internal/domain"

// GameRepository defines the contract for game data access.
// Implement with in-memory store now, swap to Postgres later.
type GameRepository interface {
	ListGames() ([]domain.Game, error)
	GetGame(id int) (*domain.Game, error)
	RegisterGame(name, vmID, accountID string) (*domain.Game, error)
	InitUpload(name, vmID, accountID string) (*domain.Game, error)
	UpdateGameStatus(id int, status domain.GameStatus, storageARN string) error
	UpdateGameManifest(id int, manifest string) error
}

// ProvisioningService defines the contract for on-demand game startup.
type ProvisioningService interface {
	ProvisionGame(gameID int) error
}

// AuthService defines the contract for authentication operations.
type AuthService interface {
	GenerateToken(username string) (string, error)
	ValidateToken(tokenStr string) (map[string]interface{}, error)
}
