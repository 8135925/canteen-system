package main

import (
	"io/fs"
	"net/http/httptest"
	"strings"
	"testing"
)

// 从 admin/index.html 提取引用的 JS 文件名（构建哈希会变，不能硬编码）。
func adminMainJS(t *testing.T, sub fs.FS) string {
	t.Helper()
	html, err := fs.ReadFile(sub, "admin/index.html")
	if err != nil {
		t.Fatalf("read admin/index.html: %v", err)
	}
	s := string(html)
	marker := `/admin/assets/index-`
	i := strings.Index(s, marker)
	if i < 0 {
		t.Fatalf("no admin js reference in index.html: %s", s)
	}
	rest := s[i+len(marker):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("bad js reference in index.html")
	}
	return "admin/assets/index-" + rest[:end]
}

// 验证静态处理：embed 数据完整 + handler 输出完整（带正确 MIME）。
func TestStaticServing(t *testing.T) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		t.Fatal(err)
	}
	adminJS := adminMainJS(t, sub)
	b, err := fs.ReadFile(sub, adminJS)
	if err != nil {
		t.Fatalf("embed read admin js: %v", err)
	}
	if len(b) < 1000 {
		t.Fatalf("embedded admin js too small: %d", len(b))
	}

	// handler 输出完整 + 正确 MIME
	h := staticHandler()
	for _, tc := range []struct {
		path     string
		wantCT   string
		minBytes int
	}{
		{"/" + adminJS, "text/javascript", 1000},
		{"/assets/index-Bh_c2dMR.css", "text/css", 1000},
		{"/admin/", "text/html", 200},
		{"/admin/verify", "text/html", 200},
		{"/", "text/html", 200},
		{"/my/records", "text/html", 200},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
		if rec.Code != 200 {
			t.Errorf("%s: code=%d", tc.path, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); len(ct) < len(tc.wantCT) || ct[:len(tc.wantCT)] != tc.wantCT {
			t.Errorf("%s: content-type=%q want prefix %q", tc.path, ct, tc.wantCT)
		}
		if rec.Body.Len() < tc.minBytes {
			t.Errorf("%s: body %d bytes < %d", tc.path, rec.Body.Len(), tc.minBytes)
		}
	}
}
