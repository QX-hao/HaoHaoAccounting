package accounts

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestRejectsInvalidPathID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(NewService(testutil.NewStore(t), nil)).Register(router.Group(""))

	for _, path := range []string{
		"/accounts/0",
		"/accounts/abc",
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

func TestCreateRejectsInvalidBalanceAmounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(NewService(testutil.NewStore(t), nil)).Register(router.Group(""))

	for _, body := range []string{
		`{"name":"Cash","type":"cash","balance":-1}`,
		`{"name":"Cash","type":"cash","balance":1.234}`,
	} {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body = %s", resp.Code, resp.Body.String())
			}
		})
	}
}
