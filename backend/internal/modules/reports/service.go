package reports

import (
	"context"
	"fmt"
	"sort"
	"strings"
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

func (s *Service) Summary(ctx context.Context, userID uint, filter SummaryFilter) (gin.H, error) {
	filter.Trend = normalizeTrend(filter.Trend)
	cacheKey := cache.UserReportKey(userID, filter.Start, filter.End) + filter.CacheSuffix()
	if s.cache != nil && s.cache.Enabled() {
		cacheCtx, cancel := context.WithTimeout(requestContext(ctx), time.Second)
		defer cancel()
		var cached map[string]any
		ok, err := s.cache.GetJSON(cacheCtx, cacheKey, &cached)
		if err == nil && ok {
			return cached, nil
		}
	}

	incomeCents, expenseCents, err := s.sumIncomeExpense(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	categoryList, err := s.categoryBreakdown(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	accountList, err := s.accountBreakdown(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	monthlyTrend, err := s.monthlyTrend(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	trend, err := s.trend(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	categoryTrend, err := s.categoryTrend(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	accountBalanceTrend, err := s.accountBalanceTrend(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	budgetExecution, err := s.budgetExecution(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	dailySummaries, monthlySummaries, err := s.refreshSummaryTables(ctx, userID, filter)
	if err != nil {
		return nil, err
	}

	summary := gin.H{"start": filter.Start, "end": filter.End}

	prevStart := filter.Start.Add(-filter.End.Sub(filter.Start) - time.Second)
	prevEnd := filter.Start.Add(-time.Second)
	prevIncomeCents, prevExpenseCents, err := s.sumIncomeExpense(ctx, userID, SummaryFilter{
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
	summary["trendGranularity"] = filter.Trend
	summary["trend"] = trend
	summary["categoryTrend"] = categoryTrend
	summary["accountBalanceTrend"] = accountBalanceTrend
	summary["budgetExecution"] = budgetExecution
	summary["dailySummaries"] = dailySummaries
	summary["monthlySummaries"] = monthlySummaries
	summary["periodCompare"] = gin.H{
		"current":  gin.H{"income": money.FromCents(incomeCents), "expense": money.FromCents(expenseCents)},
		"previous": gin.H{"income": money.FromCents(prevIncomeCents), "expense": money.FromCents(prevExpenseCents)},
	}

	if s.cache != nil && s.cache.Enabled() {
		cacheCtx, cancel := context.WithTimeout(cacheWriteContext(ctx), time.Second)
		defer cancel()
		_ = s.cache.SetJSON(cacheCtx, cacheKey, summary, 2*time.Minute)
	}

	return summary, nil
}

func (s *Service) reportQuery(ctx context.Context, userID uint, filter SummaryFilter) *gorm.DB {
	query := s.db(ctx).Model(&models.Transaction{}).
		Where("transactions.user_id = ? AND transactions.occurred_at >= ? AND transactions.occurred_at <= ?", userID, filter.Start, filter.End)
	if filter.CategoryID > 0 {
		query = query.Where("transactions.category_id = ?", filter.CategoryID)
	}
	if filter.AccountID > 0 {
		query = query.Where("transactions.account_id = ?", filter.AccountID)
	}
	return query
}

func (s *Service) sumIncomeExpense(ctx context.Context, userID uint, filter SummaryFilter) (int64, int64, error) {
	var rows []struct {
		Type        string
		AmountCents int64
	}

	if err := s.reportQuery(ctx, userID, filter).
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

func (s *Service) categoryBreakdown(ctx context.Context, userID uint, filter SummaryFilter) ([]CategoryStat, error) {
	var rows []struct {
		CategoryID  uint
		Category    string
		AmountCents int64
	}
	if err := s.reportQuery(ctx, userID, filter).
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

func (s *Service) accountBreakdown(ctx context.Context, userID uint, filter SummaryFilter) ([]AccountStat, error) {
	var rows []struct {
		AccountID   uint
		Account     string
		AmountCents int64
	}
	if err := s.reportQuery(ctx, userID, filter).
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

func (s *Service) monthlyTrend(ctx context.Context, userID uint, filter SummaryFilter) ([]MonthTrend, error) {
	monthExpr := s.monthExpression()
	var rows []struct {
		Month       string
		Type        string
		AmountCents int64
	}
	if err := s.reportQuery(ctx, userID, filter).
		Select(monthExpr + " AS month, type, COALESCE(SUM(amount_cents), 0) AS amount_cents").
		Group(monthExpr).
		Group("type").
		Order("month ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	monthly := map[string]map[string]int64{}
	for _, row := range rows {
		if _, ok := monthly[row.Month]; !ok {
			monthly[row.Month] = map[string]int64{"income": 0, "expense": 0}
		}
		if row.Type == "income" {
			monthly[row.Month]["income"] = row.AmountCents
		} else {
			monthly[row.Month]["expense"] = row.AmountCents
		}
	}

	monthKeys := make([]string, 0, len(monthly))
	for key := range monthly {
		monthKeys = append(monthKeys, key)
	}
	sort.Strings(monthKeys)

	result := make([]MonthTrend, 0, len(monthKeys))
	for _, key := range monthKeys {
		item := monthly[key]
		result = append(result, MonthTrend{
			Month:   key,
			Income:  money.FromCents(item["income"]),
			Expense: money.FromCents(item["expense"]),
		})
	}
	return result, nil
}

func (s *Service) trend(ctx context.Context, userID uint, filter SummaryFilter) ([]TrendPoint, error) {
	periodExpr := s.periodExpression(filter.Trend)
	var rows []struct {
		Period      string
		Type        string
		AmountCents int64
	}
	if err := s.reportQuery(ctx, userID, filter).
		Select(periodExpr + " AS period, type, COALESCE(SUM(amount_cents), 0) AS amount_cents").
		Group(periodExpr).
		Group("type").
		Order("period ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	grouped := map[string]map[string]int64{}
	for _, row := range rows {
		if _, ok := grouped[row.Period]; !ok {
			grouped[row.Period] = map[string]int64{"income": 0, "expense": 0}
		}
		if row.Type == "income" {
			grouped[row.Period]["income"] = row.AmountCents
		} else {
			grouped[row.Period]["expense"] = row.AmountCents
		}
	}

	periods := make([]string, 0, len(grouped))
	for period := range grouped {
		periods = append(periods, period)
	}
	sort.Strings(periods)

	result := make([]TrendPoint, 0, len(periods))
	for _, period := range periods {
		item := grouped[period]
		result = append(result, TrendPoint{
			Period:  period,
			Income:  money.FromCents(item["income"]),
			Expense: money.FromCents(item["expense"]),
		})
	}
	return result, nil
}

func (s *Service) categoryTrend(ctx context.Context, userID uint, filter SummaryFilter) ([]CategoryTrendPoint, error) {
	periodExpr := s.periodExpression(filter.Trend)
	var rows []struct {
		Period      string
		CategoryID  uint
		Category    string
		AmountCents int64
	}
	if err := s.reportQuery(ctx, userID, filter).
		Joins("JOIN categories ON categories.id = transactions.category_id").
		Where("transactions.type = ?", "expense").
		Select(periodExpr + " AS period, transactions.category_id AS category_id, categories.name AS category, COALESCE(SUM(transactions.amount_cents), 0) AS amount_cents").
		Group(periodExpr).
		Group("transactions.category_id").
		Group("categories.name").
		Order("period ASC, amount_cents DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]CategoryTrendPoint, 0, len(rows))
	for _, row := range rows {
		result = append(result, CategoryTrendPoint{
			Period:     row.Period,
			CategoryID: row.CategoryID,
			Category:   row.Category,
			Amount:     money.FromCents(row.AmountCents),
		})
	}
	return result, nil
}

func (s *Service) accountBalanceTrend(ctx context.Context, userID uint, filter SummaryFilter) ([]AccountBalancePoint, error) {
	periodExpr := s.periodExpression(filter.Trend)
	var rows []struct {
		Period      string
		AccountID   uint
		Account     string
		Type        string
		AmountCents int64
	}
	if err := s.reportQuery(ctx, userID, filter).
		Joins("JOIN accounts ON accounts.id = transactions.account_id").
		Select(periodExpr + " AS period, transactions.account_id AS account_id, accounts.name AS account, transactions.type AS type, COALESCE(SUM(transactions.amount_cents), 0) AS amount_cents").
		Group(periodExpr).
		Group("transactions.account_id").
		Group("accounts.name").
		Group("transactions.type").
		Order("period ASC, transactions.account_id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	type accountPeriod struct {
		period    string
		accountID uint
		account   string
	}
	keys := make([]accountPeriod, 0)
	netByKey := map[accountPeriod]int64{}
	for _, row := range rows {
		key := accountPeriod{period: row.Period, accountID: row.AccountID, account: row.Account}
		if _, ok := netByKey[key]; !ok {
			keys = append(keys, key)
		}
		delta := row.AmountCents
		if row.Type == "expense" {
			delta = -row.AmountCents
		}
		netByKey[key] += delta
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].accountID == keys[j].accountID {
			return keys[i].period < keys[j].period
		}
		return keys[i].accountID < keys[j].accountID
	})

	running := map[uint]int64{}
	result := make([]AccountBalancePoint, 0, len(keys))
	for _, key := range keys {
		net := netByKey[key]
		running[key.accountID] += net
		result = append(result, AccountBalancePoint{
			Period:    key.period,
			AccountID: key.accountID,
			Account:   key.account,
			Net:       money.FromCents(net),
			Balance:   money.FromCents(running[key.accountID]),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Period == result[j].Period {
			return result[i].AccountID < result[j].AccountID
		}
		return result[i].Period < result[j].Period
	})
	return result, nil
}

func (s *Service) budgetExecution(ctx context.Context, userID uint, filter SummaryFilter) ([]BudgetExecution, error) {
	months := monthsInRange(filter.Start, filter.End)
	if len(months) == 0 {
		return []BudgetExecution{}, nil
	}

	var budgets []models.Budget
	if err := s.db(ctx).Where("user_id = ? AND month IN ?", userID, months).
		Order("month asc, category_id asc").
		Find(&budgets).Error; err != nil {
		return nil, err
	}
	if len(budgets) == 0 {
		return []BudgetExecution{}, nil
	}

	categoryNames, err := s.categoryNames(ctx, userID)
	if err != nil {
		return nil, err
	}
	expenses, err := s.expenseByMonthCategory(ctx, userID, filter, months)
	if err != nil {
		return nil, err
	}

	result := make([]BudgetExecution, 0, len(budgets))
	for _, budget := range budgets {
		expenseCents := int64(0)
		if budget.CategoryID == 0 {
			expenseCents = expenses[budget.Month][0]
		} else {
			expenseCents = expenses[budget.Month][budget.CategoryID]
		}
		remaining := budget.AmountCents - expenseCents
		usageRate := float64(0)
		if budget.AmountCents > 0 {
			usageRate = float64(expenseCents) / float64(budget.AmountCents)
		}
		result = append(result, BudgetExecution{
			BudgetID:   budget.ID,
			Month:      budget.Month,
			CategoryID: budget.CategoryID,
			Category:   budgetCategoryName(budget.CategoryID, categoryNames),
			Budget:     money.FromCents(budget.AmountCents),
			Expense:    money.FromCents(expenseCents),
			Remaining:  money.FromCents(remaining),
			UsageRate:  usageRate,
		})
	}
	return result, nil
}

func (s *Service) refreshSummaryTables(ctx context.Context, userID uint, filter SummaryFilter) ([]SummaryTableRow, []SummaryTableRow, error) {
	daily, err := s.summaryTableRows(ctx, userID, filter, "day")
	if err != nil {
		return nil, nil, err
	}
	monthly, err := s.summaryTableRows(ctx, userID, filter, "month")
	if err != nil {
		return nil, nil, err
	}

	snapshotFilter := SummaryFilter{Start: filter.Start, End: filter.End, Trend: filter.Trend}
	snapshotDaily, err := s.summaryTableRows(ctx, userID, snapshotFilter, "day")
	if err != nil {
		return nil, nil, err
	}
	snapshotMonthly, err := s.summaryTableRows(ctx, userID, snapshotFilter, "month")
	if err != nil {
		return nil, nil, err
	}
	for _, row := range snapshotDaily {
		if err := s.upsertDailySummary(ctx, userID, row); err != nil {
			return nil, nil, err
		}
	}
	for _, row := range snapshotMonthly {
		if err := s.upsertMonthlySummary(ctx, userID, row); err != nil {
			return nil, nil, err
		}
	}
	return daily, monthly, nil
}

func (s *Service) summaryTableRows(ctx context.Context, userID uint, filter SummaryFilter, granularity string) ([]SummaryTableRow, error) {
	periodExpr := s.periodExpression(granularity)
	var rows []struct {
		Period      string
		Type        string
		AmountCents int64
		TxCount     int
	}
	if err := s.reportQuery(ctx, userID, filter).
		Select(periodExpr + " AS period, type, COALESCE(SUM(amount_cents), 0) AS amount_cents, COUNT(*) AS tx_count").
		Group(periodExpr).
		Group("type").
		Order("period ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	grouped := map[string]struct {
		incomeCents  int64
		expenseCents int64
		txCount      int
	}{}
	for _, row := range rows {
		item := grouped[row.Period]
		item.txCount += row.TxCount
		if row.Type == "income" {
			item.incomeCents += row.AmountCents
		} else {
			item.expenseCents += row.AmountCents
		}
		grouped[row.Period] = item
	}

	periods := make([]string, 0, len(grouped))
	for period := range grouped {
		periods = append(periods, period)
	}
	sort.Strings(periods)

	result := make([]SummaryTableRow, 0, len(periods))
	for _, period := range periods {
		item := grouped[period]
		result = append(result, SummaryTableRow{
			Period:  period,
			Income:  money.FromCents(item.incomeCents),
			Expense: money.FromCents(item.expenseCents),
			Balance: money.FromCents(item.incomeCents - item.expenseCents),
			TxCount: item.txCount,
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

func (s *Service) periodExpression(granularity string) string {
	switch granularity {
	case "day":
		return s.dayExpression()
	case "week":
		return s.weekExpression()
	default:
		return s.monthExpression()
	}
}

func (s *Service) dayExpression() string {
	switch s.store.DB.Dialector.Name() {
	case "postgres":
		return "to_char(occurred_at, 'YYYY-MM-DD')"
	case "mysql":
		return "DATE_FORMAT(occurred_at, '%Y-%m-%d')"
	default:
		return "strftime('%Y-%m-%d', occurred_at)"
	}
}

func (s *Service) weekExpression() string {
	switch s.store.DB.Dialector.Name() {
	case "postgres":
		return "to_char(occurred_at, 'IYYY-\"W\"IW')"
	case "mysql":
		return "DATE_FORMAT(occurred_at, '%x-W%v')"
	default:
		return "strftime('%Y-W%W', occurred_at)"
	}
}

func (s *Service) db(ctx context.Context) *gorm.DB {
	return s.store.DB.WithContext(requestContext(ctx))
}

func requestContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func cacheWriteContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func (s *Service) upsertDailySummary(ctx context.Context, userID uint, row SummaryTableRow) error {
	summary := models.DailySummary{
		UserID:       userID,
		Day:          row.Period,
		IncomeCents:  money.ToCents(row.Income),
		ExpenseCents: money.ToCents(row.Expense),
		TxCount:      row.TxCount,
	}
	return s.db(ctx).Where("user_id = ? AND day = ?", userID, row.Period).
		Assign(summary).
		FirstOrCreate(&summary).Error
}

func (s *Service) upsertMonthlySummary(ctx context.Context, userID uint, row SummaryTableRow) error {
	summary := models.MonthlySummary{
		UserID:       userID,
		Month:        row.Period,
		IncomeCents:  money.ToCents(row.Income),
		ExpenseCents: money.ToCents(row.Expense),
		TxCount:      row.TxCount,
	}
	return s.db(ctx).Where("user_id = ? AND month = ?", userID, row.Period).
		Assign(summary).
		FirstOrCreate(&summary).Error
}

func (s *Service) categoryNames(ctx context.Context, userID uint) (map[uint]string, error) {
	var categories []models.Category
	if err := s.db(ctx).Where("is_system = ? OR user_id = ?", true, userID).Find(&categories).Error; err != nil {
		return nil, err
	}
	result := make(map[uint]string, len(categories))
	for _, category := range categories {
		result[category.ID] = category.Name
	}
	return result, nil
}

func (s *Service) expenseByMonthCategory(ctx context.Context, userID uint, filter SummaryFilter, months []string) (map[string]map[uint]int64, error) {
	monthExpr := s.monthExpression()
	var rows []struct {
		Month       string
		CategoryID  uint
		AmountCents int64
	}
	query := s.reportQuery(ctx, userID, filter).
		Where("transactions.type = ?", "expense").
		Where(monthExpr+" IN ?", months)
	if err := query.Select(monthExpr + " AS month, transactions.category_id AS category_id, COALESCE(SUM(transactions.amount_cents), 0) AS amount_cents").
		Group(monthExpr).
		Group("transactions.category_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[string]map[uint]int64, len(months))
	for _, month := range months {
		result[month] = map[uint]int64{0: 0}
	}
	for _, row := range rows {
		if _, ok := result[row.Month]; !ok {
			result[row.Month] = map[uint]int64{0: 0}
		}
		result[row.Month][row.CategoryID] += row.AmountCents
		result[row.Month][0] += row.AmountCents
	}
	return result, nil
}

func budgetCategoryName(categoryID uint, categoryNames map[uint]string) string {
	if categoryID == 0 {
		return "全部支出"
	}
	if name := categoryNames[categoryID]; name != "" {
		return name
	}
	return "未知分类"
}

func monthsInRange(start, end time.Time) []string {
	if end.Before(start) {
		return nil
	}
	cursor := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
	last := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, end.Location())
	months := make([]string, 0)
	for !cursor.After(last) {
		months = append(months, cursor.Format("2006-01"))
		cursor = cursor.AddDate(0, 1, 0)
	}
	return months
}

func normalizeTrend(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "day", "daily":
		return "day"
	case "week", "weekly":
		return "week"
	default:
		return "month"
	}
}

func (f SummaryFilter) CacheSuffix() string {
	trend := normalizeTrend(f.Trend)
	if f.CategoryID == 0 && f.AccountID == 0 && trend == "month" {
		return ""
	}
	return fmt.Sprintf(":category:%d:account:%d:trend:%s", f.CategoryID, f.AccountID, trend)
}
