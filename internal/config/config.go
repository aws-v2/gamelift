package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"backend/pkg/database"

	"github.com/joho/godotenv"
)

type EurekaConfig struct {
	ServerURL         string
	AppName           string
	HostName          string
	IPAddr            string
	Port              int
	VipAddress        string
	InstanceID        string
	HeartbeatInterval time.Duration
}

type Config struct {
	AppEnv        string
	ServerPort    string
	JWTSecret     []byte
	JWTExpiration time.Duration
	NatsURL       string
	NatsUser      string
	NatsPassword  string
	PublicURL     string
	Debug         bool
	GodotPath     string
	Eureka        EurekaConfig
	DB            database.Config
	S3            struct {
		Endpoint  string
		AccessKey string
		SecretKey string
		UseSSL    bool
	}
	DocsPath string
	NatsPrefix string
}

func Load() *Config {
	// Load .env if it exists
	_ = godotenv.Load()
	port := getEnv("SERVER_PORT", ":8091")
	if !strings.HasPrefix(port, ":") {
	port = ":" + port
}
	

	cfg := &Config{
		AppEnv:        getEnv("APP_ENV", "dev"),
		ServerPort:    port,
		NatsURL:       getEnv("NATS_URL", "nats://nats-prod:4222"),
		NatsUser:      getEnv("NATS_USER", "auth-server"),
		NatsPassword:  getEnv("NATS_PASSWORD", "auth-secret"),
		PublicURL:     getEnv("PUBLIC_URL", "http://localhost:8091"),
		Debug:         getEnv("DEBUG_MODE", "false") == "false",
		GodotPath:     getEnv("GODOT_PATH", "/usr/local/bin/godot"),
		JWTExpiration: time.Duration(getEnvInt("JWT_EXPIRATION_MS", 86400000)) * time.Millisecond,
		DB: database.Config{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			Name:     getEnv("DB_NAME", "gamelift"),
		},
		DocsPath: getEnv("DOCS_PATH", "./docs"),
		NatsPrefix: getEnv("NATS_PREFIX", "dev.v1"),	
	}

	rawSecret := "404E635266556A586E3272357538782F413F4428472B4B6250645367566B5970404E635266556A586E3272357538782F413F4428472B4B6250645367566B5970"
	secret, err := hex.DecodeString(rawSecret)
	if err != nil {
		// Fallback to raw bytes if not valid hex
		cfg.JWTSecret = []byte(rawSecret)
	} else {
		cfg.JWTSecret = secret
	}

	cfg.Eureka = EurekaConfig{
		ServerURL:         getEnv("EUREKA_SERVER_URL", "http://localhost:8761/eureka"),
		AppName:           getEnv("EUREKA_APP_NAME", "gamelift-server"),
		HostName:          getEnv("EUREKA_HOSTNAME", "localhost"),
		IPAddr:            getEnv("EUREKA_IP_ADDR", "127.0.0.1"),
		Port:              getEnvInt("SERVICE_PORT", 8091),
		VipAddress:        getEnv("EUREKA_VIP_ADDRESS", "gamelift-server"),
		InstanceID:        getEnv("EUREKA_INSTANCE_ID", "localhost:8091"),
		HeartbeatInterval: 30 * time.Second,
	}
 

	cfg.S3.Endpoint = getEnv("S3_ENDPOINT", "localhost:9000")
	cfg.S3.AccessKey = getEnv("S3_ACCESS_KEY", "minioadmin")
	cfg.S3.SecretKey = getEnv("S3_SECRET_KEY", "minioadmin123")
	cfg.S3.UseSSL = getEnv("S3_USE_SSL", "false") == "true"
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		fmt.Sscanf(v, "%d", &i)
		if i > 0 {
			return i
		}
	}
	return fallback
}

