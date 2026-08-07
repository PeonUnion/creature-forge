#!/usr/bin/env bash
# =============================================================================
# 停止 CreatureForge 开发环境进程（后端 --dev 与前端 Vite dev）
# 双保险：按进程名 + 按端口（确保端口释放）
# 用法: ./scripts/stop-dev.sh
# =============================================================================
set -u

# 1) 按进程名
pkill -f "creatureforge/server.py --dev" 2>/dev/null && echo "[ok] 已停止后端 API (server.py --dev)" || echo "[i] 后端 API 未在运行"
pkill -f "vite" 2>/dev/null && echo "[ok] 已停止前端 Vite dev" || echo "[i] 前端 Vite 未在运行"

# 2) 按端口兜底（覆盖默认端口；实际端口可通过 API_PORT/WEB_PORT 覆盖）
for p in "${API_PORT:-8765}" "${WEB_PORT:-5173}"; do
  pids="$(lsof -t -iTCP:"$p" -sTCP:LISTEN 2>/dev/null || true)"
  if [ -n "$pids" ]; then
    kill $pids 2>/dev/null && echo "[i] 已释放端口 $p" || true
  fi
done

echo "[ok] 开发环境已停止"
