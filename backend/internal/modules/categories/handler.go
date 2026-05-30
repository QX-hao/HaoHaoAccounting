package categories

import (
	"net/http"
	"strings"

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
	categories, err := h.service.List(uid, strings.TrimSpace(c.Query("type")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, categories)
}

func (h *Handler) create(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	category, err := h.service.Create(uid, req)
	if err != nil {
		if err.Error() == "type must be income or expense" || err.Error() == "name is required" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, category)
}

func (h *Handler) update(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id := queryutil.ParseUint(c.Param("id"))
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	category, err := h.service.Update(uid, id, req)
	if err != nil {
		switch err.Error() {
		case "category not found":
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case "system category cannot be modified":
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, category)
}

func (h *Handler) delete(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id := queryutil.ParseUint(c.Param("id"))

	if err := h.service.Delete(uid, id); err != nil {
		switch err.Error() {
		case "category not found":
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case "system category cannot be deleted":
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case "category in use by transactions":
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
