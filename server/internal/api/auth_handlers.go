package api

import (
	"net"
	"net/http"
	"time"

	"canteen/server/internal/auth"
	"canteen/server/internal/httpx"
	"canteen/server/internal/store"
)

type loginReq struct {
	EmpNo    string `json:"emp_no"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := httpx.DecodeJSON(r, &req); err != nil || req.EmpNo == "" || req.Password == "" {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误：emp_no 与 password 必填")
		return
	}
	guardKey := req.EmpNo + "|" + clientIP(r)
	if locked, remaining := s.Guard.Check(guardKey); locked {
		httpx.FailWith(w, httpx.CodeLocked, "失败次数过多，请 "+remaining.Round(time.Second).String()+" 后重试")
		return
	}
	u, err := s.St.GetUserByEmpNo(req.EmpNo)
	if err != nil {
		s.Guard.Fail(guardKey)
		httpx.FailWith(w, httpx.CodeInvalidCredentials, "工号或密码错误")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		s.Guard.Fail(guardKey)
		httpx.FailWith(w, httpx.CodeInvalidCredentials, "工号或密码错误")
		return
	}
	if u.Status == "disabled" {
		httpx.FailWith(w, httpx.CodeForbidden, "账号已被禁用，请联系超级管理员")
		return
	}
	s.Guard.Reset(guardKey)
	token, err := auth.IssueToken(s.Secret, u.ID, u.Role, u.MustChangePassword, req.Remember)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{
		"token": token,
		"user": map[string]any{
			"id": u.ID, "emp_no": u.EmpNo, "name": u.Name,
			"department": u.Department, "role": u.Role,
			"must_change_password": u.MustChangePassword,
		},
	})
}

func userView(u *store.User) map[string]any {
	return map[string]any{
		"id": u.ID, "emp_no": u.EmpNo, "name": u.Name,
		"department": u.Department, "role": u.Role,
		"must_change_password": u.MustChangePassword,
	}
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u, err := s.St.GetUserByID(auth.UID(r))
	if err != nil {
		httpx.FailWith(w, httpx.CodeNotFound, "用户不存在")
		return
	}
	view := userView(u)
	// 附加：演示模式（前端据此对超管隐藏改密入口）
	view["demo_mode"] = s.DemoMode
	httpx.OK(w, view)
}

type changePwdReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	uid := auth.UID(r)
	// 演示环境：超管账号密码固定（123123），不允许修改
	if s.DemoMode && auth.RoleFrom(r) == "super_admin" {
		httpx.FailWith(w, httpx.CodeForbidden, "演示环境不允许修改超管密码")
		return
	}
	var req changePwdReq
	if err := httpx.DecodeJSON(r, &req); err != nil || req.OldPassword == "" || req.NewPassword == "" {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误：old_password 与 new_password 必填")
		return
	}
	if len(req.NewPassword) < 6 {
		httpx.FailWith(w, httpx.CodeBadRequest, "新密码至少 6 位")
		return
	}
	u, err := s.St.GetUserByID(uid)
	if err != nil {
		httpx.FailWith(w, httpx.CodeNotFound, "用户不存在")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.OldPassword) {
		httpx.FailWith(w, httpx.CodeInvalidCredentials, "旧密码错误")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	if err := s.St.UpdateUserPassword(uid, hash, false); err != nil {
		httpx.Fail(w, err)
		return
	}
	s.Lookup.Invalidate(uid)
	// 改密成功重签 mcp=false token（非记住登录 12h；记住登录由前端重新选择）
	token, err := auth.IssueToken(s.Secret, uid, u.Role, false, false)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]string{"token": token})
}
