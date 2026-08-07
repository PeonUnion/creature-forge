#!/usr/bin/env bash
# =============================================================================
# CreatureForge 开发环境一键启动（前后端分离，热更新）
#   后端: python creatureforge/server.py --dev  (http://127.0.0.1:8765, 含 CORS)
#   前端: pnpm dev                               (http://127.0.0.1:5173, proxy /api /run -> 8765)
# 用法: ./scripts/start-dev.sh            （可设环境变量 API_PORT / WEB_PORT）
# 停止: Ctrl+C 自动清理（或 ./scripts/stop-dev.sh）
# =============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

API_PORT="${API_PORT:-8765}"
WEB_PORT="${WEB_PORT:-5173}"
PY="$ROOT/.venv/bin/python"

# -- 环境检查 ---------------------------------------------------------------
if [ ! -x "$PY" ]; then
  echo "[x] 未找到 $PY"
  echo "    请先准备虚拟环境：python3 -m venv .venv && .venv/bin/pip install -r requirements.txt"
  exit 1
fi
if [ ! -d "$ROOT/creatureforge/web/node_modules" ]; then
  echo "[x] 前端依赖未安装：cd creatureforge/web && pnpm install"
  exit 1
fi

# 端口占用检查
if lsof -iTCP:"$API_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "[x] 端口 $API_PORT 已被占用（后端 API）"
  exit 1
fi
if lsof -iTCP:"$WEB_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "[x] 端口 $WEB_PORT 已被占用（前端 Vite）"
  exit 1
fi

# -- 启动后端 API（--dev：CORS） -------------------------------------------
"$PY" "$ROOT/creatureforge/server.py" --dev --port "$API_PORT" &
API_PID=$!

# -- 启动前端 Vite dev（proxy /api /run -> API_PORT） -----------------------
( cd "$ROOT/creatureforge/web" && API_TARGET="http://127.0.0.1:$API_PORT" pnpm dev --port "$WEB_PORT" ) &
WEB_PID=$!

cleanup() {
  echo
  echo "[i] 停止开发环境..."
  kill "$API_PID" "$WEB_PID" 2>/dev/null || true
}
trap cleanup INT TERM EXIT

echo "[i] 后端 API: http://127.0.0.1:$API_PORT  (--dev, CORS)"
echo "[i] 前端 Web: http://127.0.0.1:$WEB_PORT  (proxy /api /run -> $API_PORT)"
echo "[i] 按 Ctrl+C 停止"

wait
