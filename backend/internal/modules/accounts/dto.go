package accounts

type accountRequest struct {
	Name    string  `json:"name" binding:"required,min=1"`
	Type    string  `json:"type" binding:"required,min=1"`
	Balance float64 `json:"balance" binding:"omitempty,min=0"`
}
