package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"backend/internal/application"
	"backend/internal/config"
	"backend/internal/infrastructure/messaging"
	"backend/internal/infrastructure/repository"
	"backend/internal/infrastructure/storage"
	httpRouter "backend/internal/transport/http"
	"backend/internal/transport/http/handlers"
	appNats "backend/internal/transport/nats"
	"backend/internal/transport/websocket"
	"backend/pkg/database"
	// "net/http"
 
	"backend/internal/infrastructure/discovery"
	// "backend/pkg/database"
	"backend/pkg/logger"
	"backend/migrations"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func main() {
	logr := logger.NewSugared()
	defer logr.Sync()

	cfg := config.Load()

	// ── Eureka registration ───────────────────────────────────────────────────
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
	go discovery.SendHeartbeat(cfg.Eureka, logr)

	// ── NATS ─────────────────────────────────────────────────────────────────
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
	logr.Infow("Connected to NATS", "url", cfg.NatsURL)

	// ── Database (PostgreSQL → SQLite fallback) ───────────────────────────────
	var db *database.DB
	for attempt := 1; attempt <= 4; attempt++ {
		db, err = database.ConnectPostgres(cfg.DB, logr)
		if err == nil {
			logr.Info("Connected to PostgreSQL")
			break
		}
		logr.Warnw("PostgreSQL connection failed, retrying", "attempt", attempt, "error", err)
		if attempt < 4 {
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		logr.Warn("All PostgreSQL attempts failed, falling back to SQLite")
		sqlitePath := os.Getenv("SQLITE_PATH")
		if sqlitePath == "" {
			sqlitePath = "lambda.db"
		}
		db, err = database.ConnectSQLite(sqlitePath, logr)
		if err != nil {
			logr.Fatalw("Failed to connect to SQLite fallback", "error", err)
		}
		logr.Infow("Connected to SQLite fallback", "path", sqlitePath)
	}
	defer db.Close()

	// ── Migrations ────────────────────────────────────────────────────────────
	logr.Info("Running database migrations...")
	if err := db.Migrate(migrations.MigrationFS, "sql"); err != nil {
		logr.Fatalw("Failed to run database migrations", "error", err)
	}
 

	// ── Wire + start application ──────────────────────────────────────────────
	c := NewContainer(cfg, db, nc, logr)
	c.Start()

	// ── HTTP server ───────────────────────────────────────────────────────────
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: c.Router,
	}
	go func() {
		logr.Infow("Listening", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logr.Fatalf("Server failed", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	<-ctx.Done()
	logr.Info("Shutting down gracefully...")

	if err := discovery.DeregisterFromEureka(cfg.Eureka, logr); err != nil {
		logr.Errorw("Failed to deregister from Eureka", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logr.Errorw("Server forced to shutdown", "error", err)
	}

	logr.Info("Backend stopped")
}


// Container holds all wired-up application components.
type Container struct {
	Config *config.Config
	Router http.Handler

	Hub               *websocket.Hub
	S3Listener        *appNats.S3Listener
	NodeAgent         *application.NodeAgent
	GameStateListener *appNats.GameStateListener
	InstanceListener  *appNats.InstanceLifecycleListener
	SSERegistry       *application.SSERegistry
}

// NewContainer wires every dependency and returns a ready Container.
func NewContainer(cfg *config.Config, db *database.DB, nc *nats.Conn, logr *zap.SugaredLogger) *Container {
	// ── infrastructure ────────────────────────────────────────────────────────
	natsClient := messaging.NewNatsClient(nc, logr, cfg.NatsPrefix)

	minioAdapter, err := storage.NewMinIOAdapter(
		cfg.S3.Endpoint,
		cfg.S3.AccessKey,
		cfg.S3.SecretKey,
		cfg.S3.UseSSL,
		logr,
	)
	if err != nil {
		logr.Fatalw("failed to connect to MinIO", "error", err)
	}


	// ── repositories ─────────────────────────────────────────────────────────
	gameRepo    := repository.NewPostgresGameRepository(db, logr)
	sessionRepo := repository.NewPostgresSessionRepository(db, logr)



	// ── services ─────────────────────────────────────────────────────────────
	validationSvc   := application.NewValidationService(logr)
	provisioningSvc := application.NewProvisioningService(gameRepo, natsClient, minioAdapter, cfg.Debug, cfg.GodotPath, cfg.PublicURL, cfg.AppEnv, logr,cfg.NatsPrefix, cfg.VMAssetPath,cfg)
	sessionSvc      := application.NewSessionService(sessionRepo,provisioningSvc, logr, cfg.Debug, natsClient)

	// ── websocket hub ─────────────────────────────────────────────────────────
	hub := websocket.NewHub(logr)

	// ── SSE registry ─────────────────────────────────────────────────────────
	sseRegistry := application.NewSSERegistry(logr)


	// ── Game service + handler ────────────────────────────────────────────────
	gameService := application.NewGameService(gameRepo, logr, natsClient, sessionSvc, sseRegistry)
	gameHandler := handlers.NewGameHandler(gameService, logr, sseRegistry)





	// ── background listeners ──────────────────────────────────────────────────
	s3Listener        := appNats.NewS3Listener(gameRepo, natsClient, validationSvc, minioAdapter, cfg.PublicURL, cfg.AppEnv, logr, sseRegistry)
	nodeAgent         := application.NewNodeAgent("local-dev-node", gameRepo, natsClient, minioAdapter, cfg.Debug, cfg.GodotPath, cfg.AppEnv, logr)
	gameStateListener := appNats.NewGameStateListener(natsClient, hub, cfg.AppEnv, logr)
	instanceListener  := appNats.NewInstanceLifecycleListener(gameRepo, natsClient, hub, cfg.AppEnv, logr)



	router := httpRouter.NewRouter( gameHandler, hub, natsClient, provisioningSvc, minioAdapter, logr, cfg,)

	return &Container{
		Config:            cfg,
		Router:            router,
		Hub:               hub,
		S3Listener:        s3Listener,
		NodeAgent:         nodeAgent,
		GameStateListener: gameStateListener,
		InstanceListener:  instanceListener,
		SSERegistry:       sseRegistry,
	}
}

// Start launches all background goroutines.
func (c *Container) Start() {
	go c.Hub.Run()
	go c.S3Listener.Start()
	go c.NodeAgent.Start()
	go c.GameStateListener.Start()
	go c.InstanceListener.Start()
}