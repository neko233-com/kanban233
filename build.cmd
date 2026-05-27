@echo off
setlocal
cd /d "%~dp0"

echo [build] tidy modules...
go mod tidy
if errorlevel 1 exit /b 1

echo [build] running tests...
go test ./...
if errorlevel 1 exit /b 1

echo [build] compiling kanban...
if not exist bin mkdir bin
go build -o bin\kanban.exe .\cmd\kanban
if errorlevel 1 exit /b 1

echo [build] done: bin\kanban.exe
