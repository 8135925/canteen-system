// 工厂食堂报餐系统 API 服务（对齐 doc/sheji.md）。
// 监听 0.0.0.0:$PORT（Render 注入，本地缺省 8080）；
// 静态路由：/api/* → API；/admin* → admin SPA；其余 → h5 SPA（均 SPA fallback）。
package main

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"canteen/server/internal/api"
	"canteen/server/internal/auth"
	"canteen/server/internal/config"
	"canteen/server/internal/httpx"
	pgstore "canteen/server/internal/store/pg"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	st, err := pgstore.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect error", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	if cfg.AutoMigrate {
		if err := st.Migrate(ctx); err != nil {
			slog.Error("migrate error", "err", err)
			os.Exit(1)
		}
		slog.Info("migrations applied")
	}

	lookup := auth.NewUserLookup(func(uid int64) (string, string, bool, error) {
		u, err := st.GetUserByID(uid)
		if err != nil {
			return "", "", false, err
		}
		return u.Role, u.Status, u.MustChangePassword, nil
	})
	srv := api.NewServer(st, cfg.JWTSecret, lookup, auth.NewLoginGuard(), cfg.DemoMode)

	mux := http.NewServeMux()
	mux.Handle("/api/", srv.Handler())
	mux.Handle("/", staticHandler())

	addr := "0.0.0.0:" + cfg.Port
	slog.Info("server listening", "addr", addr)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           logMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       55 * time.Second, // 主动关闭闲置 keep-alive 连接，避免浏览器复用半开死连接
	}
	if err := httpSrv.ListenAndServe(); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

// logMiddleware 请求日志：方法、路径、耗时。
func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
			"cost_ms", time.Since(start).Milliseconds(),
		)
	})
}

// staticHandler 前端静态资源 + SPA fallback。
// 构建时 Dockerfile 将 h5/admin 产物注入 static/h5、static/admin（go:embed all:static）。
// 路径映射：/admin* → static/admin/*；h5 资产（/assets/*、/favicon.ico 等）→ static/h5/*；其余 → h5 index。
// 说明：不使用 http.FileServer——对 embed.FS 的空响应异常已实测复现，改为 ReadFile 直出。
func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		slog.Error("static fs error", "err", err)
		os.Exit(1)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		switch {
		case p == "admin" || strings.HasPrefix(p, "admin/"):
			if !exists(sub, p) {
				serveIndex(sub, "admin/index.html", w, r)
				return
			}
			serveAsset(sub, p, w, r)
		case p == "" || p == "index.html":
			serveIndex(sub, "h5/index.html", w, r)
		case exists(sub, "h5/"+p):
			serveAsset(sub, "h5/"+p, w, r)
		default:
			serveIndex(sub, "h5/index.html", w, r)
		}
	})
}

// mimeByExt 静态资源 Content-Type。
var mimeByExt = map[string]string{
	".js":    "text/javascript; charset=utf-8",
	".mjs":   "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".html":  "text/html; charset=utf-8",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".ico":   "image/x-icon",
	".woff2": "font/woff2",
	".woff":  "font/woff",
	".ttf":   "font/ttf",
	".json":  "application/json",
	".map":   "application/json",
	".txt":   "text/plain; charset=utf-8",
}

// serveAsset 读取嵌入文件并按扩展名输出（带一年缓存；文件名均含内容哈希）。
func serveAsset(fsys fs.FS, name string, w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		serveIndex(fsys, "h5/index.html", w, r)
		return
	}
	if ct := mimeByExt[strings.ToLower(filepath.Ext(name))]; ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(data)
}

func exists(fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func serveIndex(fsys fs.FS, name string, w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		httpx.FailWith(w, httpx.CodeNotFound, "frontend not built")
		return
	}
	// index.html 禁止缓存：确保前端更新后浏览器总拿到最新引用的资源哈希
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
