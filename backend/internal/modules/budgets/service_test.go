package budgets

import (
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
)

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
	budget, err := service.Create(user.ID, budgetRequest{Month: "2026-06", CategoryID: category.ID, Amount: 1200})
	if err != nil {
		t.Fatalf("create budget: %v", err)
	}
	if budget.AmountCents != 120000 {
		t.Fatalf("amount cents = %d", budget.AmountCents)
	}

	updated, err := service.Update(user.ID, budget.ID, budgetRequest{Month: "2026-06", CategoryID: 0, Amount: 2000})
	if err != nil {
		t.Fatalf("update budget: %v", err)
	}
	if updated.CategoryID != 0 || updated.AmountCents != 200000 {
		t.Fatalf("updated budget = %#v", updated)
	}

	list, err := service.List(user.ID, "2026-06")
	if err != nil {
		t.Fatalf("list budgets: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("budget list length = %d", len(list))
	}

	if err := service.Delete(user.ID, budget.ID); err != nil {
		t.Fatalf("delete budget: %v", err)
	}
	list, err = service.List(user.ID, "2026-06")
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

	_, err := NewService(s, nil).Create(user.ID, budgetRequest{Month: "2026-06", CategoryID: category.ID, Amount: 1000})
	if err == nil || err.Error() != "budget category must be expense" {
		t.Fatalf("err = %v", err)
	}
}
