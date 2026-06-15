package budgets

import (
	"net/http"

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
	group.GET("/budgets", h.list)
	group.POST("/budgets", h.create)
	group.PUT("/budgets/:id", h.update)
	group.DELETE("/budgets/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var query listQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		httputil.InvalidRequest(c, "invalid query parameters")
		return
	}
	budgets, err := h.service.List(c.Request.Context(), uid, query.Month)
	if err != nil {
		httputil.InternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, budgets)
}

func (h *Handler) create(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req budgetRequest
	if err := httputil.BindJSONBody(c, &req); err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.InvalidRequest(c, "invalid request body")
		return
	}

	budget, err := h.service.Create(c.Request.Context(), uid, req)
	if err != nil {
		writeBudgetError(c, err)
		return
	}
	c.JSON(http.StatusCreated, budget)
}

func (h *Handler) update(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, ok := queryutil.ParsePositiveUint(c.Param("id"))
	if !ok {
		httputil.InvalidRequest(c, "invalid id")
		return
	}
	var req budgetRequest
	if err := httputil.BindJSONBody(c, &req); err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.InvalidRequest(c, "invalid request body")
		return
	}

	budget, err := h.service.Update(c.Request.Context(), uid, id, req)
	if err != nil {
		writeBudgetError(c, err)
		return
	}
	c.JSON(http.StatusOK, budget)
}

func (h *Handler) delete(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, ok := queryutil.ParsePositiveUint(c.Param("id"))
	if !ok {
		httputil.InvalidRequest(c, "invalid id")
		return
	}

	if err := h.service.Delete(c.Request.Context(), uid, id); err != nil {
		writeBudgetError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func writeBudgetError(c *gin.Context, err error) {
	switch err.Error() {
	case "budget not found", "category not found":
		httputil.NotFound(c, err.Error())
	case "month must be YYYY-MM", "amount must be >= 0", "budget category must be expense", "category not accessible":
		httputil.BadRequest(c, err.Error())
	default:
		httputil.InternalError(c, err)
	}
}
