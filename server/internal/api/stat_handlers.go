package api

import (
	"net/http"

	"canteen/server/internal/httpx"
	"canteen/server/internal/timewindow"
)

// —— 食堂管理员统计（对齐 sheji.md §5.3 统计口径） ——

func (s *Server) statsSignups(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = timewindow.TodayDateStr()
	}
	rows, err := s.St.CountSignupsByMeal(date)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"date": date, "signups": rows})
}

func (s *Server) statsDepartments(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = timewindow.TodayDateStr()
	}
	rows, err := s.St.SignupByDepartment(date)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"date": date, "departments": rows})
}

func (s *Server) statsDaily(w http.ResponseWriter, r *http.Request) {
	from, to := rangeOrDefault(r)
	rows, err := s.St.DailyActual(from, to)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"from": from, "to": to, "daily": rows})
}
