@echo off
rem Start local service on port 8080 (H5 + admin + API, Supabase DB).
rem Close this window to stop the service.
cd /d %~dp0..\server
set PORT=8080
set AUTO_MIGRATE=1
set DATABASE_URL=postgresql://postgres.yikizzibtsokolgnnkfi:MyDb2026Secure!789@aws-0-eu-west-1.pooler.supabase.com:5432/postgres?sslmode=require
set JWT_SECRET=local-dev-secret
api.exe
pause
