package http

import (
	"backend/internal/application"
	"backend/internal/config"
	"backend/internal/infrastructure/messaging"
	"backend/internal/infrastructure/repository"
	"backend/internal/infrastructure/storage"
	"backend/internal/transport/http/handlers"
	"backend/internal/transport/middleware"

	// "backend/internal/transport/middleware"
	"backend/internal/transport/websocket"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(
	authSvc      repository.AuthService,
	gameHandler  *handlers.GameHandler,
	hub          *websocket.Hub,
	natsClient   *messaging.NatsClient,
	provisioningSvc *application.ProvisioningService,
	storage      *storage.MinIOAdapter,
	logger       *zap.SugaredLogger,
	cfg          *config.Config,
) *gin.Engine {
	r := gin.Default()

	authHandler   := handlers.NewAuthHandler(authSvc, logger)
	wsHandler     := websocket.NewWebSocketHandler(hub)
	webrtcHandler := handlers.NewWebRTCSignalingHandler(logger)
	docsSvc       := application.NewDocsService(cfg.DocsPath)
	docsHandler   := handlers.NewDocsHandler(docsSvc)

	base := r.Group("/api/v1/gamelift")
	base.Use(middleware.AuthMiddleware())

	// ── 1. Docs ───────────────────────────────────────────────────────────────
	{
		pub := base.Group("/docs")
		pub.GET("", docsHandler.GetPublicManifest)
		pub.GET("/:slug", docsHandler.GetPublicDoc)

		priv := r.Group("/api/v1/internal/docs")
		priv.GET("", docsHandler.GetInternalManifest)
		priv.GET("/:slug", docsHandler.GetInternalDoc)
	}

	// ── 2. WebSockets + WebRTC ────────────────────────────────────────────────
	{
		r.GET("/api/v1/ws", wsHandler.Handle)
		r.GET("/ws", wsHandler.Handle)
		r.GET("/api/v1/webrtc/signaling", webrtcHandler.HandleSignaling)
	}

	// ── 3. Sessions ───────────────────────────────────────────────────────────
	{
		sessions := base.Group("/games/:id/session")
		sessions.POST("", gameHandler.CreateSession)
		sessions.GET("/status", gameHandler.GetSessionStatus)

		base.POST("/games/play", gameHandler.PlayGame)
	}

	// ── 4. Games ──────────────────────────────────────────────────────────────
	{
		base.POST("/login", authHandler.Login)

		games := base.Group("/games")
		games.GET("", gameHandler.ListGames)
		games.GET("/:id", gameHandler.GetGame)
		games.GET("/:id/manifest", gameHandler.GetManifest)
		games.GET("/:id/package", gameHandler.DownloadPackage)
		games.Static("/static", "./uploads/games")

		protected := games.Group("/")
		// protected.Use(AuthMiddleware(authSvc))
		protected.POST("/init-upload", gameHandler.InitUpload)
		protected.POST("", gameHandler.CreateGame)
		protected.PUT("/:id", gameHandler.UpdateGame)
		protected.DELETE("/:id", gameHandler.DeleteGame)
	}

	return r
}