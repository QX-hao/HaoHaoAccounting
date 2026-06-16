package budgets

import (
	"context"
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"gorm.io/gorm"
)

type budgetContextKey struct{}

func TestBudgetCreateUpdateDelete(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	category := models.Category{Name: "餐饮", Type: "expense", IsSystem: true}
	if err := s.DB.Create(&category).Error; err != nil {
		t.Fatal(err)
	}

	service := NewService(s, nil)
	ctx := context.Background()
	budget, err := service.Create(ctx, user.ID, budgetRequest{Month: "2026-06", CategoryID: category.ID, Amount: 1200})
	if err != nil {
		t.Fatalf("create budget: %v", err)
	}
	if budget.AmountCents != 120000 {
		t.Fatalf("amount cents = %d", budget.AmountCents)
	}

	updated, err := service.Update(ctx, user.ID, budget.ID, budgetRequest{Month: "2026-06", CategoryID: 0, Amount: 2000})
	if err != nil {
		t.Fatalf("update budget: %v", err)
	}
	if updated.CategoryID != 0 || updated.AmountCents != 200000 {
		t.Fatalf("updated budget = %#v", updated)
	}

	list, err := service.List(ctx, user.ID, "2026-06")
	if err != nil {
		t.Fatalf("list budgets: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("budget list length = %d", len(list))
	}

	if err := service.Delete(ctx, user.ID, budget.ID); err != nil {
		t.Fatalf("delete budget: %v", err)
	}
	list, err = service.List(ctx, user.ID, "2026-06")
	if err != nil {
		t.Fatalf("list budgets after delete: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("budget list after delete = %#v", list)
	}
}

func TestBudgetRejectsIncomeCategory(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	category := models.Category{Name: "工资", Type: "income", IsSystem: true}
	if err := s.DB.Create(&category).Error; err != nil {
		t.Fatal(err)
	}

	_, err := NewService(s, nil).Create(context.Background(), user.ID, budgetRequest{Month: "2026-06", CategoryID: category.ID, Amount: 1000})
	if err == nil || err.Error() != "budget category must be expense" {
		t.Fatalf("err = %v", err)
	}
}

func TestBudgetRejectsAmountsWithMoreThanTwoFractionDigits(t *testing.T) {
	s := testutil.NewStore(t)

	_, err := NewService(s, nil).Create(context.Background(), 1, budgetRequest{Month: "2026-06", Amount: 1.234})
	if err == nil {
		t.Fatal("expected invalid amount precision error")
	}
	if err.Error() != "amount must be a non-negative number with at most two decimal places" {
		t.Fatalf("err = %q", err.Error())
	}
}

func TestBudgetServicePassesContextToGORMQueries(t *testing.T) {
	s := testutil.NewStore(t)
	service := NewService(s, nil)
	ctx := context.WithValue(context.Background(), budgetContextKey{}, "request-context")

	callbackName := "budgets:test_context"
	var got any
	if err := s.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(db *gorm.DB) {
		got = db.Statement.Context.Value(budgetContextKey{})
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = s.DB.Callback().Query().Remove(callbackName)
	})

	if _, err := service.List(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	if got != "request-context" {
		t.Fatalf("gorm context value = %#v", got)
	}
}
