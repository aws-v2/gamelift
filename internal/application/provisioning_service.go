package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"time"

	"backend/internal/config"
	"backend/internal/domain"
	"backend/internal/infrastructure/messaging"
	"backend/internal/infrastructure/repository"
	"backend/internal/infrastructure/storage"

	"go.uber.org/zap"
)

type ProvisioningService struct {
	gameRepo   repository.GameRepository
	natsClient repository.MessagingClient
	storage    *storage.MinIOAdapter
	debug      bool
	godotPath  string
	backendURL string
	appEnv     string
	logger     *zap.SugaredLogger
	natsPrefix string
	assetsPath string
	cfg *config.Config
	err *domain.AppError
}

func NewProvisioningService(
	gameRepo repository.GameRepository,
	natsClient repository.MessagingClient,
	storage *storage.MinIOAdapter,
	debug bool,
	godotPath string,
	backendURL string,
	appEnv string,
	logger *zap.SugaredLogger,
	natsPrefix string,
	assetsPath string,
	cfg *config.Config,	
) *ProvisioningService {
	return &ProvisioningService{
		gameRepo:   gameRepo,
		natsClient: natsClient,
		storage:    storage,
		debug:      debug,
		godotPath:  godotPath,
		backendURL: backendURL,
		appEnv:     appEnv,
		logger:     logger,
		natsPrefix: natsPrefix,
		assetsPath: assetsPath,
		cfg:        cfg,
	}
}

