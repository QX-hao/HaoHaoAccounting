package reports

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/cache"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/money"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	store *store.Store
	cache *cache.RedisCache
}

func NewService(s *store.Store, redisCache *cache.RedisCache) *Service {
	return &Service{store: s, cache: redisCache}
}

func (s *Service) Summary(userID uint, filter SummaryFilter) (gin.H, error) {
	cacheKey := cache.UserReportKey(userID, filter.Start, filter.End) + filter.CacheSuffix()
	if s.cache != nil && s.cache.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var cached map[string]any
		ok, err := s.cache.GetJSON(ctx, cacheKey, &cached)
		if err == nil && ok {
			return cached, nil
		}
	}

	incomeCents, expenseCents, err := s.sumIncomeExpense(userID, filter)
	if err != nil {
		return nil, err
	}
	categoryList, err := s.categoryBreakdown(userID, filter)
	if err != nil {
		return nil, err
	}
	accountList, err := s.accountBreakdown(userID, filter)
	if err != nil {
		return nil, err
	}
	monthlyTrend, err := s.monthlyTrend(userID, filter)
	if err != nil {
		return nil, err
	}

	summary := gin.H{"start": filter.Start, "end": filter.End}

	prevStart := filter.Start.Add(-filter.End.Sub(filter.Start) - time.Second)
	prevEnd := filter.Start.Add(-time.Second)
	prevIncomeCents, prevExpenseCents, err := s.sumIncomeExpense(userID, SummaryFilter{
		Start:      prevStart,
		End:        prevEnd,
		CategoryID: filter.CategoryID,
		AccountID:  filter.AccountID,
	})
	if err != nil {
		return nil, err
	}

	summary["income"] = money.FromCents(incomeCents)
	summary["expense"] = money.FromCents(expenseCents)
	summary["balance"] = money.FromCents(incomeCents - expenseCents)
	summary["byCategory"] = categoryList
	summary["byAccount"] = accountList
	summary["monthlyTrend"] = monthlyTrend
	summary["periodCompare"] = gin.H{
		"current":  gin.H{"income": money.FromCents(incomeCents), "expense": money.FromCents(expenseCents)},
		"previous": gin.H{"income": money.FromCents(prevIncomeCents), "expense": money.FromCents(prevExpenseCents)},
	}

	if s.cache != nil && s.cache.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.cache.SetJSON(ctx, cacheKey, summary, 2*time.Minute)
	}

	return summary, nil
}

func (s *Service) reportQuery(userID uint, filter SummaryFilter) *gorm.DB {
	query := s.store.DB.Model(&models.Transaction{}).
		Where("transactions.user_id = ? AND transactions.occurred_at >= ? AND transactions.occurred_at <= ?", userID, filter.Start, filter.End)
	if filter.CategoryID > 0 {
		query = query.Where("transactions.category_id = ?", filter.CategoryID)
	}
	if filter.AccountID > 0 {
		query = query.Where("transactions.account_id = ?", filter.AccountID)
	}
	return query
}

func (s *Service) sumIncomeExpense(userID uint, filter SummaryFilter) (int64, int64, error) {
	var rows []struct {
		Type        string
		AmountCents int64
	}

	if err := s.reportQuery(userID, filter).
		Select("type, COALESCE(SUM(amount_cents), 0) AS amount_cents").
		Group("type").
		Scan(&rows).Error; err != nil {
		return 0, 0, err
	}

	income, expense := int64(0), int64(0)
	for _, row := range rows {
		if row.Type == "income" {
			income += row.AmountCents
		} else {
			expense += row.AmountCents
		}
	}
	return income, expense, nil
}

func (s *Service) categoryBreakdown(userID uint, filter SummaryFilter) ([]CategoryStat, error) {
	var rows []struct {
		CategoryID  uint
		Category    string
		AmountCents int64
	}
	if err := s.reportQuery(userID, filter).
		Joins("JOIN categories ON categories.id = transactions.category_id").
		Where("transactions.type = ?", "expense").
		Select("transactions.category_id AS category_id, categories.name AS category, COALESCE(SUM(transactions.amount_cents), 0) AS amount_cents").
		Group("transactions.category_id, categories.name").
		Order("amount_cents DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]CategoryStat, 0, len(rows))
	for _, row := range rows {
		result = append(result, CategoryStat{
			CategoryID:  row.CategoryID,
			Category:    row.Category,
			Amount:      money.FromCents(row.AmountCents),
			amountCents: row.AmountCents,
		})
	}
	return result, nil
}

func (s *Service) accountBreakdown(userID uint, filter SummaryFilter) ([]AccountStat, error) {
	var rows []struct {
		AccountID   uint
		Account     string
		AmountCents int64
	}
	if err := s.reportQuery(userID, filter).
		Joins("JOIN accounts ON accounts.id = transactions.account_id").
		Where("transactions.type = ?", "expense").
		Select("transactions.account_id AS account_id, accounts.name AS account, COALESCE(SUM(transactions.amount_cents), 0) AS amount_cents").
		Group("transactions.account_id, accounts.name").
		Order("amount_cents DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]AccountStat, 0, len(rows))
	for _, row := range rows {
		result = append(result, AccountStat{
			AccountID:   row.AccountID,
			Account:     row.Account,
			Amount:      money.FromCents(row.AmountCents),
			amountCents: row.AmountCents,
		})
	}
	return result, nil
}

func (s *Service) monthlyTrend(userID uint, filter SummaryFilter) ([]gin.H, error) {
	monthExpr := s.monthExpression()
	var rows []struct {
		Month       string
		Type        string
		AmountCents int64
	}
	if err := s.reportQuery(userID, filter).
		Select(monthExpr + " AS month, type, COALESCE(SUM(amount_cents), 0) AS amount_cents").
		Group(monthExpr).
		Group("type").
		Order("month ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	monthly := map[string]gin.H{}
	for _, row := range rows {
		if _, ok := monthly[row.Month]; !ok {
			monthly[row.Month] = gin.H{"month": row.Month, "incomeCents": int64(0), "expenseCents": int64(0)}
		}
		if row.Type == "income" {
			monthly[row.Month]["incomeCents"] = row.AmountCents
		} else {
			monthly[row.Month]["expenseCents"] = row.AmountCents
		}
	}

	monthKeys := make([]string, 0, len(monthly))
	for key := range monthly {
		monthKeys = append(monthKeys, key)
	}
	sort.Strings(monthKeys)

	result := make([]gin.H, 0, len(monthKeys))
	for _, key := range monthKeys {
		item := monthly[key]
		result = append(result, gin.H{
			"month":   item["month"],
			"income":  money.FromCents(item["incomeCents"].(int64)),
			"expense": money.FromCents(item["expenseCents"].(int64)),
		})
	}
	return result, nil
}

func (s *Service) monthExpression() string {
	switch s.store.DB.Dialector.Name() {
	case "postgres":
		return "to_char(occurred_at, 'YYYY-MM')"
	case "mysql":
		return "DATE_FORMAT(occurred_at, '%Y-%m')"
	default:
		return "strftime('%Y-%m', occurred_at)"
	}
}

func (f SummaryFilter) CacheSuffix() string {
	if f.CategoryID == 0 && f.AccountID == 0 {
		return ""
	}
	return fmt.Sprintf(":category:%d:account:%d", f.CategoryID, f.AccountID)
}
