@echo off
rem ============================================================================
rem 停止 CreatureForge 开发环境（Windows）：按端口清理 gocore-server 与 Vite
rem ============================================================================
setlocal
echo [*] 清理端口 8765 (API) 与 5173 (WEB) ...
for %%P in (8765 5173) do (
  for /f "tokens=5" %%a in ('netstat -ano ^| findstr :%%P ^| findstr LISTENING') do (
    taskkill /F /PID %%a >nul 2>&1 && echo [*] 已停止端口 %%P 进程 (pid %%a)
  )
)
echo [ok] 开发环境已停止
endlocal
