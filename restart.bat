@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0"

REM Port is set in .env (DEV_PORT/PROD_PORT). Change here if you change it there.
set PORT=5152

echo ===============================================
echo   Stopping process holding port %PORT%
echo ===============================================

set FOUND=0
for /f "tokens=5" %%p in ('netstat -aon ^| findstr /R /C:":%PORT% .*LISTENING"') do (
    echo Found PID=%%p on port %PORT%, killing it...
    taskkill /PID %%p /F >nul 2>&1
    set FOUND=1
)

if "!FOUND!"=="0" (
    echo No process found on port %PORT% - probably already stopped.
)

REM Also kill any stuck delve debug binaries (__debug_bin*.exe) that VS Code
REM can leave behind if an F5 debug session was interrupted uncleanly.
taskkill /IM __debug_bin*.exe /F >nul 2>&1

timeout /t 1 /nobreak >nul

echo.
echo ===============================================
echo   Building project (go build .\cmd)
echo ===============================================
go build -o registry.exe .\cmd
if errorlevel 1 (
    echo.
    echo BUILD FAILED - see output above.
    pause
    exit /b 1
)

echo.
echo ===============================================
echo   Starting registry.exe
echo ===============================================
start "GO-Registry" registry.exe

echo.
echo Done: application started in a separate window "GO-Registry".
pause
