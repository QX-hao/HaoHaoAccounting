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

	var rows []models.Transaction
	query := s.store.DB.Where("user_id = ? AND occurred_at >= ? AND occurred_at <= ?", userID, filter.Start, filter.End)
	if filter.CategoryID > 0 {
		query = query.Where("category_id = ?", filter.CategoryID)
	}
	if filter.AccountID > 0 {
		query = query.Where("account_id = ?", filter.AccountID)
	}
	if err := query.Preload("Category").Preload("Account").Order("occurred_at asc").Find(&rows).Error; err != nil {
		return nil, err
	}

	summary := gin.H{"start": filter.Start, "end": filter.End}
	incomeCents := int64(0)
	expenseCents := int64(0)
	categoryMap := map[uint]*CategoryStat{}
	accountMap := map[uint]*AccountStat{}
	monthly := map[string]gin.H{}

	for _, row := range rows {
		if row.Type == "income" {
			incomeCents += row.AmountCents
		} else {
			expenseCents += row.AmountCents
		}

		if row.Type == "expense" {
			categoryStat, ok := categoryMap[row.CategoryID]
			if !ok {
				categoryStat = &CategoryStat{CategoryID: row.CategoryID, Category: row.Category.Name}
				categoryMap[row.CategoryID] = categoryStat
			}
			categoryStat.amountCents += row.AmountCents
			categoryStat.Amount = money.FromCents(categoryStat.amountCents)

			accountStat, ok := accountMap[row.AccountID]
			if !ok {
				accountStat = &AccountStat{AccountID: row.AccountID, Account: row.Account.Name}
				accountMap[row.AccountID] = accountStat
			}
			accountStat.amountCents += row.AmountCents
			accountStat.Amount = money.FromCents(accountStat.amountCents)
		}

		monthKey := row.OccurredAt.Format("2006-01")
		if _, ok := monthly[monthKey]; !ok {
			monthly[monthKey] = gin.H{"month": monthKey, "incomeCents": int64(0), "expenseCents": int64(0)}
		}
		if row.Type == "income" {
			monthly[monthKey]["incomeCents"] = monthly[monthKey]["incomeCents"].(int64) + row.AmountCents
		} else {
			monthly[monthKey]["expenseCents"] = monthly[monthKey]["expenseCents"].(int64) + row.AmountCents
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
		item := monthly[k]
		monthlyTrend = append(monthlyTrend, gin.H{
			"month":   item["month"],
			"income":  money.FromCents(item["incomeCents"].(int64)),
			"expense": money.FromCents(item["expenseCents"].(int64)),
		})
	}

	prevStart := filter.Start.Add(-filter.End.Sub(filter.Start) - time.Second)
	prevEnd := filter.Start.Add(-time.Second)
	prevIncomeCents, prevExpenseCents := s.sumIncomeExpense(userID, SummaryFilter{
		Start:      prevStart,
		End:        prevEnd,
		CategoryID: filter.CategoryID,
		AccountID:  filter.AccountID,
	})

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

func (s *Service) sumIncomeExpense(userID uint, filter SummaryFilter) (int64, int64) {
	var rows []models.Transaction
	query := s.store.DB.Where("user_id = ? AND occurred_at >= ? AND occurred_at <= ?", userID, filter.Start, filter.End)
	if filter.CategoryID > 0 {
		query = query.Where("category_id = ?", filter.CategoryID)
	}
	if filter.AccountID > 0 {
		query = query.Where("account_id = ?", filter.AccountID)
	}
	if err := query.Select("type, amount_cents").Find(&rows).Error; err != nil {
		return 0, 0
	}
	income, expense := int64(0), int64(0)
	for _, row := range rows {
		if row.Type == "income" {
			income += row.AmountCents
		} else {
			expense += row.AmountCents
		}
	}
	return income, expense
}

func (f SummaryFilter) CacheSuffix() string {
	if f.CategoryID == 0 && f.AccountID == 0 {
		return ""
	}
	return fmt.Sprintf(":category:%d:account:%d", f.CategoryID, f.AccountID)
}
