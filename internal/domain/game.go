package domain

import "time"

type GameStatus string

const (
	GameStatusPending      GameStatus = "pending"
	GameStatusStored       GameStatus = "stored"
	GameStatusProvisioning GameStatus = "provisioning"
	GameStatusActive       GameStatus = "active"
	GameStatusFailed       GameStatus = "failed"
	GameStatusInactive GameStatus = "inactive"

)

type StreamingMode string

const (
	StreamingModeState StreamingMode = "state_sync"
	StreamingModeVideo StreamingMode = "video_streaming"
)

type Game struct {
	
	ID             int        `json:"id" gorm:"primaryKey"`
	Name           string     `json:"game_name" gorm:"not null"`
	FolderLocation string     `json:"game_folder_location"`
	VMID           string     `json:"vm_id"`
	UserID         string     `json:"user_id"`
	ARN            string     `json:"arn" gorm:"uniqueIndex"`
	Status         GameStatus    `json:"status"`
	StreamingMode  StreamingMode `json:"streaming_mode"`
	StorageARN     string        `json:"storage_arn,omitempty"`
	Manifest       string     `json:"manifest,omitempty" gorm:"type:text"`
CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	}

type SyncNode struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type GameManifest struct {
	Name        string     `json:"name"`
	Version     string     `json:"version"`
	HeadlessBin string     `json:"headless_bin"`
	MainScene   string     `json:"main_scene"`
	PlayerNode  string     `json:"player_node"`
	SyncNodes   []SyncNode `json:"sync_nodes"`
}
type CreateSessionRequest struct {
	GameID    string `json:"game_id" binding:"required"`
	GameImage string `json:"game_image" binding:"required"`
	UserID    string `json:"user_id"`
}

type GameSession struct {
	ID          string    `db:"id"`
	GameID      string    `db:"game_id"`
	UserID      string    `db:"user_id"`
	Status      string    `db:"status"`       // provisioning | ready | closed
	AgentWSURL  string    `db:"agent_ws_url"` // ws://agent-ip:port/game — empty until ready
	Token       string    `db:"token"`        // short-lived JWT or UUID for agent auth
	NodeID      string    `db:"node_id"`      // which bare metal node the VM landed on
	CreatedAt   time.Time `db:"created_at"`
	ExpiresAt   time.Time `db:"expires_at"`
}

 
 
type Session struct {
	ID        string    `gorm:"primaryKey"`
	GameID    string    `gorm:"column:game_id"`
	UserID    string    `gorm:"column:user_id"`
	Status    string    `gorm:"column:status"`
	NodeID    string    `gorm:"column:node_id"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}