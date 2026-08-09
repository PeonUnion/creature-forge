# CreatureForge（中文说明）

[English](README.md) | [**简体中文**](README_ZH.md)

数据驱动角色素材管线：**Go 全量重写（无 Python）**。3D 动作引擎（CMU 动捕真实数据）+ 物种/预设 + CLI/HTTP 双入口 + Web 前端预览（Vue 3，embed 进 server）+ Godot demo。

## 项目内容

- **纯 Go 后端**（`gocore/`）：从 Python 全量迁移而来——数据仍全部在外部 JSON，代码只做引擎处理，**不硬编码任何数据**。
- **3D 动作引擎**：骨骼拓扑（`skeleton.json`）+ FK 关节旋转驱动动作，任意视角（轨道相机）渲染 PNG / GIF。
- **真实动捕**：骨骼与 `walk3d` 完全按 **CMU MoCap（subject16, `16_15.bvh`）** 数据重建——骨长比例精确一致、全关节旋转照搬。
- **物种 / 预设**：物种定义骨骼拓扑与动作；预设是基于物种的实例（调体型 + 动作幅度）。
- **Web 前端**（Vue 3）：物种 / 预设独立入口；动作预览（播放 + 导出 GIF）；3D 相机。**前端构建产物 `//go:embed` 进 server 二进制**——单文件即可运行（CLI 二进制不含前端）。
- **Godot demo**：`prototype/`（Godot 4.7 工程）保留。

## 目录结构

```
gocore/                                            ← Go 全量后端（数据驱动引擎 + API + 渲染）
  cmd/gocore-server/                               ← HTTP server（//go:embed 前端，单二进制）
  cmd/gocore/                                      ← CLI（不含前端）
  expr/                                            ← 动作表达式 DSL（const/param/sin/table…）
  skeleton/                                        ← 3D 引擎（FK 姿态 / LBS 蒙皮，与 Python golden 一致）
  internal/store/                                  ← 数据层（species/preset/skin/motion JSON CRUD）
  internal/server/                                 ← HTTP API 路由（镜像原 server.py 契约）
  internal/server/static/                          ← 前端构建产物（embed，由 scripts/build.sh 同步）
  internal/render/                                 ← 渲染（标准库替代 Pillow：PNG/GIF/sprite）
  internal/logging/  internal/config/              ← zap 日志封装 + viper 读 config.yaml
data/                                              ← 数据目录（默认仓库根 data/，--data-dir 可覆盖）
  species/human/                                   ← 物种：骨骼 + 默认体型 + 动作
    skeleton.json / preset_schema.json / default.json
    actions3d/walk3d.json                          ← 3D 动作（FK 旋转，真实 CMU 数据）
    skin/                                          ← 蒙皮基底（mesh/weights/skin_params）
  presets/  skins/  templates/                     ← 预设 / 皮肤 / 形态模板
creatureforge/web/                                 ← Vue 3 前端（唯一保留的 Python 时代模块）
scripts/
  build.sh                                         ← 前端构建 → 同步 static/ → 构建两个 Go 二进制
  start-dev.sh / stop-dev.sh                       ← 开发环境（gocore-server --dev + Vite 热更新）
  test.sh                                          ← 全量测试（go vet/test + 前端 E2E）
prototype/                                         ← Godot 4.7 demo（保留）
config.yaml                                        ← 应用配置（server/data/log，viper 读取）
```

> **数据目录**：默认仓库根 `data/`（物种为资产提交；预设为运行时用户数据）。后端可用 `--data-dir <dir>` 覆盖。
> **前端 embed**：`scripts/build.sh` 把 `creatureforge/web/dist` 同步到 `gocore/internal/server/static/` 后 `go build`，`gocore-server` 单二进制同时提供 API + SPA。

## 快速开始

### 环境要求

- **Go 1.23+**（`gocore/` 全量后端）
- Node.js + **pnpm**（前端）

```bash
# 前端依赖
cd creatureforge/web && pnpm install
# Go 后端测试
cd ../../gocore && go test ./...
```

## 开发辅助脚本

**Linux / macOS（bash）**

```bash
./scripts/start-dev.sh     # 一键启动开发环境（gocore-server --dev:8765 + 前端 Vite:5173，热更新，局域网可访问，Ctrl+C 停止）
./scripts/stop-dev.sh      # 停止开发环境
./scripts/test.sh          # 全量测试（go vet/test + E2E）
./scripts/build.sh         # 构建（前端 → embed static → 两个 Go 二进制 dist/）
```

**Windows**

```bat
scripts\start-dev.bat      # 一键启动（gocore-server + Vite）
scripts\stop-dev.bat       # 停止（按端口清理）
scripts\test.bat           # 全量测试
```

