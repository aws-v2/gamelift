package domain

// InstanceLifecycleEvent represents the payload received from the EC2 service
// detailing the status of a VM provisioning or lifecycle event.
type InstanceLifecycleEvent struct {
	InstanceID string `json:"instance_id"`
	EventType  string `json:"event_type"`
	Stage      string `json:"stage"`
	Message    string `json:"message"`
	Timestamp  string `json:"timestamp"`
}

// S3StoredEvent represents the metadata received when a game is successfully
// uploaded to MinIO/S3.
type S3StoredEvent struct {
	GameID      int    `json:"game_id"`
	StorageARN  string `json:"s3_arn"`
	DownloadURL string `json:"download_url"`
	Status      string `json:"status"`
}

// EC2ProvisionRequest represents the payload dispatched to request a new
// VM instance from the compute service.
type EC2ProvisionRequest struct {
	Profile    string            `json:"profile"`
	Specs      map[string]int    `json:"specs"`
	Parameters map[string]string `json:"parameters"`
	UserID     string            `json:"user_id"`
	StorageARN string            `json:"storage_arn"`
	Manifest   GameManifest      `json:"manifest"`
}

// S3PresignedURLResponse defines the expected response from the S3 service
// containing the temporary upload link.
type S3PresignedURLResponse struct {
	UploadURL string `json:"upload_url"`
}

// ProvisionGameRequest defines the internal request to start a game on a node.
type ProvisionGameRequest struct {
	GameID        int           `json:"game_id"`
	StorageARN    string        `json:"storage_arn"`
	TargetNode    string        `json:"target_node"`
	StreamingMode StreamingMode `json:"streaming_mode"`
}

// GameReadyEvent is published when a game server is fully launched and listening.
type GameReadyEvent struct {
	GameID int    `json:"game_id"`
	NodeID string `json:"node_id"`
	Port   int    `json:"port"`
}
