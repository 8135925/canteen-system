// Package memory 内存实现 Store：供单元/集成测试使用（无外部依赖）。
package memory

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"canteen/server/internal/store"
)

type Store struct {
	mu sync.Mutex

	nextID       int64
	users        map[int64]*store.User
	menuItems    map[int64]*store.MenuItem
	verifyRules  map[string]string
	signups      map[int64]*store.Signup
	verifications map[int64]*store.Verification // key: signupID
	walkIns      map[int64]*store.WalkIn
	walkInKey    map[string]int64 // user|date|meal -> walkIn id
	reviews      map[int64]*store.Review
	reviewKey    map[string]int64 // user|date|meal|dish
	suggestions  map[int64]*store.Suggestion
}

func New() *Store {
	return &Store{
		nextID:        1,
		users:         map[int64]*store.User{},
		menuItems:     map[int64]*store.MenuItem{},
		verifyRules:   map[string]string{},
		signups:       map[int64]*store.Signup{},
		verifications: map[int64]*store.Verification{},
		walkIns:       map[int64]*store.WalkIn{},
		walkInKey:     map[string]int64{},
		reviews:       map[int64]*store.Review{},
		reviewKey:     map[string]int64{},
		suggestions:   map[int64]*store.Suggestion{},
	}
}

func (s *Store) id() int64 {
	id := s.nextID
	s.nextID++
	return id
}

func (s *Store) Ping() error { return nil }

// DBNow 内存实现：直接返回当前时间。
func (s *Store) DBNow() (time.Time, error) { return time.Now(), nil }

// —— users ——

func (s *Store) CreateUser(u *store.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.users {
		if x.EmpNo == u.EmpNo {
			return fmt.Errorf("duplicate emp_no")
		}
	}
	u.ID = s.id()
	cp := *u
	s.users[u.ID] = &cp
	return nil
}

func (s *Store) GetUserByID(id int64) (*store.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (s *Store) GetUserByEmpNo(empNo string) (*store.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.EmpNo == empNo {
			cp := *u
			return &cp, nil
		}
	}
	return nil, store.ErrNotFound
}

func (s *Store) UpdateUserProfile(id int64, name, dept string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return store.ErrNotFound
	}
	u.Name = name
	u.Department = dept
	return nil
}

func (s *Store) UpdateUserStatus(id int64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return store.ErrNotFound
	}
	u.Status = status
	return nil
}

func (s *Store) UpdateUserPassword(id int64, hash string, mcp bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return store.ErrNotFound
	}
	u.PasswordHash = hash
	u.MustChangePassword = mcp
	return nil
}

