package dataio

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/queryutil"
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
	var query exportQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		httputil.InvalidRequest(c, "invalid query parameters")
		return
	}
	format := strings.TrimSpace(query.Format)
	if format == "" {
		format = "csv"
	}
	start, end, err := timeutil.ResolveRangeStrict(query.Start, query.End)
	if err != nil {
		httputil.InvalidRequest(c, err.Error())
		return
	}

	rows, err := h.service.ExportRows(c.Request.Context(), uid, start, end)
	if err != nil {
		httputil.InternalError(c, err)
		return
	}

	if format == "xlsx" {
		if err := writeXLSX(c, rows); err != nil {
			httputil.InternalError(c, err)
		}
		return
	}
	if err := writeCSV(c, rows); err != nil {
		httputil.InternalError(c, err)
	}
}

func (h *Handler) importData(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	file, err := c.FormFile("file")
	if err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.BadRequest(c, "file is required")
		return
	}

	result, err := h.service.ImportWithOptions(c.Request.Context(), uid, file, ImportOptions{
		SkipDuplicates: parseBoolDefault(c.PostForm("skipDuplicates"), true),
	})
	if err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) previewImport(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	file, err := c.FormFile("file")
	if err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.BadRequest(c, "file is required")
		return
	}

	result, err := h.service.Preview(c.Request.Context(), uid, file)
	if err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) createImportJob(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	file, err := c.FormFile("file")
	if err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.BadRequest(c, "file is required")
		return
	}

	job, err := h.service.StartImportJob(c.Request.Context(), uid, file, ImportOptions{
		SkipDuplicates: parseBoolDefault(c.PostForm("skipDuplicates"), true),
	})
	if err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}
	httputil.SetCreatedLocation(c, job.ID)
	c.JSON(http.StatusAccepted, job)
}

func (h *Handler) listImportJobs(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	jobs, err := h.service.ListImportJobs(c.Request.Context(), uid)
	if err != nil {
		httputil.InternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, jobs)
}

func (h *Handler) getImportJob(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, ok := queryutil.ParsePositiveUint(c.Param("id"))
	if !ok {
		httputil.InvalidRequest(c, "invalid id")
		return
	}
	job, err := h.service.ImportJob(c.Request.Context(), uid, id)
	if err != nil {
		httputil.NotFound(c, "import job not found")
		return
	}
	c.JSON(http.StatusOK, job)
}

func (h *Handler) importText(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req ImportTextRequest
	if err := httputil.BindJSONBody(c, &req); err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.InvalidRequest(c, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		httputil.BadRequest(c, "content is required")
		return
	}

	result, err := h.service.ImportText(c.Request.Context(), uid, req)
	if err != nil {
		httputil.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) previewImportText(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req ImportTextRequest
	if err := httputil.BindJSONBody(c, &req); err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.InvalidRequest(c, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		httputil.BadRequest(c, "content is required")
		return
	}

	result, err := h.service.PreviewText(c.Request.Context(), uid, req)
	if err != nil {
		httputil.BadRequest(c, err.Error())
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
