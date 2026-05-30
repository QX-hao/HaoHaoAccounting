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
