package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"canteen/server/internal/auth"
	"canteen/server/internal/httpx"
	"canteen/server/internal/store"
	memory "canteen/server/internal/store/memory"
	"canteen/server/internal/timewindow"
)

// —— 测试脚手架 ——

type testEnv struct {
	ts *httptest.Server
	st *memory.Store
}

func hashForTest(t *testing.T, plain string) string {
	t.Helper()
	h, err := auth.HashPassword(plain)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func seedUser(st *memory.Store, t *testing.T, empNo, name, dept, role, password string) *store.User {
	t.Helper()
	u := &store.User{
		EmpNo: empNo, Name: name, Department: dept,
		PasswordHash: hashForTest(t, password), Role: role, MustChangePassword: false,
	}
	if err := st.CreateUser(u); err != nil {
		t.Fatal(err)
	}
	return u
}

func seedUserMCP(st *memory.Store, t *testing.T, empNo, name, dept, role, password string) *store.User {
	t.Helper()
	u := &store.User{
		EmpNo: empNo, Name: name, Department: dept,
		PasswordHash: hashForTest(t, password), Role: role, MustChangePassword: true,
	}
	if err := st.CreateUser(u); err != nil {
		t.Fatal(err)
	}
	return u
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	st := memory.New()
	seedUser(st, t, "admin", "admin", "", "super_admin", "admin123")
	seedUser(st, t, "CA01", "食堂管理员", "后勤", "canteen_admin", "ca123456")
	seedUser(st, t, "E001", "张三", "生产部", "employee", "123456")
	seedUser(st, t, "E002", "李四", "质检部", "employee", "123456")

	lookup := auth.NewUserLookup(func(uid int64) (string, string, bool, error) {
		u, err := st.GetUserByID(uid)
		if err != nil {
			return "", "", false, err
		}
		return u.Role, u.Status, u.MustChangePassword, nil
	})
	srv := NewServer(st, "test-secret", lookup, auth.NewLoginGuard())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return &testEnv{ts: ts, st: st}
}

func (e *testEnv) login(t *testing.T, empNo, password string, remember bool) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"emp_no": empNo, "password": password, "remember": remember})
	resp, err := http.Post(e.ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Code string `json:"code"`
		Data *struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.Data == nil {
		t.Fatalf("login failed for %s: code=%s", empNo, out.Code)
	}
	return out.Data.Token
}

func (e *testEnv) do(t *testing.T, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, e.ts.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func codeOf(t *testing.T, out map[string]any) string {
	t.Helper()
	c, _ := out["code"].(string)
	return c
}

func todayStr() string  { return timewindow.TodayDateStr() }
func tomorrow() string  { return timewindow.TodayPlusDays(1) }

// —— 认证 ——

func TestLogin(t *testing.T) {
	e := newTestEnv(t)

	// 正常登录
	code, out := e.do(t, "POST", "/api/v1/auth/login", "", map[string]any{"emp_no": "E001", "password": "123456"})
	if code != 200 || codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("login should ok: %d %v", code, out)
	}
	// 密码错误
	code, out = e.do(t, "POST", "/api/v1/auth/login", "", map[string]any{"emp_no": "E001", "password": "wrong"})
	if codeOf(t, out) != httpx.CodeInvalidCredentials {
		t.Fatalf("expect INVALID_CREDENTIALS: %v", out)
	}
	// 禁用账号
	u, _ := e.st.GetUserByEmpNo("E002")
	_ = e.st.UpdateUserStatus(u.ID, "disabled")
	code, out = e.do(t, "POST", "/api/v1/auth/login", "", map[string]any{"emp_no": "E002", "password": "123456"})
	if codeOf(t, out) != httpx.CodeForbidden {
		t.Fatalf("disabled login expect FORBIDDEN: %v", out)
	}
}

func TestLoginLockout(t *testing.T) {
	e := newTestEnv(t)
	for i := 0; i < 5; i++ {
		e.do(t, "POST", "/api/v1/auth/login", "", map[string]any{"emp_no": "E001", "password": "wrong"})
	}
	code, out := e.do(t, "POST", "/api/v1/auth/login", "", map[string]any{"emp_no": "E001", "password": "123456"})
	if codeOf(t, out) != httpx.CodeLocked {
		t.Fatalf("expect LOCKED after 5 fails: %d %v", code, out)
	}
}

