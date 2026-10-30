package timewindow

import (
	"testing"
	"time"
)

// 上海时区构造器
func sh(y, m, d, hh, mm, ss int) time.Time {
	return time.Date(y, time.Month(m), d, hh, mm, ss, 0, Loc)
}

// —— 规则 1：可报名/修改/取消：now < menu_date 00:00 ——

func TestCanSignupModifyCancel(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		date string
		want bool
	}{
		{"前一天23:59:59可报", sh(2026, 10, 29, 23, 59, 59), "2026-10-30", true},
		{"当日00:00:00不可报", sh(2026, 10, 30, 0, 0, 0), "2026-10-30", false},
		{"当日00:00:01不可报", sh(2026, 10, 30, 0, 0, 1), "2026-10-30", false},
		{"当日午餐时间不可报", sh(2026, 10, 30, 12, 0, 0), "2026-10-30", false},
		{"提前15天可报", sh(2026, 10, 30, 18, 0, 0), "2026-11-14", true},
		{"过去日期不可报", sh(2026, 10, 30, 8, 0, 0), "2026-10-29", false},
		{"非法日期返回false", sh(2026, 10, 30, 8, 0, 0), "bad-date", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CanSignupModifyCancel(c.now, c.date); got != c.want {
				t.Fatalf("CanSignupModifyCancel(%v, %s) = %v, want %v", c.now, c.date, got, c.want)
			}
		})
	}
}

// —— 规则 2：发布/可见范围 today ≤ menu_date ≤ today+14 ——

func TestInFutureMenuRange(t *testing.T) {
	now := sh(2026, 10, 29, 10, 0, 0)
	cases := []struct {
		date string
		want bool
	}{
		{"2026-10-29", true},  // today
		{"2026-11-12", true},  // today+14
		{"2026-11-13", false}, // today+15
		{"2026-10-28", false}, // 昨天
		{"2026-12-01", false}, // 远未来
	}
	for _, c := range cases {
		if got := InFutureMenuRange(now, c.date); got != c.want {
			t.Fatalf("InFutureMenuRange(%s) = %v, want %v", c.date, got, c.want)
		}
	}
	if InFutureMenuRange(now, "bad") {
		t.Fatal("bad date should be false")
	}
}

// —— 规则 3：历史可见范围 menu_date ≥ today-30 ——

func TestInHistoryRange(t *testing.T) {
	now := sh(2026, 10, 29, 10, 0, 0)
	cases := []struct {
		date string
		want bool
	}{
		{"2026-09-29", true},  // today-30
		{"2026-09-28", false}, // today-31
		{"2026-10-29", true},  // today
		{"2026-11-15", true},  // 历史范围规则不限制未来（未来范围由规则 2 管）
	}
	for _, c := range cases {
		if got := InHistoryRange(now, c.date); got != c.want {
			t.Fatalf("InHistoryRange(%s) = %v, want %v", c.date, got, c.want)
		}
	}
}

// —— 规则 2+3 联动：查询窗口截断 ——

func TestQueryWindow(t *testing.T) {
	now := sh(2026, 10, 29, 10, 0, 0)

	// 未来 15 天完整范围
	s, e, err := QueryWindow(now, "2026-10-29", "2026-11-12")
	if err != nil || s != "2026-10-29" || e != "2026-11-12" {
		t.Fatalf("future window: %s %s %v", s, e, err)
	}
	// 越界截断：end 超过 today+14
	s, e, _ = QueryWindow(now, "2026-10-29", "2026-12-31")
	if e != "2026-11-12" {
		t.Fatalf("future truncate got end=%s", e)
	}
	// 历史窗口
	s, e, _ = QueryWindow(now, "2026-09-29", "2026-10-28")
	if s != "2026-09-29" || e != "2026-10-28" {
		t.Fatalf("history window: %s %s", s, e)
	}
	// 历史 31 天截断
	s, e, _ = QueryWindow(now, "2026-09-01", "2026-10-28")
	if s != "2026-09-29" {
		t.Fatalf("history truncate got start=%s", s)
	}
	// 非法区间
	if _, _, err := QueryWindow(now, "2026-10-29", "2026-10-28"); err == nil {
		t.Fatal("end<start should error")
	}
	// 非法格式
	if _, _, err := QueryWindow(now, "x", "2026-10-29"); err == nil {
		t.Fatal("bad start should error")
	}
}

