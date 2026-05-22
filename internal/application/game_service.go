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
	InitUpload(ctx context.Context, req InitUploadRequest, log *zap.SugaredLogger) (*InitUploadResult, error)
	PlayGame(ctx context.Context, req PlayGameRequest) (*PlayGameResult, error)
	CreateSession(ctx context.Context, gameID string, req domain.CreateSessionRequest) (*domain.GameSession, error)
	GetSessionStatus(ctx context.Context, gameID string) (*domain.GameSession, error)
	StreamSessionEvents(ctx context.Context, gameID string) (chan domain.GameSessionEvent, error)
}

// ── Request / Response types ──────────────────────────────────────────────────

type CreateGameRequest struct {
	Name           string   `json:"name"    binding:"required"`
	FolderLocation string   `json:"folder_location"`
	UserID         string   `json:"user_id"`
	Manifest       Manifest `json:"manifest" binding:"required"`
}

type UpdateGameRequest struct {
	Name   string            `json:"name"`
	Status domain.GameStatus `json:"status"`
}

type Manifest struct {
	Name        string            `json:"name" binding:"required"`
	PlayerNode  string            `json:"player_node" binding:"required"`
	SyncNodes   []domain.SyncNode `json:"sync_nodes"`
	Version     string            `json:"version"`
	HeadlessBin string            `json:"headless_bin"`
	MainScene   string            `json:"main_scene"`
}

type InitUploadRequest struct {
	UserID   string   `json:"game_id"  binding:"required"`
	Name     string   `json:"game_name" binding:"required"`
	Manifest Manifest `json:"manifest" binding:"required"`
	Version  string   `json:"vm_id" binding:"required"`
	SHA256   string   `json:"sha256"`
}

type InitUploadResult struct {
	UploadURL string `json:"upload_url"`
	ObjectKey string `json:"object_key"`
	GameID    string `json:"game_id"`
	SHA256Hint string `json:"sha256_hint"`
}

type PlayGameRequest struct {
	GameID string `json:"game_id" binding:"required"`
	UserID string `json:"user_id" binding:"required"`
}

type PlayGameResult struct {
	SessionID string `json:"session_id"`
	StreamURL string `json:"stream_url"`
	Status    string `json:"status"`
}

type CreateSessionRequest struct {
	UserID string `json:"user_id" binding:"required"`
	GameId string `json:"game_id" binding:"required"`
}

// ── Implementation ────────────────────────────────────────────────────────────

type gameService struct {
	repo           repository.GameRepository
	log            *zap.SugaredLogger
	natsClient     *messaging.NatsClient
	sessionService *Service
	sseRegistry    *SSERegistry
}

func NewGameService(repo repository.GameRepository, log *zap.SugaredLogger, natsClient *messaging.NatsClient, sessionService *Service, sseRegistry *SSERegistry) GameService {
	return &gameService{repo: repo, log: log, natsClient: natsClient, sessionService: sessionService, sseRegistry: sseRegistry}
}

func (s *gameService) ListGames(ctx context.Context) ([]domain.Game, error) {
	return s.repo.ListGames(ctx, s.log)
}

func (s *gameService) GetGame(ctx context.Context, id string) (*domain.Game, error) {
	return s.repo.GetGame(ctx, id, s.log)
}

func (s *gameService) StreamSessionEvents(ctx context.Context, gameID string) (chan domain.GameSessionEvent, error) {
	return s.sseRegistry.Register(gameID), nil
}

func (s *gameService) CreateGame(ctx context.Context, req CreateGameRequest) (*domain.Game, error) {
	manifestID := uuid.New().String()
	game := &domain.Game{
		Name:           req.Name,
		FolderLocation: req.FolderLocation,
		UserID:         req.UserID,
		Status:         domain.GameStatusPending,
		Manifest:       manifestID,
	}
	if err := s.repo.CreateGame(ctx, game, s.log); err != nil {
		return nil, fmt.Errorf("create game: %w", err)
	}
	manifest := domain.GameManifest{
		ID:         manifestID,
		Name:       req.Name,
		PlayerNode: req.Manifest.PlayerNode,
		SyncNodes:  req.Manifest.SyncNodes,
		Version:    req.Manifest.Version,

		HeadlessBin: req.Manifest.HeadlessBin,
		MainScene:   req.Manifest.MainScene,
	}
	if err := s.repo.SetManifest(ctx, game.ID, manifest, s.log); err != nil {
		return nil, fmt.Errorf("set manifest: %w", err)
	}
	if err := s.repo.SetManifest(ctx, game.ID, manifest, s.log); err != nil {
		return nil, fmt.Errorf("set manifest: %w", err)
	}
	return game, nil
}

