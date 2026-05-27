@echo off
setlocal
cd /d "%~dp0"

if not exist data mkdir data

set KANBAN_CONFIG=%~dp0server.yaml

echo [dev] starting kanban233 on http://localhost:61333
go run .\cmd\kanban -config "%KANBAN_CONFIG%"
