package api

import (
	"net/http"
	"strconv"

	"canteen/server/internal/httpx"
)

// attendanceDetail 某天用餐明细分页（谁报名了来没来、谁没报名来吃了、谁没报名也没来）。
// GET /api/v1/admin/stats/attendance-detail?date=&dept=&keyword=&page=&page_size=
func (s *Server) attendanceDetail(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if len(date) != 10 {
		httpx.FailWith(w, httpx.CodeBadRequest, "date 必填（YYYY-MM-DD）")
		return
	}
	dept := r.URL.Query().Get("dept")
	keyword := r.URL.Query().Get("keyword")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	rows, total, err := s.St.AttendanceDetail(date, dept, keyword, page, pageSize)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"rows":      rows,
	})
}
