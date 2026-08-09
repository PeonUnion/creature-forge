#!/usr/bin/env bash
# =============================================================================
# 停止 CreatureForge 开发环境（读 .dev/runtime.json 精确停止，端口兜底）
# =============================================================================
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEV_DIR="$ROOT/.dev"
RF="$DEV_DIR/runtime.json"

if [ -f "$RF" ]; then
  API_PID="$(sed -n 's/.*"api_pid": *\([0-9]*\).*/\1/p' "$RF" | head -1)"
  WEB_PID="$(sed -n 's/.*"web_pid": *\([0-9]*\).*/\1/p' "$RF" | head -1)"
  [ -n "$API_PID" ] && kill "$API_PID" 2>/dev/null && echo "[*] 已停止后端 (pid $API_PID)"
  [ -n "$WEB_PID" ] && kill "$WEB_PID" 2>/dev/null && echo "[*] 已停止前端 (pid $WEB_PID)"
  rm -f "$RF"
fi

# 端口兜底：仍占用则按端口清理
for PORT in "${API_PORT:-8765}" "${WEB_PORT:-5173}"; do
  PIDS="$(lsof -tiTCP:"$PORT" -sTCP:LISTEN 2>/dev/null || true)"
  [ -n "$PIDS" ] && kill $PIDS 2>/dev/null && echo "[*] 端口 $PORT 进程已清理"
done

echo "[ok] 开发环境已停止"
