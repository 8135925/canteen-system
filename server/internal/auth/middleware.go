package auth

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"canteen/server/internal/httpx"
)

// —— 请求上下文键 ——
type ctxKey int

const (
	ctxUID ctxKey = iota
	ctxRole
	ctxMCP
)

// WithUser 注入用户信息（测试辅助）。
func WithUser(ctx context.Context, uid int64, role string, mcp bool) context.Context {
	ctx = context.WithValue(ctx, ctxUID, uid)
	ctx = context.WithValue(ctx, ctxRole, role)
	ctx = context.WithValue(ctx, ctxMCP, mcp)
	return ctx
}

// UID 从请求上下文取当前用户 ID。
func UID(r *http.Request) int64 {
	v, _ := r.Context().Value(ctxUID).(int64)
	return v
}

// RoleFrom 取当前角色。
func RoleFrom(r *http.Request) string {
	v, _ := r.Context().Value(ctxRole).(string)
	return v
}

// UserLookup 供中间件实时校验 status / must_change_password（60s 缓存）。
type UserLookup struct {
	mu    sync.Mutex
	cache map[int64]cachedUser
	ttl   time.Duration
	fn    func(uid int64) (role string, status string, mcp bool, err error)
}

type cachedUser struct {
	role   string
	status string
	mcp    bool
	expire time.Time
}

func NewUserLookup(fn func(uid int64) (role string, status string, mcp bool, err error)) *UserLookup {
	return &UserLookup{cache: map[int64]cachedUser{}, ttl: 60 * time.Second, fn: fn}
}

func (l *UserLookup) Get(uid int64) (role string, status string, mcp bool, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if c, ok := l.cache[uid]; ok && time.Now().Before(c.expire) {
		return c.role, c.status, c.mcp, nil
	}
	role, status, mcp, err = l.fn(uid)
	if err != nil {
		return "", "", false, err
	}
	l.cache[uid] = cachedUser{role: role, status: status, mcp: mcp, expire: time.Now().Add(l.ttl)}
	return
}

// Invalidate 清除指定用户缓存（改密/禁用/重置后调用）。
func (l *UserLookup) Invalidate(uid int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.cache, uid)
}

// Middleware 认证中间件：解析 JWT、校验 status 与 mcp（DB 优先）、注入用户信息。
func Middleware(secret string, lookup *UserLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ""
			if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
				token = strings.TrimPrefix(h, "Bearer ")
			} else if q := r.URL.Query().Get("token"); q != "" {
				token = q
			}
			if token == "" {
				httpx.FailWith(w, httpx.CodeUnauthorized, "missing token")
				return
			}
			claims, err := ParseToken(secret, token)
			if err != nil {
				httpx.FailWith(w, httpx.CodeUnauthorized, "invalid token")
				return
			}
			role, status, mcp, err := lookup.Get(claims.UID)
			if err != nil {
				httpx.FailWith(w, httpx.CodeUnauthorized, "user not found")
				return
			}
			if status == "disabled" {
				httpx.FailWith(w, httpx.CodeUnauthorized, "account disabled")
				return
			}
			// mcp 与 DB 不一致时以 DB 为准：防密码被重置后旧 token 绕过强制改密
			if mcp != claims.MCP && !mcp {
				mcp = false // DB 已 false：以 DB 为准视为已改
			}
			if mcp {
				p := r.URL.Path
				allow := (r.Method == http.MethodPut && strings.HasSuffix(p, "/my/password")) ||
					(r.Method == http.MethodGet && strings.HasSuffix(p, "/me"))
				if !allow {
					httpx.FailWith(w, httpx.CodeMustChangePassword, "must change password first")
					return
				}
			}
			ctx := WithUser(r.Context(), claims.UID, role, mcp)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole 角色中间件。
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, want := range roles {
				if RoleFrom(r) == want {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpx.FailWith(w, httpx.CodeForbidden, "role not allowed")
		})
	}
}

// —— 防爆破：emp_no+IP 失败计数，5 次锁 10 分钟（内存实现，单实例） ——

type failRecord struct {
	count    int
	lockedTo time.Time
}

const (
	maxFails     = 5
	lockDuration = 10 * time.Minute
)

type LoginGuard struct {
	mu    sync.Mutex
	fails map[string]*failRecord
}

func NewLoginGuard() *LoginGuard { return &LoginGuard{fails: map[string]*failRecord{}} }

// Check 返回是否被锁定。
func (g *LoginGuard) Check(key string) (locked bool, remaining time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	r, ok := g.fails[key]
	if !ok {
		return false, 0
	}
	if time.Now().Before(r.lockedTo) {
		return true, time.Until(r.lockedTo)
	}
	return false, 0
}

// Fail 记录一次失败，达阈值即锁定。
func (g *LoginGuard) Fail(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	r, ok := g.fails[key]
	if !ok {
		r = &failRecord{}
		g.fails[key] = r
	}
	r.count++
	if r.count >= maxFails {
		r.lockedTo = time.Now().Add(lockDuration)
		r.count = 0
	}
}

// Reset 登录成功后清零。
func (g *LoginGuard) Reset(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.fails, key)
}

// LockRemaining 供测试：剩余锁定时长（未锁定返回 0）。
func (g *LoginGuard) LockRemaining(key string) time.Duration {
	locked, remaining := g.Check(key)
	if !locked {
		return 0
	}
	return remaining
}
