// Package money 负责金额在“元”和“分”之间转换，避免业务层直接处理浮点精度问题。
package money

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ToCents 用于已经确认合法的金额转换，导入去重等非强校验路径也会复用它。
func ToCents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

// ToCentsExact 只接受最多两位小数的非负金额，避免 10.001 这类金额被静默四舍五入。
func ToCentsExact(amount float64) (int64, error) {
	if !math.IsInf(amount, 0) && !math.IsNaN(amount) && amount >= 0 {
		cents := ToCents(amount)
		if math.Abs(amount*100-float64(cents)) < 1e-9 {
			return cents, nil
		}
	}
	return 0, fmt.Errorf("amount must be a non-negative number with at most two decimal places")
}

// FromCents 把数据库里的分转换成前端/API 需要展示的元。
func FromCents(cents int64) float64 {
	return float64(cents) / 100
}

// FormatCents 用固定两位小数输出金额，主要用于导出和文本展示。
func FormatCents(cents int64) string {
	return strconv.FormatFloat(FromCents(cents), 'f', 2, 64)
}

// ParseCents 解析导入文件里的金额文本，并复用精确金额校验。
func ParseCents(value string) (int64, error) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return 0, fmt.Errorf("empty amount")
	}
	amount, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, err
	}
	return ToCentsExact(amount)
}
