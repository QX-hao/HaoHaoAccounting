package dataio

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
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

func TestReadmeDocumentsExportDownloadAndSpreadsheetSafetyContracts(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	source := string(data)

	for _, want := range []string{
		"`Content-Disposition`",
		"ASCII-safe `filename` fallback",
		"`filename`",
		"`filename*`",
		"without losing non-ASCII names",
		"`safeCSVCell`",
		"case normalization",
		"`invalid_request`",
		"formula prefixes",
		"leading whitespace",
		"`skipDuplicates`",
		"defaults to true",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("README.md is missing dataio maintenance guidance %q", want)
		}
	}
}

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

func TestExportInvalidFormatReturnsBadRequestAfterAcceptNegotiation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.Accept(middleware.APIAcceptRules()))
	store := testutil.NewStore(t)
	NewHandler(NewService(store, transactions.NewService(store, nil), nil)).Register(router.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/io/export?format=pdf", nil)
	req.Header.Set("Accept", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", resp.Code, resp.Body.String())
	}
	assertErrorCode(t, resp, httputil.CodeInvalidRequest)
}

func TestExportAcceptsDefaultAndCaseInsensitiveFormatQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := testutil.NewStore(t)
	NewHandler(NewService(store, transactions.NewService(store, nil), nil)).Register(router.Group(""))

	for _, path := range []string{"/io/export", "/io/export?format=CSV", "/io/export?format=%20csv%20"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
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
		})
	}
}

func TestNormalizedExportFormat(t *testing.T) {
	for _, tc := range []struct {
		name      string
		value     string
		want      string
		wantValid bool
	}{
		{name: "empty defaults to csv", want: "csv", wantValid: true},
		{name: "csv", value: "csv", want: "csv", wantValid: true},
		{name: "uppercase csv", value: " CSV ", want: "csv", wantValid: true},
		{name: "uppercase xlsx", value: "XLSX", want: "xlsx", wantValid: true},
		{name: "invalid", value: "pdf", wantValid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := normalizedExportFormat(tc.value)
			if got != tc.want || ok != tc.wantValid {
				t.Fatalf("normalizedExportFormat(%q) = (%q, %v), want (%q, %v)", tc.value, got, ok, tc.want, tc.wantValid)
			}
		})
	}
}

func TestWriteCSVReturnsWriterError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(&failingResponseWriter{ResponseRecorder: httptest.NewRecorder()})

	if err := writeCSV(c, nil); !errors.Is(err, errFailingWrite) {
		t.Fatalf("writeCSV error = %v, want %v", err, errFailingWrite)
	}
}

