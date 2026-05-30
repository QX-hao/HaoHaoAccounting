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
	group.POST("/io/import", h.importData)
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

func importLineError(index int, err error) string {
	return fmt.Sprintf("line %d: %v", index+2, err)
}
