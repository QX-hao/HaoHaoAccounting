package transactions

import (
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/money"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"gorm.io/gorm"
)

func TestServiceMaintainsAccountBalanceAcrossCreateUpdateDelete(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	account := models.Account{UserID: user.ID, Name: "现金", Type: "cash", BalanceCents: money.ToCents(100)}
	category := models.Category{Name: "餐饮", Type: "expense", IsSystem: true}
	if err := s.DB.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Create(&category).Error; err != nil {
		t.Fatal(err)
	}

	service := NewService(s, nil)
	tx, err := service.Create(user.ID, Request{
		Type:       "expense",
		Amount:     12.34,
		CategoryID: category.ID,
		AccountID:  account.ID,
		Note:       "午饭",
		OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("create tx: %v", err)
	}
	assertAccountBalance(t, s.DB, account.ID, 8766)

	if _, err := service.Update(user.ID, tx.ID, Request{
		Type:       "expense",
		Amount:     2.34,
		CategoryID: category.ID,
		AccountID:  account.ID,
		Note:       "午饭少花",
		OccurredAt: tx.OccurredAt,
	}); err != nil {
		t.Fatalf("update tx: %v", err)
	}
	assertAccountBalance(t, s.DB, account.ID, 9766)

	if err := service.Delete(user.ID, tx.ID); err != nil {
		t.Fatalf("delete tx: %v", err)
	}
	assertAccountBalance(t, s.DB, account.ID, 10000)
}

func assertAccountBalance(t *testing.T, db *gorm.DB, accountID uint, want int64) {
	t.Helper()
	var account models.Account
	if err := db.First(&account, accountID).Error; err != nil {
		t.Fatalf("load account: %v", err)
	}
	if account.BalanceCents != want {
		t.Fatalf("balance cents = %d, want %d", account.BalanceCents, want)
	}
}
