# CreatureForge

[**English**](README.md) | [简体中文](README_ZH.md)

A data-driven character asset pipeline: **pure Go backend + Vue 3 front-end**. 3D motion engine (real CMU MoCap data) + species/preset + CLI/HTTP dual entry + Web front-end (embedded into the server binary) + Godot demo.

## Highlights

- **Pure Go backend** (`gocore/`) — data stays in external JSON, code is engine-only, **no hardcoded values**.
- **3D motion engine**: skeleton topology (`skeleton.json`) + FK joint rotations, rendered from any orbit-camera angle to PNG / GIF.
- **Real MoCap**: skeleton & `walk3d` rebuilt exactly from **CMU MoCap (subject16, `16_15.bvh`)**.
- **Species / Preset**: species defines topology & actions; preset is a species instance (body + action params).
- **Vue 3 front-end** (`creatureforge/web/`) built and `//go:embed`ed into the server binary — one file runs the whole app (the CLI binary stays front-end-free).
- **Godot demo** kept at `prototype/`.

## Layout

```
gocore/                        ← Go backend
  cmd/gocore-server/           ← HTTP server (embedded front-end, single binary)
  cmd/gocore/                  ← CLI (no front-end)
  expr/                        ← motion expression DSL
  skeleton/                    ← 3D engine (FK pose / LBS skinning, matches golden data)
  internal/store/              ← data layer (species/preset/skin/motion JSON CRUD)
  internal/server/             ← HTTP API
  internal/server/static/      ← built front-end (embedded; synced by scripts/build.sh)
  internal/render/             ← rendering (PNG/GIF/sprite)
  internal/logging/ internal/config/ ← zap logging + viper config
data/                          ← data root (species / presets / skins / templates)
creatureforge/web/             ← Vue 3 front-end
scripts/
  build.sh                     ← build front-end → sync static/ → build both Go binaries
  start-dev.sh / stop-dev.sh   ← dev environment (gocore-server --dev + Vite)
  test.sh                      ← full tests (go vet/test + front-end E2E)
prototype/                     ← Godot 4.7 demo
config.yaml                    ← app config (server/data/log, read by viper)
```

## Quick start

Requirements: **Go 1.23+**, Node.js + **pnpm**.

```bash
cd creatureforge/web && pnpm install
cd ../../gocore && go test ./...
```

### Dev (hot reload)

```bash
./scripts/start-dev.sh   # gocore-server --dev:8765 (CORS) + Vite:5173 (proxy /api)
```

### Production (single binary, embedded front-end)

```bash
./scripts/build.sh                        # front-end → embed → dist/
./dist/gocore-server --port 8765 --data-dir data   # API + SPA at http://localhost:8765
```

### Tests

```bash
./scripts/test.sh         # go vet/test + front-end E2E
```

Go tests use the real `data/` and match the golden outputs (FK pose / LBS vertices within 1e-6).

### Release

GitHub Actions builds cross-platform binaries on `v*` tags (Go cross-compile): `creature-forge-server` (embedded front-end) and `creature-forge-cli` (no front-end). Release notes are generated from git history (Conventional Commits).

## Docs

- `PROJECT.md` — architecture & constraints (data-driven)
- `TODO.md` — project todo / handoff