func TestMustChangePasswordEnforcement(t *testing.T) {
	e := newTestEnv(t)
	// 新建一个首登未改密的员工（mcp=true）
	seedUserMCP(e.st, t, "E009", "王五", "生产部", "employee", "123456")
	// 员工首登（mcp=true）：除改密/me 外一律 403 MUST_CHANGE_PASSWORD
	tok := e.login(t, "E009", "123456", false)
	_, out := e.do(t, "GET", "/api/v1/my/signups", tok, nil)
	if codeOf(t, out) != httpx.CodeMustChangePassword {
		t.Fatalf("expect MUST_CHANGE_PASSWORD: %v", out)
	}
	// 改密后放行
	_, out = e.do(t, "PUT", "/api/v1/my/password", tok, map[string]any{"old_password": "123456", "new_password": "newpass123"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("change password should ok: %v", out)
	}
	// 旧 token mcp=true，DB 已 false → 放行
	_, out = e.do(t, "GET", "/api/v1/my/signups", tok, nil)
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("after change password, old token should pass: %v", out)
	}
	// 新密码可登录
	_, out = e.do(t, "POST", "/api/v1/auth/login", "", map[string]any{"emp_no": "E009", "password": "newpass123"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("new password login should ok: %v", out)
	}
	// 错误旧密码
	_, out = e.do(t, "PUT", "/api/v1/my/password", tok, map[string]any{"old_password": "wrong", "new_password": "whatever1"})
	if codeOf(t, out) != httpx.CodeInvalidCredentials {
		t.Fatalf("wrong old password expect INVALID_CREDENTIALS: %v", out)
	}
}

// —— 菜谱发布 + 查询 ——

func TestPublishMenuAndQuery(t *testing.T) {
	e := newTestEnv(t)
	caTok := e.login(t, "CA01", "ca123456", false)

	// 员工无权发布
	empTok := e.login(t, "E001", "123456", false)
	code, out := e.do(t, "POST", "/api/v1/admin/menus", empTok, map[string]any{"items": []any{}})
	if code != 403 {
		t.Fatalf("employee publish should 403: %d %v", code, out)
	}

	// 越界日期整批拒绝
	code, out = e.do(t, "POST", "/api/v1/admin/menus", caTok, map[string]any{"items": []any{
		map[string]any{"menu_date": tomorrow(), "meal": "lunch", "dishes": []any{map[string]any{"dish_name": "红烧肉"}}},
		map[string]any{"menu_date": timewindow.TodayPlusDays(20), "meal": "lunch", "dishes": []any{map[string]any{"dish_name": "白菜"}}},
	}})
	if codeOf(t, out) != httpx.CodeOutOfRange {
		t.Fatalf("out of range publish should reject whole batch: %v", out)
	}

	// 正常发布 + 同名冲突跳过
	code, out = e.do(t, "POST", "/api/v1/admin/menus", caTok, map[string]any{"items": []any{
		map[string]any{"menu_date": tomorrow(), "meal": "lunch", "dishes": []any{
			map[string]any{"dish_name": "红烧肉", "taste_tags": []string{"微辣"}},
			map[string]any{"dish_name": "清炒时蔬"},
		}},
	}})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("publish should ok: %v", out)
	}
	// 重复发布同名 → 冲突明细，其余成功
	code, out = e.do(t, "POST", "/api/v1/admin/menus", caTok, map[string]any{"items": []any{
		map[string]any{"menu_date": tomorrow(), "meal": "lunch", "dishes": []any{
			map[string]any{"dish_name": "红烧肉"},
			map[string]any{"dish_name": "番茄炒蛋"},
		}},
	}})
	data := out["data"].(map[string]any)
	if data["success_count"].(float64) != 1 || len(data["failures"].([]any)) != 1 {
		t.Fatalf("conflict skip: %v", data)
	}

	// 游客查询：明天午餐共 3 个菜（2+1）
	code, out = e.do(t, "GET", "/api/v1/menus?start="+tomorrow()+"&end="+tomorrow(), "", nil)
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("guest query should ok: %v", out)
	}
	dishes := out["data"].(map[string]any)["dishes"].([]any)
	if len(dishes) != 3 {
		t.Fatalf("expect 3 dishes, got %d", len(dishes))
	}

	// 下架
	code, out = e.do(t, "GET", "/api/v1/menus?start="+tomorrow()+"&end="+tomorrow(), "", nil)
	_ = code
	dishList := out["data"].(map[string]any)["dishes"].([]any)
	firstID := int64(dishList[0].(map[string]any)["id"].(float64))
	code, out = e.do(t, "PATCH", fmt.Sprintf("/api/v1/admin/menu-items/%d/shelf", firstID), caTok, map[string]any{"status": "off_shelf"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("shelf should ok: %v", out)
	}
	// 查询含已下架 + status 标识
	_, out = e.do(t, "GET", "/api/v1/menus?start="+tomorrow()+"&end="+tomorrow(), "", nil)
	dishList = out["data"].(map[string]any)["dishes"].([]any)
	foundOff := false
	for _, d := range dishList {
		if d.(map[string]any)["status"] == "off_shelf" {
			foundOff = true
		}
	}
	if !foundOff {
		t.Fatal("off_shelf dish should be listed with status")
	}
}

