package money

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func ToCents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

func ToCentsExact(amount float64) (int64, error) {
	if !math.IsInf(amount, 0) && !math.IsNaN(amount) && amount >= 0 {
		cents := ToCents(amount)
		if math.Abs(amount*100-float64(cents)) < 1e-9 {
			return cents, nil
		}
	}
	return 0, fmt.Errorf("amount must be a non-negative number with at most two decimal places")
}

func FromCents(cents int64) float64 {
	return float64(cents) / 100
}

func FormatCents(cents int64) string {
	return strconv.FormatFloat(FromCents(cents), 'f', 2, 64)
}

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
