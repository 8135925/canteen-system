package domain

import (
	"testing"
	"time"

	"canteen/server/internal/httpx"
	"canteen/server/internal/timewindow"
)

func sh(y, m, d, hh, mm, ss int) time.Time {
	return time.Date(y, time.Month(m), d, hh, mm, ss, 0, timewindow.Loc)
}

func TestDeriveEffectiveState(t *testing.T) {
	now := sh(2026, 10, 30, 10, 0, 0)
	cases := []struct {
		name   string
		row    *SignupRow
		date   string
		dead   string
		want   string
	}{
		{"无记录", nil, "2026-10-30", "09:00:00", "none"},
		{"已核销", &SignupRow{Status: SignupVerified}, "2026-10-30", "09:00:00", "verified"},
		{"已取消", &SignupRow{Status: SignupCancelled}, "2026-10-30", "09:00:00", "cancelled"},
		{"signed 未过截止→已报名", &SignupRow{Status: SignupSigned}, "2026-10-30", "13:30:00", "signed"},
		{"signed 已过截止→违约", &SignupRow{Status: SignupSigned}, "2026-10-30", "09:00:00", "no_show"},
		{"未来日期 signed→已报名", &SignupRow{Status: SignupSigned}, "2026-10-31", "09:00:00", "signed"},
		{"历史日期 signed→违约", &SignupRow{Status: SignupSigned}, "2026-10-29", "09:00:00", "no_show"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DeriveEffectiveState(c.row, c.date, c.dead, now)
			if got != c.want {
				t.Fatalf("got %s want %s", got, c.want)
			}
		})
	}
}

func TestDecideVerifyAction(t *testing.T) {
	now := sh(2026, 10, 30, 8, 0, 0)

	// 分支 1：无记录 → 登记未报名
	act, err := DecideVerifyAction(nil, "2026-10-30", "09:00:00", now)
	if act != CanWalkIn || err != nil {
		t.Fatalf("nil row: %v %v", act, err)
	}

	// 分支 2：cancelled → 登记未报名
	act, err = DecideVerifyAction(&SignupRow{Status: SignupCancelled}, "2026-10-30", "09:00:00", now)
	if act != CanWalkIn || err != nil {
		t.Fatalf("cancelled: %v %v", act, err)
	}

	// 分支 3：verified → 已核销提示
	act, err = DecideVerifyAction(&SignupRow{Status: SignupVerified}, "2026-10-30", "09:00:00", now)
	if act != VerifyAlreadyVerified {
		t.Fatalf("verified: %v %v", act, err)
	}
	be, ok := err.(*httpx.BizErr)
	if !ok || be.Code != httpx.CodeAlreadyVerified {
		t.Fatalf("expect ALREADY_VERIFIED, got %v", err)
	}

	// 分支 4：signed 未过截止 → 可核销
	act, err = DecideVerifyAction(&SignupRow{Status: SignupSigned}, "2026-10-30", "09:00:00", now)
	if act != VerifyOK || err != nil {
		t.Fatalf("signed in window: %v %v", act, err)
	}

	// 分支 5：signed 已过截止 → 双拒（防双重违约）
	act, err = DecideVerifyAction(&SignupRow{Status: SignupSigned}, "2026-10-30", "09:00:00", sh(2026, 10, 30, 9, 0, 1))
	if act != VerifyViolated {
		t.Fatalf("expired signed: %v %v", act, err)
	}
	be, ok = err.(*httpx.BizErr)
	if !ok || be.Code != httpx.CodeAlreadyViolated {
		t.Fatalf("expect ALREADY_VIOLATED, got %v", err)
	}

	// 分支 6：signed 但 menu_date 非当日（防提前核销未来餐次）
	act, err = DecideVerifyAction(&SignupRow{Status: SignupSigned}, "2026-10-31", "09:00:00", now)
	if act != VerifyViolated || err == nil {
		t.Fatalf("future date verify must be rejected: %v %v", act, err)
	}
}

func TestReviewChainCheck(t *testing.T) {
	now := sh(2026, 10, 30, 12, 0, 0)

	// 通过
	if err := ReviewChainCheck("2026-10-30", now, true, "normal"); err != nil {
		t.Fatalf("valid review rejected: %v", err)
	}
	// ① 跨日
	err := ReviewChainCheck("2026-10-29", now, true, "normal")
	if be, ok := err.(*httpx.BizErr); !ok || be.Code != httpx.CodeOutOfRange {
		t.Fatalf("cross-day: %v", err)
	}
	// ② 未就餐
	err = ReviewChainCheck("2026-10-30", now, false, "normal")
	if be, ok := err.(*httpx.BizErr); !ok || be.Code != httpx.CodeForbidden {
		t.Fatalf("not eaten: %v", err)
	}
	// ③ 已下架
	err = ReviewChainCheck("2026-10-30", now, true, "off_shelf")
	if be, ok := err.(*httpx.BizErr); !ok || be.Code != httpx.CodeDishOffShelf {
		t.Fatalf("off shelf: %v", err)
	}
}

func TestCanSignup(t *testing.T) {
	now := sh(2026, 10, 30, 10, 0, 0)

	// 未发布
	err := CanSignup(false, "2026-10-31", now)
	if be, ok := err.(*httpx.BizErr); !ok || be.Code != httpx.CodeMenuNotPublished {
		t.Fatalf("not published: %v", err)
	}
	// 已过 00:00
	err = CanSignup(true, "2026-10-30", now)
	if be, ok := err.(*httpx.BizErr); !ok || be.Code != httpx.CodeSignupClosed {
		t.Fatalf("past midnight: %v", err)
	}
	// 通过
	if err := CanSignup(true, "2026-10-31", now); err != nil {
		t.Fatalf("valid: %v", err)
	}
}

func TestCountViolations(t *testing.T) {
	if CountViolations(3, 2) != 5 {
		t.Fatal("violation count wrong")
	}
}

func TestValidMeal(t *testing.T) {
	for _, m := range []string{MealBreakfast, MealLunch, MealDinner} {
		if !ValidMeal(m) {
			t.Fatalf("meal %s should be valid", m)
		}
	}
	if ValidMeal("brunch") {
		t.Fatal("brunch invalid")
	}
}
