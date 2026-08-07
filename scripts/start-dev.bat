@echo off
rem ============================================================
rem CreatureForge dev - start backend + frontend (LAN-accessible)
rem Usage: scripts\start-dev.bat   (env: API_PORT / WEB_PORT)
rem Stop:  scripts\stop-dev.bat
rem ============================================================
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0start-dev.ps1"