func TestPublishMenuRequiresPublishedForSignup(t *testing.T) {
	e := newTestEnv(t)
	caTok := e.login(t, "CA01", "ca123456", false)
	empTok := e.login(t, "E001", "123456", false)

	// 未发布不可报名
	_, out := e.do(t, "POST", "/api/v1/signups", empTok, map[string]any{"menu_date": tomorrow(), "meal": "lunch"})
	if codeOf(t, out) != httpx.CodeMenuNotPublished {
		t.Fatalf("expect MENU_NOT_PUBLISHED: %v", out)
	}
	// 发布后可报名
	e.do(t, "POST", "/api/v1/admin/menus", caTok, map[string]any{"items": []any{
		map[string]any{"menu_date": tomorrow(), "meal": "lunch", "dishes": []any{map[string]any{"dish_name": "鱼香肉丝"}}},
	}})
	_, out = e.do(t, "POST", "/api/v1/signups", empTok, map[string]any{"menu_date": tomorrow(), "meal": "lunch"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("signup should ok: %v", out)
	}
	// 当日不可报名（00:00 已过）：先直接插入今日菜单（模拟昨天发布）
	_, err := e.st.PublishMenu([]store.MenuItemInput{{MenuDate: todayStr(), Meal: "lunch", DishName: "宫保鸡丁"}})
	if err != nil {
		t.Fatal(err)
	}
	_, out = e.do(t, "POST", "/api/v1/signups", empTok, map[string]any{"menu_date": todayStr(), "meal": "lunch"})
	if codeOf(t, out) != httpx.CodeSignupClosed {
		t.Fatalf("today signup expect SIGNUP_CLOSED: %v", out)
	}
	// 重复提交（修改）= 取消旧行 + 新行
	e.do(t, "POST", "/api/v1/signups", empTok, map[string]any{"menu_date": tomorrow(), "meal": "lunch"})
	_, out = e.do(t, "GET", "/api/v1/my/signups", empTok, nil)
	recs := out["data"].(map[string]any)["records"].([]any)
	if len(recs) != 1 || recs[0].(map[string]any)["status"] != "signed" {
		t.Fatalf("modify should keep one signed latest row: %v", recs)
	}
	// 取消
	_, out = e.do(t, "DELETE", "/api/v1/signups?menu_date="+tomorrow()+"&meal=lunch", empTok, nil)
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("cancel should ok: %v", out)
	}
	_, out = e.do(t, "GET", "/api/v1/my/signups", empTok, nil)
	recs = out["data"].(map[string]any)["records"].([]any)
	if recs[0].(map[string]any)["status"] != "cancelled" {
		t.Fatalf("after cancel status should be cancelled: %v", recs)
	}
	// 重复取消 → NO_ACTIVE_SIGNUP
	_, out = e.do(t, "DELETE", "/api/v1/signups?menu_date="+tomorrow()+"&meal=lunch", empTok, nil)
	if codeOf(t, out) != httpx.CodeNoActiveSignup {
		t.Fatalf("double cancel expect NO_ACTIVE_SIGNUP: %v", out)
	}
	// 取消后重新报名
	_, out = e.do(t, "POST", "/api/v1/signups", empTok, map[string]any{"menu_date": tomorrow(), "meal": "lunch"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("resign after cancel should ok: %v", out)
	}
}

// —— 核销 / 未报名登记 全分支矩阵 ——

// setupTodayMenu 直接经 Store 插入今日菜单（模拟"昨天发布"——发布范围校验在 handler 层）。
func setupTodayMenu(t *testing.T, e *testEnv) {
	t.Helper()
	_, err := e.st.PublishMenu([]store.MenuItemInput{
		{MenuDate: todayStr(), Meal: "breakfast", DishName: "包子"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

// seedSignup 直接经 Store 插入报名行（模拟"昨天报名"——报名窗口校验在 handler 层）。
func seedSignup(t *testing.T, e *testEnv, empNo, date, meal string) {
	t.Helper()
	u, err := e.st.GetUserByEmpNo(empNo)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.st.InsertSignup(u.ID, date, meal, time.Now())
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerifyMatrix(t *testing.T) {
	e := newTestEnv(t)
	setupTodayMenu(t, e)
	caTok := e.login(t, "CA01", "ca123456", false)

	// 将早餐截止覆盖为 23:59:59 → 当前可核销
	e.do(t, "PUT", "/api/v1/admin/verify-rules", caTok, map[string]any{"rules": []any{
		map[string]any{"meal": "breakfast", "deadline": "23:59:59"},
	}})

	// lookup：有报名（E001 昨天报了今天的餐，模拟）
	seedSignup(t, e, "E001", todayStr(), "breakfast")
	code, out := e.do(t, "GET", "/api/v1/admin/verify/lookup?emp_no=E001&menu_date="+todayStr()+"&meal=breakfast", caTok, nil)
	if code != 200 || codeOf(t, out) != httpx.CodeOK || out["data"].(map[string]any)["action"] != "verify" {
		t.Fatalf("lookup should say verify: %v", out)
	}

	// 核销成功
	code, out = e.do(t, "POST", "/api/v1/admin/verify", caTok, map[string]any{"emp_no": "E001", "menu_date": todayStr(), "meal": "breakfast"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("verify should ok: %v", out)
	}
	// 重复核销
	_, out = e.do(t, "POST", "/api/v1/admin/verify", caTok, map[string]any{"emp_no": "E001", "menu_date": todayStr(), "meal": "breakfast"})
	if codeOf(t, out) != httpx.CodeAlreadyVerified {
		t.Fatalf("double verify expect ALREADY_VERIFIED: %v", out)
	}

	// 分支：未报名员工 → walk_in 登记（截止内）
	code, out = e.do(t, "POST", "/api/v1/admin/walk-in", caTok, map[string]any{"emp_no": "E002", "menu_date": todayStr(), "meal": "breakfast"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("walk-in should ok: %v", out)
	}
	// 重复登记
	_, out = e.do(t, "POST", "/api/v1/admin/walk-in", caTok, map[string]any{"emp_no": "E002", "menu_date": todayStr(), "meal": "breakfast"})
	if codeOf(t, out) != httpx.CodeAlreadyWalkIn {
		t.Fatalf("double walk-in expect ALREADY_WALK_IN: %v", out)
	}
	// 已报名未核销走 walk-in → HAS_ACTIVE_SIGNUP（互斥）：E003 昨天报名
	seedUser(e.st, t, "E003", "孙七", "生产部", "employee", "123456")
	seedSignup(t, e, "E003", todayStr(), "breakfast")
	_, out = e.do(t, "POST", "/api/v1/admin/walk-in", caTok, map[string]any{"emp_no": "E003", "menu_date": todayStr(), "meal": "breakfast"})
	if codeOf(t, out) != httpx.CodeHasActiveSignup {
		t.Fatalf("walk-in with active signup expect HAS_ACTIVE_SIGNUP: %v", out)
	}

	// 分支：signed 已过截止 → 核销与登记双拒（防双重违约）：E003 仍是 signed
	// 截止覆盖回 00:00:00 → 已过截止
	e.do(t, "PUT", "/api/v1/admin/verify-rules", caTok, map[string]any{"rules": []any{
		map[string]any{"meal": "breakfast", "deadline": "00:00:00"},
	}})
	_, out = e.do(t, "POST", "/api/v1/admin/verify", caTok, map[string]any{"emp_no": "E003", "menu_date": todayStr(), "meal": "breakfast"})
	if codeOf(t, out) != httpx.CodeAlreadyViolated {
		t.Fatalf("expired verify expect ALREADY_VIOLATED: %v", out)
	}
	_, out = e.do(t, "POST", "/api/v1/admin/walk-in", caTok, map[string]any{"emp_no": "E003", "menu_date": todayStr(), "meal": "breakfast"})
	if codeOf(t, out) != httpx.CodeAlreadyViolated {
		t.Fatalf("expired walk-in for signed-user expect ALREADY_VIOLATED: %v", out)
	}

	// 防提前核销未来餐次
	e.do(t, "PUT", "/api/v1/admin/verify-rules", caTok, map[string]any{"rules": []any{
		map[string]any{"meal": "breakfast", "deadline": "23:59:59"},
	}})
	_, out = e.do(t, "POST", "/api/v1/admin/verify", caTok, map[string]any{"emp_no": "E001", "menu_date": tomorrow(), "meal": "breakfast"})
	if codeOf(t, out) != httpx.CodeOutOfRange {
		t.Fatalf("future date verify expect DATE_OUT_OF_RANGE: %v", out)
	}
}

func TestViolationDerivation(t *testing.T) {
	e := newTestEnv(t)
	caTok := e.login(t, "CA01", "ca123456", false)

	// E001 昨天报了早餐未核销 → 推导违约（截止已过）。菜单与报名均模拟"昨天"。
	_, err := e.st.PublishMenu([]store.MenuItemInput{{MenuDate: timewindow.TodayMinusDays(1), Meal: "breakfast", DishName: "油条"}})
	if err != nil {
		t.Fatal(err)
	}
	seedSignup(t, e, "E001", timewindow.TodayMinusDays(1), "breakfast")
	// E002 今天 walk-in（未报名就餐 → 违约；登记不限时点，仅需当日）
	e.do(t, "PUT", "/api/v1/admin/verify-rules", caTok, map[string]any{"rules": []any{
		map[string]any{"meal": "breakfast", "deadline": "23:59:59"},
	}})
	e.do(t, "POST", "/api/v1/admin/walk-in", caTok, map[string]any{"emp_no": "E002", "menu_date": todayStr(), "meal": "breakfast"})

	empTok := e.login(t, "E001", "123456", false)
	_, out := e.do(t, "GET", "/api/v1/my/violation-count", empTok, nil)
	data := out["data"].(map[string]any)
	if data["no_show"].(float64) != 1 || data["total"].(float64) != 1 {
		t.Fatalf("E001 violation expect no_show=1 total=1: %v", data)
	}
	// E002 违约 = walk_in 1
	emp2Tok := e.login(t, "E002", "123456", false)
	_, out = e.do(t, "GET", "/api/v1/my/violation-count", emp2Tok, nil)
	data = out["data"].(map[string]any)
	if data["walk_in"].(float64) != 1 || data["total"].(float64) != 1 {
		t.Fatalf("E002 violation expect walk_in=1 total=1: %v", data)
	}

	// 超管违约名单
	superTok := e.login(t, "admin", "admin123", false)
	_, out = e.do(t, "GET", "/api/v1/super/violations", superTok, nil)
	violations := out["data"].(map[string]any)["violations"].([]any)
	found := false
	for _, v := range violations {
		vv := v.(map[string]any)
		if vv["emp_no"] == "E001" && vv["no_show"].(float64) == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("E001 should appear in violations: %v", violations)
	}
}

// —— 点评校验链 ——

func TestReviewChain(t *testing.T) {
	e := newTestEnv(t)
	setupTodayMenu(t, e)
	caTok := e.login(t, "CA01", "ca123456", false)
	e.do(t, "PUT", "/api/v1/admin/verify-rules", caTok, map[string]any{"rules": []any{
		map[string]any{"meal": "breakfast", "deadline": "23:59:59"},
	}})
	empTok := e.login(t, "E001", "123456", false)

	// 未就餐不可点评
	_, out := e.do(t, "POST", "/api/v1/reviews", empTok, map[string]any{"menu_date": todayStr(), "meal": "breakfast", "dish_name": "包子", "rating": 1})
	if codeOf(t, out) != httpx.CodeForbidden {
		t.Fatalf("not eaten expect FORBIDDEN: %v", out)
	}

	// 核销后可点评：E001 昨天报了今天的早餐（模拟）
	seedSignup(t, e, "E001", todayStr(), "breakfast")
	e.do(t, "POST", "/api/v1/admin/verify", caTok, map[string]any{"emp_no": "E001", "menu_date": todayStr(), "meal": "breakfast"})
	_, out = e.do(t, "POST", "/api/v1/reviews", empTok, map[string]any{"menu_date": todayStr(), "meal": "breakfast", "dish_name": "包子", "rating": 1})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("review after verify should ok: %v", out)
	}
	// 一人一餐一菜一次
	_, out = e.do(t, "POST", "/api/v1/reviews", empTok, map[string]any{"menu_date": todayStr(), "meal": "breakfast", "dish_name": "包子", "rating": -1})
	if codeOf(t, out) != httpx.CodeConflict {
		t.Fatalf("double review expect CONFLICT: %v", out)
	}

	// walk-in 员工可点评
	emp2Tok := e.login(t, "E002", "123456", false)
	e.do(t, "POST", "/api/v1/admin/walk-in", caTok, map[string]any{"emp_no": "E002", "menu_date": todayStr(), "meal": "breakfast"})
	_, out = e.do(t, "POST", "/api/v1/reviews", emp2Tok, map[string]any{"menu_date": todayStr(), "meal": "breakfast", "dish_name": "包子", "rating": -1})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("walk-in review should ok: %v", out)
	}

	// 下架菜品不可点评
	dishes, _ := e.st.ListMenuItems(todayStr(), todayStr())
	_ = e.st.SetMenuItemStatus(dishes[0].ID, "off_shelf")
	_, out = e.do(t, "POST", "/api/v1/reviews", empTok, map[string]any{"menu_date": todayStr(), "meal": "breakfast", "dish_name": "包子", "rating": 1})
	if codeOf(t, out) != httpx.CodeDishOffShelf {
		t.Fatalf("off shelf review expect DISH_OFF_SHELF: %v", out)
	}
	_ = e.st.SetMenuItemStatus(dishes[0].ID, "normal")

	// 跨日不可点评
	_, out = e.do(t, "POST", "/api/v1/reviews", empTok, map[string]any{"menu_date": timewindow.TodayMinusDays(1), "meal": "breakfast", "dish_name": "油条", "rating": 1})
	if codeOf(t, out) != httpx.CodeOutOfRange {
		t.Fatalf("cross-day review expect DATE_OUT_OF_RANGE: %v", out)
	}

	// 赞踩数合并展示
	_, out = e.do(t, "GET", "/api/v1/menus?start="+todayStr()+"&end="+todayStr(), "", nil)
	dishesOut := out["data"].(map[string]any)["dishes"].([]any)
	for _, d := range dishesOut {
		dd := d.(map[string]any)
		if dd["dish_name"] == "包子" {
			if dd["like_count"].(float64) != 1 || dd["dislike_count"].(float64) != 1 {
				t.Fatalf("review counts wrong: %v", dd)
			}
		}
	}
}

// —— 建议 ——

func TestSuggestions(t *testing.T) {
	e := newTestEnv(t)
	empTok := e.login(t, "E001", "123456", false)
	caTok := e.login(t, "CA01", "ca123456", false)

	_, out := e.do(t, "POST", "/api/v1/suggestions", empTok, map[string]any{"content": "想吃锅包肉"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("create suggestion should ok: %v", out)
	}
	// 空内容
	_, out = e.do(t, "POST", "/api/v1/suggestions", empTok, map[string]any{"content": "  "})
	if codeOf(t, out) != httpx.CodeBadRequest {
		t.Fatalf("empty content expect BAD_REQUEST: %v", out)
	}
	// 我的建议
	_, out = e.do(t, "GET", "/api/v1/my/suggestions", empTok, nil)
	list := out["data"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["status"] != "pending" {
		t.Fatalf("my suggestions: %v", list)
	}
	// 管理员标记状态
	id := list[0].(map[string]any)["id"].(float64)
	_, out = e.do(t, "PATCH", fmt.Sprintf("/api/v1/admin/suggestions/%d", int64(id)), caTok, map[string]any{"status": "adopted"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("mark status should ok: %v", out)
	}
	// 非法状态
	_, out = e.do(t, "PATCH", fmt.Sprintf("/api/v1/admin/suggestions/%d", int64(id)), caTok, map[string]any{"status": "bad"})
	if codeOf(t, out) != httpx.CodeBadRequest {
		t.Fatalf("bad status expect BAD_REQUEST: %v", out)
	}
}

// —— 超管账号管理 ——

func TestSuperAdminUserManagement(t *testing.T) {
	e := newTestEnv(t)
	superTok := e.login(t, "admin", "admin123", false)

	// 创建食堂管理员
	_, out := e.do(t, "POST", "/api/v1/super/canteen-admins", superTok, map[string]any{"emp_no": "CA02", "name": "王管", "department": "后勤"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("create canteen admin should ok: %v", out)
	}
	// 重复工号
	_, out = e.do(t, "POST", "/api/v1/super/canteen-admins", superTok, map[string]any{"emp_no": "CA02", "name": "重复"})
	if codeOf(t, out) != httpx.CodeEmpNoExists {
		t.Fatalf("duplicate emp_no expect EMP_NO_EXISTS: %v", out)
	}

	// 禁用员工：先在未禁用时拿 token，再禁用
	e002, _ := e.st.GetUserByEmpNo("E002")
	emp2Tok := e.login(t, "E002", "123456", false)
	_, out = e.do(t, "PATCH", fmt.Sprintf("/api/v1/super/users/%d/status", e002.ID), superTok, map[string]any{"status": "disabled"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("disable should ok: %v", out)
	}
	// 被禁用账号：登录被拒；在线旧 token 请求 401
	_, out = e.do(t, "POST", "/api/v1/auth/login", "", map[string]any{"emp_no": "E002", "password": "123456"})
	if codeOf(t, out) != httpx.CodeForbidden {
		t.Fatalf("disabled login expect FORBIDDEN: %v", out)
	}
	_, out = e.do(t, "GET", "/api/v1/my/signups", emp2Tok, nil)
	if codeOf(t, out) != httpx.CodeUnauthorized {
		t.Fatalf("disabled token expect UNAUTHORIZED: %v", out)
	}
	// 启用后可登录
	_, out = e.do(t, "PATCH", fmt.Sprintf("/api/v1/super/users/%d/status", e002.ID), superTok, map[string]any{"status": "active"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("enable should ok: %v", out)
	}
	_, out = e.do(t, "POST", "/api/v1/auth/login", "", map[string]any{"emp_no": "E002", "password": "123456"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("enabled login should ok: %v", out)
	}

	// 禁止操作超管
	adminUser, _ := e.st.GetUserByEmpNo("admin")
	_, out = e.do(t, "PATCH", fmt.Sprintf("/api/v1/super/users/%d/status", adminUser.ID), superTok, map[string]any{"status": "disabled"})
	if codeOf(t, out) != httpx.CodeForbidden {
		t.Fatalf("disable super admin expect FORBIDDEN: %v", out)
	}
	// 禁止重置超管密码
	_, out = e.do(t, "POST", fmt.Sprintf("/api/v1/super/users/%d/reset-password", adminUser.ID), superTok, nil)
	if codeOf(t, out) != httpx.CodeForbidden {
		t.Fatalf("reset super admin password expect FORBIDDEN: %v", out)
	}

	// 重置员工密码 → 123456 + 强制改密
	e001, _ := e.st.GetUserByEmpNo("E001")
	_, out = e.do(t, "POST", fmt.Sprintf("/api/v1/super/users/%d/reset-password", e001.ID), superTok, nil)
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("reset password should ok: %v", out)
	}
	_, out = e.do(t, "POST", "/api/v1/auth/login", "", map[string]any{"emp_no": "E001", "password": "123456"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("login with reset password should ok: %v", out)
	}
	tok := out["data"].(map[string]any)["token"].(string)
	_, out = e.do(t, "GET", "/api/v1/my/signups", tok, nil)
	if codeOf(t, out) != httpx.CodeMustChangePassword {
		t.Fatalf("after reset expect MUST_CHANGE_PASSWORD: %v", out)
	}
}

// —— 统计 ——

func TestStats(t *testing.T) {
	e := newTestEnv(t)
	setupTodayMenu(t, e)
	caTok := e.login(t, "CA01", "ca123456", false)

	e.do(t, "PUT", "/api/v1/admin/verify-rules", caTok, map[string]any{"rules": []any{
		map[string]any{"meal": "breakfast", "deadline": "23:59:59"},
	}})
	seedSignup(t, e, "E001", todayStr(), "breakfast")

	// 报餐人数
	_, out := e.do(t, "GET", "/api/v1/admin/stats/signups?date="+todayStr(), caTok, nil)
	signups := out["data"].(map[string]any)["signups"].([]any)
	found := false
	for _, s := range signups {
		if s.(map[string]any)["meal"] == "breakfast" && s.(map[string]any)["count"].(float64) == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("breakfast signup count should be 1: %v", signups)
	}

	// 实际就餐 vs 报名差异
	e.do(t, "POST", "/api/v1/admin/verify", caTok, map[string]any{"emp_no": "E001", "menu_date": todayStr(), "meal": "breakfast"})
	e.do(t, "POST", "/api/v1/admin/walk-in", caTok, map[string]any{"emp_no": "E002", "menu_date": todayStr(), "meal": "breakfast"})
	_, out = e.do(t, "GET", "/api/v1/admin/stats/daily?from="+todayStr()+"&to="+todayStr(), caTok, nil)
	daily := out["data"].(map[string]any)["daily"].([]any)
	row := daily[0].(map[string]any)
	if row["verified"].(float64) != 1 || row["walk_in"].(float64) != 1 || row["signups"].(float64) != 1 || row["diff"].(float64) != 1 {
		t.Fatalf("daily actual wrong: %v", row)
	}

	// 部门分布
	_, out = e.do(t, "GET", "/api/v1/super/dashboard/departments", e.login(t, "admin", "admin123", false), nil)
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("dept dashboard should ok: %v", out)
	}
}

// —— 员工导入 ——

func TestEmployeeImport(t *testing.T) {
	e := newTestEnv(t)
	caTok := e.login(t, "CA01", "ca123456", false)

	csvContent := "工号,姓名,部门\nE100,赵六,生产部\n,E007,空工号\nE100,重复,生产部\nE101,钱七,\n"
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "employees.csv")
	part.Write([]byte(csvContent))
	writer.Close()

	req, _ := http.NewRequest("POST", e.ts.URL+"/api/v1/admin/employees/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+caTok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	data := out["data"].(map[string]any)
	if data["success_count"].(float64) != 2 {
		t.Fatalf("expect 2 success: %v", data)
	}
	failures := data["failures"].([]any)
	if len(failures) != 2 {
		t.Fatalf("expect 2 failures: %v", failures)
	}
	// 导入的员工可用默认密码登录
	_, out = e.do(t, "POST", "/api/v1/auth/login", "", map[string]any{"emp_no": "E100", "password": "123456"})
	if codeOf(t, out) != httpx.CodeOK {
		t.Fatalf("imported employee login should ok: %v", out)
	}
}

// —— 静态资源占位与时间 —— 

func TestHelperClocks(t *testing.T) {
	if !validClock("09:00") || !validClock("09:00:00") || validClock("24:00") || validClock("abc") {
		t.Fatal("validClock wrong")
	}
	if normalizeClock("09:00") != "09:00:00" {
		t.Fatal("normalizeClock wrong")
	}
	if weekdayOf("2026-10-26") != "1" { // 周一
		t.Fatal("weekdayOf wrong")
	}
	if weekdayOf("2026-11-01") != "7" { // 周日
		t.Fatal("weekdayOf wrong")
	}
	_ = time.Now
	_ = strings.TrimSpace
}
