// Package migrations 内嵌迁移 SQL 文件（保持 server/migrations 路径与设计文档一致）。
package migrations

import (
	"embed"
	"io/fs"
)

//go:embed *.up.sql *.down.sql
var files embed.FS

// Ups 返回按文件名排序的全部 up 迁移（版本号, 内容）。
func Ups() ([]Migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, err
	}
	var out []Migration
	for _, e := range entries {
		if len(e.Name()) > 7 && e.Name()[len(e.Name())-7:] == ".up.sql" {
			b, err := files.ReadFile(e.Name())
			if err != nil {
				return nil, err
			}
			out = append(out, Migration{Version: e.Name()[:len(e.Name())-7], SQL: string(b)})
		}
	}
	// 按版本号排序
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Version < out[j-1].Version; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}

type Migration struct {
	Version string
	SQL     string
}
