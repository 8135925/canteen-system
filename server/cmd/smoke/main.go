// 端到端冒烟客户端（幂等版）：可对同一服务重复运行。
// 用法：先启动服务（连接真实 DB），再 SMOKE_BASE=http://localhost:8081/api/v1 go run ./cmd/smoke
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	base   = envDefault("SMOKE_BASE", "http://localhost:8080/api/v1")
	client = &http.Client{Timeout: 30 * time.Second}
	passN  = 0
	failN  = 0
)

func envDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

var lastBody string

func call(method, path, token string, body any) (int, map[string]any) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, base+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		lastBody = err.Error()
		return 0, map[string]any{"_err": err.Error()}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	lastBody = string(raw)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func code(m map[string]any) string {
	s, _ := m["code"].(string)
	return s
}

func data(m map[string]any) map[string]any {
	d, _ := m["data"].(map[string]any)
	return d
}

func assert(name string, cond bool) {
	if cond {
		fmt.Printf("PASS  %s\n", name)
		passN++
	} else {
		fmt.Printf("FAIL  %s  last_resp=%s\n", name, lastBody)
		failN++
	}
}

func todayStr() string   { return time.Now().Format("2006-01-02") }
func tomorrowStr() string { return time.Now().AddDate(0, 0, 1).Format("2006-01-02") }

// ensureUser 幂等获取用户 token：尝试创建（可失败）→ 尝试多套密码登录 → mcp 时改密。
func ensureUser(empNo, name, dept, role string, passwords []string) string {
	// 创建（幂等：已存在则失败忽略）
	call("POST", roleToCreatePath(role), tryAdminToken(), map[string]any{"emp_no": empNo, "name": name, "department": dept})
	// 登录（逐套密码尝试）
	var token string
	for _, pwd := range passwords {
		_, m := call("POST", "/auth/login", "", map[string]any{"emp_no": empNo, "password": pwd})
		if code(m) == "OK" {
			token, _ = data(m)["token"].(string)
			break
		}
	}
	if token == "" {
		return ""
	}
	// mcp → 改密到第一套密码
	_, m := call("GET", "/me", token, nil)
	if data(m) != nil && data(m)["must_change_password"] == true {
		_, m = call("PUT", "/my/password", token, map[string]any{"old_password": passwords[0], "new_password": passwords[0]})
		token, _ = data(m)["token"].(string)
	}
	return token
}

func tryAdminToken() string {
	for _, pwd := range []string{"admin123", "Admin2026!"} {
		_, m := call("POST", "/auth/login", "", map[string]any{"emp_no": "admin", "password": pwd})
		if code(m) == "OK" {
			tok, _ := data(m)["token"].(string)
			_, m = call("GET", "/me", tok, nil)
			if data(m) != nil && data(m)["must_change_password"] == true {
				_, m = call("PUT", "/my/password", tok, map[string]any{"old_password": pwd, "new_password": "Admin2026!"})
				tok, _ = data(m)["token"].(string)
			}
			return tok
		}
	}
	return ""
}

func roleToCreatePath(role string) string {
	if role == "canteen_admin" {
		return "/super/canteen-admins"
	}
	return "/admin/employees"
}

