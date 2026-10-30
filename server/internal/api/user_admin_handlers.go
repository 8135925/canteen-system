package api

import (
	"encoding/csv"
	"io"
	"net/http"
	"strconv"
	"strings"

	"canteen/server/internal/auth"
	"canteen/server/internal/httpx"
	"canteen/server/internal/store"
)

const defaultPassword = "123456"

type addEmployeeReq struct {
	EmpNo      string `json:"emp_no"`
	Name       string `json:"name"`
	Department string `json:"department"`
}

func (s *Server) addEmployee(w http.ResponseWriter, r *http.Request) {
	var req addEmployeeReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误")
		return
	}
	req.EmpNo = strings.TrimSpace(req.EmpNo)
	req.Name = strings.TrimSpace(req.Name)
	if req.EmpNo == "" || req.Name == "" {
		httpx.FailWith(w, httpx.CodeBadRequest, "工号与姓名必填")
		return
	}
	exists, err := s.St.EmpNoExists(req.EmpNo)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	if exists {
		httpx.FailWith(w, httpx.CodeEmpNoExists, "工号已存在："+req.EmpNo)
		return
	}
	hash, err := auth.HashPassword(defaultPassword)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	u := &store.User{
		EmpNo: req.EmpNo, Name: req.Name, Department: strings.TrimSpace(req.Department),
		PasswordHash: hash, Role: "employee", MustChangePassword: true,
	}
	if err := s.St.CreateUser(u); err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, userView(u))
}

func (s *Server) listEmployees(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := parsePagination(r)
	users, total, err := s.St.ListUsers(q.Get("role"), q.Get("keyword"), page, pageSize)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"total": total, "page": page, "page_size": pageSize, "users": users})
}

func (s *Server) editEmployee(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "非法的用户 ID")
		return
	}
	var req struct {
		Name       string `json:"name"`
		Department string `json:"department"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "参数错误")
		return
	}
	u, err := s.St.GetUserByID(id)
	if err != nil {
		httpx.FailWith(w, httpx.CodeNotFound, "用户不存在")
		return
	}
	if u.Role != "employee" {
		httpx.FailWith(w, httpx.CodeForbidden, "只能编辑普通员工信息")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.FailWith(w, httpx.CodeBadRequest, "姓名不能为空")
		return
	}
	if err := s.St.UpdateUserProfile(id, strings.TrimSpace(req.Name), strings.TrimSpace(req.Department)); err != nil {
		httpx.Fail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"id": id, "name": req.Name, "department": req.Department})
}

// importEmployees Excel/CSV 批量导入：模板列 `工号,姓名,部门`（Excel 请另存为 CSV，UTF-8 带 BOM）。
func (s *Server) importEmployees(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "请使用 multipart/form-data 上传文件（字段名 file）")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "缺少文件字段 file")
		return
	}
	defer file.Close()

	reader := csv.NewReader(io.LimitReader(file, 10<<20))
	reader.FieldsPerRecord = -1 // 列数不强制
	records, err := reader.ReadAll()
	if err != nil {
		httpx.FailWith(w, httpx.CodeBadRequest, "CSV 解析失败："+err.Error())
		return
	}
	hash, err := auth.HashPassword(defaultPassword)
	if err != nil {
		httpx.Fail(w, err)
		return
	}
	type failure struct {
		Row    int    `json:"row"`
		Reason string `json:"reason"`
	}
	successCount := 0
	var failures []failure
	seen := map[string]bool{}
	for i, rec := range records {
		rowNo := i + 1
		if i == 0 && len(rec) >= 1 && (strings.Contains(rec[0], "工号") || strings.Contains(rec[0], "emp_no")) {
			continue // 表头
		}
		if len(rec) < 2 {
			failures = append(failures, failure{Row: rowNo, Reason: "至少需要 工号,姓名 两列"})
			continue
		}
		empNo := strings.TrimSpace(rec[0])
		name := strings.TrimSpace(rec[1])
		dept := ""
		if len(rec) >= 3 {
			dept = strings.TrimSpace(rec[2])
		}
		if empNo == "" || name == "" {
			failures = append(failures, failure{Row: rowNo, Reason: "工号与姓名不能为空"})
			continue
		}
		if seen[empNo] {
			failures = append(failures, failure{Row: rowNo, Reason: "文件内工号重复：" + empNo})
			continue
		}
		seen[empNo] = true
		exists, err := s.St.EmpNoExists(empNo)
		if err != nil {
			httpx.Fail(w, err)
			return
		}
		if exists {
			failures = append(failures, failure{Row: rowNo, Reason: "工号已存在：" + empNo})
			continue
		}
		u := &store.User{EmpNo: empNo, Name: name, Department: dept, PasswordHash: hash, Role: "employee", MustChangePassword: true}
		if err := s.St.CreateUser(u); err != nil {
			failures = append(failures, failure{Row: rowNo, Reason: "写入失败：" + err.Error()})
			continue
		}
		successCount++
	}
	httpx.OK(w, map[string]any{"success_count": successCount, "failures": failures})
}

var _ = io.EOF
