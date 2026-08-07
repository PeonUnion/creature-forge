#!/usr/bin/env bash
# =============================================================================
# 停止 CreatureForge 开发环境（后端 --dev 与前端 Vite dev）
# 优先读取 .dev/runtime.json 中的 PID 精确停止；缺失时按进程名兜底；
# 最后按端口兜底（确保端口释放）。
# 用法: ./scripts/stop-dev.sh
# =============================================================================
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEV_DIR="$ROOT/.dev"
RUNTIME="$DEV_DIR/runtime.json"

# -- 1) 读 .dev/runtime.json 精确 kill ---------------------------------------
if [ -f "$RUNTIME" ]; then
  api_pid="$(python3 -c "import json;print(json.load(open('$RUNTIME'))['api_pid'])" 2>/dev/null || true)"
  web_pid="$(python3 -c "import json;print(json.load(open('$RUNTIME'))['web_pid'])" 2>/dev/null || true)"
  if [ -n "$api_pid" ] && [ "$api_pid" != "0" ]; then
    kill "$api_pid" 2>/dev/null && echo "[ok] 已停止后端 API (pid $api_pid)" || echo "[i] 后端 API 已不在运行 (pid $api_pid)"
  fi
  if [ -n "$web_pid" ] && [ "$web_pid" != "0" ]; then
    kill "$web_pid" 2>/dev/null && echo "[ok] 已停止前端 Vite (pid $web_pid)" || echo "[i] 前端 Vite 已不在运行 (pid $web_pid)"
  fi
  rm -f "$RUNTIME"
else
  # 无 runtime 信息 → 按进程名兜底
  pkill -f "creatureforge/server.py --dev" 2>/dev/null && echo "[ok] 已停止后端 API (server.py --dev)" || echo "[i] 后端 API 未在运行"
  pkill -f "vite" 2>/dev/null && echo "[ok] 已停止前端 Vite dev" || echo "[i] 前端 Vite 未在运行"
fi

# -- 2) 按端口兜底（确保端口释放） -------------------------------------------
for p in "${API_PORT:-8765}" "${WEB_PORT:-5173}"; do
  pids="$(lsof -t -iTCP:"$p" -sTCP:LISTEN 2>/dev/null || true)"
  if [ -n "$pids" ]; then
    kill $pids 2>/dev/null && echo "[i] 已释放端口 $p" || true
  fi
done

echo "[ok] 开发环境已停止"
