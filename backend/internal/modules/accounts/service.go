package accounts

import (
	"errors"
	"strings"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/transactions"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/stringutil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
)

type Service struct {
	store       *store.Store
	invalidator transactions.CacheInvalidator
}

func NewService(s *store.Store, invalidator transactions.CacheInvalidator) *Service {
	return &Service{store: s, invalidator: invalidator}
}

func (s *Service) List(userID uint) ([]models.Account, error) {
	var accounts []models.Account
	err := s.store.DB.Where("user_id = ?", userID).Order("id asc").Find(&accounts).Error
	return accounts, err
}

func (s *Service) Create(userID uint, req accountRequest) (models.Account, error) {
	account := models.Account{
		UserID:  userID,
		Name:    strings.TrimSpace(req.Name),
		Type:    stringutil.FallbackName(req.Type, "custom"),
		Balance: req.Balance,
	}
	if account.Name == "" {
		return models.Account{}, errors.New("name is required")
	}
	if err := s.store.DB.Create(&account).Error; err != nil {
		return models.Account{}, err
	}
	s.invalidator.InvalidateUser(userID)
	return account, nil
}

func (s *Service) Update(userID, id uint, req accountRequest) (models.Account, error) {
	var account models.Account
	if err := s.store.DB.Where("id = ? AND user_id = ?", id, userID).First(&account).Error; err != nil {
		return models.Account{}, errors.New("account not found")
	}

	if strings.TrimSpace(req.Name) != "" {
		account.Name = strings.TrimSpace(req.Name)
	}
	if strings.TrimSpace(req.Type) != "" {
		account.Type = strings.TrimSpace(req.Type)
	}
	account.Balance = req.Balance

	if err := s.store.DB.Save(&account).Error; err != nil {
		return models.Account{}, err
	}
	s.invalidator.InvalidateUser(userID)
	return account, nil
}

func (s *Service) Delete(userID, id uint) error {
	var count int64
	if err := s.store.DB.Model(&models.Transaction{}).Where("user_id = ? AND account_id = ?", userID, id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("account in use by transactions")
	}

	if err := s.store.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Account{}).Error; err != nil {
		return err
	}
	s.invalidator.InvalidateUser(userID)
	return nil
}
