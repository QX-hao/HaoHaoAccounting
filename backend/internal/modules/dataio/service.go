package dataio

import (
	"fmt"
	"mime/multipart"
	"strings"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/transactions"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"gorm.io/gorm"
)

type Service struct {
	store              *store.Store
	transactionService *transactions.Service
	invalidator        transactions.CacheInvalidator
}

func NewService(s *store.Store, transactionService *transactions.Service, invalidator transactions.CacheInvalidator) *Service {
	return &Service{store: s, transactionService: transactionService, invalidator: invalidator}
}

func (s *Service) ExportRows(userID uint, start, end time.Time) ([]models.Transaction, error) {
	var rows []models.Transaction
	err := s.store.DB.Where("user_id = ? AND occurred_at >= ? AND occurred_at <= ?", userID, start, end).
		Preload("Category").Preload("Account").
		Order("occurred_at asc").
		Find(&rows).Error
	return rows, err
}

func (s *Service) Preview(userID uint, file *multipart.FileHeader) (ImportPreview, error) {
	sourceRows, err := readImportRows(file)
	if err != nil {
		return ImportPreview{}, err
	}
	return s.PreviewRows(file.Filename, file.Size, sourceRows), nil
}

func (s *Service) PreviewText(userID uint, req ImportTextRequest) (ImportPreview, error) {
	sourceRows, err := readImportRowsFromCSVContent(req.Content)
	if err != nil {
		return ImportPreview{}, err
	}
	filename := strings.TrimSpace(req.Filename)
	if filename == "" {
		filename = "mobile-import.csv"
	}
	return s.PreviewRows(filename, int64(len([]byte(req.Content))), sourceRows), nil
}

func (s *Service) PreviewRows(filename string, size int64, sourceRows [][]string) ImportPreview {
	preview := ImportPreview{
		Filename:     filename,
		Size:         size,
		TotalRows:    len(sourceRows),
		MaxRows:      MaxImportRows,
		MaxFileBytes: MaxImportFileBytes,
		Rows:         make([]ImportPreviewRow, 0, min(len(sourceRows), ImportPreviewRows)),
	}
	for i, row := range sourceRows {
		record, err := parseImportRecord(row)
		if err != nil {
			preview.FailedRows++
		} else {
			preview.ValidRows++
		}
		if len(preview.Rows) < ImportPreviewRows {
			preview.Rows = append(preview.Rows, importPreviewRow(i, record, err))
		}
	}
	preview.Truncated = len(sourceRows) > len(preview.Rows)
	return preview
}

func (s *Service) Import(userID uint, file *multipart.FileHeader) (ImportResult, error) {
	sourceRows, err := readImportRows(file)
	if err != nil {
		return ImportResult{}, err
	}
	return s.ImportRows(userID, sourceRows), nil
}

func (s *Service) ImportText(userID uint, req ImportTextRequest) (ImportResult, error) {
	sourceRows, err := readImportRowsFromCSVContent(req.Content)
	if err != nil {
		return ImportResult{}, err
	}
	return s.ImportRows(userID, sourceRows), nil
}

func (s *Service) ImportRows(userID uint, sourceRows [][]string) ImportResult {
	result := ImportResult{Total: len(sourceRows), Errors: make([]string, 0)}
	records := make([]importRecord, 0, len(sourceRows))
	for i, row := range sourceRows {
		if strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}
		record, err := parseImportRecord(row)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, importLineError(i, err))
			continue
		}
		record.Line = i
		records = append(records, record)
	}

	if len(records) > 0 {
		if err := s.store.DB.Transaction(func(dbtx *gorm.DB) error {
			requests := make([]transactions.Request, 0, len(records))
			for _, record := range records {
				category, err := s.store.FindOrCreateCategoryWithDB(dbtx, userID, record.Type, record.Category)
				if err != nil {
					return fmt.Errorf("line %d: %w", record.Line+2, err)
				}
				account, err := s.store.FindOrCreateAccountWithDB(dbtx, userID, record.Account)
				if err != nil {
					return fmt.Errorf("line %d: %w", record.Line+2, err)
				}

				requests = append(requests, transactions.Request{
					Type:           record.Type,
					Amount:         record.Amount,
					CategoryID:     category.ID,
					AccountID:      account.ID,
					Note:           record.Note,
					Tags:           splitTags(record.Tags),
					Source:         "import",
					OccurredAt:     record.OccurredAt,
					AllowEmptyNote: true,
				})
			}
			_, err := s.transactionService.CreateManyWithDB(dbtx, userID, requests)
			return err
		}); err != nil {
			result.Failed += len(records)
			result.Errors = append(result.Errors, "batch import: "+err.Error())
			return result
		}
		result.Success = len(records)
	}
	return result
}

func importPreviewRow(index int, record importRecord, err error) ImportPreviewRow {
	row := ImportPreviewRow{Line: index + 2, Valid: err == nil}
	if err != nil {
		row.Error = err.Error()
		return row
	}
	row.OccurredAt = record.OccurredAt.Format(time.RFC3339)
	row.Type = record.Type
	row.Amount = record.Amount
	row.Category = record.Category
	row.Account = record.Account
	row.Note = record.Note
	row.Tags = record.Tags
	return row
}

func splitTags(tags string) []string {
	if strings.TrimSpace(tags) == "" {
		return nil
	}
	values := strings.Split(tags, ",")
	result := make([]string, 0, len(values))
	for _, value := range values {
		if clean := strings.TrimSpace(value); clean != "" {
			result = append(result, clean)
		}
	}
	return result
}
