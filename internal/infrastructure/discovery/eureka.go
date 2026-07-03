package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"backend/internal/config"
)
 
// RegisterWithEureka registers the service instance with Eureka server
func RegisterWithEureka(cfg config.EurekaConfig, logger *zap.SugaredLogger) error {
	instance := map[string]interface{}{

		"instance": map[string]interface{}{
			"instanceId": cfg.InstanceID,
			"hostName":   cfg.HostName,
			"app":        cfg.AppName,
			"ipAddr":     cfg.IPAddr,
			"vipAddress": cfg.VipAddress,
			"status":     "UP",
			"port": map[string]interface{}{
				"$":        cfg.Port,
				"@enabled": "true",
			},
			"dataCenterInfo": map[string]interface{}{
				"@class": "com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo",
				"name":   "MyOwn",
			},
			"healthCheckUrl": fmt.Sprintf("http://%s:%d/health", cfg.HostName, cfg.Port),
			"statusPageUrl":  fmt.Sprintf("http://%s:%d/health", cfg.HostName, cfg.Port),
			"homePageUrl":    fmt.Sprintf("http://%s:%d/", cfg.HostName, cfg.Port),
		},
	}

	jsonData, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("failed to marshal Eureka registration data: %w", err)
	}

	url := fmt.Sprintf("%s/apps/%s", cfg.ServerURL, cfg.AppName)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create registration request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// TODO: add some sort of security for services registering with Eureka
	req.Header.Set("Authorization","Bearer ")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register with Eureka: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("eureka registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	logger.Infow("Successfully registered with Eureka server", "url", url)
	return nil
}

// SendHeartbeat sends periodic heartbeats to Eureka server
func SendHeartbeat(cfg config.EurekaConfig, logger *zap.SugaredLogger) {
	ticker := time.NewTicker(cfg.HeartbeatInterval)
	defer ticker.Stop()

	url := fmt.Sprintf("%s/apps/%s/%s", cfg.ServerURL, cfg.AppName, cfg.InstanceID)
	client := &http.Client{Timeout: 5 * time.Second}

	for range ticker.C {
		req, err := http.NewRequest("PUT", url, nil)
		if err != nil {
			logger.Errorw("Failed to create heartbeat request", "error", err)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			logger.Errorw("Failed to send heartbeat to Eureka", "error", err)
			continue
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			body, _ := io.ReadAll(resp.Body)
			logger.Warnw("Heartbeat failed", "status", resp.StatusCode, "body", string(body))
		} else {
			logger.Debugw("Heartbeat sent successfully to Eureka")
		}

		resp.Body.Close()
	}
}


func DeregisterFromEureka(cfg config.EurekaConfig, logger *zap.SugaredLogger) error {
	url := fmt.Sprintf("%s/apps/%s/%s", cfg.ServerURL, cfg.AppName, cfg.InstanceID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create deregistration request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to deregister from Eureka: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deregistration failed with status %d: %s", resp.StatusCode, string(body))
	}

	logger.Infow("Successfully deregistered from Eureka server")
	return nil
}
