package categories

import (
	"context"
	"errors"
	"strings"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/transactions"
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

func (s *Service) List(ctx context.Context, userID uint, txType string) ([]models.Category, error) {
	query := s.db(ctx).Model(&models.Category{}).
		Where("is_system = ? OR user_id = ?", true, userID)
	if strings.TrimSpace(txType) != "" {
		query = query.Where("type = ?", strings.TrimSpace(txType))
	}

	var categories []models.Category
	err := query.Order("is_system desc, id asc").Find(&categories).Error
	return categories, err
}

func (s *Service) Create(ctx context.Context, userID uint, req categoryRequest) (models.Category, error) {
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if req.Type != "income" && req.Type != "expense" {
		return models.Category{}, errors.New("type must be income or expense")
	}
	if strings.TrimSpace(req.Name) == "" {
		return models.Category{}, errors.New("name is required")
	}

	category := models.Category{UserID: &userID, Name: strings.TrimSpace(req.Name), Type: req.Type, IsSystem: false}
	if err := s.db(ctx).Create(&category).Error; err != nil {
		return models.Category{}, err
	}
	s.invalidateUser(ctx, userID)
	return category, nil
}

func (s *Service) Update(ctx context.Context, userID, id uint, req categoryRequest) (models.Category, error) {
	var category models.Category
	if err := s.db(ctx).Where("id = ?", id).First(&category).Error; err != nil {
		return models.Category{}, errors.New("category not found")
	}
	if category.IsSystem || category.UserID == nil || *category.UserID != userID {
		return models.Category{}, errors.New("system category cannot be modified")
	}

	if strings.TrimSpace(req.Name) != "" {
		category.Name = strings.TrimSpace(req.Name)
	}
	if t := strings.ToLower(strings.TrimSpace(req.Type)); t == "income" || t == "expense" {
		category.Type = t
	}

	if err := s.db(ctx).Save(&category).Error; err != nil {
		return models.Category{}, err
	}
	s.invalidateUser(ctx, userID)
	return category, nil
}

func (s *Service) Delete(ctx context.Context, userID, id uint) error {
	var category models.Category
	if err := s.db(ctx).First(&category, id).Error; err != nil {
		return errors.New("category not found")
	}
	if category.IsSystem || category.UserID == nil || *category.UserID != userID {
		return errors.New("system category cannot be deleted")
	}

	var count int64
	if err := s.db(ctx).Model(&models.Transaction{}).Where("user_id = ? AND category_id = ?", userID, id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("category in use by transactions")
	}

	if err := s.db(ctx).Delete(&category).Error; err != nil {
		return err
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