func TestAttachmentDispositionIncludesFallbackAndEncodedFilename(t *testing.T) {
	for _, tc := range []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "ascii",
			filename: "transactions.csv",
			want:     `attachment; filename="transactions.csv"; filename*=UTF-8''transactions.csv`,
		},
		{
			name:     "non-ascii",
			filename: "交易记录.csv",
			want:     `attachment; filename="download.csv"; filename*=UTF-8''%E4%BA%A4%E6%98%93%E8%AE%B0%E5%BD%95.csv`,
		},
		{
			name:     "all non-ascii",
			filename: "交易记录",
			want:     `attachment; filename="download"; filename*=UTF-8''%E4%BA%A4%E6%98%93%E8%AE%B0%E5%BD%95`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := attachmentDisposition(tc.filename); got != tc.want {
				t.Fatalf("attachmentDisposition(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		})
	}
}

func TestASCIIFilenameFallback(t *testing.T) {
	for _, tc := range []struct {
		name     string
		filename string
		want     string
	}{
		{name: "ascii", filename: "transactions.csv", want: "transactions.csv"},
		{name: "non-ascii", filename: "交易记录.csv", want: "download.csv"},
		{name: "all non-ascii", filename: "交易记录", want: "download"},
		{name: "control", filename: "bad\nname.csv", want: "bad_name.csv"},
		{name: "path separators", filename: "../reports\\transactions:2026.csv", want: "reports_transactions_2026.csv"},
		{name: "leading separators", filename: "__hidden.csv", want: "hidden.csv"},
		{name: "blank", filename: "  ", want: "download"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := asciiFilenameFallback(tc.filename); got != tc.want {
				t.Fatalf("asciiFilenameFallback(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		})
	}
}

func TestASCIIFilenameStemHasAlphanumeric(t *testing.T) {
	for _, tc := range []struct {
		name     string
		filename string
		want     bool
	}{
		{name: "ascii stem", filename: "transactions.csv", want: true},
		{name: "non-ascii stem with ascii extension", filename: "____.csv", want: false},
		{name: "digit stem", filename: "202606.csv", want: true},
		{name: "underscore stem", filename: "____", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := asciiFilenameStemHasAlphanumeric(tc.filename); got != tc.want {
				t.Fatalf("asciiFilenameStemHasAlphanumeric(%q) = %v, want %v", tc.filename, got, tc.want)
			}
		})
	}
}

func TestASCIIFilenameExtension(t *testing.T) {
	for _, tc := range []struct {
		name     string
		filename string
		want     string
	}{
		{name: "extension", filename: "____.csv", want: ".csv"},
		{name: "no extension", filename: "____", want: ""},
		{name: "trailing dot", filename: "____.", want: ""},
		{name: "unsafe extension", filename: "____.c\nsv", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := asciiFilenameExtension(tc.filename); got != tc.want {
				t.Fatalf("asciiFilenameExtension(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		})
	}
}

func TestASCIIFilenameCharAllowed(t *testing.T) {
	for _, tc := range []struct {
		r    rune
		want bool
	}{
		{r: 'a', want: true},
		{r: 'Z', want: true},
		{r: '7', want: true},
		{r: '.', want: true},
		{r: '_', want: true},
		{r: '-', want: true},
		{r: '/', want: false},
		{r: '\\', want: false},
		{r: ':', want: false},
		{r: '交', want: false},
	} {
		t.Run(string(tc.r), func(t *testing.T) {
			if got := asciiFilenameCharAllowed(tc.r); got != tc.want {
				t.Fatalf("asciiFilenameCharAllowed(%q) = %v, want %v", tc.r, got, tc.want)
			}
		})
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
		Source:      "  =source",
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

	want := []string{"'=category", "'+account", "'-note", "'@tags", "'  =source"}
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
		Source:      "  =source",
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
		"H2": "'  =source",
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
		{name: "leading spaces", value: "  =cmd", want: "'  =cmd"},
		{name: "leading spaces before at", value: "  @cmd", want: "'  @cmd"},
		{name: "only whitespace with tab", value: " \t\n", want: "' \t\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeCSVCell(tc.value); got != tc.want {
				t.Fatalf("safeCSVCell(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestDangerousCSVFormulaPrefix(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "empty", value: "", want: false},
		{name: "plain", value: "groceries", want: false},
		{name: "equals", value: "=1+1", want: true},
		{name: "tab", value: "\ttext", want: true},
		{name: "carriage return", value: "\rtext", want: true},
		{name: "newline", value: "\ntext", want: true},
		{name: "spaces before formula", value: "  +1", want: true},
		{name: "spaces before tab", value: " \ttext", want: true},
		{name: "only spaces", value: "   ", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := dangerousCSVFormulaPrefix(tc.value); got != tc.want {
				t.Fatalf("dangerousCSVFormulaPrefix(%q) = %v, want %v", tc.value, got, tc.want)
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

func TestCreateImportJobReturnsLocationHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Next()
	})
	store := testutil.NewStore(t)
	NewHandler(NewService(store, transactions.NewService(store, nil), nil)).Register(router.Group("/api/v1"))

	body, contentType := multipartBody(t, "file", "transactions.csv", "occurred_at,type,amount,category,account,note,tags\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\n")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/io/import/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202, body = %s", resp.Code, resp.Body.String())
	}
	var job ImportJobResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &job); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	if job.ID == 0 {
		t.Fatalf("job id = %d", job.ID)
	}
	if got := resp.Header().Get("Location"); got != "/api/v1/io/import/jobs/"+strconv.FormatUint(uint64(job.ID), 10) {
		t.Fatalf("Location = %q", got)
	}
}

func TestMultipartImportRejectsInvalidSkipDuplicates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := testutil.NewStore(t)
	NewHandler(NewService(store, transactions.NewService(store, nil), nil)).Register(router.Group(""))

	for _, path := range []string{"/io/import", "/io/import/jobs"} {
		t.Run(path, func(t *testing.T) {
			body, contentType := multipartBodyWithFields(t, "file", "transactions.csv", "occurred_at,type,amount,category,account,note,tags\n", map[string]string{
				"skipDuplicates": "maybe",
			})
			req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
			req.Header.Set("Content-Type", contentType)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body = %s", resp.Code, resp.Body.String())
			}
			assertErrorCode(t, resp, httputil.CodeInvalidRequest)
		})
	}
}

func TestMultipartImportRejectsMissingFileAsInvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := testutil.NewStore(t)
	NewHandler(NewService(store, transactions.NewService(store, nil), nil)).Register(router.Group(""))

	for _, path := range []string{"/io/import/preview", "/io/import", "/io/import/jobs"} {
		t.Run(path, func(t *testing.T) {
			body, contentType := multipartFieldsOnly(t, map[string]string{
				"skipDuplicates": "true",
			})
			req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
			req.Header.Set("Content-Type", contentType)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body = %s", resp.Code, resp.Body.String())
			}
			assertErrorCode(t, resp, httputil.CodeInvalidRequest)
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

func TestParseBoolDefaultRequiresValidExplicitValues(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fallback  bool
		wantValue bool
		wantOK    bool
	}{
		{name: "empty true default", value: "", fallback: true, wantValue: true, wantOK: true},
		{name: "empty false default", value: " ", fallback: false, wantValue: false, wantOK: true},
		{name: "explicit true", value: "true", fallback: false, wantValue: true, wantOK: true},
		{name: "explicit false", value: "false", fallback: true, wantValue: false, wantOK: true},
		{name: "invalid", value: "maybe", fallback: true, wantValue: false, wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotValue, gotOK := parseBoolDefault(tc.value, tc.fallback)
			if gotValue != tc.wantValue || gotOK != tc.wantOK {
				t.Fatalf("parseBoolDefault(%q, %v) = (%v, %v), want (%v, %v)", tc.value, tc.fallback, gotValue, gotOK, tc.wantValue, tc.wantOK)
			}
		})
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
	return multipartBodyWithFields(t, fieldName, fileName, content, nil)
}

func multipartBodyWithFields(t *testing.T, fieldName string, fileName string, content string, fields map[string]string) ([]byte, string) {
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
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatalf("write multipart field %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func multipartFieldsOnly(t *testing.T, fields map[string]string) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatalf("write multipart field %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func assertErrorCode(t *testing.T, resp *httptest.ResponseRecorder, want string) {
	t.Helper()
	var errorBody httputil.ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &errorBody); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if errorBody.Code != want {
		t.Fatalf("code = %q, want %q", errorBody.Code, want)
	}
}

type failingResponseWriter struct {
	*httptest.ResponseRecorder
}

func (w *failingResponseWriter) Write(_ []byte) (int, error) {
	return 0, errFailingWrite
}
