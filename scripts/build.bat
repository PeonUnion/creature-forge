@echo off
rem ============================================================
rem CreatureForge build (Windows): front-end dist + binaries dist\
rem Usage: scripts\build.bat
rem ============================================================
setlocal
set "ROOT=%~dp0.."

echo === [1/2] build front-end (Vue -> creatureforge\web\dist) ===
cd /d "%ROOT%\creatureforge\web"
call pnpm build
if errorlevel 1 exit /b 1

echo.
echo === [2/2] build binaries (pyinstaller -> dist\) ===
cd /d "%ROOT%"
".venv\Scripts\python.exe" scripts\build_release.py
if errorlevel 1 exit /b 1

echo.
echo [ok] build done:
dir /b "%ROOT%\dist\creature-forge-*" 2>nul || echo     (no binaries found)
