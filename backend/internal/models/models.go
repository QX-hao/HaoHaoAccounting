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
	UserID      uint      `gorm:"index;not null" json:"userId"`
	Type        string    `gorm:"size:16;not null" json:"type"`
	AmountCents int64     `gorm:"not null;default:0" json:"-"`
	CategoryID  uint      `gorm:"index;not null" json:"categoryId"`
	AccountID   uint      `gorm:"index;not null" json:"accountId"`
	Note        string    `gorm:"size:255" json:"note"`
	Tags        string    `gorm:"size:255" json:"tags"`
	Source      string    `gorm:"size:32;not null;default:manual" json:"source"`
	OccurredAt  time.Time `gorm:"index;not null" json:"occurredAt"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Category    Category  `gorm:"foreignKey:CategoryID" json:"category"`
	Account     Account   `gorm:"foreignKey:AccountID" json:"account"`
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
