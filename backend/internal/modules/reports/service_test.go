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

	summary, err := NewService(s, nil).Summary(user.ID, SummaryFilter{Start: now.Add(-time.Hour), End: now.Add(time.Hour)})
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

func TestSummaryFiltersByCategoryAndAccount(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	cash := models.Account{UserID: user.ID, Name: "现金", Type: "cash"}
	card := models.Account{UserID: user.ID, Name: "银行卡", Type: "bank"}
	food := models.Category{Name: "餐饮", Type: "expense", IsSystem: true}
	traffic := models.Category{Name: "交通", Type: "expense", IsSystem: true}
	if err := s.DB.Create(&cash).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Create(&card).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Create(&food).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Create(&traffic).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	rows := []models.Transaction{
		{UserID: user.ID, Type: "expense", AmountCents: money.ToCents(10), CategoryID: food.ID, AccountID: cash.ID, Note: "饭", OccurredAt: now},
		{UserID: user.ID, Type: "expense", AmountCents: money.ToCents(20), CategoryID: traffic.ID, AccountID: cash.ID, Note: "车", OccurredAt: now},
		{UserID: user.ID, Type: "expense", AmountCents: money.ToCents(30), CategoryID: food.ID, AccountID: card.ID, Note: "饭", OccurredAt: now},
	}
	if err := s.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	summary, err := NewService(s, nil).Summary(user.ID, SummaryFilter{
		Start:      now.Add(-time.Hour),
		End:        now.Add(time.Hour),
		CategoryID: food.ID,
		AccountID:  cash.ID,
	})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if got := summary["expense"].(float64); got != 10 {
		t.Fatalf("filtered expense = %v", got)
	}
}

func TestSummaryIncludesTrendsBudgetsAndSummaryTables(t *testing.T) {
	s := testutil.NewStore(t)
	user := models.User{Username: "alice", PasswordHash: "hash"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	account := models.Account{UserID: user.ID, Name: "现金", Type: "cash"}
	if err := s.DB.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	food := models.Category{Name: "餐饮", Type: "expense", IsSystem: true}
	salary := models.Category{Name: "工资", Type: "income", IsSystem: true}
	if err := s.DB.Create(&food).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Create(&salary).Error; err != nil {
		t.Fatal(err)
	}

	rows := []models.Transaction{
		{UserID: user.ID, Type: "income", AmountCents: money.ToCents(3000), CategoryID: salary.ID, AccountID: account.ID, Note: "工资", OccurredAt: time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)},
		{UserID: user.ID, Type: "expense", AmountCents: money.ToCents(100), CategoryID: food.ID, AccountID: account.ID, Note: "早餐", OccurredAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)},
		{UserID: user.ID, Type: "expense", AmountCents: money.ToCents(150), CategoryID: food.ID, AccountID: account.ID, Note: "晚餐", OccurredAt: time.Date(2026, 6, 2, 18, 0, 0, 0, time.UTC)},
	}
	if err := s.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Create(&models.Budget{UserID: user.ID, Month: "2026-06", CategoryID: food.ID, AmountCents: money.ToCents(500)}).Error; err != nil {
		t.Fatal(err)
	}

	summary, err := NewService(s, nil).Summary(user.ID, SummaryFilter{
		Start: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 6, 30, 23, 59, 59, 0, time.UTC),
		Trend: "day",
	})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if got := summary["trendGranularity"].(string); got != "day" {
		t.Fatalf("trend granularity = %s", got)
	}
	trend := summary["trend"].([]TrendPoint)
	if len(trend) != 2 || trend[0].Period != "2026-06-01" || trend[0].Income != 3000 || trend[0].Expense != 100 {
		t.Fatalf("trend = %#v", trend)
	}
	categoryTrend := summary["categoryTrend"].([]CategoryTrendPoint)
	if len(categoryTrend) != 2 || categoryTrend[0].Category != "餐饮" || categoryTrend[0].Amount != 100 {
		t.Fatalf("category trend = %#v", categoryTrend)
	}
	accountTrend := summary["accountBalanceTrend"].([]AccountBalancePoint)
	if len(accountTrend) != 2 || accountTrend[0].Balance != 2900 || accountTrend[1].Balance != 2750 {
		t.Fatalf("account balance trend = %#v", accountTrend)
	}
	budgets := summary["budgetExecution"].([]BudgetExecution)
	if len(budgets) != 1 || budgets[0].Expense != 250 || budgets[0].Remaining != 250 || budgets[0].UsageRate != 0.5 {
		t.Fatalf("budget execution = %#v", budgets)
	}
	daily := summary["dailySummaries"].([]SummaryTableRow)
	if len(daily) != 2 || daily[0].TxCount != 2 || daily[1].Balance != -150 {
		t.Fatalf("daily summaries = %#v", daily)
	}
	monthly := summary["monthlySummaries"].([]SummaryTableRow)
	if len(monthly) != 1 || monthly[0].Income != 3000 || monthly[0].Expense != 250 {
		t.Fatalf("monthly summaries = %#v", monthly)
	}

	var dailyRows int64
	if err := s.DB.Model(&models.DailySummary{}).Where("user_id = ?", user.ID).Count(&dailyRows).Error; err != nil {
		t.Fatal(err)
	}
	if dailyRows != 2 {
		t.Fatalf("daily summary table rows = %d", dailyRows)
	}
	var monthlyRows int64
	if err := s.DB.Model(&models.MonthlySummary{}).Where("user_id = ?", user.ID).Count(&monthlyRows).Error; err != nil {
		t.Fatal(err)
	}
	if monthlyRows != 1 {
		t.Fatalf("monthly summary table rows = %d", monthlyRows)
	}
}
