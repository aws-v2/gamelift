package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// =====================
// Models (match frontend)
// =====================

type DocItem struct {
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

type DocCategory struct {
	Title string    `json:"title"`
	Items []DocItem `json:"items"`
}

type DocManifest struct {
	Service   string        `json:"service"`
	Version   string        `json:"version,omitempty"`
	Categories []DocCategory `json:"categories"`
}

type Metadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	LastUpdated string   `json:"lastUpdated"`
	Tags        []string `json:"tags"`
}

type DocResponse struct {
	Metadata Metadata `json:"metadata"`
	Content  string   `json:"content"`
}

// =====================
// Service
// =====================

type DocsService struct {
	basePath string
	logger   *zap.SugaredLogger
}

func NewDocsService(basePath string, logger *zap.SugaredLogger) *DocsService {
	logger.Infow("DocsService initialized",
		"basePath", basePath,
	)

	return &DocsService{
		basePath: basePath,
		logger:   logger,
	}
}

// =====================
// Public API
// =====================

// GetManifestsForRole returns all manifests the user is authorized to see
func (s *DocsService) GetManifestsForRole(role string) (map[string]*DocManifest, error) {
	s.logger.Infow("fetching manifests for role", "role", role)

	res := make(map[string]*DocManifest)

	// Public is always available
	pub, err := s.GetManifest(false)
	if err == nil {
		res["public"] = pub
	}

	// Internal available for admin/system
	if role == "admin" || role == "system" {
		priv, err := s.GetManifest(true)
		if err == nil {
			res["internal"] = priv
		}
	}

	return res, nil
}

// GetDocForRole searches for a document across authorized scopes
func (s *DocsService) GetDocForRole(slug, role string) (*DocResponse, error) {
	s.logger.Infow("fetching document for role", "slug", slug, "role", role)

	// 1. Try public first
	doc, err := s.GetDoc(slug, false)
	if err == nil {
		return doc, nil
	}

	// 2. Try internal if authorized
	if role == "admin" || role == "system" {
		doc, err = s.GetDoc(slug, true)
		if err == nil {
			return doc, nil
		}
	}

	return nil, errors.New("document not found or access denied")
}

// GetManifest loads manifest.json from public/internal folder
func (s *DocsService) GetManifest(internal bool) (*DocManifest, error) {
	scope := s.getScope(internal)
	path := filepath.Join(s.basePath, scope, "manifest.json")

	s.logger.Infow("loading manifest",
		"path", path,
		"internal", internal,
		"scope", scope,
	)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.Warnw("manifest not found",
				"path", path,
			)
			return nil, fmt.Errorf("manifest not found at %q", path)
		}

		s.logger.Errorw("failed to read manifest",
			"path", path,
			"error", err,
		)
		return nil, fmt.Errorf("failed to read manifest at %q: %w", path, err)
	}

	var manifest DocManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		s.logger.Errorw("invalid manifest JSON",
			"path", path,
			"error", err,
		)
		return nil, fmt.Errorf("invalid manifest JSON at %q: %w", path, err)
	}

	s.logger.Infow("manifest loaded",
		"path", path,
		"service", manifest.Service,
		"categories", len(manifest.Categories),
	)

	return &manifest, nil
}

// GetDoc loads a markdown file and parses frontmatter
func (s *DocsService) GetDoc(slug string, internal bool) (*DocResponse, error) {
	start := time.Now()

	if !isValidSlug(slug) {
		s.logger.Warnw("invalid slug rejected",
			"slug", slug,
			"internal", internal,
		)
		return nil, errors.New("invalid slug")
	}

	scope := s.getScope(internal)
	path := filepath.Join(s.basePath, scope, slug+".md")

	s.logger.Debugw("loading document",
		"slug", slug,
		"path", path,
		"internal", internal,
	)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.Warnw("document not found",
				"slug", slug,
				"path", path,
			)
			return nil, errors.New("not found")
		}

		s.logger.Errorw("failed to read document",
			"slug", slug,
			"path", path,
			"error", err,
		)
		return nil, err
	}

	meta, content := parseMarkdownWithFrontmatter(string(data))

	s.logger.Infow("document loaded",
		"slug", slug,
		"path", path,
		"title", meta.Title,
		"tags", len(meta.Tags),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return &DocResponse{
		Metadata: meta,
		Content:  content,
	}, nil
}

// =====================
// Helpers
// =====================

func (s *DocsService) getScope(internal bool) string {
	if internal {
		return "internal"
	}
	return "public"
}

// Prevent path traversal attacks
func isValidSlug(slug string) bool {
	if slug == "" {
		return false
	}
	if strings.Contains(slug, "..") ||
		strings.Contains(slug, "/") ||
		strings.Contains(slug, "\\") {
		return false
	}
	return true
}

// =====================
// Markdown Parser
// =====================

func parseMarkdownWithFrontmatter(input string) (Metadata, string) {
	var meta Metadata

	parts := strings.SplitN(input, "---", 3)

	if len(parts) < 3 {
		meta.LastUpdated = time.Now().Format("2006-01-02")
		return meta, strings.TrimSpace(input)
	}

	rawMeta := parts[1]
	content := parts[2]

	lines := strings.Split(rawMeta, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "title:"):
			meta.Title = cleanValue(line, "title:")
		case strings.HasPrefix(line, "description:"):
			meta.Description = cleanValue(line, "description:")
		case strings.HasPrefix(line, "icon:"):
			meta.Icon = cleanValue(line, "icon:")
		case strings.HasPrefix(line, "tags:"):
			meta.Tags = parseTags(cleanValue(line, "tags:"))
		}
	}

	meta.LastUpdated = time.Now().Format("2006-01-02")

	return meta, strings.TrimSpace(content)
}

func cleanValue(line, prefix string) string {
	val := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	return strings.Trim(val, `"`)
}

func parseTags(input string) []string {
	input = strings.Trim(input, "[]")
	parts := strings.Split(input, ",")

	var tags []string
	for _, t := range parts {
		tag := strings.TrimSpace(strings.Trim(t, `"`))
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}