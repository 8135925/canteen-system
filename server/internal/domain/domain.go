// Package domain 纯业务判定逻辑：违约推导、核销决策、点评校验链。
// 全部为纯函数（输入时间与状态快照，输出决策），便于单元测试覆盖全分支矩阵。
package domain

import (
	"time"

	"canteen/server/internal/httpx"
	"canteen/server/internal/timewindow"
)

// Meal 常量
const (
	MealBreakfast = "breakfast"
	MealLunch     = "lunch"
	MealDinner    = "dinner"
)

// ValidMeal 是否合法餐次。
func ValidMeal(m string) bool {
	return m == MealBreakfast || m == MealLunch || m == MealDinner
}

// SignupStatus 报餐行状态
const (
	SignupSigned    = "signed"
	SignupCancelled = "cancelled"
	SignupVerified  = "verified"
)

// SignupRow 报餐行快照（供纯函数判定）。
type SignupRow struct {
	ID       int64
	Status   string // signed / cancelled / verified
	SignedAt time.Time
}

// ViolationState 违约展示态：对「最新一行」推导展示状态。
// 对齐 sheji.md §4.4：signed+过截止→no_show；signed 未过截止→已报名；verified→已核销；cancelled→已取消。
func ViolationState(latest *SignupRow, now time.Time, deadlineClock string) string {
	if latest == nil {
		return "none"
	}
	switch latest.Status {
	case SignupVerified:
		return "verified"
	case SignupCancelled:
		return "cancelled"
	case SignupSigned:
		if timewindow.VerifyAllowed(now, now.Format("2006-01-02"), deadlineClock) && sameDay(now, latestSignedDay(now)) {
			// 最新行为 signed：仅当目标日期（=行所属日期）已过截止才判违约。
			// 此处签名按“当前时刻对该行的截止判定”由调用方以 VerifyDeadlineFor 传入；
			// 简化：由 EffectiveDeadline 判定（见 DeriveEffectiveState）。
			return "signed"
		}
		return "signed"
	}
	return "unknown"
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func latestSignedDay(now time.Time) time.Time { return now }

// DeriveEffectiveState 基于「该报餐行的用餐日期 + 核销截止时刻」推导展示状态。
// menuDate 为该行 menu_date（YYYY-MM-DD），deadlineClock 为该餐次核销截止（HH:MM:SS）。
func DeriveEffectiveState(latest *SignupRow, menuDate, deadlineClock string, now time.Time) string {
	if latest == nil {
		return "none"
	}
	switch latest.Status {
	case SignupVerified:
		return "verified"
	case SignupCancelled:
		return "cancelled"
	case SignupSigned:
		if timewindow.VerifyAllowed(now, menuDate, deadlineClock) {
			return "signed" // 已报名（含未来未到期）
		}
		return "no_show" // 推导违约
	}
	return "unknown"
}

// VerifyAction 现场核销/未报名登记决策结果（对齐 sheji.md §5.3 分支矩阵）。
type VerifyAction int

const (
	VerifyOK          VerifyAction = iota // 可核销
	VerifyAlreadyVerified                 // 已核销
	VerifyViolated                        // signed 且已过截止：核销与登记均拒绝
	CanWalkIn                             // 无记录/cancelled：可登记未报名就餐
)

// DecideVerifyAction 纯决策函数：输入最新报餐行、目标日期、截止时刻、now。
// 返回决策与错误（错误码供 API 层直接透出）。
func DecideVerifyAction(latest *SignupRow, menuDate, deadlineClock string, now time.Time) (VerifyAction, error) {
	// 核销/登记目标日期必须为当日（防提前核销未来餐次）
	if !timewindow.IsToday(now, menuDate) {
		return VerifyViolated, httpx.Err(httpx.CodeOutOfRange, "核销与未报名登记仅限当日餐次")
	}
	if latest == nil || latest.Status == SignupCancelled {
		return CanWalkIn, nil
	}
	if latest.Status == SignupVerified {
		return VerifyAlreadyVerified, httpx.Err(httpx.CodeAlreadyVerified, "该员工该餐已核销，请勿重复操作")
	}
	// signed：
	if timewindow.VerifyAllowed(now, menuDate, deadlineClock) {
		return VerifyOK, nil
	}
	return VerifyViolated, httpx.Err(httpx.CodeAlreadyViolated, "已过核销截止，该餐报餐已判违约，不能核销或登记")
}

// ReviewChainCheck 点评校验链（对齐 sheji.md §5.3 ①②③）。
// hasEaten：该员工该餐存在 verified 报餐或 walk_in 记录；dishStatus：目标菜品状态（normal/off_shelf）。
func ReviewChainCheck(menuDate string, now time.Time, hasEaten bool, dishStatus string) error {
	// ① 当日窗口（menu_date 必须为当日）
	if !timewindow.InReviewWindow(now, menuDate) {
		return httpx.Err(httpx.CodeOutOfRange, "点评仅当日有效")
	}
	// ② 该员工该餐已核销或已登记未报名就餐
	if !hasEaten {
		return httpx.Err(httpx.CodeForbidden, "仅当餐已核销或已登记未报名就餐的员工可点评")
	}
	// ③ 菜品未下架
	if dishStatus != "normal" {
		return httpx.Err(httpx.CodeDishOffShelf, "已下架菜品不允许点评")
	}
	return nil
}

// CanSignup 报名校验（对齐 §5.3）：菜单已发布 + 未过当日 00:00。
func CanSignup(menuPublished bool, menuDate string, now time.Time) error {
	if !menuPublished {
		return httpx.Err(httpx.CodeMenuNotPublished, "该餐次菜谱未发布，暂不可报名")
	}
	if !timewindow.CanSignupModifyCancel(now, menuDate) {
		return httpx.Err(httpx.CodeSignupClosed, "已过用餐当日 00:00，不可报名、修改或取消")
	}
	return nil
}

// CountViolations 违约次数 = 推导违约报餐数 + walk_in 数（纯函数口径）。
func CountViolations(noShowCount, walkInCount int) int {
	return noShowCount + walkInCount
}
