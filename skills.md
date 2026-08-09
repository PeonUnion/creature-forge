# CreatureForge — CLI / Server 技能文档（Go 版）

数据驱动角色素材管线（Go 全量重写，无 Python）。数据始终在外部 JSON（`data/`），代码只做引擎处理。

## 二进制

Go 迁移后提供两个二进制（`scripts/build.sh` 构建，产物在 `dist/`）：

| 二进制 | 说明 | 前端 |
|---|---|---|
| `gocore-server` | HTTP API server（单二进制，`//go:embed` Vue 前端） | ✅ 含 SPA |
| `gocore` | CLI（计算内核） | ❌ 不含 |

**从 GitHub Releases 下载**（跨平台 `v*` tag 由 Actions 构建）：

```bash
# Linux x64
curl -L -o gocore-server https://github.com/PeonUnion/creature-forge/releases/latest/download/creature-forge-server-<ver>-linux-x64
curl -L -o gocore https://github.com/PeonUnion/creature-forge/releases/latest/download/creature-forge-cli-<ver>-linux-x64
chmod +x gocore-server gocore
```

## gocore-server（HTTP API + 前端）

```bash
./dist/gocore-server --port 8765 --host 127.0.0.1 --data-dir data       # 生产：API + SPA
./dist/gocore-server --dev --port 8765 --host 0.0.0.0 --data-dir data   # 开发：CORS（配合 Vite 5173）
./dist/gocore-server --config config.yaml                                # viper 读配置
```

打开 `http://localhost:8765` 即完整前端（单二进制）。

### 主要 API

| 端点 | 说明 |
|---|---|
| `GET /api/species` · `GET /api/species/<id>` | 物种列表 / 详情（骨架 + 动作） |
| `POST/PUT/DELETE /api/species...` | 物种 CRUD + `default` / `actions` / `preset_schema` |
| `GET /api/presets` · `/api/presets/new?species=` · CRUD | 预设（保存即 bake 固化骨架） |
| `GET /api/skins` · `/api/skins/new?preset=` · CRUD | 皮肤 + 部件 + 上传 |
| `GET /api/skeleton3d/<id>?data=1` | 骨架 3D 数据（WebGL） |
| `GET /api/skeleton3d/<id>?yaw=45&pitch=12` | 骨架 PNG（data_url） |
| `GET /api/motion3d/<id>?data=1` | 动作每帧关节数据 |
| `GET /api/motion3d/<id>?gif=1` · `sprite=1` · `frame=n` | 动作 GIF / sprite / 单帧 |
| `GET /api/skin3d/<id>?preset=` | LBS 蒙皮数据（网格 + 每帧顶点） |
| `GET /api/preset3d/<id>?action=walk3d` | 预设渲染（骨架 / 动作） |

## gocore（CLI 计算内核）

Go 迁移中——当前实现计算任务，全命令（species/action/preset/skin/render/upgrade）见 `TODO.md`。

```bash
gocore --data-dir data --species human --task build                       # 骨架：joints/bones/fk_tree
gocore --data-dir data --species human --action walk3d --task pose --frame 0   # 单帧姿态 {joint:[x,y,z]}
gocore --data-dir data --species human --action walk3d --task lbs --frame 0    # LBS 蒙皮 flat 顶点
gocore --stdin    # 批量帧（Python bridge 遗留的 stdin JSON 协议，Go 直接消费）
```

## 数据目录

- 默认仓库根 `data/`（物种为资产提交；预设为运行时用户数据）
- `--data-dir <dir>` 覆盖（server 与 CLI 均可）
- 配置 `config.yaml`（server/data/log），环境变量 `CFG_` 前缀覆盖（如 `CFG_DATA_ROOT`、`CFG_LOG_LEVEL=debug`）

## 开发 / 测试

```bash
./scripts/start-dev.sh    # gocore-server --dev:8765 (CORS) + Vite:5173 热更新
./scripts/test.sh         # go vet/test ./... + 前端 E2E
./scripts/build.sh        # 前端 → embed static → 两个 Go 二进制 dist/
```
