@echo off
rem 启动服务（连接 Supabase，端口可传参，默认 8080）
cd /d %~dp0..\server
set PORT=%1
if "%PORT%"=="" set PORT=8080
set AUTO_MIGRATE=1
set DATABASE_URL=postgresql://postgres.yikizzibtsokolgnnkfi:MyDb2026Secure!789@aws-0-eu-west-1.pooler.supabase.com:5432/postgres?sslmode=require
set JWT_SECRET=smoke-local-secret
api.exe 1>svc_%PORT%.log 2>&1
