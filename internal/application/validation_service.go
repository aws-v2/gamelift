package application

import (
	"archive/zip"
	"backend/internal/domain"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

type ValidationService struct {
	logger *zap.SugaredLogger
}

func NewValidationService(logger *zap.SugaredLogger) *ValidationService {
	return &ValidationService{logger: logger}
}

func (s *ValidationService) ValidateStructure(extractDir string, manifest domain.GameManifest) error {
	s.logger.Infow("VALIDATION_STRUCTURE_STARTING",
		"extract_dir", extractDir,
		"headless_bin", manifest.HeadlessBin,
		"main_scene", manifest.MainScene,
	)

	requiredFolders := []string{"server", "client", "client/levels", "client/characters"}
	for _, folder := range requiredFolders {
		path := filepath.Join(extractDir, folder)
		if _, err := os.Stat(path); err != nil {
			s.logger.Errorw("VALIDATION_MISSING_FOLDER",
				"extract_dir", extractDir,
				"missing_folder", folder,
				"error", err,
			)
			return fmt.Errorf("missing required folder: %s", folder)
		}
		s.logger.Debugw("VALIDATION_FOLDER_OK", "folder", folder)
	}

	pckPath := strings.TrimSuffix(manifest.HeadlessBin, filepath.Ext(manifest.HeadlessBin)) + ".pck"

	requiredFiles := []string{
		manifest.HeadlessBin,
		manifest.MainScene,
		pckPath,
	}
	for _, file := range requiredFiles {
		path := filepath.Join(extractDir, file)
		if _, err := os.Stat(path); err != nil {
			s.logger.Errorw("VALIDATION_MISSING_FILE",
				"extract_dir", extractDir,
				"missing_file", file,
				"error", err,
			)
			return fmt.Errorf("missing required file: %s", file)
		}
		s.logger.Debugw("VALIDATION_FILE_OK", "file", file)
	}

	s.logger.Infow("VALIDATION_STRUCTURE_PASSED",
		"extract_dir", extractDir,
		"folders_checked", len(requiredFolders),
		"files_checked", len(requiredFiles),
	)
	return nil
}

func (s *ValidationService) Unzip(src, dest string) error {
	s.logger.Infow("UNZIP_STARTING", "src", src, "dest", dest)

	r, err := zip.OpenReader(src)
	if err != nil {
		s.logger.Errorw("UNZIP_OPEN_FAILED", "src", src, "error", err)
		return err
	}
	defer r.Close()

	s.logger.Infow("UNZIP_OPENED", "src", src, "total_entries", len(r.File))

	extracted := 0
	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				s.logger.Errorw("UNZIP_MKDIR_FAILED", "path", fpath, "error", err)
				return err
			}
			s.logger.Debugw("UNZIP_DIR_CREATED", "path", fpath)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			s.logger.Errorw("UNZIP_MKDIR_FAILED", "path", filepath.Dir(fpath), "error", err)
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			s.logger.Errorw("UNZIP_FILE_CREATE_FAILED", "path", fpath, "error", err)
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			s.logger.Errorw("UNZIP_ENTRY_OPEN_FAILED", "entry", f.Name, "error", err)
			return err
		}

		written, err := io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			s.logger.Errorw("UNZIP_COPY_FAILED", "entry", f.Name, "dest", fpath, "error", err)
			return err
		}

		s.logger.Debugw("UNZIP_FILE_EXTRACTED", "entry", f.Name, "dest", fpath, "bytes", written)
		extracted++
	}

	s.logger.Infow("UNZIP_SUCCESS", "src", src, "dest", dest, "files_extracted", extracted)
	return nil
}