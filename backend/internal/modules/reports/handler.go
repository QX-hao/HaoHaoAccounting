package reports

import (
	"net/http"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
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
	var query summaryQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		httputil.InvalidRequest(c, "invalid query parameters")
		return
	}
	start, end, err := timeutil.ResolveRangeStrict(query.Start, query.End)
	if err != nil {
		httputil.InvalidRequest(c, err.Error())
		return
	}
	categoryID := uint(0)
	if query.CategoryID != nil {
		categoryID = *query.CategoryID
	}
	accountID := uint(0)
	if query.AccountID != nil {
		accountID = *query.AccountID
	}

	summary, err := h.service.Summary(c.Request.Context(), uid, SummaryFilter{
		Start:      start,
		End:        end,
		CategoryID: categoryID,
		AccountID:  accountID,
		Trend:      query.Trend,
	})
	if err != nil {
		httputil.InternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}
