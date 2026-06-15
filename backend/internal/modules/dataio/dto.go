package dataio

import "time"

const (
	MaxImportFileBytes = 5 * 1024 * 1024
	MaxImportRows      = 5000
	ImportPreviewRows  = 20
)

type importRecord struct {
	Line       int
	OccurredAt time.Time
	Type       string
	Amount     float64
	Category   string
	Account    string
	Note       string
	Tags       string
}

type ImportResult struct {
	Total   int      `json:"total"`
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors"`
}

type ImportJobResponse struct {
	ID        uint      `json:"id"`
	Filename  string    `json:"filename"`
	Status    string    `json:"status"`
	Total     int       `json:"total"`
	Success   int       `json:"success"`
	Failed    int       `json:"failed"`
	Skipped   int       `json:"skipped"`
	Errors    []string  `json:"errors"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ImportTextRequest struct {
	Filename       string `json:"filename"`
	Content        string `json:"content"`
	SkipDuplicates *bool  `json:"skipDuplicates"`
}

type ImportPreview struct {
	Filename      string             `json:"filename"`
	Size          int64              `json:"size"`
	TotalRows     int                `json:"totalRows"`
	ValidRows     int                `json:"validRows"`
	FailedRows    int                `json:"failedRows"`
	DuplicateRows int                `json:"duplicateRows"`
	MaxRows       int                `json:"maxRows"`
	MaxFileBytes  int64              `json:"maxFileBytes"`
	Truncated     bool               `json:"truncated"`
	Rows          []ImportPreviewRow `json:"rows"`
}

type ImportPreviewRow struct {
	Line            int     `json:"line"`
	OccurredAt      string  `json:"occurredAt"`
	Type            string  `json:"type"`
	Amount          float64 `json:"amount"`
	Category        string  `json:"category"`
	Account         string  `json:"account"`
	Note            string  `json:"note"`
	Tags            string  `json:"tags"`
	Valid           bool    `json:"valid"`
	Duplicate       bool    `json:"duplicate"`
	Error           string  `json:"error,omitempty"`
	DuplicateReason string  `json:"duplicateReason,omitempty"`
}

type exportQuery struct {
	Format string `form:"format" binding:"omitempty,oneof=csv xlsx"`
	Start  string `form:"start"`
	End    string `form:"end"`
}
