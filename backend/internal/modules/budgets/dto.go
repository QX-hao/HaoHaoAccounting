package budgets

type budgetRequest struct {
	Month      string  `json:"month"`
	CategoryID uint    `json:"categoryId"`
	Amount     float64 `json:"amount"`
}
