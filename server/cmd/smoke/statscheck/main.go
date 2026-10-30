// 自测：验证统计接口（数据统计页的三个数据源）在正常 date 参数下返回 200。
// 用法：SMOKE_BASE=http://localhost:8082/api/v1 go run ./cmd/smoke/statscheck
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var base = envDefault("SMOKE_BASE", "http://localhost:8080/api/v1")

func envDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	c := &http.Client{Timeout: 30 * time.Second}

	// 1. 登录（食堂管理员，有统计权限）
	lb, _ := json.Marshal(map[string]string{empKey(): os.Getenv("SMK_EMP"), "password": os.Getenv("SMK_PWD")})
	resp, err := c.Post(base+"/auth/login", "application/json", bytes.NewReader(lb))
	if err != nil {
		fmt.Println("FATAL login:", err)
		os.Exit(1)
	}
	lbBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var login struct {
		Code string `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(lbBody, &login)
	if login.Code != "OK" || login.Data.Token == "" {
		fmt.Println("FATAL login failed:", string(lbBody))
		os.Exit(1)
	}
	tok := login.Data.Token
	fmt.Println("PASS login")

	// 2. 三个统计接口，date/from/to 均为正常字符串
	today := time.Now().Format("2006-01-02")
	monthAgo := time.Now().AddDate(0, 0, -29).Format("2006-01-02")
	checks := []struct {
		name, url string
	}{
		{"stats/signups", "/admin/stats/signups?date=" + today},
		{"stats/departments", "/admin/stats/departments?date=" + today},
		{"stats/daily", "/admin/stats/daily?from=" + monthAgo + "&to=" + today},
		{"attendance-detail", "/admin/stats/attendance-detail?date=" + today + "&page=1&page_size=20"},
	}
	allOK := true
	for _, ck := range checks {
		req, _ := http.NewRequest("GET", base+ck.url, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := c.Do(req)
		if err != nil {
			fmt.Printf("FAIL %-20s %v\n", ck.name, err)
			allOK = false
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var env struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(body, &env)
		ok := resp.StatusCode == 200 && env.Code == "OK"
		status := "PASS"
		if !ok {
			status = "FAIL"
			allOK = false
		}
		fmt.Printf("%s %-20s http=%d code=%s body_head=%.60s\n", status, ck.name, resp.StatusCode, env.Code, string(body))
	}
	if !allOK {
		os.Exit(1)
	}
	fmt.Println("=== SELFTEST ALL PASS ===")
}

func empKey() string {
	if v := os.Getenv("SMK_EMP_NO"); v != "" {
		return "emp_no"
	}
	return "emp_no"
}
