package dataio

import (
	"bytes"
	"context"
	"mime/multipart"
	"strings"
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/transactions"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"gorm.io/gorm"
)

type dataioContextKey struct{}

func TestImportPreviewReportsValidAndInvalidRows(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	file := writeImportFile(t, "preview.csv", "occurred_at,type,amount,category,account,note,tags\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\nwrong,expense,1,餐饮,现金,坏行,\n")
	preview, err := NewService(s, transactions.NewService(s, nil), nil).Preview(context.Background(), user.ID, file)
	if err != nil {
		t.Fatalf("preview import: %v", err)
	}
	if preview.TotalRows != 2 || preview.ValidRows != 1 || preview.FailedRows != 1 {
		t.Fatalf("unexpected preview counts: %#v", preview)
	}
	if len(preview.Rows) != 2 || preview.Rows[1].Valid || preview.Rows[1].Error == "" {
		t.Fatalf("unexpected preview rows: %#v", preview.Rows)
	}
}

func TestImportPreviewIgnoresBlankRowsAndKeepsSourceLine(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	file := writeImportFile(t, "preview.csv", "occurred_at,type,amount,category,account,note,tags\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\n,,,,,,\nwrong,expense,1,餐饮,现金,坏行,\n")
	preview, err := NewService(s, transactions.NewService(s, nil), nil).Preview(context.Background(), user.ID, file)
	if err != nil {
		t.Fatalf("preview import: %v", err)
	}
	if preview.TotalRows != 2 || preview.ValidRows != 1 || preview.FailedRows != 1 {
		t.Fatalf("unexpected preview counts: %#v", preview)
	}
	if len(preview.Rows) != 2 || preview.Rows[1].Line != 4 {
		t.Fatalf("unexpected preview rows: %#v", preview.Rows)
	}
}

func TestImportPreviewMarksDuplicateRows(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	file := writeImportFile(t, "preview.csv", "occurred_at,type,amount,category,account,note,tags\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\n")
	preview, err := NewService(s, transactions.NewService(s, nil), nil).Preview(context.Background(), user.ID, file)
	if err != nil {
		t.Fatalf("preview import: %v", err)
	}
	if preview.DuplicateRows != 1 || !preview.Rows[1].Duplicate {
		t.Fatalf("expected duplicate row: %#v", preview)
	}
}

func TestImportCreatesRowsInBatchAndUpdatesBalance(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	file := writeImportFile(t, "import.csv", "occurred_at,type,amount,category,account,note,tags\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\n2026-06-01T13:00:00+08:00,income,100,工资,现金,工资,\n")
	result, err := NewService(s, transactions.NewService(s, nil), nil).Import(context.Background(), user.ID, file)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Success != 2 || result.Failed != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}

	var account models.Account
	if err := s.DB.Where("user_id = ? AND name = ?", user.ID, "现金").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if account.BalanceCents != 6450 {
		t.Fatalf("balance cents = %d, want 6450", account.BalanceCents)
	}
}

func TestImportIgnoresBlankRowsInTotals(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	file := writeImportFile(t, "import.csv", "occurred_at,type,amount,category,account,note,tags\n,,,,,,\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\n")
	result, err := NewService(s, transactions.NewService(s, nil), nil).Import(context.Background(), user.ID, file)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Total != 1 || result.Success != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestImportSkipsDuplicateRows(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	file := writeImportFile(t, "import.csv", "occurred_at,type,amount,category,account,note,tags\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\n")
	result, err := NewService(s, transactions.NewService(s, nil), nil).Import(context.Background(), user.ID, file)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Success != 1 || result.Skipped != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}

	var count int64
	if err := s.DB.Model(&models.Transaction{}).Where("user_id = ?", user.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("transaction count = %d, want 1", count)
	}
}

func TestImportCanAllowDuplicateRows(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	file := writeImportFile(t, "import.csv", "occurred_at,type,amount,category,account,note,tags\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\n")
	result, err := NewService(s, transactions.NewService(s, nil), nil).ImportWithOptions(context.Background(), user.ID, file, ImportOptions{SkipDuplicates: false})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Success != 2 || result.Skipped != 0 || result.Failed != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestImportTextRejectsInvalidHeader(t *testing.T) {
	s := testutil.NewStore(t)

	_, err := NewService(s, transactions.NewService(s, nil), nil).ImportText(context.Background(), 1, ImportTextRequest{
		Content: "when,type,amount,category,account,note,tags\n2026-06-01T12:30:00+08:00,expense,1,餐饮,现金,午饭,\n",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid header") {
		t.Fatalf("ImportText error = %v, want invalid header", err)
	}
}

func TestStartImportJobPersistsProgressAndHistory(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	service := NewService(s, transactions.NewService(s, nil), nil)
	file := writeImportFile(t, "job.csv", "occurred_at,type,amount,category,account,note,tags\n,,,,,,\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\n")
	ctx := context.Background()
	job, err := service.StartImportJob(ctx, user.ID, file, ImportOptions{SkipDuplicates: true})
	if err != nil {
		t.Fatalf("start import job: %v", err)
	}
	if job.ID == 0 || job.Status != "queued" || job.Total != 1 {
		t.Fatalf("unexpected queued job: %#v", job)
	}

	var completed ImportJobResponse
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		completed, err = service.ImportJob(ctx, user.ID, job.ID)
		if err != nil {
			t.Fatalf("load import job: %v", err)
		}
		if completed.Status == "completed" || completed.Status == "failed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if completed.Status != "completed" || completed.Total != 1 || completed.Success != 1 || completed.Failed != 0 {
		t.Fatalf("unexpected completed job: %#v", completed)
	}

	jobs, err := service.ListImportJobs(ctx, user.ID)
	if err != nil {
		t.Fatalf("list import jobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != job.ID {
		t.Fatalf("unexpected job history: %#v", jobs)
	}
}

func TestServicePassesContextToGORMQueries(t *testing.T) {
	s := testutil.NewStore(t)
	service := NewService(s, transactions.NewService(s, nil), nil)
	ctx := context.WithValue(context.Background(), dataioContextKey{}, "request-context")
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	callbackName := "dataio:test_context"
	var got any
	if err := s.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(db *gorm.DB) {
		got = db.Statement.Context.Value(dataioContextKey{})
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = s.DB.Callback().Query().Remove(callbackName)
	})

	if _, err := service.ExportRows(ctx, 1, start, end); err != nil {
		t.Fatal(err)
	}
	if got != "request-context" {
		t.Fatalf("gorm context value = %#v", got)
	}
}

func TestDetachedContextKeepsValuesWithoutCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), dataioContextKey{}, "request-context"))
	cancel()

	detached := detachedContext(ctx)
	if got := detached.Value(dataioContextKey{}); got != "request-context" {
		t.Fatalf("detached context value = %#v", got)
	}
	if err := detached.Err(); err != nil {
		t.Fatalf("detached context error = %v", err)
	}
}

func writeImportFile(t *testing.T, name, content string) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	reader := multipart.NewReader(bytes.NewReader(buf.Bytes()), writer.Boundary())
	form, err := reader.ReadForm(MaxImportFileBytes)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = form.RemoveAll() })
	return form.File["file"][0]
}
