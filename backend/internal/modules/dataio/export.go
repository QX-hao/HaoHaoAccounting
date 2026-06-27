package dataio

import (
	"encoding/csv"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/money"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

func writeCSV(c *gin.Context, rows []models.Transaction) error {
	filename := "transactions_" + time.Now().Format("20060102150405") + ".csv"
	c.Header("Content-Disposition", attachmentDisposition(filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")

	writer := csv.NewWriter(c.Writer)

	if err := writer.Write([]string{"occurred_at", "type", "amount", "category", "account", "note", "tags", "source"}); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write([]string{
			row.OccurredAt.Format(time.RFC3339),
			row.Type,
			money.FormatCents(row.AmountCents),
			safeCSVCell(row.Category.Name),
			safeCSVCell(row.Account.Name),
			safeCSVCell(row.Note),
			safeCSVCell(row.Tags),
			safeCSVCell(row.Source),
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeXLSX(c *gin.Context, rows []models.Transaction) error {
	filename := "transactions_" + time.Now().Format("20060102150405") + ".xlsx"
	c.Header("Content-Disposition", attachmentDisposition(filename))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := []string{"occurred_at", "type", "amount", "category", "account", "note", "tags", "source"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, header)
	}
	for idx, row := range rows {
		line := idx + 2
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", line), row.OccurredAt.Format(time.RFC3339))
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", line), row.Type)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", line), money.FromCents(row.AmountCents))
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", line), safeCSVCell(row.Category.Name))
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", line), safeCSVCell(row.Account.Name))
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", line), safeCSVCell(row.Note))
		_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", line), safeCSVCell(row.Tags))
		_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", line), safeCSVCell(row.Source))
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return err
	}
	_, err = c.Writer.Write(buf.Bytes())
	return err
}

// attachmentDisposition 同时写入 filename 和 filename*，兼容浏览器下载和客户端文件名解析。
func attachmentDisposition(filename string) string {
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", asciiFilenameFallback(filename), url.PathEscape(filename))
}

func asciiFilenameFallback(filename string) string {
	var builder strings.Builder
	builder.Grow(len(filename))
	for _, r := range filename {
		if asciiFilenameCharAllowed(r) {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('_')
	}
	fallback := strings.TrimSpace(builder.String())
	if fallback == "" || !asciiFilenameStemHasAlphanumeric(fallback) {
		if extension := asciiFilenameExtension(fallback); extension != "" {
			return "download" + extension
		}
		return "download"
	}
	fallback = strings.TrimLeft(fallback, "._-")
	if fallback == "" {
		return "download"
	}
	return fallback
}

func asciiFilenameCharAllowed(r rune) bool {
	return (r >= '0' && r <= '9') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		r == '.' ||
		r == '_' ||
		r == '-'
}

func asciiFilenameStemHasAlphanumeric(filename string) bool {
	stem := filename
	if dot := strings.LastIndexByte(filename, '.'); dot > 0 {
		stem = filename[:dot]
	}
	for _, r := range stem {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return true
		}
	}
	return false
}

func asciiFilenameExtension(filename string) string {
	dot := strings.LastIndexByte(filename, '.')
	if dot <= 0 || dot == len(filename)-1 {
		return ""
	}
	extension := filename[dot:]
	for _, r := range extension {
		if r != '.' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return extension
}

// safeCSVCell 中和表格公式前缀，避免导出的 CSV/XLSX 被电子表格软件当作公式执行。
func safeCSVCell(value string) string {
	if dangerousCSVFormulaPrefix(value) {
		return "'" + value
	}
	return value
}

func dangerousCSVFormulaPrefix(value string) bool {
	if value == "" {
		return false
	}
	switch value[0] {
	case '\t', '\r', '\n':
		return true
	}

	// 部分表格软件会忽略公式前的普通空格；判断时跳过空格，但保留原始导出内容。
	trimmed := strings.TrimLeft(value, " ")
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '=', '+', '-', '@', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}
