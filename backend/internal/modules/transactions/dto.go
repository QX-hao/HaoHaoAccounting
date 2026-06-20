package transactions

import "time"

type Request struct {
	Type       string    `json:"type"`
	Amount     float64   `json:"amount"`
	CategoryID uint      `json:"categoryId"`
	AccountID  uint      `json:"accountId"`
	Note       string    `json:"note"`
	Tags       []string  `json:"tags"`
	OccurredAt time.Time `json:"occurredAt"`
	Source     string    `json:"source"`

	// Import files historically allowed blank notes. Keep that behavior while
	// still requiring notes for manual API-created transactions.
	AllowEmptyNote bool `json:"-"`
}

type transactionRequestBody struct {
	Type       string    `json:"type" binding:"required,oneof=income expense"`
	Amount     float64   `json:"amount" binding:"required,gt=0"`
	CategoryID uint      `json:"categoryId" binding:"required,min=1"`
	AccountID  uint      `json:"accountId" binding:"required,min=1"`
	Note       string    `json:"note" binding:"required,min=1"`
	Tags       []string  `json:"tags"`
	OccurredAt time.Time `json:"occurredAt"`
	Source     string    `json:"source"`
}

func (body transactionRequestBody) serviceRequest() Request {
	return Request{
		Type:       body.Type,
		Amount:     body.Amount,
		CategoryID: body.CategoryID,
		AccountID:  body.AccountID,
		Note:       body.Note,
		Tags:       body.Tags,
		OccurredAt: body.OccurredAt,
		Source:     body.Source,
	}
}

type ListFilter struct {
	Page       int
	PageSize   int
	Start      time.Time
	End        time.Time
	Type       string
	CategoryID uint
	AccountID  uint
	Keyword    string
}

type listQuery struct {
	Page       *int   `form:"page" binding:"omitempty,min=1"`
	PageSize   *int   `form:"pageSize" binding:"omitempty,min=1,max=200"`
	Start      string `form:"start"`
	End        string `form:"end"`
	Type       string `form:"type" binding:"omitempty,oneof=income expense"`
	CategoryID *uint  `form:"categoryId" binding:"omitempty,min=1"`
	AccountID  *uint  `form:"accountId" binding:"omitempty,min=1"`
	Keyword    string `form:"q" binding:"omitempty,max=100"`
}
