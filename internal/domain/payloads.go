package domain

// Auth & User Payloads
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

// Game Payloads
type GameInitUploadRequest struct {
	GameName string       `json:"game_name" binding:"required"`
	VMID     string       `json:"vm_id" binding:"required"`
	Manifest GameManifest `json:"manifest" binding:"required"`
}

type InitUploadResponse struct {
	GameID    int    `json:"game_id"`
	UploadURL string `json:"upload_url"`
	ARN       string `json:"arn"`
}

type PlayGameRequest struct {
	GameID int           `json:"game_id" binding:"required"`
	Mode   StreamingMode `json:"streaming_mode"`
}

type PlayGameResponse struct {
	GameID int    `json:"game_id"`
	Status string `json:"status"`
}
