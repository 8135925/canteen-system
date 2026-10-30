// Package httpx 统一响应格式 {code, msg, data} 与错误码枚举（对齐 doc/sheji.md §5.4）。
// 实现说明：原设计用 Gin，因上游依赖 golang-asm 仓库不可访问，改用 Go 1.22+ 标准库
// net/http（支持 {id} 路径参数与 r.PathValue），功能等价。
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// 错误码枚举
const (
	CodeOK                  = "OK"
	CodeBadRequest          = "BAD_REQUEST"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeForbidden           = "FORBIDDEN"
	CodeNotFound            = "NOT_FOUND"
	CodeConflict            = "CONFLICT"
	CodeInternal            = "INTERNAL"

	CodeMustChangePassword = "MUST_CHANGE_PASSWORD"
	CodeSignupClosed       = "SIGNUP_CLOSED"
	CodeVerifyClosed       = "VERIFY_CLOSED"
	CodeDishOffShelf       = "DISH_OFF_SHELF"
	CodeAlreadyVerified    = "ALREADY_VERIFIED"
	CodeMenuNotPublished   = "MENU_NOT_PUBLISHED"
	CodeAlreadyWalkIn      = "ALREADY_WALK_IN"
	CodeInvalidCredentials = "INVALID_CREDENTIALS"
	CodeLocked             = "LOCKED"
	CodeEmpNoExists        = "EMP_NO_EXISTS"
	CodeOutOfRange         = "DATE_OUT_OF_RANGE"
	CodeNoActiveSignup     = "NO_ACTIVE_SIGNUP"
	CodeAlreadyViolated    = "ALREADY_VIOLATED"
	CodeHasActiveSignup    = "HAS_ACTIVE_SIGNUP"
)

var codeStatus = map[string]int{
	CodeOK:                  http.StatusOK,
	CodeBadRequest:          http.StatusBadRequest,
	CodeUnauthorized:        http.StatusUnauthorized,
	CodeForbidden:           http.StatusForbidden,
	CodeNotFound:            http.StatusNotFound,
	CodeConflict:            http.StatusConflict,
	CodeInternal:            http.StatusInternalServerError,
	CodeMustChangePassword:  http.StatusForbidden,
	CodeSignupClosed:        http.StatusForbidden,
	CodeVerifyClosed:        http.StatusForbidden,
	CodeDishOffShelf:        http.StatusForbidden,
	CodeAlreadyVerified:     http.StatusConflict,
	CodeMenuNotPublished:    http.StatusForbidden,
	CodeAlreadyWalkIn:       http.StatusConflict,
	CodeInvalidCredentials:  http.StatusUnauthorized,
	CodeLocked:              http.StatusTooManyRequests,
	CodeEmpNoExists:         http.StatusConflict,
	CodeOutOfRange:          http.StatusBadRequest,
	CodeNoActiveSignup:      http.StatusNotFound,
	CodeAlreadyViolated:     http.StatusConflict,
	CodeHasActiveSignup:     http.StatusConflict,
}

// BizErr 业务错误：错误码 + 用户提示。
type BizErr struct {
	Code string
	Msg  string
}

func (e *BizErr) Error() string { return e.Msg }

func Err(code, msg string) *BizErr { return &BizErr{Code: code, Msg: msg} }

func StatusOf(code string) int {
	if s, ok := codeStatus[code]; ok {
		return s
	}
	return http.StatusInternalServerError
}

type envelope struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func OK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, envelope{Code: CodeOK, Msg: "success", Data: data})
}

func Fail(w http.ResponseWriter, err error) {
	var be *BizErr
	if errors.As(err, &be) {
		writeJSON(w, StatusOf(be.Code), envelope{Code: be.Code, Msg: be.Msg})
		return
	}
	// 非业务错误：记录详情（定位 SQL/内部问题）
	slog.Error("internal error", "err", err)
	writeJSON(w, http.StatusInternalServerError, envelope{Code: CodeInternal, Msg: "internal error"})
}

func FailWith(w http.ResponseWriter, code, msg string) {
	writeJSON(w, StatusOf(code), envelope{Code: code, Msg: msg})
}

// DecodeJSON 解析请求体（限制 1MB）。
func DecodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	return dec.Decode(dst)
}
