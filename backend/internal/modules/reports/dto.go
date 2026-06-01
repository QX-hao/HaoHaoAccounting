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

type SummaryFilter struct {
	Start      time.Time
	End        time.Time
	CategoryID uint
	AccountID  uint
}
