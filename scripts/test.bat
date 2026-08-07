@echo off
rem ============================================================
rem CreatureForge full test suite (Windows): 3D motion verify + CLI workflow + front-end E2E
rem Usage: scripts\test.bat
rem ============================================================
setlocal
set "ROOT=%~dp0.."
cd /d "%ROOT%"

echo === [1/3] 3D motion verify (verify_motions3d) ===
".venv\Scripts\python.exe" scripts\verify_motions3d.py
if errorlevel 1 exit /b 1

echo.
echo === [2/3] CLI workflow (test_cli) ===
".venv\Scripts\python.exe" scripts\test_cli.py
if errorlevel 1 exit /b 1

echo.
echo === [3/3] front-end E2E (playwright) ===
cd /d "%ROOT%\creatureforge\web"
call pnpm test:e2e
if errorlevel 1 exit /b 1

echo.
echo [ok] all tests passed
