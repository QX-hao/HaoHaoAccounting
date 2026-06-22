package accounts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestCreateReturnsLocationHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(NewService(testutil.NewStore(t), nil)).Register(router.Group(""))

	req := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader(`{"name":"Cash","type":"cash","balance":0}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", resp.Code, resp.Body.String())
	}
	var account struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &account); err != nil {
		t.Fatalf("decode account: %v", err)
	}
	if account.ID == 0 {
		t.Fatal("account id = 0")
	}
	if got := resp.Header().Get("Location"); got != "/accounts/"+strconv.FormatUint(uint64(account.ID), 10) {
		t.Fatalf("Location = %q", got)
	}
}

func TestCreateRejectsInvalidBalanceAmounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(NewService(testutil.NewStore(t), nil)).Register(router.Group(""))

	for _, body := range []string{
		`{}`,
		`{"type":"cash","balance":0}`,
		`{"name":"Cash","balance":0}`,
		`{"name":"","type":"cash","balance":0}`,
		`{"name":"Cash","type":"","balance":0}`,
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
