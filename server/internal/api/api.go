// Package api 路由与处理器（对齐 doc/sheji.md §5.4 API 清单，/api/v1 前缀）。
package api

import (
	"net/http"
	"strings"

	"canteen/server/internal/auth"
	"canteen/server/internal/httpx"
	"canteen/server/internal/store"
	"canteen/server/internal/timewindow"
)

type Server struct {
	St     store.Store
	Secret string
	Lookup *auth.UserLookup
	Guard  *auth.LoginGuard
}

func NewServer(st store.Store, secret string, lookup *auth.UserLookup, guard *auth.LoginGuard) *Server {
	return &Server{St: st, Secret: secret, Lookup: lookup, Guard: guard}
}

// Handler 组装全部路由（含认证/角色中间件）。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// 公开
	mux.HandleFunc("GET /api/v1/healthz", s.healthz)
	mux.HandleFunc("GET /api/v1/menus", s.listMenus)
	mux.HandleFunc("GET /api/v1/menus/summary", s.menuSummary)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)

	// 认证 + 角色保护
	authed := auth.Middleware(s.Secret, s.Lookup)
	require := func(h http.Handler, roles ...string) http.Handler {
		return authed(auth.RequireRole(roles...)(h))
	}

	// 通用认证（员工/管理员/超管自身接口）
	mux.Handle("GET /api/v1/me", authed(http.HandlerFunc(s.me)))
	mux.Handle("PUT /api/v1/my/password", authed(http.HandlerFunc(s.changePassword)))

	// 员工端
	mux.Handle("POST /api/v1/signups", require(http.HandlerFunc(s.createSignup), "employee"))
	mux.Handle("DELETE /api/v1/signups", require(http.HandlerFunc(s.cancelSignup), "employee"))
	mux.Handle("GET /api/v1/my/signups", require(http.HandlerFunc(s.mySignups), "employee"))
	mux.Handle("GET /api/v1/my/violation-count", require(http.HandlerFunc(s.myViolationCount), "employee"))
	mux.Handle("POST /api/v1/reviews", require(http.HandlerFunc(s.createReview), "employee"))
	mux.Handle("POST /api/v1/suggestions", require(http.HandlerFunc(s.createSuggestion), "employee"))
	mux.Handle("GET /api/v1/my/suggestions", require(http.HandlerFunc(s.mySuggestions), "employee"))

	// 食堂管理员端
	mux.Handle("GET /api/v1/admin/employees", require(http.HandlerFunc(s.listEmployees), "canteen_admin"))
	mux.Handle("POST /api/v1/admin/employees", require(http.HandlerFunc(s.addEmployee), "canteen_admin"))
	mux.Handle("POST /api/v1/admin/employees/import", require(http.HandlerFunc(s.importEmployees), "canteen_admin"))
	mux.Handle("PATCH /api/v1/admin/employees/{id}", require(http.HandlerFunc(s.editEmployee), "canteen_admin"))
	mux.Handle("POST /api/v1/admin/menus", require(http.HandlerFunc(s.publishMenu), "canteen_admin"))
	mux.Handle("GET /api/v1/admin/menus/copy-last-week", require(http.HandlerFunc(s.copyLastWeek), "canteen_admin"))
	mux.Handle("PATCH /api/v1/admin/menu-items/{id}/shelf", require(http.HandlerFunc(s.shelfMenuItem), "canteen_admin"))
	mux.Handle("GET /api/v1/admin/verify-rules", require(http.HandlerFunc(s.listVerifyRules), "canteen_admin"))
	mux.Handle("PUT /api/v1/admin/verify-rules", require(http.HandlerFunc(s.updateVerifyRules), "canteen_admin"))
	mux.Handle("GET /api/v1/admin/stats/signups", require(http.HandlerFunc(s.statsSignups), "canteen_admin"))
	mux.Handle("GET /api/v1/admin/stats/departments", require(http.HandlerFunc(s.statsDepartments), "canteen_admin"))
	mux.Handle("GET /api/v1/admin/stats/daily", require(http.HandlerFunc(s.statsDaily), "canteen_admin"))
	mux.Handle("GET /api/v1/admin/verify/lookup", require(http.HandlerFunc(s.verifyLookup), "canteen_admin"))
	mux.Handle("POST /api/v1/admin/verify", require(http.HandlerFunc(s.doVerify), "canteen_admin"))
	mux.Handle("POST /api/v1/admin/walk-in", require(http.HandlerFunc(s.walkIn), "canteen_admin"))
	mux.Handle("GET /api/v1/admin/suggestions", require(http.HandlerFunc(s.listSuggestions), "canteen_admin"))
	mux.Handle("PATCH /api/v1/admin/suggestions/{id}", require(http.HandlerFunc(s.updateSuggestion), "canteen_admin"))

	// 超级管理员端
	mux.Handle("POST /api/v1/super/canteen-admins", require(http.HandlerFunc(s.createCanteenAdmin), "super_admin"))
	mux.Handle("GET /api/v1/super/users", require(http.HandlerFunc(s.superListUsers), "super_admin"))
	mux.Handle("PATCH /api/v1/super/users/{id}/status", require(http.HandlerFunc(s.superSetStatus), "super_admin"))
	mux.Handle("POST /api/v1/super/users/{id}/reset-password", require(http.HandlerFunc(s.superResetPassword), "super_admin"))
	mux.Handle("GET /api/v1/super/violations", require(http.HandlerFunc(s.superViolations), "super_admin"))
	mux.Handle("GET /api/v1/super/dashboard/trend", require(http.HandlerFunc(s.dashTrend), "super_admin"))
	mux.Handle("GET /api/v1/super/dashboard/dishes", require(http.HandlerFunc(s.dashDishes), "super_admin"))
	mux.Handle("GET /api/v1/super/dashboard/waste-rate", require(http.HandlerFunc(s.dashWasteRate), "super_admin"))
	mux.Handle("GET /api/v1/super/dashboard/departments", require(http.HandlerFunc(s.dashDepartments), "super_admin"))

	return mux
}

// effectiveDeadline 餐次核销截止：verify_rules 覆盖值 || 代码默认常量。
func (s *Server) effectiveDeadline(meal string) string {
	rules, err := s.St.ListVerifyRules()
	if err == nil {
		for _, r := range rules {
			if r.Meal == meal {
				return r.Deadline
			}
		}
	}
	if d, ok := timewindow.DefaultVerifyDeadlines()[meal]; ok {
		return d
	}
	return "19:00:00"
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if err := s.St.Ping(); err != nil {
		httpx.FailWith(w, httpx.CodeInternal, "db not ready")
		return
	}
	httpx.OK(w, map[string]string{"status": "up"})
}

// parsePagination 解析分页参数。
func parsePagination(r *http.Request) (page, pageSize int) {
	page = atoiDefault(r.URL.Query().Get("page"), 1)
	pageSize = atoiDefault(r.URL.Query().Get("page_size"), 20)
	return
}

func atoiDefault(s string, def int) int {
	n := 0
	if s == "" {
		return def
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return def
	}
	return n
}

func trimPrefix(s, prefix string) string { return strings.TrimPrefix(s, prefix) }
