package testutil

import (
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func NewStore(t *testing.T) *store.Store {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Account{}, &models.Category{}, &models.Transaction{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	return &store.Store{DB: db}
}
