package reports

import (
	"net/http"

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
	group.GET("/reports/summary", h.summary)
}

func (h *Handler) summary(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	start, end := timeutil.ResolveRange(c.Query("start"), c.Query("end"))

	summary, err := h.service.Summary(uid, SummaryFilter{
		Start:      start,
		End:        end,
		CategoryID: queryutil.ParseUint(c.Query("categoryId")),
		AccountID:  queryutil.ParseUint(c.Query("accountId")),
		Trend:      c.Query("trend"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}
