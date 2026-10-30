package main

import (
	"io/fs"
	"net/http/httptest"
	"testing"
)

// 验证静态处理：embed 数据完整 + handler 输出完整（带正确 MIME）。
func TestStaticServing(t *testing.T) {
	// 1. embed 数据完整
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		t.Fatal(err)
	}
	b, err := fs.ReadFile(sub, "admin/assets/index-C37WbOfA.js")
	if err != nil {
		t.Fatalf("embed read admin js: %v", err)
	}
	if len(b) < 1000 {
		t.Fatalf("embedded admin js too small: %d", len(b))
	}

	// 2. handler 输出完整 + 正确 MIME
	h := staticHandler()
	for _, tc := range []struct {
		path     string
		wantCT   string
		minBytes int
	}{
		{"/admin/assets/index-C37WbOfA.js", "text/javascript", 1000},
		{"/assets/index-DrCBm0KI.js", "text/javascript", 1000},
		{"/assets/index-Bh_c2dMR.css", "text/css", 1000},
		{"/admin/assets/index-WxHCO0OI.css", "text/css", 1000},
		{"/", "text/html", 200},
		{"/admin/", "text/html", 200},
		{"/admin/verify", "text/html", 200},
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
