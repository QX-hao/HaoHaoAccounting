package store

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sealos/haohaoaccounting/backend/internal/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Store struct {
	DB *gorm.DB
}

type Config struct {
	Driver string
	DSN    string
}

func New(cfg Config) (*Store, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	dsn := strings.TrimSpace(cfg.DSN)
	if driver == "" || dsn == "" {
		return nil, errors.New("database config is incomplete")
	}

	var (
		db  *gorm.DB
		err error
	)

	switch driver {
	case "postgres", "pgsql":
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	case "mysql":
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER: %s", cfg.Driver)
	}
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Account{},
		&models.Category{},
		&models.Transaction{},
	); err != nil {
		return nil, err
	}

	s := &Store{DB: db}
	if err := s.seedSystemCategories(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) seedSystemCategories() error {
	categories := []models.Category{
		{Name: "餐饮", Type: "expense", IsSystem: true},
		{Name: "交通", Type: "expense", IsSystem: true},
		{Name: "住房", Type: "expense", IsSystem: true},
		{Name: "娱乐", Type: "expense", IsSystem: true},
		{Name: "学习", Type: "expense", IsSystem: true},
		{Name: "工资", Type: "income", IsSystem: true},
		{Name: "兼职", Type: "income", IsSystem: true},
	}

	for _, c := range categories {
		var count int64
		if err := s.DB.Model(&models.Category{}).
			Where("name = ? AND type = ? AND is_system = ?", c.Name, c.Type, true).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			if err := s.DB.Create(&c).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Store) EnsureDefaultDataForUser(userID uint) error {
	defaultAccounts := []models.Account{
		{UserID: userID, Name: "现金", Type: "cash", Balance: 0},
		{UserID: userID, Name: "银行卡", Type: "bank", Balance: 0},
		{UserID: userID, Name: "支付宝", Type: "alipay", Balance: 0},
		{UserID: userID, Name: "微信", Type: "wechat", Balance: 0},
	}

	for _, a := range defaultAccounts {
		var count int64
		if err := s.DB.Model(&models.Account{}).
			Where("user_id = ? AND name = ?", userID, a.Name).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			if err := s.DB.Create(&a).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Store) FindCategoryByName(userID uint, txType, name string) (*models.Category, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, errors.New("empty category name")
	}

	var category models.Category
	err := s.DB.Where("name = ? AND type = ? AND user_id = ?", cleanName, txType, userID).First(&category).Error
	if err == nil {
		return &category, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	err = s.DB.Where("name = ? AND type = ? AND is_system = ?", cleanName, txType, true).First(&category).Error
	if err == nil {
		return &category, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return nil, fmt.Errorf("category %s not found", cleanName)
}

func (s *Store) FindOrCreateCategory(userID uint, txType, name string) (*models.Category, error) {
	if category, err := s.FindCategoryByName(userID, txType, name); err == nil {
		return category, nil
	}

	category := models.Category{
		UserID:   &userID,
		Name:     strings.TrimSpace(name),
		Type:     txType,
		IsSystem: false,
	}
	if err := s.DB.Create(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func (s *Store) FindOrCreateAccount(userID uint, name string) (*models.Account, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		cleanName = "现金"
	}

	var account models.Account
	err := s.DB.Where("user_id = ? AND name = ?", userID, cleanName).First(&account).Error
	if err == nil {
		return &account, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	account = models.Account{UserID: userID, Name: cleanName, Type: "custom", Balance: 0}
	if err := s.DB.Create(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}
