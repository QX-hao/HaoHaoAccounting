package transactions

import (
	"net/http"
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
	group.GET("/transactions", h.list)
	group.POST("/transactions", h.create)
	group.PUT("/transactions/:id", h.update)
	group.DELETE("/transactions/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var query listQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		httputil.InvalidRequest(c, "invalid query parameters")
		return
	}
	start, end, err := timeutil.ResolveOptionalRangeStrict(query.Start, query.End)
	if err != nil {
		httputil.InvalidRequest(c, err.Error())
		return
	}
	page := query.Page
	if page == nil {
		defaultPage := 1
		page = &defaultPage
	}
	pageSize := query.PageSize
	if pageSize == nil {
		defaultPageSize := 20
		pageSize = &defaultPageSize
	}
	categoryID := uint(0)
	if query.CategoryID != nil {
		categoryID = *query.CategoryID
	}
	accountID := uint(0)
	if query.AccountID != nil {
		accountID = *query.AccountID
	}

	rows, total, err := h.service.List(c.Request.Context(), uid, ListFilter{
		Page:       *page,
		PageSize:   *pageSize,
		Start:      start,
		End:        end,
		Type:       strings.TrimSpace(query.Type),
		CategoryID: categoryID,
		AccountID:  accountID,
		Keyword:    strings.TrimSpace(query.Keyword),
	})
	if err != nil {
		httputil.InternalError(c, err)
		return
	}

	httputil.SetPaginationHeaders(c, total, *page, *pageSize)
	c.JSON(http.StatusOK, gin.H{
		"items": rows,
		"pagination": gin.H{
			"page":     *page,
			"pageSize": *pageSize,
			"total":    total,
		},
	})
}

func (h *Handler) create(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req Request
	if err := httputil.BindJSONBody(c, &req); err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.InvalidRequest(c, "invalid request body")
		return
	}

	tx, err := h.service.Create(c.Request.Context(), uid, req)
	if err != nil {
		if isClientError(err) {
			httputil.BadRequest(c, err.Error())
			return
		}
		httputil.InternalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, tx)
}

func (h *Handler) update(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, ok := queryutil.ParsePositiveUint(c.Param("id"))
	if !ok {
		httputil.InvalidRequest(c, "invalid id")
		return
	}

	var req Request
	if err := httputil.BindJSONBody(c, &req); err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.InvalidRequest(c, "invalid request body")
		return
	}

	tx, err := h.service.Update(c.Request.Context(), uid, id, req)
	if err != nil {
		if err.Error() == "transaction not found" {
			httputil.NotFound(c, err.Error())
			return
		}
		if isClientError(err) {
			httputil.BadRequest(c, err.Error())
			return
		}
		httputil.InternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, tx)
}

func (h *Handler) delete(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, ok := queryutil.ParsePositiveUint(c.Param("id"))
	if !ok {
		httputil.InvalidRequest(c, "invalid id")
		return
	}

	if err := h.service.Delete(c.Request.Context(), uid, id); err != nil {
		if err.Error() == "transaction not found" {
			httputil.NotFound(c, err.Error())
			return
		}
		httputil.InternalError(c, err)
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
