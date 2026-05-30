package categories

import (
	"errors"
	"strings"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/transactions"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
)

type Service struct {
	store       *store.Store
	invalidator transactions.CacheInvalidator
}

func NewService(s *store.Store, invalidator transactions.CacheInvalidator) *Service {
	return &Service{store: s, invalidator: invalidator}
}

func (s *Service) List(userID uint, txType string) ([]models.Category, error) {
	query := s.store.DB.Model(&models.Category{}).
		Where("is_system = ? OR user_id = ?", true, userID)
	if strings.TrimSpace(txType) != "" {
		query = query.Where("type = ?", strings.TrimSpace(txType))
	}

	var categories []models.Category
	err := query.Order("is_system desc, id asc").Find(&categories).Error
	return categories, err
}

func (s *Service) Create(userID uint, req categoryRequest) (models.Category, error) {
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if req.Type != "income" && req.Type != "expense" {
		return models.Category{}, errors.New("type must be income or expense")
	}
	if strings.TrimSpace(req.Name) == "" {
		return models.Category{}, errors.New("name is required")
	}

	category := models.Category{UserID: &userID, Name: strings.TrimSpace(req.Name), Type: req.Type, IsSystem: false}
	if err := s.store.DB.Create(&category).Error; err != nil {
		return models.Category{}, err
	}
	s.invalidator.InvalidateUser(userID)
	return category, nil
}

func (s *Service) Update(userID, id uint, req categoryRequest) (models.Category, error) {
	var category models.Category
	if err := s.store.DB.Where("id = ?", id).First(&category).Error; err != nil {
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

	if err := s.store.DB.Save(&category).Error; err != nil {
		return models.Category{}, err
	}
	s.invalidator.InvalidateUser(userID)
	return category, nil
}

func (s *Service) Delete(userID, id uint) error {
	var category models.Category
	if err := s.store.DB.First(&category, id).Error; err != nil {
		return errors.New("category not found")
	}
	if category.IsSystem || category.UserID == nil || *category.UserID != userID {
		return errors.New("system category cannot be deleted")
	}

	var count int64
	if err := s.store.DB.Model(&models.Transaction{}).Where("user_id = ? AND category_id = ?", userID, id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("category in use by transactions")
	}

	if err := s.store.DB.Delete(&category).Error; err != nil {
		return err
	}
	s.invalidator.InvalidateUser(userID)
	return nil
}
