#!/usr/bin/env bash
# =============================================================================
# 构建：前端（Vue → web/dist → 同步到 gocore embed）+ Go 二进制（server 含前端 / CLI 不含）
# 用法: ./scripts/build.sh
# =============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "=== [1/3] 构建前端 (Vue → creatureforge/web/dist) ==="
( cd "$ROOT/creatureforge/web" && pnpm build )

echo
echo "=== [2/3] 同步前端到 Go server embed (gocore/internal/server/static) ==="
STATIC="$ROOT/gocore/internal/server/static"
rm -rf "$STATIC"
mkdir -p "$STATIC"
cp -r "$ROOT/creatureforge/web/dist/." "$STATIC/"
echo "  同步完成：$(du -sh "$STATIC" | cut -f1) → $STATIC"

echo
echo "=== [3/3] 构建 Go 二进制 (→ dist/) ==="
mkdir -p "$ROOT/dist"
(cd "$ROOT/gocore" && \
  go build -o "$ROOT/dist/gocore-server" ./cmd/gocore-server && \
  go build -o "$ROOT/dist/gocore" ./cmd/gocore)
echo "  gocore-server（含 embed 前端）: $(du -h "$ROOT/dist/gocore-server" | cut -f1)"
echo "  gocore（CLI，无前端）        : $(du -h "$ROOT/dist/gocore" | cut -f1)"

echo
echo "[ok] 构建完成："
ls -la "$ROOT/dist/" | grep -E 'gocore' || true
