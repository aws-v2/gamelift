package application

import (
	// "backend/internal/application"
	"backend/internal/domain"
	"backend/internal/infrastructure/messaging"
	"backend/internal/infrastructure/repository"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ── Interface ─────────────────────────────────────────────────────────────────

type GameService interface {
	ListGames(ctx context.Context) ([]domain.Game, error)
	GetGame(ctx context.Context, id string) (*domain.Game, error)
	CreateGame(ctx context.Context, req CreateGameRequest) (*domain.Game, error)
	UpdateGame(ctx context.Context, id string, req UpdateGameRequest) (*domain.Game, error)
	DeleteGame(ctx context.Context, id string) error

	GetManifest(ctx context.Context, id string) (map[string]any, error)
	GetDownloadURL(ctx context.Context, id string) (string, error)
	InitUpload(ctx context.Context, req InitUploadRequest) (*InitUploadResult, error)

	PlayGame(ctx context.Context, req PlayGameRequest) (*PlayGameResult, error)
	CreateSession(ctx context.Context, gameID string, req domain.CreateSessionRequest) (*domain.GameSession, error)
	GetSessionStatus(ctx context.Context, gameID string) (*domain.GameSession, error)
}

// ── Request / Response types ──────────────────────────────────────────────────

type CreateGameRequest struct {
	Name           string `json:"name"    binding:"required"`
	FolderLocation string `json:"folder_location"`
	UserID         string `json:"user_id"`
}

type UpdateGameRequest struct {
	Name   string           `json:"name"`
	Status domain.GameStatus `json:"status"`
}


type Manifest struct {
	Name string `json:"name" binding:"required"`
	PlayerNode string `json:"player_node" binding:"required"`
	SyncNodes []string `json:"sync_nodes"`
	Version string `json:"version"`	
	HeadlessBin string `json:"headless_bin"`
	MainScene string `json:"main_scene"`
}

type InitUploadRequest struct {
	UserID   string   `json:"game_id"  binding:"required"`
	Name string `json:"game_name" binding:"required"`
	Manifest Manifest `json:"manifest" binding:"required"`
	Version string `json:"vm_id" binding:"required"`
}

type InitUploadResult struct {
	UploadURL string `json:"upload_url"`
	Key       string `json:"key"`
}

type PlayGameRequest struct {
	GameID string   `json:"game_id" binding:"required"`
	UserID string `json:"user_id" binding:"required"`
}

type PlayGameResult struct {
	SessionID  string   `json:"session_id"`
	StreamURL  string `json:"stream_url"`
	Status     string `json:"status"`
}

type CreateSessionRequest struct {
	UserID string `json:"user_id" binding:"required"`
	GameId string`json: "game_id" binding:"required"`
}

// ── Implementation ────────────────────────────────────────────────────────────

type gameService struct {
	repo repository.GameRepository
	log  *zap.SugaredLogger
	natsClient *messaging.NatsClient
	sessionService *Service
}

func NewGameService(repo repository.GameRepository, log *zap.SugaredLogger, natsClient *messaging.NatsClient, sessionService *Service) GameService {
	return &gameService{repo: repo, log: log, natsClient: natsClient, sessionService: sessionService}
}

func (s *gameService) ListGames(ctx context.Context) ([]domain.Game, error) {
	return s.repo.ListGames(ctx)
}

func (s *gameService) GetGame(ctx context.Context, id string) (*domain.Game, error) {
	return s.repo.GetGame(ctx, id)
}

func (s *gameService) CreateGame(ctx context.Context, req CreateGameRequest) (*domain.Game, error) {
	game := &domain.Game{
		Name:           req.Name,
		FolderLocation: req.FolderLocation,
		UserID:         req.UserID,
		Status:         domain.GameStatusPending,
	}
	if err := s.repo.CreateGame(ctx, game); err != nil {
		return nil, fmt.Errorf("create game: %w", err)
	}
	return game, nil
}

func (s *gameService) UpdateGame(ctx context.Context, id string, req UpdateGameRequest) (*domain.Game, error) {
	game, err := s.repo.GetGame(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		game.Name = req.Name
	}
	if req.Status != "" {
		game.Status = req.Status
	}
	if err := s.repo.UpdateGame(ctx, game); err != nil {
		return nil, fmt.Errorf("update game: %w", err)
	}
	return game, nil
}

func (s *gameService) DeleteGame(ctx context.Context, id string) error {
	return s.repo.DeleteGame(ctx, id)
}

func (s *gameService) GetManifest(ctx context.Context, id string) (map[string]any, error) {
	return s.repo.GetManifest(ctx, id)
}

func (s *gameService) GetDownloadURL(ctx context.Context, id string) (string, error) {
	game, err := s.repo.GetGame(ctx, id)
	if err != nil {
		return "", err
	}
	// Build the download URL from the game folder location
	return fmt.Sprintf("/api/v1/gamelift/games/static/%s/package.zip", game.FolderLocation), nil
}
type createPresignedURLRequestPayload struct {
	GameID    string    `json:"game_id"`
	UserID    string `json:"user_id"`
	ARN       string `json:"arn"`
	Extension string `json:"extension"`
}
func (s *gameService) InitUpload(ctx context.Context, req InitUploadRequest) (*InitUploadResult, error) {
	// 1. Check if game name already exists
	s.log.Info(">>>>>>>>>>>>>>>d>>>>>>>>>>>>>>>>>>>>>")
	// userIDVal, exists := ctx.Value(string(domain.UserIDKey)).(string)
	// if !exists {
	// 	return nil, fmt.Errorf("user not found in context")
	// }
	s.log.Info(">>>>>>>>>>>>>>>a>>>>>>>>>>>>>>>>>>>>>")

	existing, err := s.repo.GetGameByName(ctx, req.Name)
	if err == nil && existing != nil {
		s.log.Info("game with name %q already exists", req.Name)
		return nil, fmt.Errorf("game with name %q already exists", req.Name)
	}

	s.log.Info(">>>>>>>>>>>>>g>>>>>>>>>>>>>>>>>>>>>>>")

	// 2. Create the game record
	game := &domain.Game{
		ID: uuid.New().String(),
		Name:          req.Name,
		UserID:        req.UserID,
		Status:        domain.GameStatusPending,
		StreamingMode: domain.StreamingModeState,
	}
	if err := s.repo.CreateGame(ctx, game); err != nil {
		return nil, fmt.Errorf("failed to create game record: %w", err)
	}
	s.log.Info(">>>>>>>>>>>>>>>k>>>>>>>>>>>>>>>>>>>>>")

	// 3. Generate ARN and folder location now that we have an ID
	game.ARN            = fmt.Sprintf("arn:serw:game:eu-north-1:%s:game/%d", "userIDVal", game.ID)
	game.FolderLocation = fmt.Sprintf("./uploads/games/%d", game.ID)
	if err := s.repo.UpdateGame(ctx, game); err != nil {
		return nil, fmt.Errorf("failed to update game ARN: %w", err)
	}

	// 4. Request presigned upload URL from S3 service via NATS
	payload, err := json.Marshal(createPresignedURLRequestPayload{
		GameID:    game.ID,
		UserID:    game.UserID,
		ARN:       game.ARN,
		Extension: "x86_64",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal presign payload: %w", err)
	}

	reply, err := s.natsClient.Request(
		messaging.GetS3GameInitUploadSubject(),
		payload,
		5*time.Second,
	)
	if err != nil {
		return nil, fmt.Errorf("presign request failed: %w", err)
	}

var presignResp struct {
    UploadURL string `json:"upload_url"`
}
if err := json.Unmarshal(reply.Data, &presignResp); err != nil {
    return nil, fmt.Errorf("failed to parse presign response: %w", err)
}

return &InitUploadResult{
    UploadURL: presignResp.UploadURL,
    Key:       fmt.Sprintf("games/%s/package.%s", game.ID, "x86_64"),
}, nil
}







func (s *gameService) PlayGame(ctx context.Context, req PlayGameRequest) (*PlayGameResult, error) {
	game, err := s.repo.GetGame(ctx, req.GameID)
	if err != nil {
		return nil, err
	}
	session := &domain.GameSession{
		ID: uuid.New().String(),
		GameID: game.ID,
		UserID: req.UserID,
		Status: "starting",
		NodeID: "lksjdaksjdak",
		
	}
	initService :=domain.CreateSessionRequest{
		GameID: game.ID,
		UserID: req.UserID,
		GameImage: "",
	}
	s.sessionService.CreateSession(ctx, initService)
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("play game: create session: %w", err)
	}
	return &PlayGameResult{
		SessionID: session.ID,
		StreamURL: fmt.Sprintf("/api/v1/ws?session=%d", session.ID),
		Status:    session.Status,
	}, nil
}






func (s *gameService) CreateSession(ctx context.Context, gameID string, req domain.CreateSessionRequest) (*domain.GameSession, error) {
	session := &domain.GameSession{
		ID: uuid.New().String(),
		GameID: gameID,
		UserID: req.UserID,
		Status: "pending",
	}
		initService :=domain.CreateSessionRequest{
		GameID: gameID,
		UserID: req.UserID,
		GameImage: "",
	}
	ses ,err:=s.sessionService.CreateSession(ctx, initService)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}


	session.AgentWSURL=ses.AgentWSURL
	session.Token=ses.Token
	session.NodeID=ses.NodeID

	
	// if err := s.repo.CreateSession(ctx, session); err != nil {
	// 	return nil, fmt.Errorf("create session: %w", err)
	// }

	return session, nil
}

func (s *gameService) GetSessionStatus(ctx context.Context, gameID string) (*domain.GameSession, error) {
	return s.repo.GetSession(ctx, gameID)
}