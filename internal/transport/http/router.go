package http

import (
	"backend/internal/infrastructure/storage"
	"backend/internal/interfaces"
	"backend/internal/messaging"
	"backend/internal/transport/http/handlers"
	"backend/internal/transport/websocket"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(
	authSvc interfaces.AuthService, 
	gameSvc interfaces.GameRepository, 
	hub *websocket.Hub, 
	natsClient *messaging.NatsClient, 
	provisioningSvc interfaces.ProvisioningService,
	storage *storage.MinIOAdapter,
	logger *zap.SugaredLogger,
) *gin.Engine {
	r := gin.Default()

	// CORS is handled by the API Gateway. 
	// Disabling local CORS to prevent duplicate Access-Control-Allow-Origin headers.
	/*
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:5173"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	config.AllowCredentials = true
	r.Use(cors.New(config))
	*/

	authHandler := handlers.NewAuthHandler(authSvc, logger)
	gameHandler := handlers.NewGameHandler(gameSvc, natsClient, provisioningSvc, storage, logger)
	wsHandler := websocket.NewWebSocketHandler(hub)
	webrtcHandler := handlers.NewWebRTCSignalingHandler(logger)

	// API Group with prefix
	api := r.Group("/api/v1/gamelift")
	{
		api.POST("/login", authHandler.Login)
		api.GET("/games", gameHandler.ListGames)
		api.GET("/games/:id/manifest", gameHandler.GetManifest)
		api.GET("/games/:id/package", gameHandler.DownloadPackage)

		// Protected routes within the group
		protected := api.Group("/")
		// protected.Use(AuthMiddleware(authSvc))
		{
			protected.POST("/games/init-upload", gameHandler.InitUpload)
			protected.POST("/games/play", gameHandler.PlayGame)
		}

		// Static assets within the group
		api.Static("/game_static", "./uploads/games")
	}

	// Public WebSocket for state-streaming
	r.GET("/api/v1/ws", wsHandler.Handle) // Direct match for Gateway requests
	r.GET("/ws", wsHandler.Handle)        // Legacy / fallback match

	// WebRTC Signaling
	r.GET("/api/v1/webrtc/signaling", webrtcHandler.HandleSignaling)

	return r
}
