package transactions

import (
	"net/http"
	"strings"

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
	group.GET("/transactions", h.list)
	group.POST("/transactions", h.create)
	group.PUT("/transactions/:id", h.update)
	group.DELETE("/transactions/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	start, _ := timeutil.ParseDateTime(c.Query("start"))
	end, _ := timeutil.ParseDateTime(c.Query("end"))

	rows, total, err := h.service.List(uid, ListFilter{
		Page:       queryutil.ParseInt(c.Query("page"), 1),
		PageSize:   queryutil.ParseInt(c.Query("pageSize"), 20),
		Start:      start,
		End:        end,
		Type:       strings.TrimSpace(c.Query("type")),
		CategoryID: queryutil.ParseUint(c.Query("categoryId")),
		AccountID:  queryutil.ParseUint(c.Query("accountId")),
		Keyword:    strings.TrimSpace(c.Query("q")),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	page := queryutil.ParseInt(c.Query("page"), 1)
	pageSize := queryutil.ParseInt(c.Query("pageSize"), 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	c.JSON(http.StatusOK, gin.H{
		"items": rows,
		"pagination": gin.H{
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
		},
	})
}

func (h *Handler) create(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	tx, err := h.service.Create(uid, req)
	if err != nil {
		if isClientError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, tx)
}

func (h *Handler) update(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id := queryutil.ParseUint(c.Param("id"))

	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	tx, err := h.service.Update(uid, id, req)
	if err != nil {
		if err.Error() == "transaction not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if isClientError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tx)
}

func (h *Handler) delete(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id := queryutil.ParseUint(c.Param("id"))

	if err := h.service.Delete(uid, id); err != nil {
		if err.Error() == "transaction not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func isClientError(err error) bool {
	switch err.Error() {
	case "type must be income or expense",
		"amount must be > 0",
		"categoryId is required",
		"accountId is required",
		"note is required",
		"category not found",
		"category type mismatch",
		"category not accessible",
		"account not found":
		return true
	default:
		return false
	}
}
