package budgets

import (
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

func (s *Service) List(userID uint, month string) ([]models.Budget, error) {
	query := s.store.DB.Where("user_id = ?", userID)
	if cleanMonth := strings.TrimSpace(month); cleanMonth != "" {
		query = query.Where("month = ?", cleanMonth)
	}

	var budgets []models.Budget
	err := query.Order("month desc, category_id asc, id asc").Find(&budgets).Error
	return budgets, err
}

func (s *Service) Create(userID uint, req budgetRequest) (models.Budget, error) {
	budget, err := s.buildBudget(userID, models.Budget{}, req)
	if err != nil {
		return models.Budget{}, err
	}
	if err := s.store.DB.Create(&budget).Error; err != nil {
		return models.Budget{}, err
	}
	s.invalidateUser(userID)
	return budget, nil
}

func (s *Service) Update(userID, id uint, req budgetRequest) (models.Budget, error) {
	var budget models.Budget
	if err := s.store.DB.Where("id = ? AND user_id = ?", id, userID).First(&budget).Error; err != nil {
		return models.Budget{}, errors.New("budget not found")
	}

	next, err := s.buildBudget(userID, budget, req)
	if err != nil {
		return models.Budget{}, err
	}
	if err := s.store.DB.Save(&next).Error; err != nil {
		return models.Budget{}, err
	}
	s.invalidateUser(userID)
	return next, nil
}

func (s *Service) Delete(userID, id uint) error {
	result := s.store.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Budget{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("budget not found")
	}
	s.invalidateUser(userID)
	return nil
}

func (s *Service) invalidateUser(userID uint) {
	if s.invalidator != nil {
		s.invalidator.InvalidateUser(userID)
	}
}

func (s *Service) buildBudget(userID uint, existing models.Budget, req budgetRequest) (models.Budget, error) {
	month := strings.TrimSpace(req.Month)
	if month == "" {
		month = existing.Month
	}
	if _, err := time.Parse("2006-01", month); err != nil {
		return models.Budget{}, errors.New("month must be YYYY-MM")
	}
	if req.Amount < 0 {
		return models.Budget{}, errors.New("amount must be >= 0")
	}
	if req.CategoryID > 0 {
		if err := s.ensureExpenseCategory(userID, req.CategoryID); err != nil {
			return models.Budget{}, err
		}
	}

	existing.UserID = userID
	existing.Month = month
	existing.CategoryID = req.CategoryID
	existing.AmountCents = money.ToCents(req.Amount)
	return existing, nil
}

func (s *Service) ensureExpenseCategory(userID, categoryID uint) error {
	var category models.Category
	if err := s.store.DB.Where("id = ?", categoryID).First(&category).Error; err != nil {
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
