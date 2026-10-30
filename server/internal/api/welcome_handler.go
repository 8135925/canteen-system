package api

import (
	"net/http"
	"time"

	"canteen/server/internal/httpx"
)

// welcomeStats 欢迎首页聚合数据（对齐 pure-admin 仪表盘布局）：
// 今日报名/核销/未报名、本月汇总、满意度、近 7 天报名 vs 就餐、就餐率、最新建议。
func (s *Server) welcomeStats(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	monthStart := now.Format("2006-01-01")
	weekStart := now.AddDate(0, 0, -6).Format("2006-01-02")

	// 今日报名（按餐次求和）
	mealCounts, err := s.St.CountSignupsByMeal(today)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	todaySignup := 0
	for _, mc := range mealCounts {
		todaySignup += mc.Count
	}

	// 本月实际就餐 vs 报名（求和）
	daily, err := s.St.DailyActual(monthStart, today)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	monthVerified, monthWalkIn, monthSignups, monthNoShow := 0, 0, 0, 0
	for _, row := range daily {
		monthVerified += row.Verified
		monthWalkIn += row.WalkIn
		monthSignups += row.Signups
		monthNoShow += row.Diff
	}

	// 近 7 天：报名 vs 实际就餐 + 就餐率
	weekDaily, err := s.St.DailyActual(weekStart, today)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	type trendRow struct {
		Date   string  `json:"date"`
		Signups int `json:"signups"`
		Dining int `json:"dining"`
	}
	trend := make([]trendRow, 0, len(weekDaily))
	for _, row := range weekDaily {
		trend = append(trend, trendRow{
			Date:   row.Date,
			Signups: row.Signups,
			Dining: row.Verified + row.WalkIn,
		})
	}
	type rateRow struct {
		Date    string  `json:"date"`
		Rate    float64 `json:"rate"` // 0~1
		Signups int `json:"signups"`
	}
	rates := make([]rateRow, 0, len(weekDaily))
	for i := len(weekDaily) - 1; i >= 0; i-- { // 倒序：最新在前
		row := weekDaily[i]
		rate := 0.0
		if row.Signups > 0 {
			rate = float64(row.Verified+row.WalkIn) / float64(row.Signups)
		}
		rates = append(rates, rateRow{Date: row.Date, Rate: rate, Signups: row.Signups})
	}

	// 满意度
	like, dislike, err := s.St.DishSatisfaction()
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	satisfaction := 0.0
	if like+dislike > 0 {
		satisfaction = float64(like) / float64(like+dislike)
	}

	// 最新建议（前 5 条）
	sugs, err := s.St.ListSuggestions("")
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	if len(sugs) > 5 {
		sugs = sugs[:5]
	}

	// 昨日报名（涨幅对比）
	yd, err := s.St.DailyActual(yesterday, yesterday)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	yesterdaySignup := 0
	if len(yd) > 0 {
		yesterdaySignup = yd[0].Signups
	}

	httpx.OK(w, map[string]any{
		"today_signup":     todaySignup,
		"yesterday_signup": yesterdaySignup,
		"today_verify":     mealVerifiedToday(s),
		"month_verified":   monthVerified,
		"month_walk_in":    monthWalkIn,
		"month_signups":    monthSignups,
		"month_noshow":     monthNoShow,
		"satisfaction":     satisfaction,
		"trend":            trend,
		"rates":            rates,
		"latest_suggestions": sugs,
	})
}

// mealVerifiedToday 今日已核销数（从本月 daily 中取今日行）。
func mealVerifiedToday(s *Server) int {
	today := time.Now().Format("2006-01-02")
	daily, err := s.St.DailyActual(today, today)
	if err != nil || len(daily) == 0 {
		return 0
	}
	return daily[0].Verified
}
