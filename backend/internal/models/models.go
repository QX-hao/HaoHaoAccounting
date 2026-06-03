package models

import (
	"encoding/json"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/money"
)

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex" json:"username"`
	PasswordHash string    `gorm:"size:255" json:"-"`
	Phone        string    `gorm:"size:32;index" json:"phone"`
	Email        string    `gorm:"size:128;index" json:"email"`
	WechatID     string    `gorm:"size:128;index" json:"wechatId"`
	Name         string    `gorm:"size:64" json:"name"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Account struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"userId"`
	Name         string    `gorm:"size:64;not null" json:"name"`
	Type         string    `gorm:"size:32;not null" json:"type"`
	BalanceCents int64     `gorm:"not null;default:0" json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Category struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    *uint      `gorm:"index" json:"userId,omitempty"`
	Name      string     `gorm:"size:64;not null" json:"name"`
	Type      string     `gorm:"size:16;not null" json:"type"`
	IsSystem  bool       `gorm:"not null;default:false" json:"isSystem"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `gorm:"index" json:"-"`
}

type Transaction struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index;index:idx_transactions_user_occurred,priority:1;index:idx_transactions_user_type_occurred,priority:1;index:idx_transactions_user_category_occurred,priority:1;index:idx_transactions_user_account_occurred,priority:1;not null" json:"userId"`
	Type        string    `gorm:"size:16;index:idx_transactions_user_type_occurred,priority:2;not null" json:"type"`
	AmountCents int64     `gorm:"not null;default:0" json:"-"`
	CategoryID  uint      `gorm:"index;index:idx_transactions_user_category_occurred,priority:2;not null" json:"categoryId"`
	AccountID   uint      `gorm:"index;index:idx_transactions_user_account_occurred,priority:2;not null" json:"accountId"`
	Note        string    `gorm:"size:255" json:"note"`
	Tags        string    `gorm:"size:255" json:"tags"`
	Source      string    `gorm:"size:32;not null;default:manual" json:"source"`
	OccurredAt  time.Time `gorm:"index;index:idx_transactions_user_occurred,priority:2;index:idx_transactions_user_type_occurred,priority:3;index:idx_transactions_user_category_occurred,priority:3;index:idx_transactions_user_account_occurred,priority:3;not null" json:"occurredAt"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Category    Category  `gorm:"foreignKey:CategoryID" json:"category"`
	Account     Account   `gorm:"foreignKey:AccountID" json:"account"`
}

type ImportJob struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"userId"`
	Filename  string    `gorm:"size:255;not null" json:"filename"`
	Status    string    `gorm:"size:24;index;not null" json:"status"`
	Total     int       `gorm:"not null;default:0" json:"total"`
	Success   int       `gorm:"not null;default:0" json:"success"`
	Failed    int       `gorm:"not null;default:0" json:"failed"`
	Skipped   int       `gorm:"not null;default:0" json:"skipped"`
	Errors    string    `gorm:"type:text" json:"errors"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Budget struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"uniqueIndex:idx_budgets_user_month_category,priority:1;index;not null" json:"userId"`
	Month       string    `gorm:"size:7;uniqueIndex:idx_budgets_user_month_category,priority:2;index;not null" json:"month"`
	CategoryID  uint      `gorm:"uniqueIndex:idx_budgets_user_month_category,priority:3;index;not null;default:0" json:"categoryId"`
	AmountCents int64     `gorm:"not null;default:0" json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type DailySummary struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"uniqueIndex:idx_daily_summaries_user_day,priority:1;index;not null" json:"userId"`
	Day          string    `gorm:"size:10;uniqueIndex:idx_daily_summaries_user_day,priority:2;index;not null" json:"day"`
	IncomeCents  int64     `gorm:"not null;default:0" json:"-"`
	ExpenseCents int64     `gorm:"not null;default:0" json:"-"`
	TxCount      int       `gorm:"not null;default:0" json:"txCount"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type MonthlySummary struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"uniqueIndex:idx_monthly_summaries_user_month,priority:1;index;not null" json:"userId"`
	Month        string    `gorm:"size:7;uniqueIndex:idx_monthly_summaries_user_month,priority:2;index;not null" json:"month"`
	IncomeCents  int64     `gorm:"not null;default:0" json:"-"`
	ExpenseCents int64     `gorm:"not null;default:0" json:"-"`
	TxCount      int       `gorm:"not null;default:0" json:"txCount"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (a Account) MarshalJSON() ([]byte, error) {
	type alias Account
	return json.Marshal(struct {
		alias
		Balance float64 `json:"balance"`
	}{
		alias:   alias(a),
		Balance: money.FromCents(a.BalanceCents),
	})
}

func (t Transaction) MarshalJSON() ([]byte, error) {
	type alias Transaction
	return json.Marshal(struct {
		alias
		Amount float64 `json:"amount"`
	}{
		alias:  alias(t),
		Amount: money.FromCents(t.AmountCents),
	})
}

func (b Budget) MarshalJSON() ([]byte, error) {
	type alias Budget
	return json.Marshal(struct {
		alias
		Amount float64 `json:"amount"`
	}{
		alias:  alias(b),
		Amount: money.FromCents(b.AmountCents),
	})
}
