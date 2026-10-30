// Package timewindow 时间规则引擎：所有业务时间边界单点实现（对齐 doc/sheji.md §5.1）。
// 全部以 Asia/Shanghai 为基准。
package timewindow

import (
	"time"

	_ "time/tzdata" // 内嵌时区数据，容器内无 tzdata 也可用
)

var Loc = mustLoc("Asia/Shanghai")

func mustLoc(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic("load location " + name + ": " + err.Error())
	}
	return loc
}

// NowShanghai 当前 Asia/Shanghai 时间。
func NowShanghai() time.Time { return time.Now().In(Loc) }

// ParseDate 解析 YYYY-MM-DD 为上海时区当日 00:00。
func ParseDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, Loc)
}

// TodayDateStr 今日日期字符串 YYYY-MM-DD（上海时区）。
func TodayDateStr() string { return NowShanghai().Format("2006-01-02") }

// TodayMinusDays 今日减 n 天的日期字符串。
func TodayMinusDays(n int) string { return NowShanghai().AddDate(0, 0, -n).Format("2006-01-02") }

// TodayPlusDays 今日加 n 天的日期字符串。
func TodayPlusDays(n int) string { return NowShanghai().AddDate(0, 0, n).Format("2006-01-02") }

// —— 规则 1：可报名/修改/取消 —— now < menu_date 当日 00:00（Asia/Shanghai）
func CanSignupModifyCancel(now time.Time, menuDateStr string) bool {
	d, err := ParseDate(menuDateStr)
	if err != nil {
		return false
	}
	return now.Before(d)
}

// —— 规则 2：菜谱可发布/可见范围（未来） —— today ≤ menu_date ≤ today+14（含今日共 15 天）
func InFutureMenuRange(now time.Time, menuDateStr string) bool {
	d, err := ParseDate(menuDateStr)
	if err != nil {
		return false
	}
	today := toDate(now)
	last := today.AddDate(0, 0, 14)
	return !d.Before(today) && !d.After(last)
}

// —— 规则 3：历史菜谱可见范围 —— menu_date ≥ today-30
func InHistoryRange(now time.Time, menuDateStr string) bool {
	d, err := ParseDate(menuDateStr)
	if err != nil {
		return false
	}
	first := toDate(now).AddDate(0, 0, -30)
	return !d.Before(first)
}

// QueryWindow 菜谱查询窗口截断：返回 [start, end] 与今日未来 15 天 / 历史 30 天窗口的交集。
// 未来部分：today ≤ end ≤ today+14；历史部分：start ≥ today-30。方向按查询起止相对今日判断。
func QueryWindow(now time.Time, startStr, endStr string) (string, string, error) {
	start, err := ParseDate(startStr)
	if err != nil {
		return "", "", err
	}
	end, err := ParseDate(endStr)
	if err != nil {
		return "", "", err
	}
	if end.Before(start) {
		return "", "", errInvalidRange
	}
	today := toDate(now)
	lo := today.AddDate(0, 0, -30)
	hi := today.AddDate(0, 0, 14)
	if start.Before(lo) {
		start = lo
	}
	if end.After(hi) {
		end = hi
	}
	return start.Format("2006-01-02"), end.Format("2006-01-02"), nil
}

// —— 规则 4：核销截止 —— deadline 为当日 time（HH:MM:SS），now ≤ deadline 允许核销
// verifyDeadline 返回当日截止时刻；verifyAllowed 判定 canVerify。
func VerifyAllowed(now time.Time, menuDateStr string, deadlineHHMM string) bool {
	d, err := ParseDate(menuDateStr)
	if err != nil {
		return false
	}
	dl, err := time.ParseInLocation("15:04:05", normClock(deadlineHHMM), Loc)
	if err != nil {
		return false
	}
	deadline := time.Date(d.Year(), d.Month(), d.Day(), dl.Hour(), dl.Minute(), dl.Second(), 0, Loc)
	return !now.After(deadline)
}

// —— 规则 5：核销/登记目标日期必须为当日
func IsToday(now time.Time, menuDateStr string) bool {
	d, err := ParseDate(menuDateStr)
	if err != nil {
		return false
	}
	return sameDay(d, now)
}

// —— 规则 6：点评窗口 —— 当日 00:00 ≤ now ≤ 当日 23:59:59（即 menu_date 必须为当日）
func InReviewWindow(now time.Time, menuDateStr string) bool {
	return IsToday(now, menuDateStr)
}

// DefaultVerifyDeadlines 核销截止默认值（代码常量）。
func DefaultVerifyDeadlines() map[string]string {
	return map[string]string{
		"breakfast": "09:00:00",
		"lunch":     "13:30:00",
		"dinner":    "19:00:00",
	}
}

// ---- helpers ----

func toDate(t time.Time) time.Time {
	t = t.In(Loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, Loc)
}

func sameDay(a, b time.Time) bool {
	a, b = a.In(Loc), b.In(Loc)
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

// normClock 兼容 "9:00"、"09:00"、"13:30" 等输入。
func normClock(s string) string {
	// 补秒
	parts := 0
	for _, r := range s {
		if r == ':' {
			parts++
		}
	}
	switch parts {
	case 1:
		if len(s) > 0 && (s[0] == ' ' || s[0] == '0') {
			// keep
		}
		s = "0" + s
	}
	return s
}

var errInvalidRange = errRange{}

type errRange struct{}

func (errRange) Error() string { return "end before start" }
