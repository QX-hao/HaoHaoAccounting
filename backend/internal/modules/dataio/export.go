package dataio

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

func writeCSV(c *gin.Context, rows []models.Transaction) {
	filename := "transactions_" + time.Now().Format("20060102150405") + ".csv"
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	_ = writer.Write([]string{"occurred_at", "type", "amount", "category", "account", "note", "tags", "source"})
	for _, row := range rows {
		_ = writer.Write([]string{
			row.OccurredAt.Format(time.RFC3339),
			row.Type,
			strconv.FormatFloat(row.Amount, 'f', 2, 64),
			row.Category.Name,
			row.Account.Name,
			row.Note,
			row.Tags,
			row.Source,
		})
	}
}

func writeXLSX(c *gin.Context, rows []models.Transaction) error {
	filename := "transactions_" + time.Now().Format("20060102150405") + ".xlsx"
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
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
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", line), row.Amount)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", line), row.Category.Name)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", line), row.Account.Name)
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", line), row.Note)
		_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", line), row.Tags)
		_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", line), row.Source)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return err
	}
	_, err = c.Writer.Write(buf.Bytes())
	return err
}
