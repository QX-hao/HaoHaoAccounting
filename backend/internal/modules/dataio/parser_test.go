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
