package dataio

import (
	"fmt"
	"net/http"
	"strconv"
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
	group.POST("/io/import/jobs", h.createImportJob)
	group.GET("/io/import/jobs", h.listImportJobs)
	group.GET("/io/import/jobs/:id", h.getImportJob)
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

	result, err := h.service.ImportWithOptions(uid, file, ImportOptions{
		SkipDuplicates: parseBoolDefault(c.PostForm("skipDuplicates"), true),
	})
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

func (h *Handler) createImportJob(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	job, err := h.service.StartImportJob(uid, file, ImportOptions{
		SkipDuplicates: parseBoolDefault(c.PostForm("skipDuplicates"), true),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, job)
}

func (h *Handler) listImportJobs(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	jobs, err := h.service.ListImportJobs(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, jobs)
}

func (h *Handler) getImportJob(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id := uint(parseInt(c.Param("id")))
	job, err := h.service.ImportJob(uid, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "import job not found"})
		return
	}
	c.JSON(http.StatusOK, job)
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

func parseBoolDefault(value string, fallback bool) bool {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(clean)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}
