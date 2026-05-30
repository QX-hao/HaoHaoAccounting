package dataio

import (
	"mime/multipart"
	"strings"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/transactions"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
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

func (s *Service) Import(userID uint, file *multipart.FileHeader) (ImportResult, error) {
	sourceRows, err := readImportRows(file)
	if err != nil {
		return ImportResult{}, err
	}

	result := ImportResult{Errors: make([]string, 0)}
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

		category, err := s.store.FindOrCreateCategory(userID, record.Type, record.Category)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, importLineError(i, err))
			continue
		}
		account, err := s.store.FindOrCreateAccount(userID, record.Account)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, importLineError(i, err))
			continue
		}

		// Imported rows go through the transaction service to reuse validation,
		// balance updates, and cache invalidation behavior from manual entries.
		_, err = s.transactionService.Create(userID, transactions.Request{
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
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, importLineError(i, err))
			continue
		}

		result.Success++
	}

	s.invalidator.InvalidateUser(userID)
	return result, nil
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
