package api

import (
	"net/http"
	"strconv"
	"strings"

	"canteen/server/internal/auth"
	"canteen/server/internal/domain"
	"canteen/server/internal/httpx"
	"canteen/server/internal/store"
	"canteen/server/internal/timewindow"
)

// —— 菜品点评 ——

type reviewReq struct {
	MenuDate string `json:"menu_date"`
	Meal     string `json:"meal"`
	DishName string `json:"dish_name"`
	Rating   int    `json:"rating"`
}

func (s *Server) createReview(w http.ResponseWriter, r *http.Request) {
	uid := auth.UID(r)
	var req reviewReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误")
		return
	}
	if !domain.ValidMeal(req.Meal) || req.DishName == "" || (req.Rating != 1 && req.Rating != -1) {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误：meal/dish_name/rating(1 或 -1)")
		return
	}
	now := timewindow.NowShanghai()

	// ② 该员工该餐已核销或已登记未报名就餐
	hasEaten, err := s.St.HasEaten(uid, req.MenuDate, req.Meal)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	// ③ 目标菜品状态
	dishes, err := s.St.ListMenuItems(req.MenuDate, req.MenuDate)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	dishStatus := ""
	for _, d := range dishes {
		if d.DishName == req.DishName {
			dishStatus = d.Status
			break
		}
	}
	// 校验链 ①②③
	if err := domain.ReviewChainCheck(req.MenuDate, now, hasEaten, dishStatus); err != nil {
		httpx.Fail(w, err)
		return
	}
	// 菜品不存在（当日该餐次无此菜）
	if dishStatus == "" {
		httpx.FailWith(w, httpx.CodeNotFound, "当日该餐次不存在此菜品")
		return
	}
	if err := s.St.InsertReview(uid, req.MenuDate, req.Meal, req.DishName, req.Rating, now); err != nil {
		if err == store.ErrConflict {
			httpx.FailWith(w, httpx.CodeConflict, "该菜品已评价过（一人一餐一菜一次）")
			return
		}
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"dish_name": req.DishName, "rating": req.Rating})
}

// —— 建议 ——

type suggestionReq struct {
	Content string `json:"content"`
}

func (s *Server) createSuggestion(w http.ResponseWriter, r *http.Request) {
	uid := auth.UID(r)
	var req suggestionReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误")
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		httpx.FailWith(w, httpx.CodeBadRequest, "建议内容不能为空")
		return
	}
	if len(content) > 500 {
		httpx.FailWith(w, httpx.CodeBadRequest, "建议内容不能超过 500 字")
		return
	}
	row, err := s.St.InsertSuggestion(uid, content, timewindow.NowShanghai())
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, row)
}

func (s *Server) mySuggestions(w http.ResponseWriter, r *http.Request) {
	rows, err := s.St.ListMySuggestions(auth.UID(r))
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, rows)
}

func (s *Server) listSuggestions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != "pending" && status != "adopted" && status != "rejected" {
		httpx.FailWith(w, httpx.CodeBadRequest, "status 只能是 pending/adopted/rejected")
		return
	}
	rows, err := s.St.ListSuggestions(status)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"suggestions": rows})
}

func (s *Server) updateSuggestion(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "非法的建议 ID")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误")
		return
	}
	if req.Status != "pending" && req.Status != "adopted" && req.Status != "rejected" {
		httpx.FailWith(w, httpx.CodeBadRequest, "status 只能是 pending/adopted/rejected")
		return
	}
	if _, err := s.St.GetSuggestion(id); err != nil {
		httpx.FailWith(w, httpx.CodeNotFound, "建议不存在")
		return
	}
	if err := s.St.UpdateSuggestionStatus(id, req.Status); err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"id": id, "status": req.Status})
}
