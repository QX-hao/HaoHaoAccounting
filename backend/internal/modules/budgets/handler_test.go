package budgets

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestListRejectsInvalidQueryParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(NewService(testutil.NewStore(t), nil)).Register(router.Group(""))

	for _, path := range []string{
		"/budgets?month=2026-6",
		"/budgets?month=invalid",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body = %s", resp.Code, resp.Body.String())
			}
		})
	}
}

func TestListAcceptsDefaultQueryParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(NewService(testutil.NewStore(t), nil)).Register(router.Group(""))

	req := httptest.NewRequest(http.MethodGet, "/budgets", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", resp.Code, resp.Body.String())
	}
}

func TestCreateRejectsInvalidRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(NewService(testutil.NewStore(t), nil)).Register(router.Group(""))

	for _, body := range []string{
		`{}`,
		`{"categoryId":1,"amount":100}`,
		`{"month":"2026-06","amount":100}`,
		`{"month":"2026-06","categoryId":1}`,
		`{"month":"2026-6","categoryId":1,"amount":100}`,
		`{"month":"2026-06","categoryId":0,"amount":100}`,
		`{"month":"2026-06","categoryId":1,"amount":0}`,
		`{"month":"2026-06","categoryId":1,"amount":-1}`,
	} {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/budgets", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body = %s", resp.Code, resp.Body.String())
			}
		})
	}
}

func TestRejectsInvalidPathID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(NewService(testutil.NewStore(t), nil)).Register(router.Group(""))

	for _, path := range []string{
		"/budgets/0",
		"/budgets/abc",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body = %s", resp.Code, resp.Body.String())
			}
		})
	}
}
