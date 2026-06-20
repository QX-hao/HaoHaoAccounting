package transactions

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
		"/transactions?page=0",
		"/transactions?page=abc",
		"/transactions?pageSize=201",
		"/transactions?type=transfer",
		"/transactions?categoryId=0",
		"/transactions?categoryId=abc",
		"/transactions?accountId=0",
		"/transactions?start=not-a-date",
		"/transactions?end=not-a-date",
		"/transactions?start=2026-07-01&end=2026-06-30",
		"/transactions?q=abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvw",
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

	req := httptest.NewRequest(http.MethodGet, "/transactions", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("X-Total-Count"); got != "0" {
		t.Fatalf("X-Total-Count = %q, want 0", got)
	}
}

func TestCreateRejectsInvalidRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(NewService(testutil.NewStore(t), nil)).Register(router.Group(""))

	for _, body := range []string{
		`{}`,
		`{"type":"expense","amount":1,"categoryId":1,"accountId":1}`,
		`{"type":"transfer","amount":1,"categoryId":1,"accountId":1,"note":"lunch"}`,
		`{"type":"expense","amount":0,"categoryId":1,"accountId":1,"note":"lunch"}`,
		`{"type":"expense","amount":1,"categoryId":0,"accountId":1,"note":"lunch"}`,
		`{"type":"expense","amount":1,"categoryId":1,"accountId":0,"note":"lunch"}`,
		`{"type":"expense","amount":1,"categoryId":1,"accountId":1,"note":""}`,
	} {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/transactions", strings.NewReader(body))
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
		"/transactions/0",
		"/transactions/abc",
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
