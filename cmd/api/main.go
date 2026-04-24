package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend/internal/config"
	"backend/internal/domain"
	"backend/internal/infrastructure/discovery"
	"backend/internal/infrastructure/storage"
	"backend/internal/application"
	"backend/internal/infrastructure/repository"
	appNats "backend/internal/transport/nats"
	httpRouter "backend/internal/transport/http"
	"backend/internal/transport/websocket"
	"backend/internal/messaging"
	"backend/pkg/database"
	"backend/pkg/logger"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func main() {
	logr := logger.NewSugared()
	defer logr.Sync()

	cfg := config.Load()

	// 1. Register with Eureka (with retries)
	logr.Infow("Attempting Eureka registration", "app", cfg.Eureka.AppName)
	for i := 0; i < 3; i++ {
		if err := discovery.RegisterWithEureka(cfg.Eureka, logr); err != nil {
			logr.Warnw("Eureka registration attempt failed", "attempt", i+1, "error", err)
			if i < 2 {
				time.Sleep(5 * time.Second)
			}
		} else {
			break
		}
	}
	// Start heartbeat
	go discovery.SendHeartbeat(cfg.Eureka, logr)

	natsOpts := []nats.Option{
		nats.Name("Gamelift Backend"),
		nats.Timeout(10 * time.Second),
	}

	if cfg.NatsUser != "" && cfg.NatsPassword != "" {
		natsOpts = append(natsOpts, nats.UserInfo(cfg.NatsUser, cfg.NatsPassword))
	}

	nc, err := nats.Connect(cfg.NatsURL, natsOpts...)
	if err != nil {

		logr.Fatalw("Failed to connect to NATS", "error", err)
	}
	defer nc.Close()
	logr.Infow("Successfully connected to NATS", "url", cfg.NatsURL)

	natsClient := messaging.NewNatsClient(nc, logr, cfg.NatsPrefix)

	var db *database.DB
	for attempt := 1; attempt <= 4; attempt++ {
		db, err = database.ConnectPostgres(cfg.DB)
		if err == nil {
			logr.Info("Successfully connected to PostgreSQL")
			break
		}
		logr.Warnw("Failed to connect to PostgreSQL, retrying", "attempt", attempt, "error", err)
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
			logr.Fatalw("Failed to connect to SQLite fallback", "error", err)
		}
		logr.Infow("Successfully connected to SQLite fallback", "path", sqlitePath)
	}
	defer db.Close()

	// Perform Migrations
	logr.Info("Running database migrations...")
	if err := db.Migrate("migrations/sql"); err != nil {
		logr.Fatalw("Failed to run database migrations", "error", err)
	}

	authSvc := application.NewAuthService(cfg, logr)
	gameSvc := repository.NewPostgresGameRepository(db, logr)
	
	hub := websocket.NewHub(logr)
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
			UserID:         "system",
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
		logr.Fatalw("Failed to connect to MinIO", "error", err)
	}

	validationSvc := application.NewValidationService()
	s3Listener := appNats.NewS3Listener(gameSvc, natsClient, validationSvc, minioAdapter, cfg.PublicURL, cfg.AppEnv, logr)
	go s3Listener.Start()

	// Initialize Provisioning Logic
	provisioningSvc := application.NewProvisioningService(gameSvc, natsClient, minioAdapter, cfg.Debug, cfg.GodotPath, cfg.PublicURL, cfg.AppEnv, logr)
	nodeAgent := application.NewNodeAgent("local-dev-node", gameSvc, natsClient, minioAdapter, cfg.Debug, cfg.GodotPath, cfg.AppEnv, logr)
	go nodeAgent.Start()

	gameStateListener := appNats.NewGameStateListener(natsClient, hub, cfg.AppEnv, logr)
	go gameStateListener.Start()

	instanceListener := appNats.NewInstanceLifecycleListener(gameSvc, natsClient, hub, cfg.AppEnv, logr)
	go instanceListener.Start()

	router := httpRouter.NewRouter(authSvc, gameSvc, hub, natsClient, provisioningSvc, minioAdapter, logr, cfg)

	// --- Graceful Shutdown Setup ---
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		logr.Infow("Listening on", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logr.Fatalf("Server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	logr.Info("Shutting down gracefully...")

	// 1. Deregister from Eureka
	if err := discovery.DeregisterFromEureka(cfg.Eureka, logr); err != nil {
		logr.Errorw("Failed to deregister from Eureka", "error", err)
	}

	// 2. Shutdown HTTP Server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logr.Errorw("Server forced to shutdown", "error", err)
	}

	logr.Info("Backend stopped")
}
