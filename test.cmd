@echo off
setlocal
cd /d "%~dp0"

echo [test] go test ./...
go test ./...
exit /b %ERRORLEVEL%
