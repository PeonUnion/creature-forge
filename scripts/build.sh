#!/usr/bin/env bash
# =============================================================================
# 构建：前端（Vue → web/dist）+ 二进制（pyinstaller → dist/）
# 用法: ./scripts/build.sh
# =============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
PY="$ROOT/.venv/bin/python"

echo "=== [1/2] 构建前端 (Vue → creatureforge/web/dist) ==="
( cd "$ROOT/creatureforge/web" && pnpm build )

echo
echo "=== [2/2] 构建二进制 (pyinstaller → dist/) ==="
"$PY" "$ROOT/scripts/build_release.py"

echo
echo "[ok] 构建完成："
ls -la "$ROOT/dist/" | grep creature-forge || true
