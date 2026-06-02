package dataio

import (
	"bytes"
	"mime/multipart"
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/transactions"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
)

func TestImportPreviewReportsValidAndInvalidRows(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	file := writeImportFile(t, "preview.csv", "occurred_at,type,amount,category,account,note,tags\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\nwrong,expense,1,餐饮,现金,坏行,\n")
	preview, err := NewService(s, transactions.NewService(s, nil), nil).Preview(user.ID, file)
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

func TestImportCreatesRowsInBatchAndUpdatesBalance(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	file := writeImportFile(t, "import.csv", "occurred_at,type,amount,category,account,note,tags\n2026-06-01T12:30:00+08:00,expense,35.50,餐饮,现金,午饭,\n2026-06-01T13:00:00+08:00,income,100,工资,现金,工资,\n")
	result, err := NewService(s, transactions.NewService(s, nil), nil).Import(user.ID, file)
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
