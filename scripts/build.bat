@echo off
rem ============================================================
rem CreatureForge build (Windows): front-end -> embed -> Go binaries dist\
rem Usage: scripts\build.bat
rem ============================================================
setlocal
set "ROOT=%~dp0.."

echo === [1/3] build front-end (Vue -> creatureforge\web\dist) ===
cd /d "%ROOT%\creatureforge\web"
call pnpm build
if errorlevel 1 exit /b 1

echo.
echo === [2/3] sync front-end into Go embed (gocore\internal\server\static) ===
set "STATIC=%ROOT%\gocore\internal\server\static"
if exist "%STATIC%" rmdir /s /q "%STATIC%"
mkdir "%STATIC%"
xcopy /e /i /q "%ROOT%\creatureforge\web\dist\." "%STATIC%\" >nul
if errorlevel 1 exit /b 1

echo.
echo === [3/3] build Go binaries (-> dist\) ===
cd /d "%ROOT%"
if not exist "dist" mkdir "dist"
cd /d "%ROOT%\gocore"
go build -o "%ROOT%\dist\gocore-server.exe" ./cmd/gocore-server
if errorlevel 1 exit /b 1
go build -o "%ROOT%\dist\gocore.exe" ./cmd/gocore
if errorlevel 1 exit /b 1

echo.
echo [ok] build done:
dir /b "%ROOT%\dist\gocore-*" 2>nul || echo     (no binaries found)
endlocal
