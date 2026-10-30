@echo off
rem Start service on 8081 then run the end-to-end smoke client.
cd /d %~dp0..
set SMOKE_BASE=http://localhost:8081/api/v1
start "svc" /min cmd /c "scripts\start-service.cmd 8081"
ping -n 7 127.0.0.1 >nul
cd server
go run ./cmd/smoke
