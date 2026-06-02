package dataio

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/stringutil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/timeutil"
	"github.com/xuri/excelize/v2"
)

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
	return rows, nil
}

func readCSVDataRows(r io.Reader) ([][]string, error) {
	reader := csv.NewReader(r)
	rows := make([][]string, 0)
	line := 0
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
			continue
		}
		rows = append(rows, row)
		if len(rows) > MaxImportRows {
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
	for iter.Next() {
		row, err := iter.Columns()
		if err != nil {
			return nil, err
		}
		line++
		if line == 1 {
			continue
		}
		rows = append(rows, row)
		if len(rows) > MaxImportRows {
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
	return rows, nil
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
	amount, err := strconv.ParseFloat(get(2), 64)
	if err != nil || amount <= 0 {
		return importRecord{}, fmt.Errorf("invalid amount")
	}

	return importRecord{
		OccurredAt: occurredAt,
		Type:       typeVal,
		Amount:     amount,
		Category:   stringutil.FallbackName(get(3), "餐饮"),
		Account:    stringutil.FallbackName(get(4), "现金"),
		Note:       get(5),
		Tags:       get(6),
	}, nil
}