func (s *gameService) UpdateGame(ctx context.Context, id string, req UpdateGameRequest) (*domain.Game, error) {
	game, err := s.repo.GetGame(ctx, id, s.log)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		game.Name = req.Name
	}
	if req.Status != "" {
		game.Status = req.Status
	}
	if err := s.repo.UpdateGame(ctx, game, s.log); err != nil {
		return nil, fmt.Errorf("update game: %w", err)
	}
	return game, nil
}

func (s *gameService) DeleteGame(ctx context.Context, id string) error {
	return s.repo.DeleteGame(ctx, id, s.log)
}

func (s *gameService) GetManifest(ctx context.Context, id string) (map[string]any, error) {
	return s.repo.GetManifest(ctx, id, s.log)
}

func (s *gameService) GetDownloadURL(ctx context.Context, id string) (string, error) {
	game, err := s.repo.GetGame(ctx, id, s.log)
	if err != nil {
		return "", err
	}
	// Build the download URL from the game folder location
	return fmt.Sprintf("/api/v1/gamelift/games/static/%s/package.zip", game.FolderLocation), nil
}

type createPresignedURLRequestPayload struct {
	GameID    string `json:"game_id"`
	UserID    string `json:"user_id"`
	ARN       string `json:"arn"`
	Extension string `json:"extension"`
}
func (s *gameService) InitUpload(
	ctx context.Context,
	req InitUploadRequest,
	log *zap.SugaredLogger,
) (*InitUploadResult, error) {

	log = log.With(
		"layer", "service",
		"game_name", req.Name,
	)

	log.Infow("INIT_UPLOAD_STARTED")

	// 1. Check existing game
	existing, err := s.repo.GetGameByName(ctx, req.Name, log)
	if err == nil && existing != nil {
		log.Warnw("GAME_ALREADY_EXISTS",
			"existing_game_id", existing.ID,
		)
		return nil, fmt.Errorf("game with name %q already exists", req.Name)
	}

	log.Infow("GAME_NAME_AVAILABLE")

	// 2. Create game record (PENDING STATE)
	game := &domain.Game{
		ID:            uuid.New().String(),
		Name:          req.Name,
		UserID:        req.UserID,
		Status:        domain.GameStatusPending,
		StreamingMode: domain.StreamingModeState,
		Sha256:  req.SHA256,
	}

	if err := s.repo.CreateGame(ctx, game, log); err != nil {
		log.Errorw("GAME_CREATE_FAILED", "error", err)
		return nil, fmt.Errorf("failed to create game record: %w", err)
	}

	log.Infow("GAME_CREATED",
		"game_id", game.ID,
	)

	// 3. CAS OBJECT KEY (IMPORTANT SHIFT)
	// If version is empty, we default to "latest" or just the game ID path
	version := req.Version
	if version == "" {
		version = "latest"
	}
	objectKey := fmt.Sprintf("games/%s/%s", game.ID, version)

	log.Infow("OBJECT_KEY_GENERATED",
		"object_key", objectKey,
	)

	// 4. Create upload session request (CAS DESIGN)
	payload, err := json.Marshal(map[string]any{
		"asset_id":    game.ID,
		"user_id":    game.UserID,
		"object_key": objectKey,
		"asset_type":    "game",
		"sha256":   req.SHA256,
	})
	if err != nil {
		log.Errorw("PAYLOAD_MARSHAL_FAILED", "error", err)
		return nil, fmt.Errorf("marshal presign payload: %w", err)
	}

	log.Infow("REQUESTING_S3_UPLOAD_SESSION",req.SHA256)

	reply, err := s.natsClient.Request(
		messaging.GetS3GameInitUploadSubject(),
		payload,
		5*time.Second,
	)

	if err != nil {
		log.Errorw("S3_REQUEST_FAILED", "error", err)
		return nil, fmt.Errorf("presign request failed: %w", err)
	}

	log.Infow("S3_RESPONSE_RECEIVED")

	var presignResp struct {
		UploadURL string `json:"upload_url"`
		SHA256    string `json:"sha256_hint,omitempty"` // Match the key we sent or the S3 actual response
	}

	if err := json.Unmarshal(reply.Data, &presignResp); err != nil {
		log.Errorw("S3_RESPONSE_UNMARSHAL_FAILED", "error", err)
		return nil, fmt.Errorf("parse presign response: %w", err)
	}

	// 5. Update game with storage reference
	game.StorageARN = objectKey

	if err := s.repo.UpdateGame(ctx, game, log); err != nil {
		log.Errorw("GAME_UPDATE_FAILED", "error", err)
		return nil, fmt.Errorf("update game storage: %w", err)
	}

	log.Infow("INIT_UPLOAD_COMPLETED",
		"game_id", game.ID,
		"object_key", objectKey,
	)
 


	return &InitUploadResult{
		GameID:     game.ID,
		UploadURL:  presignResp.UploadURL,
		ObjectKey:  objectKey,
		SHA256Hint: presignResp.SHA256,
	}, nil
}






