package api

import (
	"net/http"

	"canteen/server/internal/auth"
	"canteen/server/internal/domain"
	"canteen/server/internal/httpx"
	"canteen/server/internal/store"
	"canteen/server/internal/timewindow"
)

// resolveForVerify 核销/登记前置解析：工号 + 当日校验 + 最新报餐行 + 决策。
type verifyContext struct {
	user    *store.User
	latest  *store.Signup
	action  domain.VerifyAction
	err     error
	deadline string
}

func (s *Server) resolveVerify(empNo, menuDate, meal string) (*verifyContext, error) {
	if !domain.ValidMeal(meal) {
		return nil, httpx.Err(httpx.CodeBadRequest, "餐次非法")
	}
	vc := &verifyContext{}
	u, err := s.St.GetUserByEmpNo(empNo)
	if err != nil {
		return nil, httpx.Err(httpx.CodeNotFound, "工号不存在")
	}
	vc.user = u
	vc.deadline = s.effectiveDeadline(meal)
	latest, err := s.St.GetLatestSignup(u.ID, menuDate, meal)
	if err != nil && err != store.ErrNotFound {
		return nil, err
	}
	vc.latest = latest
	vc.action, vc.err = domain.DecideVerifyAction(
		toDomainRow(latest), menuDate, vc.deadline, timewindow.NowShanghai())
	return vc, nil
}

func toDomainRow(r *store.Signup) *domain.SignupRow {
	if r == nil {
		return nil
	}
	return &domain.SignupRow{ID: r.ID, Status: r.Status, SignedAt: r.SignedAt}
}

func (s *Server) verifyLookup(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	empNo, menuDate, meal := q.Get("emp_no"), q.Get("menu_date"), q.Get("meal")
	if empNo == "" || menuDate == "" || meal == "" {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误：emp_no/menu_date/meal 必填")
		return
	}
	vc, err := s.resolveVerify(empNo, menuDate, meal)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	resp := map[string]any{
		"user":     map[string]any{"id": vc.user.ID, "emp_no": vc.user.EmpNo, "name": vc.user.Name, "department": vc.user.Department},
		"menu_date": menuDate, "meal": meal,
		"deadline": vc.deadline,
		"action":   actionName(vc.action),
	}
	if vc.latest != nil {
		resp["signup"] = map[string]any{"id": vc.latest.ID, "status": vc.latest.Status, "signed_at": vc.latest.SignedAt}
	}
	if vc.err != nil {
		resp["reason"] = vc.err.Error()
	}
	httpx.OK(w, resp)
}

func actionName(a domain.VerifyAction) string {
	switch a {
	case domain.VerifyOK:
		return "verify"
	case domain.VerifyAlreadyVerified:
		return "already_verified"
	case domain.VerifyViolated:
		return "violated"
	case domain.CanWalkIn:
		return "walk_in"
	}
	return "unknown"
}

type doVerifyReq struct {
	EmpNo    string `json:"emp_no"`
	MenuDate string `json:"menu_date"`
	Meal     string `json:"meal"`
}

func (s *Server) doVerify(w http.ResponseWriter, r *http.Request) {
	adminID := auth.UID(r)
	var req doVerifyReq
	if err := httpx.DecodeJSON(r, &req); err != nil || req.EmpNo == "" {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误：emp_no/menu_date/meal 必填")
		return
	}
	vc, err := s.resolveVerify(req.EmpNo, req.MenuDate, req.Meal)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	if vc.action != domain.VerifyOK {
		if vc.err != nil {
			httpx.Fail(w, vc.err)
		} else {
			httpx.FailWith(w, httpx.CodeBadRequest, "当前状态不可核销")
		}
		return
	}
	if err := s.St.InsertVerification(vc.latest.ID, adminID, timewindow.NowShanghai()); err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"signup_id": vc.latest.ID, "status": "verified"})
}

func (s *Server) walkIn(w http.ResponseWriter, r *http.Request) {
	adminID := auth.UID(r)
	var req doVerifyReq
	if err := httpx.DecodeJSON(r, &req); err != nil || req.EmpNo == "" {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误：emp_no/menu_date/meal 必填")
		return
	}
	vc, err := s.resolveVerify(req.EmpNo, req.MenuDate, req.Meal)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	if vc.action != domain.CanWalkIn {
		if vc.err != nil {
			httpx.Fail(w, vc.err)
		} else {
			// 有有效报名（signed 未过截止）：应走核销
			httpx.FailWith(w, httpx.CodeHasActiveSignup, "该员工已报名未核销，请走核销流程")
		}
		return
	}
	if err := s.St.InsertWalkIn(vc.user.ID, req.MenuDate, req.Meal, adminID, timewindow.NowShanghai()); err != nil {
		if err == store.ErrConflict {
			httpx.FailWith(w, httpx.CodeAlreadyWalkIn, "该员工该餐次已登记过未报名就餐，请勿重复登记")
			return
		}
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"user_id": vc.user.ID, "menu_date": req.MenuDate, "meal": req.Meal, "status": "walk_in"})
}

// —— 核销规则 ——

func (s *Server) listVerifyRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.St.ListVerifyRules()
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	overrides := map[string]string{}
	for _, r := range rules {
		overrides[r.Meal] = r.Deadline
	}
	defaults := timewindow.DefaultVerifyDeadlines()
	out := []map[string]any{}
	for _, meal := range []string{"breakfast", "lunch", "dinner"} {
		dl, overridden := overrides[meal]
		if !overridden {
			dl = defaults[meal]
		}
		out = append(out, map[string]any{
			"meal": meal, "deadline": dl, "is_default": !overridden,
		})
	}
	httpx.OK(w, out)
}

type updateRulesReq struct {
	Rules []struct {
		Meal     string `json:"meal"`
		Deadline string `json:"deadline"`
	} `json:"rules"`
}

func (s *Server) updateVerifyRules(w http.ResponseWriter, r *http.Request) {
	var req updateRulesReq
	if err := httpx.DecodeJSON(r, &req); err != nil || len(req.Rules) == 0 {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误：rules 不能为空")
		return
	}
	defaults := timewindow.DefaultVerifyDeadlines()
	for _, rule := range req.Rules {
		if !domain.ValidMeal(rule.Meal) {
			httpx.FailWith(w, httpx.CodeBadRequest, "餐次非法："+rule.Meal)
			return
		}
		if !validClock(rule.Deadline) {
			httpx.FailWith(w, httpx.CodeBadRequest, "时间格式错误（HH:MM 或 HH:MM:SS）："+rule.Deadline)
			return
		}
		_ = defaults
		if err := s.St.UpsertVerifyRule(rule.Meal, normalizeClock(rule.Deadline)); err != nil {
			httpx.Fail(w, err)
			return
		}
	}
	httpx.OK(w, map[string]any{"updated": len(req.Rules)})
}

func validClock(s string) bool {
	if len(s) != 5 && len(s) != 8 {
		return false
	}
	for i, c := range s {
		if i == 2 || i == 5 {
			if c != ':' {
				return false
			}
			continue
		}
		if i > 7 {
			return false
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	// 简单范围校验
	hh := int(s[0]-'0')*10 + int(s[1]-'0')
	mm := int(s[3]-'0')*10 + int(s[4]-'0')
	return hh < 24 && mm < 60
}

func normalizeClock(s string) string {
	if len(s) == 5 {
		return s + ":00"
	}
	return s
}
