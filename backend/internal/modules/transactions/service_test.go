package transactions

import (
	"context"
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/money"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"gorm.io/gorm"
)

type transactionContextKey struct{}

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
	ctx := context.Background()
	tx, err := service.Create(ctx, user.ID, Request{
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

	if _, err := service.Update(ctx, user.ID, tx.ID, Request{
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

	if err := service.Delete(ctx, user.ID, tx.ID); err != nil {
		t.Fatalf("delete tx: %v", err)
	}
	assertAccountBalance(t, s.DB, account.ID, 10000)
}

func TestServiceRejectsAmountsWithMoreThanTwoFractionDigits(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	account := models.Account{UserID: user.ID, Name: "现金", Type: "cash"}
	category := models.Category{Name: "餐饮", Type: "expense", IsSystem: true}
	if err := s.DB.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Create(&category).Error; err != nil {
		t.Fatal(err)
	}

	_, err := NewService(s, nil).Create(context.Background(), user.ID, Request{
		Type:       "expense",
		Amount:     1.234,
		CategoryID: category.ID,
		AccountID:  account.ID,
		Note:       "午饭",
		OccurredAt: time.Now(),
	})
	if err == nil {
		t.Fatal("expected invalid amount precision error")
	}
	if err.Error() != "amount must be a non-negative number with at most two decimal places" {
		t.Fatalf("err = %q", err.Error())
	}
}

func TestServicePassesContextToGORMQueries(t *testing.T) {
	s := testutil.NewStore(t)
	service := NewService(s, nil)
	ctx := context.WithValue(context.Background(), transactionContextKey{}, "request-context")

	callbackName := "transactions:test_context"
	var got any
	if err := s.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(db *gorm.DB) {
		got = db.Statement.Context.Value(transactionContextKey{})
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = s.DB.Callback().Query().Remove(callbackName)
	})

	if _, _, err := service.List(ctx, 1, ListFilter{}); err != nil {
		t.Fatal(err)
	}
	if got != "request-context" {
		t.Fatalf("gorm context value = %#v", got)
	}
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
