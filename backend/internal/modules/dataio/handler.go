package dataio

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/timeutil"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	group.POST("/io/import/preview", h.previewImport)
	group.POST("/io/import/text/preview", h.previewImportText)
	group.POST("/io/import", h.importData)
	group.POST("/io/import/text", h.importText)
	group.GET("/io/export", h.exportData)
}

func (h *Handler) exportData(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "csv")))
	start, end := timeutil.ResolveRange(c.Query("start"), c.Query("end"))

	rows, err := h.service.ExportRows(uid, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if format == "xlsx" {
		if err := writeXLSX(c, rows); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	writeCSV(c, rows)
}

func (h *Handler) importData(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	result, err := h.service.Import(uid, file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) previewImport(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	result, err := h.service.Preview(uid, file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) importText(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req ImportTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	result, err := h.service.ImportText(uid, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) previewImportText(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req ImportTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	result, err := h.service.PreviewText(uid, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func importLineError(index int, err error) string {
	return fmt.Sprintf("line %d: %v", index+2, err)
}
