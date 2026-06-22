// Package timeutil 集中处理账单查询和导入里的日期解析，避免各模块各自定义时间规则。
package timeutil

import (
	"errors"
	"strings"
	"time"
)

// ParseDateTime 支持 API 查询和导入文件常见的日期格式，统一入口可以避免模块间格式漂移。
func ParseDateTime(raw string) (time.Time, error) {
	formats := []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05", "2006/01/02"}
	for _, format := range formats {
		if t, err := time.Parse(format, strings.TrimSpace(raw)); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("invalid datetime")
}

// ResolveRange 用“本月第一天到当前时间”作为默认范围，适合统计页的宽松查询。
func ResolveRange(startRaw, endRaw string) (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := now

	if t, err := ParseDateTime(startRaw); strings.TrimSpace(startRaw) != "" && err == nil {
		start = t
	}
	if t, err := ParseDateTime(endRaw); strings.TrimSpace(endRaw) != "" && err == nil {
		end = NormalizeRangeEnd(strings.TrimSpace(endRaw), t)
	}
	return start, end
}

// ResolveRangeStrict 用于需要向调用方明确返回日期错误的接口。
func ResolveRangeStrict(startRaw, endRaw string) (time.Time, time.Time, error) {
	start, end := ResolveRange(startRaw, endRaw)
	startRaw = strings.TrimSpace(startRaw)
	endRaw = strings.TrimSpace(endRaw)
	hasStart := startRaw != ""
	hasEnd := endRaw != ""
	if hasStart {
		parsed, err := ParseDateTime(startRaw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid start datetime")
		}
		start = parsed
	}
	if hasEnd {
		parsed, err := ParseDateTime(endRaw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid end datetime")
		}
		end = NormalizeRangeEnd(endRaw, parsed)
	}
	if hasStart && hasEnd && start.After(end) {
		return time.Time{}, time.Time{}, errors.New("start datetime must be before or equal to end datetime")
	}
	return start, end, nil
}

// ResolveOptionalRangeStrict 允许起止时间都为空，适合导出和筛选这种可选范围。
func ResolveOptionalRangeStrict(startRaw, endRaw string) (time.Time, time.Time, error) {
	var start time.Time
	var end time.Time
	startRaw = strings.TrimSpace(startRaw)
	endRaw = strings.TrimSpace(endRaw)
	hasStart := startRaw != ""
	hasEnd := endRaw != ""
	if hasStart {
		parsed, err := ParseDateTime(startRaw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid start datetime")
		}
		start = parsed
	}
	if hasEnd {
		parsed, err := ParseDateTime(endRaw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid end datetime")
		}
		end = NormalizeRangeEnd(endRaw, parsed)
	}
	if hasStart && hasEnd && start.After(end) {
		return time.Time{}, time.Time{}, errors.New("start datetime must be before or equal to end datetime")
	}
	return start, end, nil
}

// NormalizeRangeEnd 把纯日期结束值扩展到当天最后一纳秒，保证 end=2026-01-01 包含当天数据。
func NormalizeRangeEnd(raw string, value time.Time) time.Time {
	if isDateOnly(raw) {
		return value.AddDate(0, 0, 1).Add(-time.Nanosecond)
	}
	return value
}

func isDateOnly(raw string) bool {
	if _, err := time.Parse("2006-01-02", raw); err == nil {
		return true
	}
	if _, err := time.Parse("2006/01/02", raw); err == nil {
		return true
	}
	return false
}
