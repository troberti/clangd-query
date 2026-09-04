@echo off
setlocal

REM Simple build script for clangd-query

cd /d "%~dp0go"
if not exist "..\bin" mkdir "..\bin"

go build -o ..\bin\clangd-query.exe .
if %ERRORLEVEL% equ 0 (
    echo Build successful: bin\clangd-query.exe
) else (
    echo Build failed
    exit /b 1
)
