package dataio

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"strings"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/transactions"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/money"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"gorm.io/gorm"
)

type Service struct {
	store              *store.Store
	transactionService *transactions.Service
	invalidator        transactions.CacheInvalidator
}

type ImportOptions struct {
	SkipDuplicates bool
}

func NewService(s *store.Store, transactionService *transactions.Service, invalidator transactions.CacheInvalidator) *Service {
	return &Service{store: s, transactionService: transactionService, invalidator: invalidator}
}

func (s *Service) ExportRows(ctx context.Context, userID uint, start, end time.Time) ([]models.Transaction, error) {
	var rows []models.Transaction
	err := s.db(ctx).Where("user_id = ? AND occurred_at >= ? AND occurred_at <= ?", userID, start, end).
		Preload("Category").Preload("Account").
		Order("occurred_at asc").
		Find(&rows).Error
	return rows, err
}

func (s *Service) Preview(ctx context.Context, userID uint, file *multipart.FileHeader) (ImportPreview, error) {
	sourceRows, err := readImportRows(file)
	if err != nil {
		return ImportPreview{}, err
	}
	return s.PreviewRows(ctx, userID, file.Filename, file.Size, sourceRows), nil
}

func (s *Service) PreviewText(ctx context.Context, userID uint, req ImportTextRequest) (ImportPreview, error) {
	sourceRows, err := readImportRowsFromCSVContent(req.Content)
	if err != nil {
		return ImportPreview{}, err
	}
	filename := strings.TrimSpace(req.Filename)
	if filename == "" {
		filename = "mobile-import.csv"
	}
	return s.PreviewRows(ctx, userID, filename, int64(len([]byte(req.Content))), sourceRows), nil
}

func (s *Service) PreviewRows(ctx context.Context, userID uint, filename string, size int64, sourceRows [][]string) ImportPreview {
	preview := ImportPreview{
		Filename:     filename,
		Size:         size,
		MaxRows:      MaxImportRows,
		MaxFileBytes: MaxImportFileBytes,
		Rows:         make([]ImportPreviewRow, 0, min(len(sourceRows), ImportPreviewRows)),
	}
	seen := map[duplicateKey]int{}
	for i, row := range sourceRows {
		if isEmptyImportRow(row) {
			continue
		}
		preview.TotalRows++
		record, err := parseImportRecord(row)
		duplicateReason := ""
		if err != nil {
			preview.FailedRows++
		} else {
			preview.ValidRows++
			key := importDuplicateKey(record)
			if firstLine, ok := seen[key]; ok {
				duplicateReason = fmt.Sprintf("与导入文件第 %d 行重复", firstLine+2)
			} else if s.hasExistingDuplicate(ctx, userID, record) {
				duplicateReason = "账本中已存在相同记录"
			} else {
				seen[key] = i
			}
			if duplicateReason != "" {
				preview.DuplicateRows++
			}
		}
		if len(preview.Rows) < ImportPreviewRows {
			preview.Rows = append(preview.Rows, importPreviewRow(i, record, err, duplicateReason))
		}
	}
	preview.Truncated = preview.TotalRows > len(preview.Rows)
	return preview
}

func (s *Service) Import(ctx context.Context, userID uint, file *multipart.FileHeader) (ImportResult, error) {
	return s.ImportWithOptions(ctx, userID, file, defaultImportOptions())
}

func (s *Service) ImportWithOptions(ctx context.Context, userID uint, file *multipart.FileHeader, options ImportOptions) (ImportResult, error) {
	sourceRows, err := readImportRows(file)
	if err != nil {
		return ImportResult{}, err
	}
	return s.ImportRowsWithOptions(ctx, userID, sourceRows, options), nil
}

func (s *Service) StartImportJob(ctx context.Context, userID uint, file *multipart.FileHeader, options ImportOptions) (ImportJobResponse, error) {
	sourceRows, err := readImportRows(file)
	if err != nil {
		return ImportJobResponse{}, err
	}
	job := models.ImportJob{
		UserID:   userID,
		Filename: strings.TrimSpace(file.Filename),
		Status:   "queued",
		Total:    countImportDataRows(sourceRows),
	}
	if job.Filename == "" {
		job.Filename = "import.csv"
	}
	if err := s.db(ctx).Create(&job).Error; err != nil {
		return ImportJobResponse{}, err
	}

	jobCtx := detachedContext(ctx)
	go s.runImportJob(jobCtx, job.ID, userID, sourceRows, options)
	return importJobResponse(job), nil
}

func (s *Service) ListImportJobs(ctx context.Context, userID uint) ([]ImportJobResponse, error) {
	var jobs []models.ImportJob
	if err := s.db(ctx).Where("user_id = ?", userID).Order("created_at desc").Limit(20).Find(&jobs).Error; err != nil {
		return nil, err
	}
	result := make([]ImportJobResponse, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, importJobResponse(job))
	}
	return result, nil
}

func (s *Service) ImportJob(ctx context.Context, userID, id uint) (ImportJobResponse, error) {
	var job models.ImportJob
	if err := s.db(ctx).Where("id = ? AND user_id = ?", id, userID).First(&job).Error; err != nil {
		return ImportJobResponse{}, err
	}
	return importJobResponse(job), nil
}

func (s *Service) runImportJob(ctx context.Context, jobID, userID uint, sourceRows [][]string, options ImportOptions) {
	s.updateImportJob(ctx, jobID, models.ImportJob{Status: "running"})
	result := s.ImportRowsWithOptions(ctx, userID, sourceRows, options)
	status := "completed"
	if result.Failed > 0 {
		status = "failed"
	}
	errorsJSON, _ := json.Marshal(result.Errors)
	s.updateImportJob(ctx, jobID, models.ImportJob{
		Status:  status,
		Total:   result.Total,
		Success: result.Success,
		Failed:  result.Failed,
		Skipped: result.Skipped,
		Errors:  string(errorsJSON),
	})
}

