package handlers

import (
	"backend/internal/application"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DocsHandler struct {
	service *application.DocsService
	log     *zap.SugaredLogger
}

func NewDocsHandler(service *application.DocsService, log *zap.SugaredLogger) *DocsHandler {
	return &DocsHandler{service: service, log: log}
}

func (h *DocsHandler) GetManifests(c *gin.Context) {
	role, _ := c.Get("userRole")
	roleStr, _ := role.(string)

	h.log.Infow("HANDLER_GET_MANIFESTS", "role", roleStr)

	data, err := h.service.GetManifestsForRole(roleStr)
	if err != nil {
		h.log.Errorw("HANDLER_GET_MANIFESTS_FAILED", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.log.Infow("HANDLER_GET_MANIFESTS_SUCCESS", "role", roleStr)
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *DocsHandler) GetDoc(c *gin.Context) {
	slug := c.Param("slug")
	role, _ := c.Get("userRole")
	roleStr, _ := role.(string)

	h.log.Infow("HANDLER_GET_DOC", "slug", slug, "role", roleStr)

	doc, err := h.service.GetDocForRole(slug, roleStr)
	if err != nil {
		h.log.Warnw("HANDLER_GET_DOC_NOT_FOUND", "slug", slug, "role", roleStr, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "not found or access denied"})
		return
	}

	h.log.Infow("HANDLER_GET_DOC_SUCCESS", "slug", slug, "role", roleStr)
	c.JSON(http.StatusOK, gin.H{"data": doc})
}


func (h *DocsHandler) GetInternalDoc(c *gin.Context) {
	slug := c.Param("slug")

	h.log.Infow("HANDLER_GET_INTERNAL_DOC", "slug", slug)

	doc, err := h.service.GetDoc(slug, true)
	if err != nil {
		h.log.Warnw("HANDLER_GET_INTERNAL_DOC_NOT_FOUND", "slug", slug, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	h.log.Infow("HANDLER_GET_INTERNAL_DOC_SUCCESS", "slug", slug)
	c.JSON(http.StatusOK, gin.H{"data": doc})
}