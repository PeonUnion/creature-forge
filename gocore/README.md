# gocore — Go 计算内核（迁移尝试）

> 分支：`feature/go-migration`。将 Python 引擎的计算热点迁移到 Go 的**可行性验证**。
> 数据仍在外部 JSON（`data/`），Go 只做引擎处理；本目录纯 Go，无第三方运行时依赖。

## 目标与范围

把 Python `creatureforge` 中**自包含、可向量化**的数值计算迁移到 Go：

| 模块 | Go 包 | 对应 Python |
|---|---|---|
| 动作表达式 DSL（求值/参数解析/信号） | `expr` | `motion._eval` / `_resolve_params` / `_build_signals` |
| 骨架构建 + FK 姿态 | `skeleton` | `build_skeleton_3d` / `pose_3d` / `solve_fk3d` / `_fk_world_pose` |
| LBS 顶点蒙皮 | `skeleton` | `skinned_vertices` |
| 独立可执行（Python 管道调用） | `cmd/gocore` | — |

未迁移（仍留 Python）：HTTP/API 编排、CLI 交互、图像渲染（PIL）、glTF 导出、向导/DSL 拼装。

## 目录结构

```
gocore/
  go.mod
  expr/
    expr.go          # Expr / ParamValue 解析 + 求值（数值|字符串|单操作 dict）
    expr_test.go     # 各 op 单元测试
  skeleton/
    skeleton.go      # 数据模型 + BuildSkeleton + Pose/SolveFK + FKWorldPose
    lbs.go           # Mesh/Weights + LBS 蒙皮
    skeleton_test.go # 与 Python golden 的一致性对比测试
    bench_test.go    # 16 帧 LBS 性能基准
    testdata/golden_human.json  # Python 生成的期望输出
  cmd/gocore/main.go # 独立 CLI：--task build|pose|lbs
```

## 构建 / 测试

```bash
cd gocore
go build ./...
go test ./...            # 单元 + 一致性对比
go test -bench=Benchmark16Frames -benchtime=10x ./skeleton/   # 性能基准
```

## 一致性验证

`golden_human.json` 由 Python 生成（`build_skeleton_3d` + `pose_3d` + `skinned_vertices`），
Go 测试逐项对比：

- `BuildSkeleton`：36 关节坐标一致（容差 1e-6）
- `Pose`：帧 0 / 帧 5 全部 36 关节一致
- `SkinnedVertices`：4450 顶点一致（0 个不匹配）

## 性能对比（本机实测）

| 操作 | Python | Go | 加速 |
|---|---|---|---|
| LBS 蒙皮 ×16 帧（4450 顶点×36 骨） | 829 ms | **52 ms** | **~16×** |

## 集成方式（Go 内核 + Python 编排）

Python 侧对数值热点 shell 到独立二进制（无需 cgo）：

```bash
# 构建单二进制
cd gocore && go build -o gocore-bin ./cmd/gocore

# 由 Python 调用（subprocess / JSON 管道）
gocore-bin --data-dir <data根> --species human --action walk3d --frame 0 --task lbs
# → JSON：{"frame":0, "vertex_count":4450, "vertices":[...]}
```

之后可在 Python 侧加 `creatureforge/core.py`，对 LBS/pose 热点检测 `gocore` 二进制并转发。

## 后续（如继续）

- [ ] 镜像 `motion` 其余路径（offsets3d 非 FK、跟随/IK）
- [ ] 渲染/GIF 迁 Go（图像库选型）
- [ ] glTF 导出迁 Go
- [ ] Python `core.py` 管道桥接 + 后端配置开关
- [ ] CI 三平台构建 + 基准回归
