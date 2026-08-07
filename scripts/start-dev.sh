#!/usr/bin/env bash
# =============================================================================
# CreatureForge 开发环境一键启动（前后端分离，热更新；局域网可访问）
#   后端: python creatureforge/server.py --dev  (0.0.0.0:8765, 含 CORS)
#   前端: pnpm dev --host                         (0.0.0.0:5173, proxy /api /run -> 8765)
# 用法: ./scripts/start-dev.sh            （可设环境变量 API_PORT / WEB_PORT / DEV_HOST）
# 停止: Ctrl+C 自动清理（或 ./scripts/stop-dev.sh）
# =============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

API_PORT="${API_PORT:-8765}"
WEB_PORT="${WEB_PORT:-5173}"
DEV_HOST="${DEV_HOST:-0.0.0.0}"
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

# -- 启动后端 API（--dev：CORS；监听 DEV_HOST 供局域网访问） -----------------
"$PY" "$ROOT/creatureforge/server.py" --dev --host "$DEV_HOST" --port "$API_PORT" &
API_PID=$!

# -- 启动前端 Vite dev（--host 暴露局域网；proxy /api /run -> API_PORT） ------
( cd "$ROOT/creatureforge/web" && API_TARGET="http://127.0.0.1:$API_PORT" pnpm dev --host --port "$WEB_PORT" ) &
WEB_PID=$!

# 局域网 IP（供提示）
LAN_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[ -n "$LAN_IP" ] || LAN_IP="localhost"

cleanup() {
  echo
  echo "[i] 停止开发环境..."
  kill "$API_PID" "$WEB_PID" 2>/dev/null || true
}
trap cleanup INT TERM EXIT

echo "[i] 后端 API: http://127.0.0.1:$API_PORT  (--dev, CORS)"
echo "[i] 前端 Web: http://127.0.0.1:$WEB_PORT  (proxy /api /run -> $API_PORT)"
echo "[i] 局域网访问: http://$LAN_IP:$WEB_PORT  （前端） / http://$LAN_IP:$API_PORT （API）"
echo "[i] 按 Ctrl+C 停止"

wait