## 测试

```bash
./scripts/test.sh          # 全量：Go 单测 + 前端 E2E

# 或分别运行：
cd gocore && go vet ./... && go test ./...   # expr/skeleton/store/server/render/config/logging
cd creatureforge/web && pnpm test:e2e        # 前端 E2E
```

Go 单测覆盖（数据用真实 `data/`，与 Python golden 一致）：
- `skeleton` — FK 姿态 / LBS 蒙皮与 Python 输出逐顶点一致（容差 1e-6）+ 性能基准（~16×）
- `store` — 全物种/预设/动作加载 + 引擎链路 + CRUD
- `server` — HTTP API 集成测试（httptest 对真实数据）+ embed 静态服务
- `render` — PNG/GIF/sprite 渲染可解码验证
- `expr` / `logging` / `config` — DSL / 日志 / 配置

### 生产

```bash
./scripts/build.sh                               # 构建前端 → embed → 两个二进制 dist/
./dist/gocore-server --port 8765 --data-dir data # 单二进制：API + 前端 SPA
# 打开 http://localhost:8765
```

### 发布（跨平台二进制）

基于 **Go 交叉编译**，GitHub Actions 在 **`v*` tag** 时跨平台发布：

```bash
# 本地构建（需已构建前端）
./scripts/build.sh    # 产物 dist/gocore-server / dist/gocore
```

**发布流程（SemVer + Conventional Commits，无需手动 changelog）**：

1. 提交遵循 Conventional Commits（`feat:` / `fix:` / `refactor:` ...）
2. 打 tag 触发 Actions 构建 → 自动生成 GitHub Release：
   - `v1.0.0` — 正式版 / `v0.4.0` — 小版本 / `v0.3.1` — fix / `v0.4.0-rc.1` — 预览版（自动 Pre-release）
3. Release Notes 由 git 历史自动生成（Conventional Commits 按 Added / Fixed / Changed 分组）

### 开发（热更新，前后端分离）

- 终端 1 — 后端 API（`--dev` 追加 CORS 头）：`./scripts/start-dev.sh`（或 `cd gocore && go run ./cmd/gocore-server --dev --port 8765`）
- 终端 2 — 前端 Vite dev（proxy `/api` → 8765）：`cd creatureforge/web && pnpm run dev`（http://localhost:5173）

### CLI（不启动 server，与 HTTP 同级）

```bash
gocore --data-dir data --species human --task build          # 骨架数据
gocore --data-dir data --species human --action walk3d --task pose --frame 0   # 动作帧姿态
gocore --data-dir data --species human --action walk3d --task lbs --frame 0    # LBS 蒙皮顶点
```

> Go CLI 全命令（species/action/preset/skin/render/upgrade）迁移中，见 `TODO.md`。

## 3D 架构（FK 关节旋转 + 真实动捕）

```
动作 walk3d.json（fk3d.rotations3d：全关节每帧真实旋转 table + root3d 根位移）
   + 骨架 skeleton.json（fk_tree/fk_local 骨向量）+ default.json（positions_3d 体型）
        ↓ skeleton.BuildSkeleton()
3D 骨架 {joint: [x,y,z]}
        ↓ skeleton.Pose()  →  FK 正向运动学（父累积旋转）+ LBS 蒙皮
3D 姿势
        ↓ render.Project3d()（yaw/pitch/dist/zoom 透视）
屏幕坐标 → render.RenderPose() → PNG / GIF / sprite
```

- **骨骼与 walk 按真实 CMU 动捕重建**（subject16, `16_15.bvh`）：骨长比例精确一致、全关节旋转照搬。
- 3D 相机 = 轨道相机（绕模型中心）：`yaw/pitch/dist/zoom`；前端支持拖拽旋转 + 快捷按钮。
- 预设 = 基于物种的实例：**体型参数**（schema 由骨架 `param_chains` 派生）+ **动作参数**（schema 由动作 JSON `params` 派生）。

## 当前状态与路线图

- ✅ Go 全量迁移：expr DSL / FK 姿态 / LBS 蒙皮 / 数据层 / HTTP API / 渲染（Pillow 替代）/ 日志 / 配置
- ✅ 前端 embed：`gocore-server` 单二进制（API + SPA），CLI 不含前端
- ✅ HTTP API 契约与原 Python server 一致（前端未改动）
- 🔜 glTF 导出（`export_glb`）；Go CLI 全命令；向导 wizard 迁移

## 相关文档

- `PROJECT.md` — 当前架构与约束（强制数据驱动）
- `TODO.md` — Go 迁移待办 / 交接
- `docs/go-migration-assessment.md` — Go 迁移评估
