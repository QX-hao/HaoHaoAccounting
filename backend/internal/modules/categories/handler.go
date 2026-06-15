package categories

import (
	"net/http"
	"strings"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/queryutil"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	group.GET("/categories", h.list)
	group.POST("/categories", h.create)
	group.PUT("/categories/:id", h.update)
	group.DELETE("/categories/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var query listQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		httputil.InvalidRequest(c, "invalid query parameters")
		return
	}
	categories, err := h.service.List(c.Request.Context(), uid, strings.TrimSpace(query.Type))
	if err != nil {
		httputil.InternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, categories)
}

func (h *Handler) create(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req categoryRequest
	if err := httputil.BindJSONBody(c, &req); err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.InvalidRequest(c, "invalid request body")
		return
	}

	category, err := h.service.Create(c.Request.Context(), uid, req)
	if err != nil {
		if err.Error() == "type must be income or expense" || err.Error() == "name is required" {
			httputil.BadRequest(c, err.Error())
			return
		}
		httputil.InternalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, category)
}

func (h *Handler) update(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, ok := queryutil.ParsePositiveUint(c.Param("id"))
	if !ok {
		httputil.InvalidRequest(c, "invalid id")
		return
	}
	var req categoryRequest
	if err := httputil.BindJSONBody(c, &req); err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.InvalidRequest(c, "invalid request body")
		return
	}

	category, err := h.service.Update(c.Request.Context(), uid, id, req)
	if err != nil {
		switch err.Error() {
		case "category not found":
			httputil.NotFound(c, err.Error())
		case "system category cannot be modified":
			httputil.Forbidden(c, err.Error())
		default:
			httputil.InternalError(c, err)
		}
		return
	}
	c.JSON(http.StatusOK, category)
}

func (h *Handler) delete(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, ok := queryutil.ParsePositiveUint(c.Param("id"))
	if !ok {
		httputil.InvalidRequest(c, "invalid id")
		return
	}

	if err := h.service.Delete(c.Request.Context(), uid, id); err != nil {
		switch err.Error() {
		case "category not found":
			httputil.NotFound(c, err.Error())
		case "system category cannot be deleted":
			httputil.Forbidden(c, err.Error())
		case "category in use by transactions":
			httputil.BadRequest(c, err.Error())
		default:
			httputil.InternalError(c, err)
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
