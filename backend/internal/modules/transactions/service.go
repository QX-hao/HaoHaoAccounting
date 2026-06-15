package transactions

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/money"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/stringutil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"gorm.io/gorm"
)

type Service struct {
	store       *store.Store
	invalidator CacheInvalidator
}

func NewService(s *store.Store, invalidator CacheInvalidator) *Service {
	if invalidator == nil {
		invalidator = noopInvalidator{}
	}
	return &Service{store: s, invalidator: invalidator}
}

func (s *Service) List(ctx context.Context, userID uint, filter ListFilter) ([]models.Transaction, int64, error) {
	page := filter.Page
	pageSize := filter.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	query := s.db(ctx).Model(&models.Transaction{}).Where("user_id = ?", userID)
	if !filter.Start.IsZero() {
		query = query.Where("occurred_at >= ?", filter.Start)
	}
	if !filter.End.IsZero() {
		query = query.Where("occurred_at <= ?", filter.End)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.CategoryID > 0 {
		query = query.Where("category_id = ?", filter.CategoryID)
	}
	if filter.AccountID > 0 {
		query = query.Where("account_id = ?", filter.AccountID)
	}
	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		query = query.Where("note LIKE ? OR tags LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []models.Transaction
	if err := query.Preload("Category").Preload("Account").
		Order("occurred_at desc, id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}

func (s *Service) Create(ctx context.Context, userID uint, req Request) (models.Transaction, error) {
	var tx models.Transaction
	if err := s.db(ctx).Transaction(func(dbtx *gorm.DB) error {
		next, err := s.createWithDB(dbtx, userID, req)
		if err != nil {
			return err
		}
		tx = next
		return nil
	}); err != nil {
		return models.Transaction{}, err
	}

	s.db(ctx).Preload("Category").Preload("Account").First(&tx, tx.ID)
	s.invalidateUser(ctx, userID)
	return tx, nil
}

func (s *Service) CreateMany(ctx context.Context, userID uint, requests []Request) ([]models.Transaction, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	created := make([]models.Transaction, 0, len(requests))
	if err := s.db(ctx).Transaction(func(dbtx *gorm.DB) error {
		next, err := s.CreateManyWithDB(dbtx, userID, requests)
		if err != nil {
			return err
		}
		created = next
		return nil
	}); err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(created))
	for _, tx := range created {
		ids = append(ids, tx.ID)
	}
	if len(ids) > 0 {
		s.db(ctx).Preload("Category").Preload("Account").Find(&created, ids)
	}
	s.invalidateUser(ctx, userID)
	return created, nil
}

func (s *Service) CreateManyWithDB(dbtx *gorm.DB, userID uint, requests []Request) ([]models.Transaction, error) {
	created := make([]models.Transaction, 0, len(requests))
	for _, req := range requests {
		tx, err := s.createWithDB(dbtx, userID, req)
		if err != nil {
			return nil, err
		}
		created = append(created, tx)
	}
	return created, nil
}

func (s *Service) createWithDB(dbtx *gorm.DB, userID uint, req Request) (models.Transaction, error) {
	if err := validateRequest(req); err != nil {
		return models.Transaction{}, err
	}
	if err := s.ensureCategoryAndAccountWithDB(dbtx, userID, req.Type, req.CategoryID, req.AccountID); err != nil {
		return models.Transaction{}, err
	}
	if req.OccurredAt.IsZero() {
		req.OccurredAt = time.Now()
	}

	tx := models.Transaction{
		UserID:      userID,
		Type:        req.Type,
		AmountCents: money.ToCents(req.Amount),
		CategoryID:  req.CategoryID,
		AccountID:   req.AccountID,
		Note:        strings.TrimSpace(req.Note),
		Tags:        strings.Join(req.Tags, ","),
		OccurredAt:  req.OccurredAt,
		Source:      stringutil.FallbackName(req.Source, "manual"),
	}

	if err := dbtx.Create(&tx).Error; err != nil {
		return models.Transaction{}, err
	}
	if err := applyAccountDelta(dbtx, tx.AccountID, tx.Type, tx.AmountCents); err != nil {
		return models.Transaction{}, err
	}
	return tx, nil
}

func (s *Service) Update(ctx context.Context, userID, id uint, req Request) (models.Transaction, error) {
	var existing models.Transaction
	if err := s.db(ctx).Where("id = ? AND user_id = ?", id, userID).First(&existing).Error; err != nil {
		return models.Transaction{}, errors.New("transaction not found")
	}

	if err := validateRequest(req); err != nil {
		return models.Transaction{}, err
	}
	if err := s.ensureCategoryAndAccount(ctx, userID, req.Type, req.CategoryID, req.AccountID); err != nil {
		return models.Transaction{}, err
	}
	if req.OccurredAt.IsZero() {
		req.OccurredAt = existing.OccurredAt
	}

	updated := existing
	updated.Type = req.Type
	updated.AmountCents = money.ToCents(req.Amount)
	updated.CategoryID = req.CategoryID
	updated.AccountID = req.AccountID
	updated.Note = strings.TrimSpace(req.Note)
	updated.Tags = strings.Join(req.Tags, ",")
	updated.OccurredAt = req.OccurredAt
	updated.Source = stringutil.FallbackName(req.Source, existing.Source)

	// Balance must be reconciled around the old and new transaction values.
	if err := s.db(ctx).Transaction(func(dbtx *gorm.DB) error {
		if err := revertAccountDelta(dbtx, existing.AccountID, existing.Type, existing.AmountCents); err != nil {
			return err
		}
		if err := applyAccountDelta(dbtx, updated.AccountID, updated.Type, updated.AmountCents); err != nil {
			return err
		}
		return dbtx.Save(&updated).Error
	}); err != nil {
		return models.Transaction{}, err
	}

	s.db(ctx).Preload("Category").Preload("Account").First(&updated, updated.ID)
	s.invalidateUser(ctx, userID)
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, userID, id uint) error {
	var tx models.Transaction
	if err := s.db(ctx).Where("id = ? AND user_id = ?", id, userID).First(&tx).Error; err != nil {
		return errors.New("transaction not found")
	}

	if err := s.db(ctx).Transaction(func(dbtx *gorm.DB) error {
		if err := revertAccountDelta(dbtx, tx.AccountID, tx.Type, tx.AmountCents); err != nil {
			return err
		}
		return dbtx.Delete(&tx).Error
	}); err != nil {
		return err
	}

	s.invalidateUser(ctx, userID)
	return nil
}

func (s *Service) ensureCategoryAndAccount(ctx context.Context, userID uint, txType string, categoryID, accountID uint) error {
	return s.ensureCategoryAndAccountWithDB(s.db(ctx), userID, txType, categoryID, accountID)
}

func (s *Service) db(ctx context.Context) *gorm.DB {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.store.DB.WithContext(ctx)
}

func (s *Service) invalidateUser(ctx context.Context, userID uint) {
	if ctx == nil {
		ctx = context.Background()
	} else {
		ctx = context.WithoutCancel(ctx)
	}
	s.invalidator.InvalidateUser(ctx, userID)
}

func (s *Service) ensureCategoryAndAccountWithDB(db *gorm.DB, userID uint, txType string, categoryID, accountID uint) error {
	var category models.Category
	if err := db.Where("id = ?", categoryID).First(&category).Error; err != nil {
		return errors.New("category not found")
	}
	if category.Type != txType {
		return errors.New("category type mismatch")
	}
	if !category.IsSystem {
		if category.UserID == nil || *category.UserID != userID {
			return errors.New("category not accessible")
		}
	}

	var account models.Account
	if err := db.Where("id = ? AND user_id = ?", accountID, userID).First(&account).Error; err != nil {
		return errors.New("account not found")
	}
	return nil
}

func validateRequest(req Request) error {
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if req.Type != "income" && req.Type != "expense" {
		return errors.New("type must be income or expense")
	}
	if req.Amount <= 0 {
		return errors.New("amount must be > 0")
	}
	if req.CategoryID == 0 {
		return errors.New("categoryId is required")
	}
	if req.AccountID == 0 {
		return errors.New("accountId is required")
	}
	if !req.AllowEmptyNote && strings.TrimSpace(req.Note) == "" {
		return errors.New("note is required")
	}
	return nil
}

func applyAccountDelta(dbtx *gorm.DB, accountID uint, txType string, amountCents int64) error {
	delta := amountCents
	if txType == "expense" {
		delta = -amountCents
	}
	return dbtx.Model(&models.Account{}).
		Where("id = ?", accountID).
		Update("balance_cents", gorm.Expr("balance_cents + ?", delta)).Error
}

func revertAccountDelta(dbtx *gorm.DB, accountID uint, txType string, amountCents int64) error {
	delta := amountCents
	if txType == "expense" {
		delta = -amountCents
	}
	return dbtx.Model(&models.Account{}).
		Where("id = ?", accountID).
		Update("balance_cents", gorm.Expr("balance_cents - ?", delta)).Error
}
