@echo off
setlocal EnableExtensions EnableDelayedExpansion
cd /d "%~dp0"

echo [deploy] kanban233 -^> GitHub
echo.

git rev-parse --is-inside-work-tree >nul 2>&1
if errorlevel 1 (
  echo [deploy] not a git repository.
  exit /b 1
)

git remote get-url origin >nul 2>&1
if errorlevel 1 (
  echo [deploy] remote "origin" not configured.
  exit /b 1
)

for /f "delims=" %%I in ('git branch --show-current 2^>nul') do set "BRANCH=%%I"
if not defined BRANCH (
  echo [deploy] cannot detect current branch.
  exit /b 1
)
echo [deploy] branch: %BRANCH%
echo [deploy] remote: origin
echo.

echo [deploy] running tests...
go test ./...
if errorlevel 1 (
  echo [deploy] tests failed, abort.
  exit /b 1
)

echo.
echo [deploy] git pull...
git pull --rebase origin %BRANCH%
if errorlevel 1 (
  echo [deploy] git pull --rebase failed, abort.
  exit /b 1
)

git status --porcelain | findstr /R "." >nul
if errorlevel 1 (
  echo [deploy] working tree clean, push only...
  goto :push
)

if "%~1"=="" (
  for /f "delims=" %%T in ('powershell -NoProfile -Command "Get-Date -Format 'yyyy-MM-dd HH:mm:ss'"') do set "STAMP=%%T"
  set "MSG=deploy: !STAMP!"
) else (
  set "MSG=%*"
)

echo [deploy] commit: %MSG%
git add -A
git commit -m "%MSG%"
if errorlevel 1 (
  echo [deploy] commit failed, abort.
  exit /b 1
)

:push
echo.
echo [deploy] git push origin %BRANCH% ...
git push origin %BRANCH%
if errorlevel 1 (
  echo [deploy] push failed, trying -u ...
  git push -u origin %BRANCH%
)
exit /b %ERRORLEVEL%
