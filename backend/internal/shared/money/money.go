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
	return ToCents(amount), nil
}