func (s *gameService) PlayGame(ctx context.Context, req PlayGameRequest) (*PlayGameResult, error) {
	game, err := s.repo.GetGame(ctx, req.GameID, s.log)
	if err != nil {
		return nil, err
	}
	session := &domain.GameSession{
		ID:     uuid.New().String(),
		GameID: game.ID,
		UserID: req.UserID,
		Status: "starting",
		NodeID: "lksjdaksjdak",
	}
	initService := domain.CreateSessionRequest{
		GameID:    game.ID,
		UserID:    req.UserID,
		GameImage: "",
	}
	s.sessionService.CreateSession(ctx, initService)
	if err := s.repo.CreateSession(ctx, session, s.log); err != nil {
		return nil, fmt.Errorf("play game: create session: %w", err)
	}
	return &PlayGameResult{
		SessionID: session.ID,
		StreamURL: fmt.Sprintf("/api/v1/ws?session=%s", session.ID),
		Status:    session.Status,
	}, nil
}
type createPresignDownloadURLResponse struct {
	URL string `json:"url"`

}
func (s *gameService) CreateSession(ctx context.Context, gameID string, req domain.CreateSessionRequest) (*domain.GameSession, error) {
// 1. Create session skeleton
	session := &domain.GameSession{
		ID:     uuid.New().String(),
		GameID: gameID,
		UserID: req.UserID,
		Status: "pending",
	}

	// 2. Fetch game (FIXED)
	game, err := s.repo.GetGame(ctx, gameID, s.log)
	if err != nil {
		s.log.Errorw("GET_GAME_FAILED",
			"game_id", gameID,
			"error", err,
		)
		return nil, fmt.Errorf("get game: %w", err)
	}


	payload, err := json.Marshal(map[string]any{
		"asset_id":    game.ID,
		"user_id":    game.UserID,
		"sha256":   game.Sha256,
		"correlation_id":uuid.New().String(),
	})



reply, err := s.natsClient.Request(
    messaging.GetS3GameInitDownloadSubject(),
    payload,
    5*time.Second,
)




if err != nil {
    s.log.Errorw("S3_REQUEST_FAILED", "error", err)
    return nil, fmt.Errorf("presign request failed: %w", err)
}

s.log.Infow("S3_RESPONSE_RECEIVED", "bytes", len(reply.Data))

var resp createPresignDownloadURLResponse
if err := json.Unmarshal(reply.Data, &resp); err != nil {
    s.log.Errorw("S3_RESPONSE_UNMARSHAL_FAILED",
        "game_id", gameID,
        "user_id", req.UserID,
        "bytes", len(reply.Data),
        "error", err,
    )
    return nil, fmt.Errorf("unmarshal presign response: %w", err)
}



// get the gamesha256 then callthe s3service toget createa presigned url forthese file

	// 4. Create session in external agent/session service
	initService := domain.CreateSessionRequest{
		GameID:   gameID,
		UserID:   req.UserID,
		AssetURL: resp.URL, // FIXED naming
	}



	ses, err := s.sessionService.CreateSession(ctx, initService)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	session.AgentWSURL = ses.AgentWSURL
	session.Token = ses.Token
	session.NodeID = ses.NodeID
	session.ID = ses.ID

	// if err := s.repo.CreateSession(ctx, session); err != nil {
	// 	return nil, fmt.Errorf("create session: %w", err)
	// }

	fmt.Printf("session check here -*-> %s", session.ID)

	return session, nil
}

func (s *gameService) GetSessionStatus(ctx context.Context, gameID string) (*domain.GameSession, error) {
	return s.repo.GetSession(ctx, gameID, s.log)
}
