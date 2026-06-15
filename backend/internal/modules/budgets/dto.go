package budgets

type budgetRequest struct {
	Month      string  `json:"month"`
	CategoryID uint    `json:"categoryId"`
	Amount     float64 `json:"amount"`
}

type listQuery struct {
	Month string `form:"month" binding:"omitempty,datetime=2006-01"`
}
