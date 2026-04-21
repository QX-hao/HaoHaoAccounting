package models

import "time"

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Phone     string    `gorm:"size:32;index" json:"phone"`
	Email     string    `gorm:"size:128;index" json:"email"`
	WechatID  string    `gorm:"size:128;index" json:"wechatId"`
	Name      string    `gorm:"size:64" json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Account struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"userId"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	Type      string    `gorm:"size:32;not null" json:"type"`
	Balance   float64   `gorm:"not null;default:0" json:"balance"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"userId"`
	Type       string    `gorm:"size:16;not null" json:"type"`
	Amount     float64   `gorm:"not null" json:"amount"`
	CategoryID uint      `gorm:"index;not null" json:"categoryId"`
	AccountID  uint      `gorm:"index;not null" json:"accountId"`
	Note       string    `gorm:"size:255" json:"note"`
	Tags       string    `gorm:"size:255" json:"tags"`
	Source     string    `gorm:"size:32;not null;default:manual" json:"source"`
	OccurredAt time.Time `gorm:"index;not null" json:"occurredAt"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Category   Category  `gorm:"foreignKey:CategoryID" json:"category"`
	Account    Account   `gorm:"foreignKey:AccountID" json:"account"`
}
