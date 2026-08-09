# gocore — 后端内核（Go）

CreatureForge 的 Go 后端。数据始终在外部 JSON（`data/`），代码只做引擎处理——不硬编码任何数据。

## 模块

| 模块 | 说明 |
|---|---|
| `expr` | 动作表达式 DSL（const/param/phase/sin/table… 求值 + 参数解析 + 信号） |
| `skeleton` | 3D 引擎：骨架构建 + FK 姿态 + LBS 蒙皮（与基准数据一致，含性能基准） |
| `internal/store` | 数据层（species/preset/skin/motion 领域模型 + JSON CRUD + 引擎转换） |
| `internal/server` | HTTP API 路由 + `//go:embed` 前端 |
| `internal/render` | 渲染（轨道相机投影 + 骨架绘制 + 地面网格，PNG/GIF/sprite） |
| `internal/logging` | uber-zap 自封装日志 |
| `internal/config` | viper 读 `config.yaml` + `CFG_` 环境变量覆盖 |
| `cmd/gocore-server` | HTTP server（embed 前端，单二进制） |
| `cmd/gocore` | CLI（不含前端） |

## 目录结构

```
gocore/
  go.mod
  expr/            # Expr / ParamValue 解析 + 求值（数值|字符串|单操作 dict）+ 测试
  skeleton/        # 数据模型 + BuildSkeleton + Pose + FKWorldPose + LBS 蒙皮
                   #   + 基准一致性测试 + 16 帧性能基准
  internal/store/  # 完整领域模型 + species/presets/skins/actions CRUD + 引擎转换
  internal/server/ # 路由框架 + CRUD + 3D 数据接口 + 渲染端点 + static/（embed 前端）
  internal/render/ # 投影/绘制 + PNG/GIF/sprite 编码
  internal/logging/ internal/config/
  cmd/gocore-server/   # gocore-server --port/--host/--dev/--data-dir/--config
  cmd/gocore/          # gocore --task build|pose|lbs
```

## 构建 / 测试

```bash
cd gocore
go build ./...
go vet ./...
go test ./...        # expr/skeleton/store/server/render/config/logging（真实 data/ + 基准一致）

# 单二进制（embed 前端，先同步前端产物）
cd ../ && ./scripts/build.sh
./dist/gocore-server --port 8765 --data-dir data

# CLI 计算内核
go run ./cmd/gocore --data-dir ../data --species human --task build
go run ./cmd/gocore --data-dir ../data --species human --action walk3d --task pose --frame 0
```

## 基准一致性

`testdata/golden_human.json` 为基准期望输出，测试逐项对比（容差 1e-6）：

- `BuildSkeleton`：36 关节坐标一致
- `Pose`：帧 0 / 帧 5 全部 36 关节一致
- `SkinnedVertices`：4450 顶点一致（0 个不匹配）

## 性能

LBS 蒙皮（16 帧，4450 顶点）约 52ms，`go test ./skeleton -bench . -benchmem` 可复测。

## 待办

glTF 导出、CLI 全命令、物种向导（wizard）开发中，见 `TODO.md`。
