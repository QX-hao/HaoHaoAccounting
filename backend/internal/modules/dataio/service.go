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

// Service 负责账单导入/导出的业务编排：解析文件、预览校验、批量写入和导入任务状态。
type Service struct {
	store              *store.Store
	transactionService *transactions.Service
	invalidator        transactions.CacheInvalidator
}

// ImportOptions 控制导入时的业务策略，当前主要用于跳过重复账单。
type ImportOptions struct {
	SkipDuplicates bool
}

func NewService(s *store.Store, transactionService *transactions.Service, invalidator transactions.CacheInvalidator) *Service {
	return &Service{store: s, transactionService: transactionService, invalidator: invalidator}
}

// ExportRows 导出指定时间范围内的原始交易，并预加载分类和账户，方便上层直接序列化。
func (s *Service) ExportRows(ctx context.Context, userID uint, start, end time.Time) ([]models.Transaction, error) {
	var rows []models.Transaction
	err := s.db(ctx).Where("user_id = ? AND occurred_at >= ? AND occurred_at <= ?", userID, start, end).
		Preload("Category").Preload("Account").
		Order("occurred_at asc").
		Find(&rows).Error
	return rows, err
}

// Preview 只读取和校验文件，不写数据库，给用户在正式导入前确认错误和重复行。
func (s *Service) Preview(ctx context.Context, userID uint, file *multipart.FileHeader) (ImportPreview, error) {
	sourceRows, err := readImportRows(file)
	if err != nil {
		return ImportPreview{}, err
	}
	return s.PreviewRows(ctx, userID, file.Filename, file.Size, sourceRows), nil
}

// PreviewText 支持移动端直接提交 CSV 文本，避免必须先生成本地文件再上传。
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

// PreviewRows 对解析后的行做格式校验和重复检测，只保留前几行样例返回给前端展示。
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

// Import 使用默认策略导入文件；默认会跳过重复行，降低误重复导入的风险。
func (s *Service) Import(ctx context.Context, userID uint, file *multipart.FileHeader) (ImportResult, error) {
	return s.ImportWithOptions(ctx, userID, file, defaultImportOptions())
}

// ImportWithOptions 负责文件读取，真正的数据写入交给 ImportRowsWithOptions 统一处理。
func (s *Service) ImportWithOptions(ctx context.Context, userID uint, file *multipart.FileHeader, options ImportOptions) (ImportResult, error) {
	sourceRows, err := readImportRows(file)
	if err != nil {
		return ImportResult{}, err
	}
	return s.ImportRowsWithOptions(ctx, userID, sourceRows, options), nil
}

// StartImportJob 把导入转为后台任务，HTTP 请求结束后仍继续处理已读入内存的数据。
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

// ListImportJobs 只展示最近的任务，避免历史导入记录过多拖慢页面。
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

// ImportJob 按用户和任务 ID 双重过滤，防止用户查看别人的导入结果。
func (s *Service) ImportJob(ctx context.Context, userID, id uint) (ImportJobResponse, error) {
	var job models.ImportJob
	if err := s.db(ctx).Where("id = ? AND user_id = ?", id, userID).First(&job).Error; err != nil {
		return ImportJobResponse{}, err
	}
	return importJobResponse(job), nil
}

// runImportJob 在后台执行导入并落库任务结果；失败行会以 JSON 字符串保存到任务表。
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

// updateImportJob 是后台任务的容错写状态入口，状态写失败不再打断已经完成的导入流程。
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

// ImportText 是移动端文本导入入口，和文件导入共用同一套解析、去重和写入规则。
func (s *Service) ImportText(ctx context.Context, userID uint, req ImportTextRequest) (ImportResult, error) {
	sourceRows, err := readImportRowsFromCSVContent(req.Content)
	if err != nil {
		return ImportResult{}, err
	}
	return s.ImportRowsWithOptions(ctx, userID, sourceRows, importOptionsFromRequest(req)), nil
}

// ImportRows 使用默认策略导入已经解析好的行，主要方便测试和内部复用。
func (s *Service) ImportRows(ctx context.Context, userID uint, sourceRows [][]string) ImportResult {
	return s.ImportRowsWithOptions(ctx, userID, sourceRows, defaultImportOptions())
}

// ImportRowsWithOptions 先逐行解析和去重，再在一个事务里创建缺失分类/账户并批量写入交易。
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
		if err := s.store.Transaction(ctx, func(dbtx *gorm.DB) error {
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

// importPreviewRow 把内部解析结果转成前端预览行，隐藏不需要暴露的内部字段。
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

// defaultImportOptions 默认跳过重复项，符合“多次上传同一文件也不重复记账”的预期。
func defaultImportOptions() ImportOptions {
	return ImportOptions{SkipDuplicates: true}
}

// importOptionsFromRequest 允许移动端显式覆盖默认去重策略。
func importOptionsFromRequest(req ImportTextRequest) ImportOptions {
	options := defaultImportOptions()
	if req.SkipDuplicates != nil {
		options.SkipDuplicates = *req.SkipDuplicates
	}
	return options
}

func (s *Service) db(ctx context.Context) *gorm.DB {
	return s.store.DBWithContext(ctx)
}

// detachedContext 去掉请求取消信号，避免用户关闭页面后后台导入任务被提前中断。
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

// importDuplicateKey 用业务字段生成去重键，分类/账户/标签做标准化后再比较。
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

// hasExistingDuplicate 检查数据库里是否已有完全相同的交易，避免重复导入历史账单。
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

// splitTags 将导入文件里的逗号分隔标签清洗成交易服务需要的字符串数组。
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

// importJobResponse 统一后台任务的 API 返回结构，并把数据库里的错误 JSON 还原成数组。
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
