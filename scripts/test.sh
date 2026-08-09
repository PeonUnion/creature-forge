#!/usr/bin/env bash
# =============================================================================
# 全量测试：Go 单测 + 一致性 + 前端 E2E
# 用法: ./scripts/test.sh
# =============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
GO="${GO:-go}"

echo "=== [1/2] Go 全量测试（expr/skeleton/store/server/render/config/logging）==="
( cd "$ROOT/gocore" && "$GO" vet ./... && "$GO" test ./... )

echo
echo "=== [2/2] 前端 E2E (playwright) ==="
( cd "$ROOT/creatureforge/web" && pnpm test:e2e )

echo
echo "[ok] 全部测试通过 ✔"
