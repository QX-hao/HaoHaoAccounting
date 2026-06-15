package accounts

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
	group.GET("/accounts", h.list)
	group.POST("/accounts", h.create)
	group.PUT("/accounts/:id", h.update)
	group.DELETE("/accounts/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	accounts, err := h.service.List(c.Request.Context(), uid)
	if err != nil {
		httputil.InternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, accounts)
}

func (h *Handler) create(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req accountRequest
	if err := httputil.BindJSONBody(c, &req); err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.InvalidRequest(c, "invalid request body")
		return
	}

	account, err := h.service.Create(c.Request.Context(), uid, req)
	if err != nil {
		if err.Error() == "name is required" {
			httputil.BadRequest(c, err.Error())
			return
		}
		httputil.InternalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, account)
}

func (h *Handler) update(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, ok := queryutil.ParsePositiveUint(c.Param("id"))
	if !ok {
		httputil.InvalidRequest(c, "invalid id")
		return
	}
	var req accountRequest
	if err := httputil.BindJSONBody(c, &req); err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.InvalidRequest(c, "invalid request body")
		return
	}

	account, err := h.service.Update(c.Request.Context(), uid, id, req)
	if err != nil {
		if err.Error() == "account not found" {
			httputil.NotFound(c, err.Error())
			return
		}
		httputil.InternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

func (h *Handler) delete(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, ok := queryutil.ParsePositiveUint(c.Param("id"))
	if !ok {
		httputil.InvalidRequest(c, "invalid id")
		return
	}

	if err := h.service.Delete(c.Request.Context(), uid, id); err != nil {
		if err.Error() == "account in use by transactions" {
			httputil.BadRequest(c, err.Error())
			return
		}
		httputil.InternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
