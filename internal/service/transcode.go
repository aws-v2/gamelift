package service

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type TranscodeService struct{}

func NewTranscodeService() *TranscodeService {
	return &TranscodeService{}
}

func (s *TranscodeService) Transcode(mp4Path string) error {
	dir := filepath.Dir(mp4Path)
	ivfPath := filepath.Join(dir, "game.ivf")
	oggPath := filepath.Join(dir, "game.ogg")

	log.Printf("[Transcoder] Starting background transcoding for %s", mp4Path)

	// Ensure the directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory for transcoding doesn't exist: %s", dir)
	}

	// 1. Convert Video to IVF (VP8)
	videoCmd := exec.Command("ffmpeg",
		"-y",
		"-i", mp4Path,
		"-c:v", "libvpx",
		"-b:v", "2M",
		"-an",
		"-f", "ivf",
		ivfPath,
	)
	if err := videoCmd.Run(); err != nil {
		return fmt.Errorf("video transcode failed: %w", err)
	}

	// 2. Convert Audio to OGG (Opus)
	audioCmd := exec.Command("ffmpeg",
		"-y",
		"-i", mp4Path,
		"-vn",
		"-c:a", "libopus",
		"-b:a", "128k",
		"-ar", "48000",
		"-ac", "2",
		oggPath,
	)
	if err := audioCmd.Run(); err != nil {
		log.Printf("[Transcoder] Audio transcode failed (likely no audio track): %v", err)
		// We don't return error here, as video might still be usable
	}

	log.Printf("[Transcoder] Successfully transcoded %s to IVF/OGG", mp4Path)
	return nil
}
