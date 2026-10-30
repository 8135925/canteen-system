// 诊断：模拟浏览器登录后的完整请求链路。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

var client = &http.Client{}

func call(method, path, token string, body any) (int, string) {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, "http://localhost:8080/api/v1"+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err.Error()
	}
	return resp.StatusCode, string(raw)
}

func main() {
	base := "http://localhost:8080/api/v1"
	_ = base
	type args struct {
		method, path, token string
		body                any
		label               string
	}
	cases := []args{
		{"POST", "/auth/login", "", map[string]any{"emp_no": "E_SMOKE", "password": "EmpSmoke2026!"}, "员工登录"},
		{"POST", "/auth/login", "", map[string]any{"emp_no": "CA_SMOKE", "password": "CaSmoke2026!"}, "管理员登录"},
		{"POST", "/auth/login", "", map[string]any{"emp_no": "admin", "password": "Admin2026!"}, "超管登录"},
	}
	tokens := map[string]string{}
	for _, c := range cases {
		sc, body := call(c.method, c.path, c.token, c.body)
		var out struct {
			Code string `json:"code"`
			Data struct {
				Token string `json:"token"`
				User  struct {
					EmpNo string `json:"emp_no"`
					Role  string `json:"role"`
					MCP   bool   `json:"must_change_password"`
				} `json:"user"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(body), &out); err != nil {
			fmt.Printf("[%s] unmarshal err: %v\n", c.label, err)
		}
		fmt.Printf("[%s] code=%d api=%s mcp=%v token_len=%d\n", c.label, sc, out.Code, out.Data.User.MCP, len(out.Data.Token))
		if out.Data.Token != "" {
			tokens[out.Data.User.EmpNo] = out.Data.Token
		}
	}
	// 各角色调各自的典型接口
	follow := []struct {
		empNo, label, method, path string
	}{
		{"E_SMOKE", "员工-用餐记录", "GET", "/my/signups"},
		{"E_SMOKE", "员工-我的建议", "GET", "/my/suggestions"},
		{"CA_SMOKE", "管理员-员工列表", "GET", "/admin/employees"},
		{"CA_SMOKE", "管理员-统计", "GET", "/admin/stats/signups"},
		{"admin", "超管-用户列表", "GET", "/super/users"},
	}
	for _, f := range follow {
		tok := tokens[f.empNo]
		sc, body := call(f.method, f.path, tok, nil)
		fmt.Printf("[%s] code=%d body_head=%.100s\n", f.label, sc, body)
	}
	os.Exit(0)
}
