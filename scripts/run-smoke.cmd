@echo off
rem 启动服务（8081）→ 冒烟 → 输出结果
cd /d %~dp0..
set SMOKE_BASE=http://localhost:8081/api/v1
start "svc" /min cmd /c "scripts\start-service.cmd 8081"
ping -n 7 127.0.0.1 >nul
cd server
go run ./cmd/smoke
