package budgets

type budgetRequest struct {
	Month      string   `json:"month" binding:"required,datetime=2006-01"`
	CategoryID uint     `json:"categoryId" binding:"required,min=1"`
	Amount     *float64 `json:"amount" binding:"required,min=0"`
}

type listQuery struct {
	Month string `form:"month" binding:"omitempty,datetime=2006-01"`
}
