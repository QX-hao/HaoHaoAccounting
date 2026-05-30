package accounts

import (
	"net/http"

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
	group.GET("/accounts", h.list)
	group.POST("/accounts", h.create)
	group.PUT("/accounts/:id", h.update)
	group.DELETE("/accounts/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	accounts, err := h.service.List(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, accounts)
}

func (h *Handler) create(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req accountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	account, err := h.service.Create(uid, req)
	if err != nil {
		if err.Error() == "name is required" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, account)
}

func (h *Handler) update(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id := queryutil.ParseUint(c.Param("id"))
	var req accountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	account, err := h.service.Update(uid, id, req)
	if err != nil {
		if err.Error() == "account not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, account)
}

func (h *Handler) delete(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id := queryutil.ParseUint(c.Param("id"))

	if err := h.service.Delete(uid, id); err != nil {
		if err.Error() == "account in use by transactions" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
