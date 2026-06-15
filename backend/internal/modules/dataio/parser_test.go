package dataio

import (
	"encoding/csv"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseImportRecord(t *testing.T) {
	row := []string{"2026-06-01T12:30:00+08:00", "expense", "35.50", "餐饮", "微信", "午饭", "工作日,外卖"}

	record, err := parseImportRecord(row)
	if err != nil {
		t.Fatalf("parse import record: %v", err)
	}
	if record.Type != "expense" {
		t.Fatalf("type = %q", record.Type)
	}
	if record.Amount != 35.50 {
		t.Fatalf("amount = %v", record.Amount)
	}
	if record.Category != "餐饮" || record.Account != "微信" || record.Note != "午饭" || record.Tags != "工作日,外卖" {
		t.Fatalf("unexpected parsed record: %#v", record)
	}
	if record.OccurredAt.Format(time.RFC3339) != "2026-06-01T12:30:00+08:00" {
		t.Fatalf("occurredAt = %s", record.OccurredAt.Format(time.RFC3339))
	}
}

func TestParseImportRecordRejectsInvalidAmount(t *testing.T) {
	_, err := parseImportRecord([]string{"2026-06-01T12:30:00+08:00", "expense", "0", "餐饮", "现金"})
	if err == nil {
		t.Fatal("expected invalid amount error")
	}
}

func TestReadCSVDataRowsRejectsTooManyRows(t *testing.T) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{"occurred_at", "type", "amount", "category", "account", "note", "tags"})
	for i := 0; i < MaxImportRows+1; i++ {
		_ = writer.Write([]string{"2026-06-01T12:30:00+08:00", "expense", "1", "餐饮", "现金", fmt.Sprintf("row-%d", i), ""})
	}
	writer.Flush()

	_, err := readCSVDataRows(strings.NewReader(buf.String()))
	if err == nil {
		t.Fatal("expected too many rows error")
	}
}

func TestReadCSVDataRowsRejectsInvalidHeader(t *testing.T) {
	_, err := readCSVDataRows(strings.NewReader("when,type,amount,category,account,note,tags\n2026-06-01T12:30:00+08:00,expense,1,餐饮,现金,午饭,\n"))
	if err == nil || !strings.Contains(err.Error(), "invalid header") {
		t.Fatalf("readCSVDataRows error = %v, want invalid header", err)
	}
}

func TestReadCSVDataRowsAllowsAdditionalColumns(t *testing.T) {
	rows, err := readCSVDataRows(strings.NewReader(" occurred_at ,TYPE,amount,category,account,note,tags,source\n2026-06-01T12:30:00+08:00,expense,1,餐饮,现金,午饭,,import\n"))
	if err != nil {
		t.Fatalf("readCSVDataRows error = %v", err)
	}
	if len(rows) != 1 || len(rows[0]) != 8 {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestReadCSVDataRowsAllowsMissingOptionalTailColumns(t *testing.T) {
	rows, err := readCSVDataRows(strings.NewReader("occurred_at,type,amount,category,account,note,tags\n2026-06-01T12:30:00+08:00,expense,1,餐饮,现金\n"))
	if err != nil {
		t.Fatalf("readCSVDataRows error = %v", err)
	}
	if len(rows) != 1 || len(rows[0]) != 5 {
		t.Fatalf("rows = %#v", rows)
	}

	record, err := parseImportRecord(rows[0])
	if err != nil {
		t.Fatalf("parseImportRecord error = %v", err)
	}
	if record.Note != "" || record.Tags != "" {
		t.Fatalf("record = %#v, want empty optional tail fields", record)
	}
}
