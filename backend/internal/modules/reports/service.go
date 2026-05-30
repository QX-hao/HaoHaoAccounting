package reports

import (
	"context"
	"sort"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/cache"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"github.com/gin-gonic/gin"
)

type Service struct {
	store *store.Store
	cache *cache.RedisCache
}

func NewService(s *store.Store, redisCache *cache.RedisCache) *Service {
	return &Service{store: s, cache: redisCache}
}

func (s *Service) Summary(userID uint, start, end time.Time) (gin.H, error) {
	cacheKey := cache.UserReportKey(userID, start, end)
	if s.cache != nil && s.cache.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var cached map[string]any
		ok, err := s.cache.GetJSON(ctx, cacheKey, &cached)
		if err == nil && ok {
			return cached, nil
		}
	}

	var rows []models.Transaction
	if err := s.store.DB.Where("user_id = ? AND occurred_at >= ? AND occurred_at <= ?", userID, start, end).
		Preload("Category").Preload("Account").
		Order("occurred_at asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	summary := gin.H{"start": start, "end": end}
	income := 0.0
	expense := 0.0
	categoryMap := map[uint]*CategoryStat{}
	accountMap := map[uint]*AccountStat{}
	monthly := map[string]gin.H{}

	for _, row := range rows {
		if row.Type == "income" {
			income += row.Amount
		} else {
			expense += row.Amount
		}

		if row.Type == "expense" {
			categoryStat, ok := categoryMap[row.CategoryID]
			if !ok {
				categoryStat = &CategoryStat{CategoryID: row.CategoryID, Category: row.Category.Name}
				categoryMap[row.CategoryID] = categoryStat
			}
			categoryStat.Amount += row.Amount

			accountStat, ok := accountMap[row.AccountID]
			if !ok {
				accountStat = &AccountStat{AccountID: row.AccountID, Account: row.Account.Name}
				accountMap[row.AccountID] = accountStat
			}
			accountStat.Amount += row.Amount
		}

		monthKey := row.OccurredAt.Format("2006-01")
		if _, ok := monthly[monthKey]; !ok {
			monthly[monthKey] = gin.H{"month": monthKey, "income": 0.0, "expense": 0.0}
		}
		if row.Type == "income" {
			monthly[monthKey]["income"] = monthly[monthKey]["income"].(float64) + row.Amount
		} else {
			monthly[monthKey]["expense"] = monthly[monthKey]["expense"].(float64) + row.Amount
		}
	}

	categoryList := make([]CategoryStat, 0, len(categoryMap))
	for _, v := range categoryMap {
		categoryList = append(categoryList, *v)
	}
	sort.Slice(categoryList, func(i, j int) bool { return categoryList[i].Amount > categoryList[j].Amount })

	accountList := make([]AccountStat, 0, len(accountMap))
	for _, v := range accountMap {
		accountList = append(accountList, *v)
	}
	sort.Slice(accountList, func(i, j int) bool { return accountList[i].Amount > accountList[j].Amount })

	monthKeys := make([]string, 0, len(monthly))
	for k := range monthly {
		monthKeys = append(monthKeys, k)
	}
	sort.Strings(monthKeys)
	monthlyTrend := make([]gin.H, 0, len(monthKeys))
	for _, k := range monthKeys {
		monthlyTrend = append(monthlyTrend, monthly[k])
	}

	prevStart := start.Add(-end.Sub(start) - time.Second)
	prevEnd := start.Add(-time.Second)
	prevIncome, prevExpense := s.sumIncomeExpense(userID, prevStart, prevEnd)

	summary["income"] = income
	summary["expense"] = expense
	summary["balance"] = income - expense
	summary["byCategory"] = categoryList
	summary["byAccount"] = accountList
	summary["monthlyTrend"] = monthlyTrend
	summary["periodCompare"] = gin.H{
		"current":  gin.H{"income": income, "expense": expense},
		"previous": gin.H{"income": prevIncome, "expense": prevExpense},
	}

	if s.cache != nil && s.cache.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.cache.SetJSON(ctx, cacheKey, summary, 2*time.Minute)
	}

	return summary, nil
}

func (s *Service) sumIncomeExpense(userID uint, start, end time.Time) (float64, float64) {
	var rows []models.Transaction
	if err := s.store.DB.Where("user_id = ? AND occurred_at >= ? AND occurred_at <= ?", userID, start, end).
		Select("type, amount").
		Find(&rows).Error; err != nil {
		return 0, 0
	}
	income, expense := 0.0, 0.0
	for _, row := range rows {
		if row.Type == "income" {
			income += row.Amount
		} else {
			expense += row.Amount
		}
	}
	return income, expense
}