func (s *Store) EmpNoExists(empNo string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.EmpNo == empNo {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) ListUsers(role, keyword string, page, pageSize int) ([]store.User, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.User
	for _, u := range s.users {
		if role != "" && u.Role != role {
			continue
		}
		if role == "" && u.Role != "employee" && u.Role != "canteen_admin" {
			continue // 列表不展示超管
		}
		if keyword != "" && !strings.Contains(u.EmpNo, keyword) && !strings.Contains(u.Name, keyword) {
			continue
		}
		out = append(out, *u)
	}
	total := len(out)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	lo := (page - 1) * pageSize
	if lo > total {
		lo = total
	}
	hi := lo + pageSize
	if hi > total {
		hi = total
	}
	return out[lo:hi], total, nil
}

// —— menu ——

func (s *Store) PublishMenu(items []store.MenuItemInput) ([]store.PublishFailure, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var failures []store.PublishFailure
	for _, it := range items {
		conflict := false
		for _, m := range s.menuItems {
			if m.MenuDate == it.MenuDate && m.Meal == it.Meal && m.DishName == it.DishName {
				conflict = true
				break
			}
		}
		if conflict {
			failures = append(failures, store.PublishFailure{MenuDate: it.MenuDate, Meal: it.Meal, DishName: it.DishName, Reason: "同一日期同一餐次菜品名重复"})
			continue
		}
		id := s.id()
		s.menuItems[id] = &store.MenuItem{
			ID: id, MenuDate: it.MenuDate, Meal: it.Meal, DishName: it.DishName,
			TasteTags: it.TasteTags, Status: "normal",
		}
	}
	return failures, nil
}

func (s *Store) ListMenuItems(from, to string) ([]store.DishWithReviews, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.DishWithReviews
	for _, m := range s.menuItems {
		if m.MenuDate < from || m.MenuDate > to {
			continue
		}
		like, dislike := s.reviewCountsLocked(m.MenuDate, m.Meal, m.DishName)
		out = append(out, store.DishWithReviews{MenuItem: *m, LikeCount: like, DislikeCount: dislike})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MenuDate != out[j].MenuDate {
			return out[i].MenuDate < out[j].MenuDate
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *Store) reviewCountsLocked(date, meal, dish string) (int, int) {
	like, dislike := 0, 0
	for _, r := range s.reviews {
		if r.MenuDate == date && r.Meal == meal && r.DishName == dish {
			if r.Rating == 1 {
				like++
			} else {
				dislike++
			}
		}
	}
	return like, dislike
}

func (s *Store) GetMenuItem(id int64) (*store.MenuItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.menuItems[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *m
	return &cp, nil
}

func (s *Store) SetMenuItemStatus(id int64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.menuItems[id]
	if !ok {
		return store.ErrNotFound
	}
	m.Status = status
	return nil
}

func (s *Store) ExistsMenuForMeal(date, meal string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.menuItems {
		if m.MenuDate == date && m.Meal == meal {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) ListMenuRange(from, to string) ([]store.MenuItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.MenuItem
	for _, m := range s.menuItems {
		if m.MenuDate >= from && m.MenuDate <= to {
			out = append(out, *m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// —— verify rules ——

func (s *Store) ListVerifyRules() ([]store.VerifyRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.VerifyRule
	for _, meal := range []string{"breakfast", "lunch", "dinner"} {
		if dl, ok := s.verifyRules[meal]; ok {
			out = append(out, store.VerifyRule{Meal: meal, Deadline: dl})
		}
	}
	return out, nil
}

func (s *Store) UpsertVerifyRule(meal, deadline string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verifyRules[meal] = deadline
	return nil
}

// —— signups ——

func (s *Store) InsertSignup(userID int64, date, meal string, now time.Time) (*store.Signup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := &store.Signup{ID: s.id(), UserID: userID, MenuDate: date, Meal: meal, Status: "signed", SignedAt: now}
	s.signups[row.ID] = row
	cp := *row
	return &cp, nil
}

func (s *Store) CancelLatestSignup(userID int64, date, meal string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	latest := s.latestLocked(userID, date, meal)
	// 仅已报名（signed）可取消；无记录/已取消/已核销 → not found
	if latest == nil || latest.Status != "signed" {
		return store.ErrNotFound
	}
	latest.Status = "cancelled"
	t := now
	latest.CancelledAt = &t
	return nil
}

func (s *Store) latestLocked(userID int64, date, meal string) *store.Signup {
	var latest *store.Signup
	for _, r := range s.signups {
		if r.UserID == userID && r.MenuDate == date && r.Meal == meal {
			if latest == nil || r.SignedAt.After(latest.SignedAt) ||
				(r.SignedAt.Equal(latest.SignedAt) && r.ID > latest.ID) {
				latest = r
			}
		}
	}
	return latest
}

func (s *Store) GetLatestSignup(userID int64, date, meal string) (*store.Signup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	latest := s.latestLocked(userID, date, meal)
	if latest == nil {
		return nil, store.ErrNotFound
	}
	cp := *latest
	return &cp, nil
}

func (s *Store) ListMySignups(userID int64, from, to string) ([]store.Signup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	type key struct{ date, meal string }
	latestMap := map[key]*store.Signup{}
	for _, r := range s.signups {
		if r.UserID != userID || r.MenuDate < from || r.MenuDate > to {
			continue
		}
		k := key{r.MenuDate, r.Meal}
		if cur, ok := latestMap[k]; !ok || r.SignedAt.After(cur.SignedAt) || (r.SignedAt.Equal(cur.SignedAt) && r.ID > cur.ID) {
			latestMap[k] = r
		}
	}
	var out []store.Signup
	for _, r := range latestMap {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MenuDate != out[j].MenuDate {
			return out[i].MenuDate < out[j].MenuDate
		}
		return out[i].Meal < out[j].Meal
	})
	return out, nil
}

func (s *Store) ListWalkInsOfUser(userID int64, from, to string) ([]store.WalkIn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.WalkIn
	for _, wk := range s.walkIns {
		if wk.UserID == userID && wk.MenuDate >= from && wk.MenuDate <= to {
			out = append(out, *wk)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// —— verify / walk-in ——

func (s *Store) InsertVerification(signupID, verifiedBy int64, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.verifications[signupID]; ok {
		return store.ErrConflict
	}
	row, ok := s.signups[signupID]
	if !ok {
		return store.ErrNotFound
	}
	row.Status = "verified"
	s.verifications[signupID] = &store.Verification{ID: s.id(), SignupID: signupID, VerifiedBy: verifiedBy, VerifiedAt: now}
	return nil
}

func (s *Store) InsertWalkIn(userID int64, date, meal string, by int64, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := fmt.Sprintf("%d|%s|%s", userID, date, meal)
	if _, ok := s.walkInKey[k]; ok {
		return store.ErrConflict
	}
	id := s.id()
	s.walkIns[id] = &store.WalkIn{ID: id, UserID: userID, MenuDate: date, Meal: meal, RegisteredBy: by, RegisteredAt: now}
	s.walkInKey[k] = id
	return nil
}

func (s *Store) HasEaten(userID int64, date, meal string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	latest := s.latestLocked(userID, date, meal)
	if latest != nil && latest.Status == "verified" {
		return true, nil
	}
	if _, ok := s.walkInKey[fmt.Sprintf("%d|%s|%s", userID, date, meal)]; ok {
		return true, nil
	}
	return false, nil
}

// —— reviews ——

func (s *Store) InsertReview(userID int64, date, meal, dish string, rating int, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := fmt.Sprintf("%d|%s|%s|%s", userID, date, meal, dish)
	if _, ok := s.reviewKey[k]; ok {
		return store.ErrConflict
	}
	id := s.id()
	s.reviews[id] = &store.Review{ID: id, UserID: userID, MenuDate: date, Meal: meal, DishName: dish, Rating: rating, CreatedAt: now}
	s.reviewKey[k] = id
	return nil
}

// —— suggestions ——

func (s *Store) InsertSuggestion(userID int64, content string, now time.Time) (*store.Suggestion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := &store.Suggestion{ID: s.id(), UserID: userID, Content: content, Status: "pending", CreatedAt: now}
	s.suggestions[row.ID] = row
	cp := *row
	return &cp, nil
}

func (s *Store) GetSuggestion(id int64) (*store.Suggestion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.suggestions[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *row
	return &cp, nil
}

func (s *Store) ListMySuggestions(userID int64) ([]store.Suggestion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.Suggestion
	for _, r := range s.suggestions {
		if r.UserID == userID {
			out = append(out, *r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (s *Store) ListSuggestions(status string) ([]store.Suggestion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.Suggestion
	for _, r := range s.suggestions {
		if status != "" && r.Status != status {
			continue
		}
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (s *Store) UpdateSuggestionStatus(id int64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.suggestions[id]
	if !ok {
		return store.ErrNotFound
	}
	r.Status = status
	return nil
}

// —— stats ——

func (s *Store) CountSignupsByMeal(date string) ([]store.MealCount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := map[string]int{}
	type key struct{ date, meal string }
	latestMap := map[key]*store.Signup{}
	for _, r := range s.signups {
		if r.MenuDate != date {
			continue
		}
		k := key{r.MenuDate, r.Meal}
		if cur, ok := latestMap[k]; !ok || r.SignedAt.After(cur.SignedAt) || (r.SignedAt.Equal(cur.SignedAt) && r.ID > cur.ID) {
			latestMap[k] = r
		}
	}
	for _, r := range latestMap {
		if r.Status == "signed" || r.Status == "verified" {
			counts[r.Meal]++
		}
	}
	var out []store.MealCount
	for _, m := range []string{"breakfast", "lunch", "dinner"} {
		out = append(out, store.MealCount{Meal: m, Count: counts[m]})
	}
	return out, nil
}

func (s *Store) SignupByDepartment(date string) ([]store.DeptCount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dept := map[int64]string{}
	for _, u := range s.users {
		dept[u.ID] = u.Department
	}
	counts := map[string]int{}
	type key struct{ date, meal string }
	latestMap := map[key]*store.Signup{}
	for _, r := range s.signups {
		if r.MenuDate != date {
			continue
		}
		k := key{r.MenuDate, r.Meal}
		if cur, ok := latestMap[k]; !ok || r.SignedAt.After(cur.SignedAt) || (r.SignedAt.Equal(cur.SignedAt) && r.ID > cur.ID) {
			latestMap[k] = r
		}
	}
	for _, r := range latestMap {
		if r.Status == "signed" || r.Status == "verified" {
			counts[dept[r.UserID]]++
		}
	}
	var out []store.DeptCount
	for d, c := range counts {
		out = append(out, store.DeptCount{Department: d, Count: c})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Department < out[j].Department })
	return out, nil
}

func (s *Store) DailyActual(from, to string) ([]store.DailyActualRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// verified per date
	verified := map[string]int{}
	for _, v := range s.verifications {
		r, ok := s.signups[v.SignupID]
		if !ok || r.MenuDate < from || r.MenuDate > to {
			continue
		}
		verified[r.MenuDate]++
	}
	walkin := map[string]int{}
	for _, w := range s.walkIns {
		if w.MenuDate < from || w.MenuDate > to {
			continue
		}
		walkin[w.MenuDate]++
	}
	// 有效报名（最新行 signed+verified）按日期
	type key struct{ date, meal string }
	latestMap := map[key]*store.Signup{}
	for _, r := range s.signups {
		if r.MenuDate < from || r.MenuDate > to {
			continue
		}
		k := key{r.MenuDate, r.Meal}
		if cur, ok := latestMap[k]; !ok || r.SignedAt.After(cur.SignedAt) || (r.SignedAt.Equal(cur.SignedAt) && r.ID > cur.ID) {
			latestMap[k] = r
		}
	}
	signups := map[string]int{}
	for _, r := range latestMap {
		if r.Status == "signed" || r.Status == "verified" {
			signups[r.MenuDate]++
		}
	}
	dates := map[string]bool{}
	for d := range verified {
		dates[d] = true
	}
	for d := range walkin {
		dates[d] = true
	}
	for d := range signups {
		dates[d] = true
	}
	var out []store.DailyActualRow
	for d := range dates {
		v, wi, sg := verified[d], walkin[d], signups[d]
		out = append(out, store.DailyActualRow{Date: d, Verified: v, WalkIn: wi, Signups: sg, Diff: (v + wi) - sg})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

func (s *Store) ViolationRank(from, to string) ([]store.ViolationRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// no_show：最新行 signed 且 menu_date+deadline < now —— 内存实现以“当前时刻无法知道截止”简化：
	// 由 service 层先用 ViolationRank 计算，这里返回 per-user 的最新行统计原料不现实。
	// 方案：service 层调 ListMySignups？为保持接口简单，这里实现通用逻辑：
	// no_show 数由调用方以日期范围逐用户推导代价过高；改为 SQL 端实现（pg），
	// 内存实现支持测试场景：以“menu_date < 今日”视为已过截止（近似），满足单测断言。
	today := time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02")
	type key struct{ user int64; date, meal string }
	latestMap := map[key]*store.Signup{}
	for _, r := range s.signups {
		if r.MenuDate < from || r.MenuDate > to {
			continue
		}
		k := key{r.UserID, r.MenuDate, r.Meal}
		if cur, ok := latestMap[k]; !ok || r.SignedAt.After(cur.SignedAt) || (r.SignedAt.Equal(cur.SignedAt) && r.ID > cur.ID) {
			latestMap[k] = r
		}
	}
	noShow := map[int64]int{}
	walkIn := map[int64]int{}
	for _, r := range latestMap {
		if r.Status == "signed" && r.MenuDate < today {
			noShow[r.UserID]++
		}
	}
	for _, w := range s.walkIns {
		if w.MenuDate < from || w.MenuDate > to {
			continue
		}
		walkIn[w.UserID]++
	}
	userIDs := map[int64]bool{}
	for uid := range noShow {
		userIDs[uid] = true
	}
	for uid := range walkIn {
		userIDs[uid] = true
	}
	var out []store.ViolationRow
	for uid := range userIDs {
		u, ok := s.users[uid]
		if !ok {
			continue
		}
		out = append(out, store.ViolationRow{
			UserID: uid, EmpNo: u.EmpNo, Name: u.Name, Department: u.Department,
			NoShow: noShow[uid], WalkIn: walkIn[uid], Total: noShow[uid] + walkIn[uid],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Total > out[j].Total })
	return out, nil
}

func (s *Store) DailyTrend(from, to string) ([]store.DailyTrendRow, error) {
	actual, err := s.DailyActual(from, to)
	if err != nil {
		return nil, err
	}
	var out []store.DailyTrendRow
	for _, r := range actual {
		out = append(out, store.DailyTrendRow{Date: r.Date, Dining: r.Verified + r.WalkIn})
	}
	return out, nil
}

// AttendanceDetail 某天用餐明细分页（内存实现，按状态优先级合成）。
func (s *Store) AttendanceDetail(date, dept, keyword string, page, pageSize int) ([]store.AttendanceRow, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prio := map[string]int{"verified": 1, "walk_in": 2, "no_show": 3, "signed": 4, "cancelled": 5}
	// user -> meal -> best st
	best := map[int64]map[string]string{}
	consider := func(uid int64, meal, st string) {
		if best[uid] == nil {
			best[uid] = map[string]string{}
		}
		if cur, ok := best[uid][meal]; !ok || prio[st] < prio[cur] {
			best[uid][meal] = st
		}
	}
	// 报餐状态
	for _, r := range s.signups {
		if r.MenuDate != date {
			continue
		}
		st := r.Status
		if st == "signed" {
			st = "no_show" // 简化：内存测试环境中签名行视为已过截止
		}
		consider(r.UserID, r.Meal, st)
	}
	// walk-in
	for _, w := range s.walkIns {
		if w.MenuDate != date {
			continue
		}
		if cur, ok := best[w.UserID][w.Meal]; !ok || cur == "cancelled" {
			consider(w.UserID, w.Meal, "walk_in")
		}
	}

	var all []*store.User
	for _, u := range s.users {
		if u.Role != "employee" {
			continue
		}
		if dept != "" && u.Department != dept {
			continue
		}
		if keyword != "" && !strings.Contains(u.EmpNo, keyword) && !strings.Contains(u.Name, keyword) {
			continue
		}
		all = append(all, u)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].EmpNo < all[j].EmpNo })
	total := int64(len(all))

	start := (page - 1) * pageSize
	if start > len(all) {
		start = len(all)
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}
	var out []store.AttendanceRow
	for _, u := range all[start:end] {
		row := store.AttendanceRow{
			UserID: u.ID, EmpNo: u.EmpNo, Name: u.Name,
			Department: u.Department, UserStatus: u.Status,
		}
		if m := best[u.ID]; m != nil {
			row.Breakfast = m["breakfast"]
			row.Lunch = m["lunch"]
			row.Dinner = m["dinner"]
		}
		out = append(out, row)
	}
	return out, total, nil
}

// DishSatisfaction 全量赞踩汇总（内存实现）。
func (s *Store) DishSatisfaction() (likeCount, dislikeCount int64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.reviews {
		if r.Rating == 1 {
			likeCount++
		} else {
			dislikeCount++
		}
	}
	return
}

func (s *Store) DishRanking(from, to string) ([]store.DishRankRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	agg := map[string][2]int{}
	for _, r := range s.reviews {
		if r.MenuDate < from || r.MenuDate > to {
			continue
		}
		cur := agg[r.DishName]
		if r.Rating == 1 {
			cur[0]++
		} else {
			cur[1]++
		}
		agg[r.DishName] = cur
	}
	var out []store.DishRankRow
	for dish, c := range agg {
		out = append(out, store.DishRankRow{DishName: dish, LikeCount: c[0], DislikeCount: c[1]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LikeCount > out[j].LikeCount })
	return out, nil
}

func (s *Store) WasteRate(from, to string) ([]store.WasteRateRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	today := time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02")
	type key struct{ date, meal string }
	latestMap := map[key]*store.Signup{}
	for _, r := range s.signups {
		if r.MenuDate < from || r.MenuDate > to {
			continue
		}
		k := key{r.MenuDate, r.Meal}
		if cur, ok := latestMap[k]; !ok || r.SignedAt.After(cur.SignedAt) || (r.SignedAt.Equal(cur.SignedAt) && r.ID > cur.ID) {
			latestMap[k] = r
		}
	}
	noShow, signups := map[string]int{}, map[string]int{}
	for _, r := range latestMap {
		if r.Status == "signed" && r.MenuDate < today {
			noShow[r.MenuDate]++
		}
		if r.Status == "signed" || r.Status == "verified" {
			signups[r.MenuDate]++
		}
	}
	dates := map[string]bool{}
	for d := range signups {
		dates[d] = true
	}
	var out []store.WasteRateRow
	for d := range dates {
		ns, sg := noShow[d], signups[d]
		rate := 0.0
		if sg > 0 {
			rate = float64(ns) / float64(sg)
		}
		out = append(out, store.WasteRateRow{Date: d, NoShow: ns, Signups: sg, WasteRate: rate})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

func (s *Store) DeptDining(from, to string) ([]store.DeptDiningRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dept := map[int64]string{}
	for _, u := range s.users {
		dept[u.ID] = u.Department
	}
	counts := map[string]int{}
	for _, v := range s.verifications {
		r, ok := s.signups[v.SignupID]
		if !ok || r.MenuDate < from || r.MenuDate > to {
			continue
		}
		counts[dept[r.UserID]]++
	}
	for _, w := range s.walkIns {
		if w.MenuDate < from || w.MenuDate > to {
			continue
		}
		counts[dept[w.UserID]]++
	}
	var out []store.DeptDiningRow
	for d, c := range counts {
		out = append(out, store.DeptDiningRow{Department: d, Dining: c})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Department < out[j].Department })
	return out, nil
}
