package api

import (
	"net/http"
	"strconv"

	"canteen/server/internal/auth"
	"canteen/server/internal/domain"
	"canteen/server/internal/httpx"
	"canteen/server/internal/store"
	"canteen/server/internal/timewindow"
)

// —— 公开菜谱查询 ——

func (s *Server) listMenus(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	start := q.Get("start")
	end := q.Get("end")
	if start == "" {
		start = timewindow.TodayDateStr()
	}
	if end == "" {
		end = timewindow.TodayPlusDays(14)
	}
	start, end, err := timewindow.QueryWindow(timewindow.NowShanghai(), start, end)
	if err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "日期范围错误：end 不得早于 start（格式 YYYY-MM-DD）")
		return
	}
	dishes, err := s.St.ListMenuItems(start, end)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"start": start, "end": end, "dishes": dishes})
}

func (s *Server) menuSummary(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = timewindow.TodayDateStr()
	}
	now := timewindow.NowShanghai()
	// 越界返回空集（未来 ≤15 天 / 历史 ≤30 天）
	if !timewindow.InFutureMenuRange(now, date) && !timewindow.InHistoryRange(now, date) {
		httpx.OK(w, map[string]any{"date": date, "dishes": []any{}})
		return
	}
	dishes, err := s.St.ListMenuItems(date, date)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"date": date, "dishes": dishes})
}

// —— 菜谱发布（食堂管理员）——

type dishInput struct {
	DishName  string   `json:"dish_name"`
	TasteTags []string `json:"taste_tags"`
}

type publishMenuReq struct {
	Items []struct {
		MenuDate string      `json:"menu_date"`
		Meal     string      `json:"meal"`
		Dishes   []dishInput `json:"dishes"`
	} `json:"items"`
}

func (s *Server) publishMenu(w http.ResponseWriter, r *http.Request) {
	var req publishMenuReq
	if err := httpx.DecodeJSON(r, &req); err != nil || len(req.Items) == 0 {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误：items 不能为空")
		return
	}
	now := timewindow.NowShanghai()
	var inputs []store.MenuItemInput
	// 日期越界 → 整批拒绝
	for _, it := range req.Items {
		if !domain.ValidMeal(it.Meal) {
			httpx.FailWith(w, httpx.CodeBadRequest, "餐次非法："+it.Meal)
			return
		}
		if !timewindow.InFutureMenuRange(now, it.MenuDate) {
			httpx.FailWith(w, httpx.CodeOutOfRange, "发布范围仅限今日往后 15 天内（含今日），非法日期："+it.MenuDate)
			return
		}
		if len(it.Dishes) == 0 {
			httpx.FailWith(w, httpx.CodeBadRequest, "日期 "+it.MenuDate+" 餐次 "+it.Meal+" 未包含任何菜品")
			return
		}
		for _, d := range it.Dishes {
			if d.DishName == "" {
				httpx.FailWith(w, httpx.CodeBadRequest, "菜品名不能为空")
				return
			}
			inputs = append(inputs, store.MenuItemInput{
				MenuDate: it.MenuDate, Meal: it.Meal, DishName: d.DishName, TasteTags: d.TasteTags,
			})
		}
	}
	failures, err := s.St.PublishMenu(inputs)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{
		"success_count": len(inputs) - len(failures),
		"failures":      failures,
	})
}

func (s *Server) copyLastWeek(w http.ResponseWriter, r *http.Request) {
	// 上周一至周日：本周一 -7 天 至 本周一 -1 天
	now := timewindow.NowShanghai()
	weekday := int(now.Weekday()) // Sunday=0
	if weekday == 0 {
		weekday = 7
	}
	thisMonday := now.AddDate(0, 0, -(weekday - 1))
	from := thisMonday.AddDate(0, 0, -7).Format("2006-01-02")
	to := thisMonday.AddDate(0, 0, -1).Format("2006-01-02")
	items, err := s.St.ListMenuRange(from, to)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	// 按星期几（1=周一…7=周日）+餐次组织，不绑定目标日期
	type dish struct {
		DishName  string   `json:"dish_name"`
		TasteTags []string `json:"taste_tags"`
	}
	week := map[string]map[string][]dish{} // weekday -> meal -> dishes
	for _, it := range items {
		wd := weekdayOf(it.MenuDate)
		if week[wd] == nil {
			week[wd] = map[string][]dish{}
		}
		week[wd][it.Meal] = append(week[wd][it.Meal], dish{DishName: it.DishName, TasteTags: it.TasteTags})
	}
	httpx.OK(w, map[string]any{"from": from, "to": to, "week": week})
}

func weekdayOf(dateStr string) string {
	d, err := timewindow.ParseDate(dateStr)
	if err != nil {
		return "0"
	}
	names := []string{"7", "1", "2", "3", "4", "5", "6"} // Sunday=7
	return names[int(d.Weekday())]
}

func (s *Server) shelfMenuItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "非法的菜品 ID")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误")
		return
	}
	if req.Status != "normal" && req.Status != "off_shelf" {
		httpx.FailWith(w, httpx.CodeBadRequest, "status 只能是 normal 或 off_shelf")
		return
	}
	if _, err := s.St.GetMenuItem(id); err != nil {
		httpx.FailWith(w, httpx.CodeNotFound, "菜品不存在")
		return
	}
	if err := s.St.SetMenuItemStatus(id, req.Status); err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"id": id, "status": req.Status})
}

var _ = auth.UID
