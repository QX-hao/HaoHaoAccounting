package ai

import (
	"net/http"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	group.POST("/ai/parse", h.parse)
}

func (h *Handler) parse(c *gin.Context) {
	var req parseRequest
	if err := httputil.BindJSONBody(c, &req); err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.InvalidRequest(c, "invalid request body")
		return
	}
	uid := middleware.UserIDFromContext(c)
	c.JSON(http.StatusOK, h.service.Parse(c.Request.Context(), uid, req.Text))
}
