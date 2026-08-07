#!/usr/bin/env bash
# =============================================================================
# 全量测试：3D 动作验证 + CLI 流程化测试 + 前端 E2E
# 用法: ./scripts/test.sh
# =============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
PY="$ROOT/.venv/bin/python"

echo "=== [1/3] 3D 动作验证 (verify_motions3d) ==="
"$PY" "$ROOT/scripts/verify_motions3d.py"

echo
echo "=== [2/3] CLI 流程化测试 (test_cli) ==="
"$PY" "$ROOT/scripts/test_cli.py"

echo
echo "=== [3/3] 前端 E2E (playwright) ==="
( cd "$ROOT/creatureforge/web" && pnpm test:e2e )

echo
echo "[ok] 全部测试通过 ✔"
