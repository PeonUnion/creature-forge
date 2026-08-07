@echo off
rem ============================================================
rem CreatureForge dev - stop backend + frontend
rem Usage: scripts\stop-dev.bat
rem ============================================================
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0stop-dev.ps1"
