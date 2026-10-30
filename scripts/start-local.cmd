@echo off
rem Start local service on port 8080 (H5 + admin + API).
rem DATABASE_URL is loaded from scripts\local.env (git-ignored, see local.env.example).
rem Close this window to stop the service.
cd /d %~dp0
if not exist local.env (
  echo Missing scripts\local.env - copy local.env.example to local.env and fill in your DATABASE_URL.
  pause
  exit /b 1
)
for /f "usebackq eol=# tokens=1,* delims==" %%a in ("local.env") do set %%a=%%b
cd /d %~dp0..\server
set PORT=8080
set AUTO_MIGRATE=1
set JWT_SECRET=local-dev-secret
if "%DATABASE_URL%"=="" (
  echo DATABASE_URL is empty in scripts\local.env
  pause
  exit /b 1
)
api.exe
pause
