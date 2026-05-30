package dataio

import "time"

type importRecord struct {
	OccurredAt time.Time
	Type       string
	Amount     float64
	Category   string
	Account    string
	Note       string
	Tags       string
}

type ImportResult struct {
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors"`
}
