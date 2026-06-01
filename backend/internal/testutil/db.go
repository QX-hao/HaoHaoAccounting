package testutil

import (
	"fmt"
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func NewStore(t *testing.T) *store.Store {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Account{}, &models.Category{}, &models.Transaction{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	return &store.Store{DB: db}
}
