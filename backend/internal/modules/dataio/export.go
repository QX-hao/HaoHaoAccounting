package dataio

import (
	"encoding/csv"
	"fmt"
	"net/url"
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
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", filename, url.PathEscape(filename))
}

// safeCSVCell 中和表格公式前缀，避免导出的 CSV/XLSX 被电子表格软件当作公式执行。
func safeCSVCell(value string) string {
	if value == "" {
		return value
	}

	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r', '\n':
		return "'" + value
	default:
		return value
	}
}
