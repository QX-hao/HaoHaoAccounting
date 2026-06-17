package dataio

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/money"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/stringutil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/timeutil"
	"github.com/xuri/excelize/v2"
)

var requiredImportHeaders = []string{"occurred_at", "type", "amount", "category", "account", "note", "tags"}

func readImportRows(file *multipart.FileHeader) ([][]string, error) {
	if file.Size > MaxImportFileBytes {
		return nil, fmt.Errorf("file too large: max %d MB", MaxImportFileBytes/1024/1024)
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if ext == ".xlsx" {
		tmp, err := io.ReadAll(io.LimitReader(f, MaxImportFileBytes+1))
		if err != nil {
			return nil, err
		}
		if len(tmp) > MaxImportFileBytes {
			return nil, fmt.Errorf("file too large: max %d MB", MaxImportFileBytes/1024/1024)
		}
		xlsx, err := excelize.OpenReader(bytes.NewReader(tmp))
		if err != nil {
			return nil, err
		}
		defer xlsx.Close()
		sheet := xlsx.GetSheetName(0)
		rows, err := readXLSXDataRows(xlsx, sheet)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, errors.New("empty xlsx")
		}
		if countImportDataRows(rows) == 0 {
			return nil, errors.New("empty xlsx")
		}
		return rows, nil
	}
	if ext != ".csv" {
		return nil, errors.New("unsupported file type")
	}

	rows, err := readCSVDataRows(io.LimitReader(f, MaxImportFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("empty csv")
	}
	if countImportDataRows(rows) == 0 {
		return nil, errors.New("empty csv")
	}
	return rows, nil
}

func readCSVDataRows(r io.Reader) ([][]string, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	rows := make([][]string, 0)
	line := 0
	dataRows := 0
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		line++
		if line == 1 {
			if err := validateImportHeader(row); err != nil {
				return nil, err
			}
			continue
		}
		rows = append(rows, row)
		if !isEmptyImportRow(row) {
			dataRows++
		}
		if dataRows > MaxImportRows {
			return nil, fmt.Errorf("too many rows: max %d", MaxImportRows)
		}
	}
	return rows, nil
}

func readXLSXDataRows(xlsx *excelize.File, sheet string) ([][]string, error) {
	iter, err := xlsx.Rows(sheet)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	rows := make([][]string, 0)
	line := 0
	dataRows := 0
	for iter.Next() {
		row, err := iter.Columns()
		if err != nil {
			return nil, err
		}
		line++
		if line == 1 {
			if err := validateImportHeader(row); err != nil {
				return nil, err
			}
			continue
		}
		rows = append(rows, row)
		if !isEmptyImportRow(row) {
			dataRows++
		}
		if dataRows > MaxImportRows {
			return nil, fmt.Errorf("too many rows: max %d", MaxImportRows)
		}
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}
	return rows, nil
}

func readImportRowsFromCSVContent(content string) ([][]string, error) {
	if len([]byte(content)) > MaxImportFileBytes {
		return nil, fmt.Errorf("file too large: max %d MB", MaxImportFileBytes/1024/1024)
	}
	rows, err := readCSVDataRows(strings.NewReader(content))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("empty csv")
	}
	if countImportDataRows(rows) == 0 {
		return nil, errors.New("empty csv")
	}
	return rows, nil
}

func validateImportHeader(row []string) error {
	if len(row) < len(requiredImportHeaders) {
		return fmt.Errorf("invalid header: expected %s", strings.Join(requiredImportHeaders, ","))
	}
	for i, want := range requiredImportHeaders {
		if normalizedImportHeader(row[i], i) != want {
			return fmt.Errorf("invalid header: expected %s", strings.Join(requiredImportHeaders, ","))
		}
	}
	return nil
}

func normalizedImportHeader(value string, index int) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if index == 0 {
		value = strings.TrimPrefix(value, "\ufeff")
	}
	return value
}

func parseImportRecord(row []string) (importRecord, error) {
	get := func(i int) string {
		if i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	occurredAt, err := timeutil.ParseDateTime(get(0))
	if err != nil {
		return importRecord{}, fmt.Errorf("invalid occurred_at")
	}
	typeVal := strings.ToLower(get(1))
	if typeVal != "income" && typeVal != "expense" {
		return importRecord{}, fmt.Errorf("invalid type")
	}
	amountCents, err := money.ParseCents(get(2))
	if err != nil || amountCents <= 0 {
		return importRecord{}, fmt.Errorf("invalid amount")
	}

	return importRecord{
		OccurredAt: occurredAt,
		Type:       typeVal,
		Amount:     money.FromCents(amountCents),
		Category:   stringutil.FallbackName(get(3), "餐饮"),
		Account:    stringutil.FallbackName(get(4), "现金"),
		Note:       get(5),
		Tags:       get(6),
	}, nil
}

func countImportDataRows(rows [][]string) int {
	total := 0
	for _, row := range rows {
		if !isEmptyImportRow(row) {
			total++
		}
	}
	return total
}

func isEmptyImportRow(row []string) bool {
	return strings.TrimSpace(strings.Join(row, "")) == ""
}
