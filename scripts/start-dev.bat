@echo off
rem ============================================================================
rem CreatureForge 开发环境一键启动（Windows，前后端分离，热更新）
rem   后端: gocore-server --dev  (0.0.0.0:8765, Go 单二进制 embed 前端)
rem   前端: pnpm dev --host         (0.0.0.0:5173, proxy /api -> 8765)
rem 用法: scripts\start-dev.bat
rem ============================================================================
setlocal
set ROOT=%~dp0..
cd /d "%ROOT%"

if not exist "gocore" (
  echo [x] 未找到 gocore 目录：%ROOT%\gocore
  exit /b 1
)
if not exist "creatureforge\web\node_modules" (
  echo [x] 前端依赖未安装：cd creatureforge\web ^&^& pnpm install
  exit /b 1
)

echo [*] 构建 gocore-server ...
pushd gocore
go build -o "..\.dev\gocore-server.exe" ./cmd/gocore-server
popd

rem 启动后端（新窗口，避免阻塞本脚本）
start "CreatureForge API" ".dev\gocore-server.exe" --dev --host 0.0.0.0 --port 8765 --data-dir "%ROOT%\data"

rem 启动前端 Vite dev
start "CreatureForge WEB" cmd /c "cd /d creatureforge\web && set API_TARGET=http://127.0.0.1:8765 && pnpm dev --host --port 5173"

echo [ok] 开发环境已启动：
echo    前端: http://localhost:5173
echo    后端: http://localhost:8765/api
echo    停止: 关闭两个窗口或运行 scripts\stop-dev.bat
endlocal
