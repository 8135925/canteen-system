@echo off
rem 本地启动：单端口 8080 提供全部功能（H5 + 管理端 + API，连接 Supabase）
rem 窗口关闭即服务停止
cd /d %~dp0..\server
set PORT=8080
set AUTO_MIGRATE=1
set DATABASE_URL=postgresql://postgres.yikizzibtsokolgnnkfi:MyDb2026Secure!789@aws-0-eu-west-1.pooler.supabase.com:5432/postgres?sslmode=require
set JWT_SECRET=local-dev-secret
api.exe
pause
