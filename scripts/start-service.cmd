@echo off
rem Start service against Supabase. Usage: start-service.cmd [port]
cd /d %~dp0..\server
set PORT=%1
if "%PORT%"=="" set PORT=8080
set AUTO_MIGRATE=1
set DATABASE_URL=postgresql://postgres.yikizzibtsokolgnnkfi:MyDb2026Secure!789@aws-0-eu-west-1.pooler.supabase.com:5432/postgres?sslmode=require
set JWT_SECRET=local-dev-secret
api.exe 1>svc_%PORT%.log 2>&1
