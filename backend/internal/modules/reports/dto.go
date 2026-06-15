package reports

import "time"

type CategoryStat struct {
	CategoryID  uint    `json:"categoryId"`
	Category    string  `json:"category"`
	Amount      float64 `json:"amount"`
	amountCents int64
}

type AccountStat struct {
	AccountID   uint    `json:"accountId"`
	Account     string  `json:"account"`
	Amount      float64 `json:"amount"`
	amountCents int64
}

type MonthTrend struct {
	Month   string  `json:"month"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
}

type TrendPoint struct {
	Period  string  `json:"period"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
}

type CategoryTrendPoint struct {
	Period     string  `json:"period"`
	CategoryID uint    `json:"categoryId"`
	Category   string  `json:"category"`
	Amount     float64 `json:"amount"`
}

type AccountBalancePoint struct {
	Period    string  `json:"period"`
	AccountID uint    `json:"accountId"`
	Account   string  `json:"account"`
	Net       float64 `json:"net"`
	Balance   float64 `json:"balance"`
}

type BudgetExecution struct {
	BudgetID   uint    `json:"budgetId"`
	Month      string  `json:"month"`
	CategoryID uint    `json:"categoryId"`
	Category   string  `json:"category"`
	Budget     float64 `json:"budget"`
	Expense    float64 `json:"expense"`
	Remaining  float64 `json:"remaining"`
	UsageRate  float64 `json:"usageRate"`
}

type SummaryTableRow struct {
	Period  string  `json:"period"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Balance float64 `json:"balance"`
	TxCount int     `json:"txCount"`
}

type SummaryFilter struct {
	Start      time.Time
	End        time.Time
	CategoryID uint
	AccountID  uint
	Trend      string
}

type summaryQuery struct {
	Start      string `form:"start"`
	End        string `form:"end"`
	CategoryID *uint  `form:"categoryId" binding:"omitempty,min=1"`
	AccountID  *uint  `form:"accountId" binding:"omitempty,min=1"`
	Trend      string `form:"trend" binding:"omitempty,oneof=day week month"`
}
