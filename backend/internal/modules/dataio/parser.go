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
	ext := strings.ToLower(filepath.Ext(file.Filename))
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if ext == ".xlsx" {
		tmp, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		xlsx, err := excelize.OpenReader(bytes.NewReader(tmp))
		if err != nil {
			return nil, err
		}
		sheet := xlsx.GetSheetName(0)
		rows, err := xlsx.GetRows(sheet)
		if err != nil {
			return nil, err
		}
		if len(rows) <= 1 {
			return nil, errors.New("empty xlsx")
		}
		return rows[1:], nil
	}

	reader := csv.NewReader(f)
	all, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(all) <= 1 {
		return nil, errors.New("empty csv")
	}
	return all[1:], nil
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