func (s *ProvisioningService) ProvisionGame(gameID string, mode domain.StreamingMode, sessionID string, assetURL string, fileSHA256 string, userID string) error {
	ctx := context.Background()

	s.logger.Infow("PROVISION_GAME_STARTING", "game_id", gameID, "mode", mode, "session_id", sessionID)

	// 1. Fetch game record
	game, err := s.gameRepo.GetGame(ctx, gameID, s.logger)
	if err != nil {
		s.logger.Errorw("PROVISION_GAME_FETCH_FAILED", "game_id", gameID, "error", err)
		return err
	}

	s.logger.Infow("PROVISION_GAME_FETCHED", "game_id", gameID, "name", game.Name, "status", game.Status, "arn", game.ARN)

	// 2. Mark as provisioning to prevent duplicate requests
	if err := s.gameRepo.UpdateGameStatus(ctx, gameID, domain.GameStatusProvisioning, s.logger); err != nil {
		s.logger.Errorw("PROVISION_GAME_STATUS_UPDATE_FAILED",
			"game_id", gameID,
			"target_status", domain.GameStatusProvisioning,
			"error", err,
		)
		s.err.Code=10
		return s.err
	}
	s.logger.Infow("PROVISION_GAME_STATUS_UPDATED", "game_id", gameID, "new_status", domain.GameStatusProvisioning)

	// 3. Fetch file info from S3 service via NATS
	fileInfo, err := s.getFileInfo(game.ARN)
	if err != nil {
		s.logger.Errorw("PROVISION_GAME_FILE_INFO_FAILED", "game_id", gameID, "arn", game.ARN, "error", err)
		return err
	}

	s.logger.Infow("PROVISION_GAME_FILE_INFO_FETCHED",
		"game_id", gameID,
		"arn", game.ARN,
		"download_url", fileInfo.DownloadURL,
		"sha256", fileInfo.SHA256,
	)

	// // 4. Parse manifest
	// var manifest domain.GameManifest
	// if err := json.Unmarshal([]byte(game.Manifest), &manifest); err != nil {
	// 	s.logger.Errorw("PROVISION_GAME_MANIFEST_PARSE_FAILED", "game_id", gameID, "error", err)
	// 	return fmt.Errorf("failed to unmarshal manifest: %w", err)
	// }

	// s.logger.Infow("PROVISION_GAME_MANIFEST_PARSED",
	// 	"game_id", gameID,
	// 	"headless_bin", manifest.HeadlessBin,
	// 	"main_scene", manifest.MainScene,
	// )

	// 5. Build and publish EC2 provision request

	payload := domain.EC2ProvisionRequest{
		Profile: "gamelift",
		Specs:   map[string]int{"cpu": 2, "ram": 4096},
		SessionID: sessionID,
		UserID:     userID,
		StorageARN: game.ARN,
		Manifest: domain.GameManifest{
			Name:        game.Name,
			HeadlessBin: "server/hh.x86_64",
			Parameters: map[string]string{
			"game_id":        game.ID,
			"storage_arn":    game.ARN,
			"headless_bin":   "manifest.HeadlessBin",
			"game_name":      game.Name,
			"backend_url":    s.backendURL,
			"ASSET_URL":            assetURL,
			"streaming_mode": string(mode),
			"ASSET_PATH":     s.cfg.VMAssetPath,
			"ASSET_SHA256":   game.Sha256,
		},
		},
	}


 

	
	data, err := json.Marshal(payload)
	if err != nil {
		s.logger.Errorw("PROVISION_GAME_PAYLOAD_MARSHAL_FAILED", "game_id", gameID, "error", err)
		return fmt.Errorf("failed to marshal provision payload: %w", err)
	}

	subj := messaging.Subject{Service: "ec2", Domain: "task", ActionType: "provision"}

	s.logger.Infow("PROVISION_GAME_PUBLISHING",
		"game_id", gameID,
		"session_id", sessionID,
		"subject", subj,
		"mode", mode,
	)

	if err := s.natsClient.Publish(subj, data); err != nil {
		s.logger.Errorw("PROVISION_GAME_PUBLISH_FAILED",
			"game_id", gameID,
			"session_id", sessionID,
			"subject", subj,
			"error", err,
		)
		return err
	}

	s.logger.Infow("PROVISION_GAME_PUBLISHED",
		"game_id", gameID,
		"session_id", sessionID,
		"subject", subj,
	)
 
 
	return nil
}


 
func (s *ProvisioningService) getFileInfo(storageARN string) (*domain.FileInfo, error) {
	s.logger.Infow("GET_FILE_INFO_REQUESTING", "storage_arn", storageARN)

	subj := messaging.Subject{
		Service:    "s3",
		Domain:     "task",
		ActionType: "task.get_file_info",
	}

	payload := map[string]string{"storage_arn": storageARN}

	data, err := json.Marshal(payload)
	if err != nil {
		s.logger.Errorw("GET_FILE_INFO_MARSHAL_FAILED", "storage_arn", storageARN, "error", err)
		return nil, fmt.Errorf("failed to marshal file info request: %w", err)
	}

	resp, err := s.natsClient.Request(subj, data, 5*time.Second)
	if err != nil {
		s.logger.Errorw("GET_FILE_INFO_NATS_REQUEST_FAILED", "storage_arn", storageARN, "subject", subj, "error", err)
		return nil, fmt.Errorf("nats request to s3 service failed: %w", err)
	}

	s.logger.Debugw("GET_FILE_INFO_RESPONSE_RECEIVED", "storage_arn", storageARN, "bytes", len(resp.Data))

	var info domain.FileInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		s.logger.Errorw("GET_FILE_INFO_UNMARSHAL_FAILED", "storage_arn", storageARN, "error", err)
		return nil, fmt.Errorf("failed to unmarshal file info: %w", err)
	}

	s.logger.Infow("GET_FILE_INFO_SUCCESS",
		"storage_arn", storageARN,
		"download_url", info.DownloadURL,
		"sha256", info.SHA256,
	)

	return &info, nil
}

func (s *ProvisioningService) getAssetsURL() string {
	url := fmt.Sprintf("%s/assets", s.backendURL)
	s.logger.Debugw("GET_ASSETS_URL", "url", url)
	return url
}