func main() {
	// 1. 健康检查
	sc, m := call("GET", "/healthz", "", nil)
	assert("健康检查", sc == 200 && data(m)["status"] == "up")

	// 2. 超管登录 + mcp 链路
	adminTok := tryAdminToken()
	assert("超管登录", adminTok != "")
	_, m = call("GET", "/me", adminTok, nil)
	assert("超管 mcp=false", data(m) != nil && data(m)["must_change_password"] == false)

	// 3. 食堂管理员（幂等）
	caTok := ensureUser("CA_SMOKE", "冒烟管理员", "后勤", "canteen_admin", []string{"CaSmoke2026!", "123456"})
	assert("管理员就绪", caTok != "")

	// 4. 员工（幂等）
	empTok := ensureUser("E_SMOKE", "冒烟员工", "生产部", "employee", []string{"EmpSmoke2026!", "123456"})
	assert("员工就绪", empTok != "")

	// 5. 发布明天午餐菜单（幂等：重复发布走冲突明细）
	tomorrow := tomorrowStr()
	sc, m = call("POST", "/admin/menus", caTok, map[string]any{"items": []any{
		map[string]any{"menu_date": tomorrow, "meal": "lunch", "dishes": []any{
			map[string]any{"dish_name": "红烧排骨", "taste_tags": []string{"微辣"}},
			map[string]any{"dish_name": "清炒油麦菜"},
		}},
	}})
	assert("发布菜谱（明天午餐）", code(m) == "OK")

	// 6. 游客查询
	_, m = call("GET", "/menus?start="+tomorrow+"&end="+tomorrow, "", nil)
	dishes, _ := data(m)["dishes"].([]any)
	assert("游客查询菜谱（≥2 道）", len(dishes) >= 2)

	// 7. 员工报名（幂等：确保最新行为 signed）
	_, m = call("GET", "/my/signups?from="+tomorrow+"&to="+tomorrow, empTok, nil)
	already := false
	for _, r := range toSlice(data(m)["records"]) {
		rv := r.(map[string]any)
		if rv["meal"] == "lunch" && rv["status"] == "signed" {
			already = true
		}
	}
	if !already {
		_, m = call("POST", "/signups", empTok, map[string]any{"menu_date": tomorrow, "meal": "lunch"})
	}
	assert("员工已报名（signed）", already || code(m) == "OK")

	// 8. 当日报名被拒
	sc, _ = call("POST", "/signups", empTok, map[string]any{"menu_date": todayStr(), "meal": "lunch"})
	assert("当日报名被拒(00:00 截止)", sc == 403)

	// 9. 防提前核销未来餐次（lookup 返回 200 + action=violated）
	sc, m = call("GET", "/admin/verify/lookup?emp_no=E_SMOKE&menu_date="+tomorrow+"&meal=lunch", caTok, nil)
	assert("防提前核销未来餐次(violated)", sc == 200 && data(m) != nil && data(m)["action"] == "violated")

	// 10. 报名统计
	_, m = call("GET", "/admin/stats/signups?date="+tomorrow, caTok, nil)
	lunchOK := false
	for _, s := range toSlice(data(m)["signups"]) {
		sv := s.(map[string]any)
		if sv["meal"] == "lunch" && sv["count"].(float64) >= 1 {
			lunchOK = true
		}
	}
	assert("报名统计（午餐 ≥1）", lunchOK)

	// 11. 未就餐不可点评（今日日期 → 校验链②未就餐 → 403）
	sc, _ = call("POST", "/reviews", empTok, map[string]any{"menu_date": todayStr(), "meal": "lunch", "dish_name": "宫保鸡丁", "rating": 1})
	assert("未就餐不可点评(403)", sc == 403)

	// 12. 建议（幂等）
	_, m = call("GET", "/admin/suggestions", caTok, nil)
	var sugID float64
	for _, s := range toSlice(data(m)["suggestions"]) {
		sv := s.(map[string]any)
		if c, _ := sv["content"].(string); strings.Contains(c, "冒烟") {
			sugID = sv["id"].(float64)
		}
	}
	if sugID == 0 {
		_, m = call("POST", "/suggestions", empTok, map[string]any{"content": "冒烟：想吃锅包肉"})
		_, m = call("GET", "/admin/suggestions", caTok, nil)
		for _, s := range toSlice(data(m)["suggestions"]) {
			sv := s.(map[string]any)
			if c, _ := sv["content"].(string); strings.Contains(c, "冒烟") {
				sugID = sv["id"].(float64)
			}
		}
	}
	assert("提交建议", sugID > 0)
	sc, m = call("PATCH", fmt.Sprintf("/admin/suggestions/%d", int64(sugID)), caTok, map[string]any{"status": "adopted"})
	assert("标记建议状态", code(m) == "OK")

	// 13. 超管看板
	sc, _ = call("GET", "/super/dashboard/trend", adminTok, nil)
	assert("看板-就餐趋势", sc == 200)
	sc, _ = call("GET", "/super/violations", adminTok, nil)
	assert("违约名单", sc == 200)

	// 14. 越界发布整批拒绝
	sc, _ = call("POST", "/admin/menus", caTok, map[string]any{"items": []any{
		map[string]any{"menu_date": time.Now().AddDate(0, 0, 30).Format("2006-01-02"), "meal": "lunch", "dishes": []any{map[string]any{"dish_name": "X"}}},
	}})
	assert("发布范围越界整批拒绝(400)", sc == 400)

	fmt.Printf("\n=== 冒烟结果: PASS=%d FAIL=%d ===\n", passN, failN)
	if failN > 0 {
		os.Exit(1)
	}
}

func toSlice(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	return nil
}
