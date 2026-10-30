package api

import (
	"net/http"
	"strconv"

	"canteen/server/internal/auth"
	"canteen/server/internal/httpx"
	"canteen/server/internal/store"
	"canteen/server/internal/timewindow"
)

// —— 超管：账号管理 ——

func (s *Server) createCanteenAdmin(w http.ResponseWriter, r *http.Request) {
	var req addEmployeeReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误")
		return
	}
	req.EmpNo = stringsTrimSpace(req.EmpNo)
	req.Name = stringsTrimSpace(req.Name)
	if req.EmpNo == "" || req.Name == "" {
		httpx.FailWith(w, httpx.CodeBadRequest, "工号与姓名必填")
		return
	}
	exists, err := s.St.EmpNoExists(req.EmpNo)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	if exists {
		httpx.FailWith(w, httpx.CodeEmpNoExists, "工号已存在："+req.EmpNo)
		return
	}
	hash, err := auth.HashPassword(defaultPassword)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	u := &store.User{
		EmpNo: req.EmpNo, Name: req.Name, Department: stringsTrimSpace(req.Department),
		PasswordHash: hash, Role: "canteen_admin", MustChangePassword: true,
	}
	if err := s.St.CreateUser(u); err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, userView(u))
}

func stringsTrimSpace(s string) string {
	b := []byte(s)
	out := b[:0]
	start := 0
	end := len(b)
	for start < end && (b[start] == ' ' || b[start] == '\t') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t') {
		end--
	}
	out = b[start:end]
	return string(out)
}

func (s *Server) superListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := parsePagination(r)
	users, total, err := s.St.ListUsers(q.Get("role"), q.Get("keyword"), page, pageSize)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"total": total, "page": page, "page_size": pageSize, "users": users})
}

func (s *Server) superSetStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "非法的用户 ID")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误")
		return
	}
	if req.Status != "active" && req.Status != "disabled" {
		httpx.FailWith(w, httpx.CodeBadRequest, "status 只能是 active 或 disabled")
		return
	}
	target, err := s.St.GetUserByID(id)
	if err != nil {
		httpx.FailWith(w, httpx.CodeNotFound, "用户不存在")
		return
	}
	// 禁止操作超管；禁止禁用超管自身（超管不可被禁用）
	if target.Role == "super_admin" {
		httpx.FailWith(w, httpx.CodeForbidden, "超级管理员账号不可被禁用/启用")
		return
	}
	if id == auth.UID(r) {
		httpx.FailWith(w, httpx.CodeForbidden, "不能操作自己的账号状态")
		return
	}
	if err := s.St.UpdateUserStatus(id, req.Status); err != nil {
		httpx.Fail(w, err)
		return
	}
	s.Lookup.Invalidate(id)
	httpx.OK(w, map[string]any{"id": id, "status": req.Status})
}

func (s *Server) superResetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "非法的用户 ID")
		return
	}
	target, err := s.St.GetUserByID(id)
	if err != nil {
		httpx.FailWith(w, httpx.CodeNotFound, "用户不存在")
		return
	}
	// 需求：仅可重置食堂管理员与员工密码
	if target.Role == "super_admin" {
		httpx.FailWith(w, httpx.CodeForbidden, "不可重置超级管理员密码")
		return
	}
	hash, err := auth.HashPassword(defaultPassword)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	if err := s.St.UpdateUserPassword(id, hash, true); err != nil {
		httpx.Fail(w, err)
		return
	}
	s.Lookup.Invalidate(id)
	httpx.OK(w, map[string]any{"id": id, "reset_to": defaultPassword, "must_change_password": true})
}

// —— 超管：违约查看 + 数据看板 ——

func rangeOrDefault(r *http.Request) (string, string) {
	q := r.URL.Query()
	from := q.Get("from")
	to := q.Get("to")
	if from == "" {
		from = timewindow.TodayMinusDays(29)
	}
	if to == "" {
		to = timewindow.TodayDateStr()
	}
	return from, to
}

func (s *Server) superViolations(w http.ResponseWriter, r *http.Request) {
	from, to := rangeOrDefault(r)
	rows, err := s.St.ViolationRank(from, to)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"from": from, "to": to, "violations": rows})
}

func (s *Server) dashTrend(w http.ResponseWriter, r *http.Request) {
	from, to := rangeOrDefault(r)
	rows, err := s.St.DailyTrend(from, to)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"from": from, "to": to, "trend": rows})
}

func (s *Server) dashDishes(w http.ResponseWriter, r *http.Request) {
	from, to := rangeOrDefault(r)
	rows, err := s.St.DishRanking(from, to)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"from": from, "to": to, "dishes": rows})
}

func (s *Server) dashWasteRate(w http.ResponseWriter, r *http.Request) {
	from, to := rangeOrDefault(r)
	rows, err := s.St.WasteRate(from, to)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"from": from, "to": to, "waste_rate": rows})
}

func (s *Server) dashDepartments(w http.ResponseWriter, r *http.Request) {
	from, to := rangeOrDefault(r)
	rows, err := s.St.DeptDining(from, to)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"from": from, "to": to, "departments": rows})
}
