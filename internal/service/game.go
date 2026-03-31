package service

import (
	"fmt"
	"sync"

	"backend/internal/domain"
)

type GameServiceImpl struct {
	games []domain.Game
	mu    sync.Mutex
	nextID int
}

func NewGameService() *GameServiceImpl {
	return &GameServiceImpl{
		games: []domain.Game{
			{
				ID:             1,
				Name:           "Demo Game",
				FolderLocation: "media",
				VMID:           "test-vm-1",
				ARN:            "arn:serw:game:eu-north-1:system:game/1",
				Status:         domain.GameStatusActive,
			},
		},
		nextID: 2,
	}
}

func (s *GameServiceImpl) ListGames() ([]domain.Game, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var activeGames []domain.Game
	for _, g := range s.games {
		if g.Status == domain.GameStatusActive {
			activeGames = append(activeGames, g)
		}
	}
	return activeGames, nil
}

func (s *GameServiceImpl) GetGame(id int) (*domain.Game, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, g := range s.games {
		if g.ID == id {
			return &g, nil
		}
	}
	return nil, fmt.Errorf("game %d not found", id)
}

func (s *GameServiceImpl) RegisterGame(name, vmID, accountID string) (*domain.Game, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextID
	s.nextID++

	arn := fmt.Sprintf("arn:serw:game:eu-north-1:%s:game/%d", accountID, id)
	folderLocation := fmt.Sprintf("./uploads/games/%d", id)

	g := domain.Game{
		ID:             id,
		Name:           name,
		FolderLocation: folderLocation,
		VMID:           vmID,
		ARN:            arn,
		Status:         domain.GameStatusActive,
	}

	s.games = append(s.games, g)
	return &g, nil
}

func (s *GameServiceImpl) InitUpload(name, vmID, accountID string) (*domain.Game, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextID
	s.nextID++

	arn := fmt.Sprintf("arn:serw:game:eu-north-1:%s:game/%d", accountID, id)

	g := domain.Game{
		ID:     id,
		Name:   name,
		VMID:   vmID,
		ARN:    arn,
		Status: domain.GameStatusPending,
	}

	s.games = append(s.games, g)
	return &g, nil
}

func (s *GameServiceImpl) UpdateGameStatus(id int, status domain.GameStatus, storageARN string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, g := range s.games {
		if g.ID == id {
			s.games[i].Status = status
			s.games[i].StorageARN = storageARN
			return nil
		}
	}
	return fmt.Errorf("game %d not found", id)
}
