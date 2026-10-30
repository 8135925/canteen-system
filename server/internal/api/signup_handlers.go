package api

import (
	"net/http"
	"time"

	"canteen/server/internal/auth"
	"canteen/server/internal/domain"
	"canteen/server/internal/httpx"
	"canteen/server/internal/store"
	"canteen/server/internal/timewindow"
)

type signupReq struct {
	MenuDate string `json:"menu_date"`
	Meal     string `json:"meal"`
}

func (s *Server) createSignup(w http.ResponseWriter, r *http.Request) {
	uid := auth.UID(r)
	var req signupReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误")
		return
	}
	if !domain.ValidMeal(req.Meal) {
		httpx.FailWith(w, httpx.CodeBadRequest, "餐次非法")
		return
	}
	now := timewindow.NowShanghai()

	// 校验菜单已发布（存在任意菜品即视为已发布；菜品全部下架不影响报名）
	published, err := s.St.ExistsMenuForMeal(req.MenuDate, req.Meal)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	if err := domain.CanSignup(published, req.MenuDate, now); err != nil {
		httpx.Fail(w, err)
		return
	}

	latest, err := s.St.GetLatestSignup(uid, req.MenuDate, req.Meal)
	if err != nil && err != store.ErrNotFound {
		httpx.Fail(w, err)
		return
	}
	// 修改 = 旧行置 cancelled + 新行 signed（以最后一次提交为准）
	if latest != nil && latest.Status == domain.SignupSigned {
		if err := s.St.CancelLatestSignup(uid, req.MenuDate, req.Meal, now); err != nil {
			httpx.Fail(w, err)
			return
		}
	}
	row, err := s.St.InsertSignup(uid, req.MenuDate, req.Meal, now)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"id": row.ID, "menu_date": row.MenuDate, "meal": row.Meal, "status": row.Status})
}

func (s *Server) cancelSignup(w http.ResponseWriter, r *http.Request) {
	uid := auth.UID(r)
	q := r.URL.Query()
	menuDate, meal := q.Get("menu_date"), q.Get("meal")
	if !domain.ValidMeal(meal) {
		httpx.FailWith(w, httpx.CodeBadRequest, "餐次非法")
		return
	}
	now := timewindow.NowShanghai()
	// 取消与报名同一时间窗：用餐当日 00:00 前
	if err := domain.CanSignup(true, menuDate, now); err != nil {
		httpx.Fail(w, err)
		return
	}
	if err := s.St.CancelLatestSignup(uid, menuDate, meal, now); err != nil {
		if err == store.ErrNotFound {
			httpx.FailWith(w, httpx.CodeNoActiveSignup, "该餐次无报名记录")
			return
		}
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"menu_date": menuDate, "meal": meal, "status": "cancelled"})
}

// viewsFor 组装用餐记录：报餐最新行（推导违约态）+ 未报名就餐记录合并。
func (s *Server) viewsFor(userID int64, from, to string) ([]store.SignupView, error) {
	rows, err := s.St.ListMySignups(userID, from, to)
	if err != nil {
		return nil, err
	}
	deadlines := map[string]string{}
	rules, err := s.St.ListVerifyRules()
	if err == nil {
		for _, r := range rules {
			deadlines[r.Meal] = r.Deadline
		}
	}
	defaults := timewindow.DefaultVerifyDeadlines()
	now := timewindow.NowShanghai()
	var out []store.SignupView
	for _, row := range rows {
		dl, ok := deadlines[row.Meal]
		if !ok {
			dl = defaults[row.Meal]
		}
		st := domain.DeriveEffectiveState(&domain.SignupRow{ID: row.ID, Status: row.Status, SignedAt: row.SignedAt}, row.MenuDate, dl, now)
		out = append(out, store.SignupView{
			MenuDate: row.MenuDate, Meal: row.Meal, Status: st,
			SignedAt: row.SignedAt, CancelledAt: row.CancelledAt,
		})
	}
	walkins, err := s.St.ListWalkInsOfUser(userID, from, to)
	if err != nil {
		return nil, err
	}
	for _, wk := range walkins {
		out = append(out, store.SignupView{
			MenuDate: wk.MenuDate, Meal: wk.Meal, Status: "walk_in",
			SignedAt: wk.RegisteredAt,
		})
	}
	sortViews(out)
	return out, nil
}

func sortViews(v []store.SignupView) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j].MenuDate < v[j-1].MenuDate; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

func defaultRange(r *http.Request) (string, string) {
	q := r.URL.Query()
	from := q.Get("from")
	to := q.Get("to")
	if from == "" {
		from = timewindow.TodayMinusDays(30)
	}
	if to == "" {
		to = timewindow.TodayPlusDays(14)
	}
	return from, to
}

func (s *Server) mySignups(w http.ResponseWriter, r *http.Request) {
	uid := auth.UID(r)
	from, to := defaultRange(r)
	views, err := s.viewsFor(uid, from, to)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"from": from, "to": to, "records": views})
}

func (s *Server) myViolationCount(w http.ResponseWriter, r *http.Request) {
	uid := auth.UID(r)
	from, to := defaultRange(r)
	views, err := s.viewsFor(uid, from, to)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	noShow, walkIn := 0, 0
	for _, v := range views {
		switch v.Status {
		case "no_show":
			noShow++
		case "walk_in":
			walkIn++
		}
	}
	httpx.OK(w, map[string]any{
		"no_show":  noShow,
		"walk_in":  walkIn,
		"total":    domain.CountViolations(noShow, walkIn),
	})
}

var _ = time.Now
