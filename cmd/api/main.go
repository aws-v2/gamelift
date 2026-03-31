package main

import (
	"net/http"
	"os"
	"time"

	"backend/internal/config"
	"backend/internal/domain"
	"backend/internal/infrastructure/discovery"
	"backend/internal/infrastructure/storage"
	"backend/internal/service"
	"backend/internal/transport"
	"backend/internal/transport/handler"
	"backend/internal/messaging"
	"backend/pkg/database"
	"backend/pkg/logger"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// ExtendedConfig wraps the existing config to add NATS and DB configurations
// by populating them from environment variables via Load(), satisfying requirements
// without modifying the internal config package directly.
type ExtendedConfig struct {
	*config.Config
	NATS struct {
		URL      string
		User     string
		Password string
	}
	DB database.Config
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func loadExtendedConfig() *ExtendedConfig {
	cfg := &ExtendedConfig{
		Config: config.Load(),
	}
	cfg.NATS.URL = getEnvOrDefault("NATS_URL", "nats://127.0.0.1:4222")
	cfg.NATS.User = getEnvOrDefault("NATS_USER", "auth-server")
	cfg.NATS.Password = getEnvOrDefault("NATS_PASSWORD", "auth-secret")

	cfg.DB.Host = getEnvOrDefault("DB_HOST", "127.0.0.1")
	cfg.DB.Port = getEnvOrDefault("DB_PORT", "5432")
	cfg.DB.User = getEnvOrDefault("DB_USER", "root")
	cfg.DB.Password = getEnvOrDefault("DB_PASSWORD", "root")
	cfg.DB.Name = getEnvOrDefault("DB_NAME", "gamelift_db")
	return cfg
}

func main() {
	logr := logger.New()
	defer logr.Sync()

	cfg := loadExtendedConfig()

	// 1. Register with Eureka (with retries)
	logr.Info("Attempting Eureka registration...", zap.String("app", cfg.Eureka.AppName))
	for i := 0; i < 3; i++ {
		if err := discovery.RegisterWithEureka(cfg.Eureka); err != nil {
			logr.Warn("Eureka registration attempt failed", zap.Int("attempt", i+1), zap.Error(err))
			if i < 2 {
				time.Sleep(5 * time.Second)
			}
		} else {
			break
		}
	}
	// Start heartbeat
	go discovery.SendHeartbeat(cfg.Eureka)

	opts := []nats.Option{}
	if cfg.NATS.User != "" && cfg.NATS.Password != "" {
		opts = append(opts, nats.UserInfo(cfg.NATS.User, cfg.NATS.Password))
	}

	nc, err := nats.Connect(cfg.NATS.URL, opts...)
	if err != nil {
		logr.Fatal("Failed to connect to NATS", zap.Error(err))
	}
	defer nc.Close()
	logr.Info("Successfully connected to NATS", zap.String("url", cfg.NATS.URL))

	natsClient := messaging.NewNatsClient(nc)

	var db *database.DB
	for attempt := 1; attempt <= 4; attempt++ {
		db, err = database.ConnectPostgres(cfg.DB)
		if err == nil {
			logr.Info("Successfully connected to PostgreSQL")
			break
		}
		logr.Warn("Failed to connect to PostgreSQL, retrying...", zap.Int("attempt", attempt), zap.Error(err))
		if attempt < 4 {
			time.Sleep(2 * time.Second)
		}
	}

	if err != nil {
		logr.Warn("All PostgreSQL connection attempts failed, falling back to SQLite")
		sqlitePath := os.Getenv("SQLITE_PATH")
		if sqlitePath == "" {
			sqlitePath = "lambda.db"
		}
		db, err = database.ConnectSQLite(sqlitePath)
		if err != nil {
			logr.Fatal("Failed to connect to SQLite fallback", zap.Error(err))
		}
		logr.Info("Successfully connected to SQLite fallback", zap.String("path", sqlitePath))
	}
	defer db.Close()

	// Perform AutoMigrate
	if err := db.GORM.AutoMigrate(&domain.Game{}); err != nil {
		logr.Fatal("Failed to auto-migrate database", zap.Error(err))
	}

	authSvc := service.NewAuthService(cfg.Config)
	gameSvc := service.NewPostgresGameRepository(db)
	
	hub := handler.NewHub()
	go hub.Run()

	// Seed Demo Game if it's a fresh database
	var count int64
	db.GORM.Model(&domain.Game{}).Count(&count)
	if count == 0 {
		logr.Info("Seeding initial Demo Game")
		db.GORM.Create(&domain.Game{
			ID:             1,
			Name:           "Demo Game",
			FolderLocation: "media",
			VMID:           "test-vm-1",
			ARN:            "arn:serw:game:eu-north-1:system:game/1",
			Status:         domain.GameStatusActive,
		})
	}

	logr.Info("Connecting to MinIO...", zap.String("endpoint", cfg.S3.Endpoint))
	minioAdapter, err := storage.NewMinIOAdapter(
		cfg.S3.Endpoint,
		cfg.S3.AccessKey,
		cfg.S3.SecretKey,
		cfg.S3.UseSSL,
	)
	if err != nil {
		logr.Fatal("Failed to connect to MinIO", zap.Error(err))
	}

	validationSvc := service.NewValidationService()
	s3Listener := service.NewS3Listener(gameSvc, natsClient, validationSvc, minioAdapter, cfg.PublicURL)
	go s3Listener.Start()

	// Initialize Provisioning Logic
	provisioningSvc := service.NewProvisioningService(gameSvc, natsClient)
	nodeAgent := service.NewNodeAgent("local-dev-node", gameSvc, natsClient, minioAdapter, cfg.Debug, cfg.GodotPath)
	go nodeAgent.Start()

	gameStateListener := service.NewGameStateListener(natsClient, hub)
	go gameStateListener.Start()

	router := transport.NewRouter(authSvc, gameSvc, hub, natsClient, provisioningSvc, minioAdapter)

	logr.Info("Listening on", zap.String("port", cfg.ServerPort))
	if err := http.ListenAndServe(cfg.ServerPort, router); err != nil {
		logr.Fatal("Server failed", zap.Error(err))
	}
}
