package reports

type CategoryStat struct {
	CategoryID uint    `json:"categoryId"`
	Category   string  `json:"category"`
	Amount     float64 `json:"amount"`
}

type AccountStat struct {
	AccountID uint    `json:"accountId"`
	Account   string  `json:"account"`
	Amount    float64 `json:"amount"`
}
