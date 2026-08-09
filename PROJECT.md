# CreatureForge — 数据驱动角色素材管线

数据驱动角色管线：3D 动作引擎 + 物种/预设 + CLI/HTTP 双入口 + Web 前端（embed 进 server）+ Godot demo。

**核心原则（强制）**：数据始终在外部 JSON（`data/`），代码只做引擎处理，**不硬编码任何数据**。

## 架构分层（Go）

```
gocore/
  expr/                    ← 动作表达式 DSL（const/param/phase/index/frame_count/signal/
                               sin/cos/neg/rect/abs/add/sub/mul/table）
  skeleton/                ← 3D 引擎：BuildSkeleton / Pose / FKWorldPose / SkinnedVertices
                               （FK 姿态 / LBS 蒙皮，与基准数据一致）
  internal/store/          ← 数据层：完整领域模型（Species/Default/Preset/Skin/Motion/Baked）
                               + JSON CRUD（species/presets/skins/actions）+ 引擎转换
  internal/server/         ← HTTP API 路由 + //go:embed static/（前端构建产物）
  internal/render/         ← 渲染：轨道相机透视投影 + 地面网格 + 骨架绘制（PNG/GIF/sprite）
  internal/logging/        ← zap 自封装（Level/Format/Output）
  internal/config/         ← viper 读 config.yaml + CFG_ 环境变量覆盖
  cmd/gocore-server/       ← HTTP server（embed 前端，单二进制）
  cmd/gocore/              ← CLI（不含前端）
```

## 数据模型（外部 JSON，`data/`）

| 文件 | 说明 |
|---|---|
| `data/species/<id>/skeleton.json` | 骨骼拓扑：joints / chains / param_chains / params / bones_3d / fk_tree / constraints |
| `data/species/<id>/default.json` | 默认姿态/体型：positions_3d / canvas / body（CMU 真实体型） |
| `data/species/<id>/preset_schema.json` | 预设 schema（随骨架自动派生，数据驱动） |
| `data/species/<id>/actions3d/*.json` | 3D 动作（fk3d 旋转 + root3d 根位移，真实 CMU 数据） |
| `data/species/<id>/skin/` | 蒙皮基底（mesh.json / weights.json / skin_params.json） |
| `data/presets/<id>.json` | 预设（物种实例：body + actions + baked 固化骨架） |
| `data/skins/<id>.json` | 皮肤（预设实例：materials + params + parts） |
| `data/templates/` | 形态模板（humanoid / custom 从 0 开始） |

## HTTP API（`gocore-server`）

| 端点 | 说明 |
|---|---|
| `GET /api/species` | 物种列表 |
| `GET /api/species/<id>` | 物种详情（骨架 + 动作） |
| `POST/PUT/DELETE /api/species...` | 物种 CRUD + `preset_schema` / `default` / `actions` |
| `GET /api/presets` · `GET /api/presets/new?species=` | 预设列表 / 新建表单（含 schema） |
| `GET/POST/PUT/DELETE /api/presets...` | 预设 CRUD（保存即 bake 固化骨架 + 动作参数数值） |
| `GET /api/skins` · `GET /api/skins/new?preset=` | 皮肤列表 / 新建表单 |
| `GET/POST/PUT/DELETE /api/skins...` | 皮肤 CRUD + 部件 CRUD + 上传（`<id>/parts/<p>/mesh|texture`） |
| `GET /api/templates` | 形态模板列表 |
| `GET /api/skeleton3d/<id>?data=1` | 骨架 3D 数据（WebGL） / PNG 渲染 |
| `GET /api/motion3d/<id>?data=1` · `POST /api/motion3d/live` | 动作帧数据 / 实时单帧 |
| `GET /api/motion3d/<id>?gif=1` | 动作 PNG / GIF / frames / sprite |
| `GET /api/skin3d/<id>?preset=` | 蒙皮数据（网格 + 每帧 LBS 顶点） |
| `GET /api/preset3d/<id>` · `live` | 预设渲染（骨架/动作，baked 或实时） |
| 其它（非 `/api`） | embed 前端 SPA（index.html fallback） |

**3D 相机参数**（轨道相机）：`yaw`（0=front/90=side/180=back）、`pitch`（±89）、`dist`（距离倍数）、`pan_x/pan_y`、`grid`。

## CLI（`gocore`，与 HTTP 同级）

```bash
gocore --data-dir data --species human --task build                    # 骨架数据
gocore --data-dir data --species human --action walk3d --task pose --frame 0
gocore --data-dir data --species human --action walk3d --task lbs --frame 0
```

> CLI 全命令（species/action/preset/skin/render/upgrade）开发中，见 `TODO.md`。

## 启动

**生产（单二进制，embed 前端）**
```bash
./scripts/build.sh
./dist/gocore-server --port 8765 --data-dir data    # http://localhost:8765
```

**开发（热更新，前后端分离）**
```bash
./scripts/start-dev.sh    # gocore-server --dev:8765 (CORS) + Vite:5173
```

## 3D 架构（FK 关节旋转 + 真实动捕）

```
动作 walk3d.json（fk3d.rotations3d + root3d）
   + 骨架 skeleton.json（fk_tree/fk_local）+ default.json（positions_3d）
        ↓ skeleton.BuildSkeleton()
3D 骨架 → skeleton.Pose()（FK 正向运动学 + LBS 蒙皮）
        ↓ render.Project3d()（轨道相机透视）
屏幕坐标 → render.RenderPose() → PNG / GIF / sprite
```

- **骨骼与 walk 按真实 CMU 动捕重建**（subject16, `16_15.bvh`）：骨长比例精确一致、全关节旋转照搬。
- 预设 = 基于物种的实例：**体型参数**（schema 由 `param_chains` 派生）+ **动作参数**（schema 由动作 `params` 派生）。

## 当前状态

- ✅ expr DSL / FK / LBS / store / HTTP API / render / logging / config / 前端 embed
- 🔜 glTF 导出 · CLI 全命令 · 向导 wizard · 动作参数提取
