package services

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

type AIParseResult struct {
	Type       string    `json:"type"`
	Amount     float64   `json:"amount"`
	Category   string    `json:"category"`
	Account    string    `json:"account"`
	Note       string    `json:"note"`
	OccurredAt time.Time `json:"occurredAt"`
	Confidence float64   `json:"confidence"`
}

var amountPattern = regexp.MustCompile(`([0-9]+(?:\.[0-9]{1,2})?)`)

func ParseNaturalLedgerText(text string) AIParseResult {
	now := time.Now()
	clean := strings.TrimSpace(text)
	result := AIParseResult{
		Type:       "expense",
		Amount:     0,
		Category:   "餐饮",
		Account:    "现金",
		Note:       clean,
		OccurredAt: now,
		Confidence: 0.65,
	}

	if clean == "" {
		return result
	}

	if strings.Contains(clean, "收入") || strings.Contains(clean, "工资") || strings.Contains(clean, "奖金") || strings.Contains(clean, "报销") {
		result.Type = "income"
		result.Category = "工资"
	}

	if strings.Contains(clean, "昨天") {
		result.OccurredAt = now.AddDate(0, 0, -1)
		result.Confidence += 0.05
	}
	if strings.Contains(clean, "今天") {
		result.OccurredAt = now
		result.Confidence += 0.05
	}

	switch {
	case strings.Contains(clean, "午饭"), strings.Contains(clean, "早餐"), strings.Contains(clean, "晚饭"), strings.Contains(clean, "奶茶"), strings.Contains(clean, "餐"):
		result.Category = "餐饮"
		result.Confidence += 0.1
	case strings.Contains(clean, "地铁"), strings.Contains(clean, "打车"), strings.Contains(clean, "公交"), strings.Contains(clean, "高铁"):
		result.Category = "交通"
		result.Confidence += 0.1
	case strings.Contains(clean, "房租"), strings.Contains(clean, "水电"), strings.Contains(clean, "物业"):
		result.Category = "住房"
		result.Confidence += 0.1
	case strings.Contains(clean, "电影"), strings.Contains(clean, "游戏"), strings.Contains(clean, "娱乐"):
		result.Category = "娱乐"
		result.Confidence += 0.1
	case strings.Contains(clean, "兼职"), strings.Contains(clean, "副业"):
		result.Category = "兼职"
		result.Type = "income"
		result.Confidence += 0.1
	}

	switch {
	case strings.Contains(clean, "微信"):
		result.Account = "微信"
	case strings.Contains(clean, "支付宝"):
		result.Account = "支付宝"
	case strings.Contains(clean, "银行卡"), strings.Contains(clean, "卡"):
		result.Account = "银行卡"
	case strings.Contains(clean, "现金"):
		result.Account = "现金"
	}

	if match := amountPattern.FindStringSubmatch(clean); len(match) > 1 {
		if amount, err := strconv.ParseFloat(match[1], 64); err == nil {
			result.Amount = amount
			result.Confidence += 0.15
		}
	}

	if result.Confidence > 0.95 {
		result.Confidence = 0.95
	}

	return result
}
