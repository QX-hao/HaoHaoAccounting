package dataio

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/transactions"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

var errFailingWrite = errors.New("write failed")

func TestExportRejectsInvalidQueryParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := testutil.NewStore(t)
	NewHandler(NewService(store, transactions.NewService(store, nil), nil)).Register(router.Group(""))

	for _, path := range []string{
		"/io/export?format=pdf",
		"/io/export?start=not-a-date",
		"/io/export?end=not-a-date",
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

func TestExportAcceptsDefaultQueryParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := testutil.NewStore(t)
	NewHandler(NewService(store, transactions.NewService(store, nil), nil)).Register(router.Group(""))

	req := httptest.NewRequest(http.MethodGet, "/io/export", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := resp.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment;") ||
		!strings.Contains(got, `filename="transactions_`) ||
		!strings.Contains(got, "filename*=UTF-8''transactions_") {
		t.Fatalf("Content-Disposition = %q", got)
	}
}

func TestWriteCSVReturnsWriterError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(&failingResponseWriter{ResponseRecorder: httptest.NewRecorder()})

	if err := writeCSV(c, nil); !errors.Is(err, errFailingWrite) {
		t.Fatalf("writeCSV error = %v, want %v", err, errFailingWrite)
	}
}

func TestWriteCSVNeutralizesFormulaCells(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)

	rows := []models.Transaction{{
		Type:        "expense",
		AmountCents: 12345,
		Category:    models.Category{Name: "=category"},
		Account:     models.Account{Name: "+account"},
		Note:        "-note",
		Tags:        "@tags",
		Source:      "\t=source",
		OccurredAt:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}}

	if err := writeCSV(c, rows); err != nil {
		t.Fatalf("writeCSV error = %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(resp.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2", len(records))
	}

	want := []string{"'=category", "'+account", "'-note", "'@tags", "'\t=source"}
	for i, value := range records[1][3:] {
		if value != want[i] {
			t.Fatalf("field %d = %q, want %q", i+3, value, want[i])
		}
	}
}

func TestWriteXLSXNeutralizesFormulaCells(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)

	rows := []models.Transaction{{
		Type:        "expense",
		AmountCents: 12345,
		Category:    models.Category{Name: "=category"},
		Account:     models.Account{Name: "+account"},
		Note:        "-note",
		Tags:        "@tags",
		Source:      "\t=source",
		OccurredAt:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}}

	if err := writeXLSX(c, rows); err != nil {
		t.Fatalf("writeXLSX error = %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(resp.Body.Bytes()))
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Fatalf("close xlsx: %v", err)
		}
	})

	sheet := f.GetSheetName(0)
	for cell, want := range map[string]string{
		"D2": "'=category",
		"E2": "'+account",
		"F2": "'-note",
		"G2": "'@tags",
		"H2": "'\t=source",
	} {
		got, err := f.GetCellValue(sheet, cell)
		if err != nil {
			t.Fatalf("read %s: %v", cell, err)
		}
		if got != want {
			t.Fatalf("%s = %q, want %q", cell, got, want)
		}
	}
}

func TestSafeCSVCellNeutralizesDangerousPrefixes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty", value: "", want: ""},
		{name: "plain", value: "groceries", want: "groceries"},
		{name: "equals", value: "=1+1", want: "'=1+1"},
		{name: "plus", value: "+1+1", want: "'+1+1"},
		{name: "minus", value: "-1+1", want: "'-1+1"},
		{name: "at", value: "@cmd", want: "'@cmd"},
		{name: "tab", value: "\t=cmd", want: "'\t=cmd"},
		{name: "carriage-return", value: "\r=cmd", want: "'\r=cmd"},
		{name: "newline", value: "\n=cmd", want: "'\n=cmd"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeCSVCell(tc.value); got != tc.want {
				t.Fatalf("safeCSVCell(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestGetImportJobRejectsInvalidPathID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := testutil.NewStore(t)
	NewHandler(NewService(store, transactions.NewService(store, nil), nil)).Register(router.Group(""))

	for _, path := range []string{
		"/io/import/jobs/0",
		"/io/import/jobs/abc",
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

func TestImportTextRejectsInvalidRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := testutil.NewStore(t)
	NewHandler(NewService(store, transactions.NewService(store, nil), nil)).Register(router.Group(""))

	for _, path := range []string{
		"/io/import/text/preview",
		"/io/import/text",
	} {
		for _, body := range []string{
			`{}`,
			`{"content":""}`,
			`{"content":"` + strings.Repeat("x", MaxImportFileBytes+1) + `"}`,
		} {
			t.Run(path+" "+body[:min(len(body), 32)], func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				resp := httptest.NewRecorder()
				router.ServeHTTP(resp, req)

				if resp.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400, body = %s", resp.Code, resp.Body.String())
				}
			})
		}
	}
}

func TestMultipartImportMapsStreamingBodyLimitToPayloadTooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.RequestID(), middleware.BodyLimit(32))
	store := testutil.NewStore(t)
	NewHandler(NewService(store, transactions.NewService(store, nil), nil)).Register(router.Group(""))

	body, contentType := multipartBody(t, "file", "transactions.csv", strings.Repeat("x", 128))
	req := httptest.NewRequest(http.MethodPost, "/io/import/preview", bytes.NewReader(body))
	req.ContentLength = -1
	req.Header.Set("Content-Type", contentType)
	req.Header.Set(middleware.RequestIDHeader, "request-upload-limit")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413, body = %s", resp.Code, resp.Body.String())
	}

	var errorBody httputil.ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &errorBody); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if errorBody.Code != httputil.CodePayloadTooLarge {
		t.Fatalf("code = %q, want %q", errorBody.Code, httputil.CodePayloadTooLarge)
	}
	if errorBody.RequestID != "request-upload-limit" {
		t.Fatalf("requestId = %q", errorBody.RequestID)
	}
}

func multipartBody(t *testing.T, fieldName string, fileName string, content string) ([]byte, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

type failingResponseWriter struct {
	*httptest.ResponseRecorder
}

func (w *failingResponseWriter) Write(_ []byte) (int, error) {
	return 0, errFailingWrite
}
