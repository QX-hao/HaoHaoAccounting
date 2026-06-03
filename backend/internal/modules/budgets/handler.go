package budgets

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
	group.GET("/budgets", h.list)
	group.POST("/budgets", h.create)
	group.PUT("/budgets/:id", h.update)
	group.DELETE("/budgets/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	budgets, err := h.service.List(uid, c.Query("month"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, budgets)
}

func (h *Handler) create(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req budgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	budget, err := h.service.Create(uid, req)
	if err != nil {
		writeBudgetError(c, err)
		return
	}
	c.JSON(http.StatusCreated, budget)
}

func (h *Handler) update(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id := queryutil.ParseUint(c.Param("id"))
	var req budgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	budget, err := h.service.Update(uid, id, req)
	if err != nil {
		writeBudgetError(c, err)
		return
	}
	c.JSON(http.StatusOK, budget)
}

func (h *Handler) delete(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id := queryutil.ParseUint(c.Param("id"))

	if err := h.service.Delete(uid, id); err != nil {
		writeBudgetError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func writeBudgetError(c *gin.Context, err error) {
	switch err.Error() {
	case "budget not found", "category not found":
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case "month must be YYYY-MM", "amount must be >= 0", "budget category must be expense", "category not accessible":
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
