package service

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"backend/internal/domain"
)

type ValidationService struct{}

func NewValidationService() *ValidationService {
	return &ValidationService{}
}

func (s *ValidationService) ValidateStructure(extractDir string, manifest domain.GameManifest) error {
	requiredFolders := []string{"server", "client", "client/levels", "client/characters"}
	for _, folder := range requiredFolders {
		if _, err := os.Stat(filepath.Join(extractDir, folder)); err != nil {
			return fmt.Errorf("missing required folder: %s", folder)
		}
	}

	requiredFiles := []string{
		manifest.HeadlessBin,
		manifest.MainScene,
		"server/game.pck",
	}
	for _, file := range requiredFiles {
		if _, err := os.Stat(filepath.Join(extractDir, file)); err != nil {
			return fmt.Errorf("missing required file: %s", file)
		}
	}

	return nil
}

func (s *ValidationService) Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}
