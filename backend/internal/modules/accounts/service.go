package accounts

import (
	"context"
	"errors"
	"strings"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/transactions"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/money"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/stringutil"
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

func (s *Service) List(ctx context.Context, userID uint) ([]models.Account, error) {
	var accounts []models.Account
	err := s.db(ctx).Where("user_id = ?", userID).Order("id asc").Find(&accounts).Error
	return accounts, err
}

func (s *Service) Create(ctx context.Context, userID uint, req accountRequest) (models.Account, error) {
	balanceCents, err := money.ToCentsExact(req.Balance)
	if err != nil {
		return models.Account{}, err
	}
	account := models.Account{
		UserID:       userID,
		Name:         strings.TrimSpace(req.Name),
		Type:         stringutil.FallbackName(req.Type, "custom"),
		BalanceCents: balanceCents,
	}
	if account.Name == "" {
		return models.Account{}, errors.New("name is required")
	}
	if err := s.db(ctx).Create(&account).Error; err != nil {
		return models.Account{}, err
	}
	s.invalidateUser(ctx, userID)
	return account, nil
}

func (s *Service) Update(ctx context.Context, userID, id uint, req accountRequest) (models.Account, error) {
	var account models.Account
	if err := s.db(ctx).Where("id = ? AND user_id = ?", id, userID).First(&account).Error; err != nil {
		return models.Account{}, errors.New("account not found")
	}

	if strings.TrimSpace(req.Name) != "" {
		account.Name = strings.TrimSpace(req.Name)
	}
	if strings.TrimSpace(req.Type) != "" {
		account.Type = strings.TrimSpace(req.Type)
	}
	balanceCents, err := money.ToCentsExact(req.Balance)
	if err != nil {
		return models.Account{}, err
	}
	account.BalanceCents = balanceCents

	if err := s.db(ctx).Save(&account).Error; err != nil {
		return models.Account{}, err
	}
	s.invalidateUser(ctx, userID)
	return account, nil
}

func (s *Service) Delete(ctx context.Context, userID, id uint) error {
	var count int64
	if err := s.db(ctx).Model(&models.Transaction{}).Where("user_id = ? AND account_id = ?", userID, id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("account in use by transactions")
	}

	if err := s.db(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&models.Account{}).Error; err != nil {
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
