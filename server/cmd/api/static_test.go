package main

import (
	"io/fs"
	"testing"
)

// 验证嵌入的前端产物完整性（构建时注入 static/h5、static/admin）。
func TestEmbeddedAssets(t *testing.T) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"h5/index.html",
		"admin/index.html",
	} {
		b, err := fs.ReadFile(sub, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(b) < 200 {
			t.Fatalf("%s: too small (%d bytes)", name, len(b))
		}
		t.Logf("%s size=%d", name, len(b))
	}
	// 遍历 admin/assets，确保无空文件
	entries, err := fs.ReadDir(sub, "admin/assets")
	if err != nil {
		t.Fatalf("admin/assets: %v", err)
	}
	for _, e := range entries {
		info, _ := e.Info()
		if info != nil && info.Size() == 0 {
			t.Errorf("empty embedded file: admin/assets/%s", e.Name())
		}
	}
}
