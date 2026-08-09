# CreatureForge — 任务交接 / TODO

> 本文件用于项目内交接：新会话/伙伴接手时先读本文件 + `README_ZH.md` + `PROJECT.md`。
> 当前主线：**后端完善 + 前端 embed + 全命令 CLI**。

## 一、已完成

- **计算内核**：`gocore/expr`（表达式 DSL）+ `gocore/skeleton`（FK 姿态 / LBS 蒙皮，与基准数据一致，容差 1e-6）。
- **基础设施**：`internal/logging`（uber-zap 自封装）+ `internal/config`（viper 读 `config.yaml` + `CFG_` 环境变量覆盖）。
- **数据层**：`internal/store`（完整领域模型 + species/presets/skins/actions JSON CRUD + 引擎转换）。
- **HTTP server 数据层**：`internal/server` 全部路由——species/presets/skins/actions CRUD + 3D 数据接口（`skeleton3d`/`motion3d`/`motion3d/live`/`skin3d`，LBS 4450 顶点实测）+ 预设 bake（固化骨架 + 动作参数数值）。
- **渲染**：`internal/render`（轨道相机透视 + 地面网格 + 骨架绘制，PNG/GIF/sprite）。
- **前端 embed**：`gocore-server` 单二进制（`//go:embed` Vue 前端，含 SPA history fallback）；`gocore` CLI 不含前端。
- **测试**：全量单测全绿（expr/skeleton/store/server/render/config/logging）+ embed 静态服务测试。

## 二、当前结构

| 路径 | 说明 |
|---|---|
| `gocore/cmd/gocore-server/` | HTTP server（embed 前端，单二进制） |
| `gocore/cmd/gocore/` | CLI（不含前端） |
| `gocore/expr/` | 动作表达式 DSL |
| `gocore/skeleton/` | 3D 引擎（FK / LBS 蒙皮 / 基准） |
| `gocore/internal/store/` | 数据层（领域模型 + CRUD） |
| `gocore/internal/server/` | HTTP API + embed 静态 |
| `gocore/internal/render/` | 渲染（PNG/GIF/sprite） |
| `gocore/internal/logging/` `gocore/internal/config/` | 日志 + 配置 |
| `creatureforge/web/` | Vue 3 前端 |
| `data/` | 数据根（species/presets/skins/templates） |
| `prototype/` | Godot 4.7 demo（保留） |
| `scripts/` | build.sh / start-dev / stop-dev / test |
| `.github/workflows/release.yml` | 跨平台发布（`v*` tag） |

## 三、下一步（待办）

1. **[P0] glTF 导出**：`gocore/internal/gltf`——`GET /api/skin3d/export/<action>` 导出 .glb（骨骼 + 蒙皮 + 动作动画 + 部件）；需 `per_frame_trs`（每帧骨骼 TRS，Y-up 欧拉）。
2. **[P0] CLI 全命令**：`cmd/gocore` 扩展 species/action/preset/skin/render/upgrade 全命令。
3. **[P1] 向导 wizard**：`internal/server` 补 `wizard` 路由（init/joint/limb/chain/pose/coord/param/commit）+ 坐标参数化 + 动作参数提取（`extract-params`）。
4. **[P1] 前端 E2E 适配**：确认前端 E2E（playwright）在 server 下全通过。
5. **[P2] Godot demo 接入** 3D 动作。

## 四、关键命令

```bash
# 全量测试
cd gocore && go vet ./... && go test ./...

# 启动 server（单二进制，embed 前端）
./scripts/build.sh && ./dist/gocore-server --port 8765 --data-dir data

# 开发环境（热更新）
./scripts/start-dev.sh
```

## 五、开发注意事项

- **数据驱动**：所有数据在外部 JSON，Go 代码不硬编码关节/参数名。
- **JSON 契约**：HTTP API 返回结构稳定（前端依赖，改动需同步）。
- **性能**：LBS 蒙皮 16 帧约 52ms（`go test ./skeleton -bench .`）。
- **create_file 自动插 package 行**：写 .go 文件后需检查重复 package。