func (s *ProvisioningService) launchLocalDebug(game *domain.Game, mode domain.StreamingMode) {
	s.logger.Infow("LOCAL_DEBUG_LAUNCH_STARTING", "game_id", game.ID, "mode", mode)

	tempDir := filepath.Join("/tmp", fmt.Sprintf("game_%s", game.ID))
	tempZip := filepath.Join("/tmp", fmt.Sprintf("game_%s.zip", game.ID))

	os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		s.logger.Errorw("LOCAL_DEBUG_MKDIR_FAILED", "game_id", game.ID, "path", tempDir, "error", err)
		return
	}

	// Parse ARN → bucket + key
	arnParts := strings.Split(game.StorageARN, ":::")
	if len(arnParts) < 2 {
		s.logger.Errorw("LOCAL_DEBUG_INVALID_ARN", "game_id", game.ID, "arn", game.StorageARN)
		return
	}
	pathParts := strings.SplitN(arnParts[1], "/", 2)
	if len(pathParts) < 2 {
		s.logger.Errorw("LOCAL_DEBUG_ARN_PARSE_FAILED", "game_id", game.ID, "arn", game.StorageARN)
		return
	}
	bucket, key := pathParts[0], pathParts[1]

	s.logger.Infow("LOCAL_DEBUG_DOWNLOADING", "game_id", game.ID, "bucket", bucket, "key", key, "dest", tempZip)

	if err := s.storage.DownloadFile(context.Background(), bucket, key, tempZip); err != nil {
		s.logger.Errorw("LOCAL_DEBUG_DOWNLOAD_FAILED", "game_id", game.ID, "bucket", bucket, "key", key, "error", err)
		return
	}

	s.logger.Infow("LOCAL_DEBUG_DOWNLOAD_SUCCESS", "game_id", game.ID, "dest", tempZip)

	if err := exec.Command("unzip", "-o", tempZip, "-d", tempDir).Run(); err != nil {
		s.logger.Errorw("LOCAL_DEBUG_UNZIP_FAILED", "game_id", game.ID, "src", tempZip, "dest", tempDir, "error", err)
		return
	}

	s.logger.Infow("LOCAL_DEBUG_UNZIP_SUCCESS", "game_id", game.ID, "dest", tempDir)

	var manifest domain.GameManifest
	if err := json.Unmarshal([]byte(game.Manifest), &manifest); err != nil {
		s.logger.Errorw("LOCAL_DEBUG_MANIFEST_PARSE_FAILED", "game_id", game.ID, "error", err)
		return
	}

	binPath := filepath.Join(tempDir, manifest.HeadlessBin)
	if err := os.Chmod(binPath, 0755); err != nil {
		s.logger.Warnw("LOCAL_DEBUG_CHMOD_FAILED", "game_id", game.ID, "bin", binPath, "error", err)
	}

	var args []string
	var cmdPrefix []string

	if mode == domain.StreamingModeVideo {
		args = append(args, "--mode=webrtc", "--rendering-method", "gl_compatibility")
		cmdPrefix = []string{"xvfb-run", "--auto-servernum", "--server-args='-screen 0 1280x720x24'"}
		s.logger.Infow("LOCAL_DEBUG_MODE_VIDEO", "game_id", game.ID, "xvfb", true)
	} else {
		args = append(args, "--headless", "--mode=state_sync")
		s.logger.Infow("LOCAL_DEBUG_MODE_HEADLESS", "game_id", game.ID)
	}

	execCmd := fmt.Sprintf("./%s %s", manifest.HeadlessBin, strings.Join(args, " "))
	if len(cmdPrefix) > 0 {
		execCmd = fmt.Sprintf("%s %s", strings.Join(cmdPrefix, " "), execCmd)
	}

	terminalCmd := fmt.Sprintf("cd %s && %s; read -p 'Press enter to close...'", tempDir, execCmd)

	s.logger.Infow("LOCAL_DEBUG_LAUNCHING", "game_id", game.ID, "cmd", execCmd)

	cmd := exec.Command("gnome-terminal", "--", "bash", "-c", terminalCmd)
	if err := cmd.Start(); err != nil {
		s.logger.Warnw("LOCAL_DEBUG_GNOME_TERMINAL_FAILED",
			"game_id", game.ID,
			"error", err,
			"fallback", "background exec",
		)
		if err := exec.Command(binPath, args...).Start(); err != nil {
			s.logger.Errorw("LOCAL_DEBUG_FALLBACK_EXEC_FAILED", "game_id", game.ID, "bin", binPath, "error", err)
			return
		}
	}

	s.logger.Infow("LOCAL_DEBUG_LAUNCHED", "game_id", game.ID, "mode", mode)

	if err := s.gameRepo.UpdateGameStatus(context.Background(), game.ID, domain.GameStatusActive, s.logger); err != nil {
		s.logger.Errorw("LOCAL_DEBUG_STATUS_UPDATE_FAILED", "game_id", game.ID, "error", err)
		return
	}

	s.logger.Infow("LOCAL_DEBUG_STATUS_UPDATED", "game_id", game.ID, "new_status", domain.GameStatusActive)
}