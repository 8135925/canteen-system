package main

import "embed"

// 前端构建产物挂载点：源码内放 .gitkeep 占位保证空目录可嵌入；
// 构建时由 Dockerfile 将 apps/h5/dist → static/h5、apps/admin/dist → static/admin 注入。
//
//go:embed all:static
var staticFS embed.FS
