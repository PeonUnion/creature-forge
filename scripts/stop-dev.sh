#!/usr/bin/env bash
# =============================================================================
# 停止 CreatureForge 开发环境进程（后端 --dev 与前端 Vite dev）
# 用法: ./scripts/stop-dev.sh
# =============================================================================
set -u

pkill -f "creatureforge/server.py --dev" 2>/dev/null && echo "[ok] 已停止后端 API (server.py --dev)" || echo "[i] 后端 API 未在运行"
pkill -f "vite" 2>/dev/null && echo "[ok] 已停止前端 Vite dev" || echo "[i] 前端 Vite 未在运行"
echo "[ok] 开发环境已停止"
