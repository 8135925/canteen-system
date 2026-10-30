@echo off
rem Rebuild frontend, inject into Go embed dir, recompile api.exe.
rem Run this AFTER any frontend code change, then restart your service.
cd /d %~dp0..
echo [1/3] building h5 + admin ...
call pnpm build
if errorlevel 1 goto :fail
echo [2/3] injecting dist into server\cmd\api\static ...
rd /s /q server\cmd\api\static\h5 2>nul
xcopy /E /I /Y apps\h5\dist server\cmd\api\static\h5 >nul
rd /s /q server\cmd\api\static\admin 2>nul
xcopy /E /I /Y apps\admin\dist server\cmd\api\static\admin >nul
echo [3/3] recompiling api.exe ...
cd server
go build -o api.exe ./cmd/api
if errorlevel 1 goto :fail
echo.
echo DONE. Now restart your service (start-local.cmd or your command).
goto :eof
:fail
echo BUILD FAILED - see messages above.
exit /b 1
