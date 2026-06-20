package budgets

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/transactions"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/money"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"gorm.io/gorm"
)

type Service struct {
	store       *store.Store
	invalidator transactions.CacheInvalidator
}

func NewService(s *store.Store, invalidator transactions.CacheInvalidator) *Service {
	return &Service{store: s, invalidator: invalidator}
}

func (s *Service) List(ctx context.Context, userID uint, month string) ([]models.Budget, error) {
	query := s.db(ctx).Where("user_id = ?", userID)
	if cleanMonth := strings.TrimSpace(month); cleanMonth != "" {
		query = query.Where("month = ?", cleanMonth)
	}

	var budgets []models.Budget
	err := query.Order("month desc, category_id asc, id asc").Find(&budgets).Error
	return budgets, err
}

func (s *Service) Create(ctx context.Context, userID uint, req budgetRequest) (models.Budget, error) {
	budget, err := s.buildBudget(ctx, userID, models.Budget{}, req)
	if err != nil {
		return models.Budget{}, err
	}
	if err := s.db(ctx).Create(&budget).Error; err != nil {
		return models.Budget{}, err
	}
	s.invalidateUser(ctx, userID)
	return budget, nil
}

func (s *Service) Update(ctx context.Context, userID, id uint, req budgetRequest) (models.Budget, error) {
	var budget models.Budget
	if err := s.db(ctx).Where("id = ? AND user_id = ?", id, userID).First(&budget).Error; err != nil {
		return models.Budget{}, errors.New("budget not found")
	}

	next, err := s.buildBudget(ctx, userID, budget, req)
	if err != nil {
		return models.Budget{}, err
	}
	if err := s.db(ctx).Save(&next).Error; err != nil {
		return models.Budget{}, err
	}
	s.invalidateUser(ctx, userID)
	return next, nil
}

func (s *Service) Delete(ctx context.Context, userID, id uint) error {
	result := s.db(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&models.Budget{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("budget not found")
	}
	s.invalidateUser(ctx, userID)
	return nil
}

func (s *Service) db(ctx context.Context) *gorm.DB {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.store.DB.WithContext(ctx)
}

func (s *Service) invalidateUser(ctx context.Context, userID uint) {
	if s.invalidator != nil {
		if ctx == nil {
			ctx = context.Background()
		} else {
			ctx = context.WithoutCancel(ctx)
		}
		s.invalidator.InvalidateUser(ctx, userID)
	}
}

func (s *Service) buildBudget(ctx context.Context, userID uint, existing models.Budget, req budgetRequest) (models.Budget, error) {
	month := strings.TrimSpace(req.Month)
	if month == "" {
		month = existing.Month
	}
	if _, err := time.Parse("2006-01", month); err != nil {
		return models.Budget{}, errors.New("month must be YYYY-MM")
	}
	if req.Amount == nil {
		return models.Budget{}, errors.New("amount is required")
	}
	if *req.Amount < 0 {
		return models.Budget{}, errors.New("amount must be >= 0")
	}
	amountCents, err := money.ToCentsExact(*req.Amount)
	if err != nil {
		return models.Budget{}, err
	}
	if req.CategoryID > 0 {
		if err := s.ensureExpenseCategory(ctx, userID, req.CategoryID); err != nil {
			return models.Budget{}, err
		}
	}

	existing.UserID = userID
	existing.Month = month
	existing.CategoryID = req.CategoryID
	existing.AmountCents = amountCents
	return existing, nil
}

func (s *Service) ensureExpenseCategory(ctx context.Context, userID, categoryID uint) error {
	var category models.Category
	if err := s.db(ctx).Where("id = ?", categoryID).First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("category not found")
		}
		return err
	}
	if category.Type != "expense" {
		return errors.New("budget category must be expense")
	}
	if !category.IsSystem && (category.UserID == nil || *category.UserID != userID) {
		return errors.New("category not accessible")
	}
	return nil
}
