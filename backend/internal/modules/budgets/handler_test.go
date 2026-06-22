package budgets

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
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

func TestCreateReturnsLocationHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := testutil.NewStore(t)
	category := models.Category{Name: "餐饮", Type: "expense", IsSystem: true}
	if err := store.DB.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	NewHandler(NewService(store, nil)).Register(router.Group(""))

	req := httptest.NewRequest(http.MethodPost, "/budgets", strings.NewReader(`{"month":"2026-06","categoryId":`+strconv.FormatUint(uint64(category.ID), 10)+`,"amount":100}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", resp.Code, resp.Body.String())
	}
	var budget struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &budget); err != nil {
		t.Fatalf("decode budget: %v", err)
	}
	if budget.ID == 0 {
		t.Fatal("budget id = 0")
	}
	if got := resp.Header().Get("Location"); got != "/budgets/"+strconv.FormatUint(uint64(budget.ID), 10) {
		t.Fatalf("Location = %q", got)
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
