// Package queryutil 放通用查询参数解析，避免 handler 里反复写字符串转换和默认值逻辑。
package queryutil

import (
	"strconv"
	"strings"
)

// ParseInt 解析失败时返回调用方给出的默认值，常用于 page/pageSize 这类宽松参数。
func ParseInt(value string, fallback int) int {
	i, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return i
}

// ParseUint 解析失败时返回 0，0 在筛选条件里通常表示“不限制”。
func ParseUint(value string) uint {
	u, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return uint(u)
}

// ParsePositiveUint 用 bool 区分“没有有效 ID”和“解析出了正整数 ID”。
func ParsePositiveUint(value string) (uint, bool) {
	u, err := strconv.ParseUint(strings.TrimSpace(value), 10, strconv.IntSize)
	if err != nil || u == 0 {
		return 0, false
	}
	return uint(u), true
}
