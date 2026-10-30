@echo off
rem Start service in background with logs. Usage: start-service.cmd [port]
rem DATABASE_URL is loaded from scripts\local.env (git-ignored).
cd /d %~dp0
if not exist local.env (
  echo Missing scripts\local.env
  exit /b 1
)
for /f "usebackq eol=# tokens=1,* delims==" %%a in ("local.env") do set %%a=%%b
cd /d %~dp0..\server
set PORT=%1
if "%PORT%"=="" set PORT=8080
set AUTO_MIGRATE=1
set JWT_SECRET=local-dev-secret
if "%DATABASE_URL%"=="" (
  echo DATABASE_URL is empty in scripts\local.env
  exit /b 1
)
api.exe 1>svc_%PORT%.log 2>&1
