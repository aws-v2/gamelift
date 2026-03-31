package domain

type GameStatus string

const (
	GameStatusPending      GameStatus = "pending"
	GameStatusStored       GameStatus = "stored"
	GameStatusProvisioning GameStatus = "provisioning"
	GameStatusActive       GameStatus = "active"
)

type Game struct {
	ID             int        `json:"id" gorm:"primaryKey"`
	Name           string     `json:"game_name" gorm:"not null"`
	FolderLocation string     `json:"game_folder_location"`
	VMID           string     `json:"vm_id"`
	ARN            string     `json:"arn" gorm:"uniqueIndex"`
	Status         GameStatus `json:"status"`
	StorageARN     string     `json:"storage_arn,omitempty"`
	Manifest       string     `json:"manifest,omitempty" gorm:"type:text"`
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
