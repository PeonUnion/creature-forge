#!/usr/bin/env bash
# =============================================================================
# CreatureForge 开发环境一键启动（前后端分离，热更新；局域网可访问）
#   后端: gocore-server --dev  (0.0.0.0:8765, 含 CORS，Go 单二进制 embed 前端)
#   前端: pnpm dev --host         (0.0.0.0:5173, proxy /api -> 8765)
# 用法: ./scripts/start-dev.sh            （可设环境变量 API_PORT / WEB_PORT / DEV_HOST）
# 停止: Ctrl+C 自动清理（或 ./scripts/stop-dev.sh）
# Runtime 信息写入 .dev/runtime.json（pid/端口/IP/日志路径），供 stop 精确读取。
# 日志: .dev/api.log / .dev/web.log
# =============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

API_PORT="${API_PORT:-8765}"
WEB_PORT="${WEB_PORT:-5173}"
DEV_HOST="${DEV_HOST:-0.0.0.0}"
GO="${GO:-go}"
DEV_DIR="$ROOT/.dev"
mkdir -p "$DEV_DIR"

# -- 环境检查 ---------------------------------------------------------------
if ! command -v "$GO" >/dev/null 2>&1; then
  echo "[x] 未找到 Go 工具链（$GO）。请先安装 Go 1.23+：https://go.dev/dl/"
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

# -- 构建并启动后端 API（--dev：CORS；监听 DEV_HOST 供局域网访问） ------------
echo "[*] 构建 gocore-server ..."
( cd "$ROOT/gocore" && "$GO" build -o "$DEV_DIR/gocore-server" ./cmd/gocore-server )
"$DEV_DIR/gocore-server" --dev --host "$DEV_HOST" --port "$API_PORT" \
  --data-dir "$ROOT/data" >"$DEV_DIR/api.log" 2>&1 &
API_PID=$!

# -- 启动前端 Vite dev（--host 暴露局域网；proxy /api -> API_PORT） -----------
( cd "$ROOT/creatureforge/web" && API_TARGET="http://127.0.0.1:$API_PORT" pnpm dev --host --port "$WEB_PORT" ) \
  >"$DEV_DIR/web.log" 2>&1 &
WEB_PID=$!

# 局域网 IP（供提示 + runtime 记录）
LAN_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[ -n "$LAN_IP" ] || LAN_IP="localhost"

# -- 记录 runtime 信息（.dev/runtime.json） ----------------------------------
cat > "$DEV_DIR/runtime.json" <<EOF
{
  "api_pid": $API_PID,
  "web_pid": $WEB_PID,
  "api_port": $API_PORT,
  "web_port": $WEB_PORT,
  "host": "$DEV_HOST",
  "lan_ip": "$LAN_IP",
  "api_log": "$DEV_DIR/api.log",
  "web_log": "$DEV_DIR/web.log",
  "backend": "gocore-server"
}
EOF

# -- 清理（Ctrl+C 或 kill 本脚本） -------------------------------------------
cleanup() {
  echo
  echo "[*] 停止开发环境 ..."
  kill "$API_PID" "$WEB_PID" 2>/dev/null || true
  rm -f "$DEV_DIR/runtime.json"
}
trap cleanup EXIT INT TERM

echo
echo "✅ CreatureForge 开发环境已启动（backend: gocore-server / frontend: Vite）"
echo "   前端:  http://localhost:$WEB_PORT  （局域网: http://$LAN_IP:$WEB_PORT）"
echo "   后端:  http://localhost:$API_PORT/api  （日志: $DEV_DIR/api.log）"
echo "   Ctrl+C 停止"
echo
wait
