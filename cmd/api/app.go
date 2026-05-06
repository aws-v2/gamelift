package main

import (
	// "backend/internal/application"
	// "backend/internal/config"
	// "backend/internal/infrastructure/messaging"
	// "backend/internal/infrastructure/repository"
	// "backend/internal/infrastructure/storage"
	// httpRouter "backend/internal/transport/http"
	// "backend/internal/transport/http/handlers"
	// appNats "backend/internal/transport/nats"
	// "backend/internal/transport/websocket"
	// "backend/pkg/database"
	// "net/http"

	// "github.com/nats-io/nats.go"
	// "go.uber.org/zap"
)

// // Container holds all wired-up application components.
// type Container struct {
// 	Config *config.Config
// 	Router http.Handler

// 	Hub               *websocket.Hub
// 	S3Listener        *appNats.S3Listener
// 	NodeAgent         *application.NodeAgent
// 	GameStateListener *appNats.GameStateListener
// 	InstanceListener  *appNats.InstanceLifecycleListener
// }

// // NewContainer wires every dependency and returns a ready Container.
// func NewContainer(cfg *config.Config, db *database.DB, nc *nats.Conn, logr *zap.SugaredLogger) *Container {
// 	// ── infrastructure ────────────────────────────────────────────────────────
// 	natsClient := messaging.NewNatsClient(nc, logr, cfg.NatsPrefix)

// 	minioAdapter, err := storage.NewMinIOAdapter(
// 		cfg.S3.Endpoint,
// 		cfg.S3.AccessKey,
// 		cfg.S3.SecretKey,
// 		cfg.S3.UseSSL,
// 	)
// 	if err != nil {
// 		logr.Fatalw("failed to connect to MinIO", "error", err)
// 	}

// 	// ── repositories ─────────────────────────────────────────────────────────
// 	gameRepo    := repository.NewPostgresGameRepository(db, logr)

// 	// ── Game service + handler ────────────────────────────────────────────────
// 	gameService := application.NewGameService(gameRepo, logr)
// 	gameHandler := handlers.NewGameHandler(gameService, logr)




// 	// ── services ─────────────────────────────────────────────────────────────
// 	authSvc         := application.NewAuthService(cfg, logr)
// 	validationSvc   := application.NewValidationService()
// 	provisioningSvc := application.NewProvisioningService(gameRepo, natsClient, minioAdapter, cfg.Debug, cfg.GodotPath, cfg.PublicURL, cfg.AppEnv, logr)
// 	// sessionSvc      := application.NewSessionService(sessionRepo, provisioningSvc, logr, cfg.Debug)

// 	// ── websocket hub ─────────────────────────────────────────────────────────
// 	hub := websocket.NewHub(logr)

// 	// ── background listeners ──────────────────────────────────────────────────
// 	s3Listener        := appNats.NewS3Listener(gameRepo, natsClient, validationSvc, minioAdapter, cfg.PublicURL, cfg.AppEnv, logr)
// 	nodeAgent         := application.NewNodeAgent("local-dev-node", gameRepo, natsClient, minioAdapter, cfg.Debug, cfg.GodotPath, cfg.AppEnv, logr)
// 	gameStateListener := appNats.NewGameStateListener(natsClient, hub, cfg.AppEnv, logr)
// 	instanceListener  := appNats.NewInstanceLifecycleListener(gameRepo, natsClient, hub, cfg.AppEnv, logr)



// 	router := httpRouter.NewRouter(authSvc, gameHandler, hub, natsClient, provisioningSvc, minioAdapter, logr, cfg)

// 	return &Container{
// 		Config:            cfg,
// 		Router:            router,
// 		Hub:               hub,
// 		S3Listener:        s3Listener,
// 		NodeAgent:         nodeAgent,
// 		GameStateListener: gameStateListener,
// 		InstanceListener:  instanceListener,
// 	}
// }

// // Start launches all background goroutines.
// func (c *Container) Start() {
// 	go c.Hub.Run()
// 	go c.S3Listener.Start()
// 	go c.NodeAgent.Start()
// 	go c.GameStateListener.Start()
// 	go c.InstanceListener.Start()
// }