func (s *Service) updateImportJob(ctx context.Context, jobID uint, changes models.ImportJob) {
	values := map[string]any{}
	if changes.Status != "" {
		values["status"] = changes.Status
	}
	if changes.Total > 0 {
		values["total"] = changes.Total
	}
	values["success"] = changes.Success
	values["failed"] = changes.Failed
	values["skipped"] = changes.Skipped
	if changes.Errors != "" {
		values["errors"] = changes.Errors
	}
	_ = s.db(ctx).Model(&models.ImportJob{}).Where("id = ?", jobID).Updates(values).Error
}

func (s *Service) ImportText(ctx context.Context, userID uint, req ImportTextRequest) (ImportResult, error) {
	sourceRows, err := readImportRowsFromCSVContent(req.Content)
	if err != nil {
		return ImportResult{}, err
	}
	return s.ImportRowsWithOptions(ctx, userID, sourceRows, importOptionsFromRequest(req)), nil
}

func (s *Service) ImportRows(ctx context.Context, userID uint, sourceRows [][]string) ImportResult {
	return s.ImportRowsWithOptions(ctx, userID, sourceRows, defaultImportOptions())
}

func (s *Service) ImportRowsWithOptions(ctx context.Context, userID uint, sourceRows [][]string, options ImportOptions) ImportResult {
	result := ImportResult{Errors: make([]string, 0)}
	records := make([]importRecord, 0, len(sourceRows))
	seen := map[duplicateKey]int{}
	for i, row := range sourceRows {
		if isEmptyImportRow(row) {
			continue
		}
		result.Total++
		record, err := parseImportRecord(row)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, importLineError(i, err))
			continue
		}
		record.Line = i
		key := importDuplicateKey(record)
		if options.SkipDuplicates {
			if firstLine, ok := seen[key]; ok {
				result.Skipped++
				result.Errors = append(result.Errors, fmt.Sprintf("line %d: duplicate of line %d skipped", i+2, firstLine+2))
				continue
			}
			if s.hasExistingDuplicate(ctx, userID, record) {
				result.Skipped++
				result.Errors = append(result.Errors, fmt.Sprintf("line %d: duplicate transaction skipped", i+2))
				continue
			}
		}
		seen[key] = i
		records = append(records, record)
	}

	if len(records) > 0 {
		if err := s.db(ctx).Transaction(func(dbtx *gorm.DB) error {
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

func importPreviewRow(index int, record importRecord, err error, duplicateReason string) ImportPreviewRow {
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
	if duplicateReason != "" {
		row.Duplicate = true
		row.DuplicateReason = duplicateReason
	}
	return row
}

func defaultImportOptions() ImportOptions {
	return ImportOptions{SkipDuplicates: true}
}

func importOptionsFromRequest(req ImportTextRequest) ImportOptions {
	options := defaultImportOptions()
	if req.SkipDuplicates != nil {
		options.SkipDuplicates = *req.SkipDuplicates
	}
	return options
}

func (s *Service) db(ctx context.Context) *gorm.DB {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.store.DB.WithContext(ctx)
}

func detachedContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

type duplicateKey struct {
	OccurredAt  int64
	Type        string
	AmountCents int64
	Category    string
	Account     string
	Note        string
	Tags        string
}

func importDuplicateKey(record importRecord) duplicateKey {
	return duplicateKey{
		OccurredAt:  record.OccurredAt.Unix(),
		Type:        record.Type,
		AmountCents: money.ToCents(record.Amount),
		Category:    strings.ToLower(strings.TrimSpace(record.Category)),
		Account:     strings.ToLower(strings.TrimSpace(record.Account)),
		Note:        strings.TrimSpace(record.Note),
		Tags:        strings.Join(splitTags(record.Tags), ","),
	}
}

func (s *Service) hasExistingDuplicate(ctx context.Context, userID uint, record importRecord) bool {
	category, err := s.store.FindCategoryByNameWithDB(s.db(ctx), userID, record.Type, record.Category)
	if err != nil {
		return false
	}
	accountName := strings.TrimSpace(record.Account)
	if accountName == "" {
		accountName = "现金"
	}
	var account models.Account
	if err := s.db(ctx).Where("user_id = ? AND name = ?", userID, accountName).First(&account).Error; err != nil {
		return false
	}

	var count int64
	err = s.db(ctx).Model(&models.Transaction{}).
		Where(
			"user_id = ? AND occurred_at = ? AND type = ? AND amount_cents = ? AND category_id = ? AND account_id = ? AND note = ? AND tags = ?",
			userID,
			record.OccurredAt,
			record.Type,
			money.ToCents(record.Amount),
			category.ID,
			account.ID,
			strings.TrimSpace(record.Note),
			strings.Join(splitTags(record.Tags), ","),
		).
		Count(&count).Error
	return err == nil && count > 0
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

func importJobResponse(job models.ImportJob) ImportJobResponse {
	var errors []string
	if strings.TrimSpace(job.Errors) != "" {
		_ = json.Unmarshal([]byte(job.Errors), &errors)
	}
	return ImportJobResponse{
		ID:        job.ID,
		Filename:  job.Filename,
		Status:    job.Status,
		Total:     job.Total,
		Success:   job.Success,
		Failed:    job.Failed,
		Skipped:   job.Skipped,
		Errors:    errors,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
	}
}