// —— 规则 4：核销截止 ——

func TestVerifyAllowed(t *testing.T) {
	cases := []struct {
		name     string
		now      time.Time
		date     string
		deadline string
		want     bool
	}{
		{"截止前1秒可核销", sh(2026, 10, 30, 8, 59, 59), "2026-10-30", "09:00:00", true},
		{"恰好截止时刻可核销", sh(2026, 10, 30, 9, 0, 0), "2026-10-30", "09:00:00", true},
		{"截止后1秒不可核销", sh(2026, 10, 30, 9, 0, 1), "2026-10-30", "09:00:00", false},
		{"午餐13:30边界", sh(2026, 10, 30, 13, 30, 0), "2026-10-30", "13:30:00", true},
		{"晚餐19:00之后", sh(2026, 10, 30, 19, 0, 1), "2026-10-30", "19:00:00", false},
		{"昨日截止对今日无效", sh(2026, 10, 30, 8, 0, 0), "2026-10-29", "09:00:00", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := VerifyAllowed(c.now, c.date, c.deadline); got != c.want {
				t.Fatalf("VerifyAllowed(%v, %s, %s) = %v, want %v", c.now, c.date, c.deadline, got, c.want)
			}
		})
	}
	// 非法时刻
	if VerifyAllowed(sh(2026, 10, 30, 8, 0, 0), "2026-10-30", "bad") {
		t.Fatal("bad deadline should be false")
	}
	// 管理员可覆盖：早餐截止改为 10:30
	if !VerifyAllowed(sh(2026, 10, 30, 10, 0, 0), "2026-10-30", "10:30:00") {
		t.Fatal("override deadline should work")
	}
}

// —— 规则 5：目标日期必须为当日 ——

func TestIsToday(t *testing.T) {
	now := sh(2026, 10, 30, 8, 0, 0)
	if !IsToday(now, "2026-10-30") {
		t.Fatal("today should be true")
	}
	if IsToday(now, "2026-10-31") {
		t.Fatal("tomorrow should be false")
	}
	if IsToday(now, "2026-10-29") {
		t.Fatal("yesterday should be false")
	}
	if IsToday(now, "bad") {
		t.Fatal("bad date should be false")
	}
}

// —— 规则 6：点评窗口 ——

func TestInReviewWindow(t *testing.T) {
	now := sh(2026, 10, 30, 23, 59, 59)
	if !InReviewWindow(now, "2026-10-30") {
		t.Fatal("same day 23:59:59 should be true")
	}
	if InReviewWindow(sh(2026, 10, 31, 0, 0, 0), "2026-10-30") {
		t.Fatal("next day should be false")
	}
}

// —— 默认核销截止常量 ——

func TestDefaultVerifyDeadlines(t *testing.T) {
	d := DefaultVerifyDeadlines()
	if d["breakfast"] != "09:00:00" || d["lunch"] != "13:30:00" || d["dinner"] != "19:00:00" {
		t.Fatalf("defaults wrong: %v", d)
	}
}

func TestTodayHelpers(t *testing.T) {
	// 只验证格式与相对关系
	today := TodayDateStr()
	if len(today) != 10 || today[4] != '-' {
		t.Fatalf("bad date format: %s", today)
	}
	if TodayMinusDays(30) >= today {
		t.Fatalf("TodayMinusDays should be before today: %s", TodayMinusDays(30))
	}
	if TodayPlusDays(14) <= today {
		t.Fatalf("TodayPlusDays should be after today: %s", TodayPlusDays(14))
	}
}
