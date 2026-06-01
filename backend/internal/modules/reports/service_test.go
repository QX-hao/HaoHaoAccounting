package reports

import (
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/money"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
)

func TestSummaryAggregatesMoneyFromCents(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	account := models.Account{UserID: user.ID, Name: "现金", Type: "cash"}
	category := models.Category{Name: "餐饮", Type: "expense", IsSystem: true}
	incomeCategory := models.Category{Name: "工资", Type: "income", IsSystem: true}
	if err := s.DB.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Create(&incomeCategory).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	rows := []models.Transaction{
		{UserID: user.ID, Type: "income", AmountCents: money.ToCents(1000), CategoryID: incomeCategory.ID, AccountID: account.ID, Note: "工资", OccurredAt: now},
		{UserID: user.ID, Type: "expense", AmountCents: money.ToCents(12.34), CategoryID: category.ID, AccountID: account.ID, Note: "午饭", OccurredAt: now},
		{UserID: user.ID, Type: "expense", AmountCents: money.ToCents(0.66), CategoryID: category.ID, AccountID: account.ID, Note: "袋子", OccurredAt: now},
	}
	if err := s.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	summary, err := NewService(s, nil).Summary(user.ID, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if got := summary["income"].(float64); got != 1000 {
		t.Fatalf("income = %v", got)
	}
	if got := summary["expense"].(float64); got != 13 {
		t.Fatalf("expense = %v", got)
	}
	if got := summary["balance"].(float64); got != 987 {
		t.Fatalf("balance = %v", got)
	}

	categoryStats := summary["byCategory"].([]CategoryStat)
	if len(categoryStats) != 1 || categoryStats[0].Amount != 13 {
		t.Fatalf("category stats = %#v", categoryStats)
	}
}
