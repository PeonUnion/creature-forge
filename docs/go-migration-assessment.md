# Go 迁移评估报告（CreatureForge）

> 状态：评估，未实施。日期 2026-08-09。
> 目标：判断把 Python 后端/CLI 迁移到 Go 的收益与成本，并给出建议路径。

---

## 1. 现状与规模

| 项 | 值 |
|---|---|
| Python 核心包 `creatureforge/` | ~6500 行（13 个模块） |
| `scripts/`（构建/测试） | ~1280 行 |
| 前端 | Vue 3 + Three.js（**Go 无关，不迁移**） |
| 数据 | 外部 JSON（`data/species|presets|skins`，**不迁移**） |
| 关键模块 | skeleton3d(950) / server(837) / wizard(799) / api(720) / cli(683) / models(380) / gltf(352) / species(330) / skins(317) / skinparts(276) / updater(226) / interfaces(192) / presets(190) / motion(176) |

**纯标准库依赖**（无 numpy/pandas）：PIL（图像渲染/GIF）是唯一外部运行时依赖；HTTP 用 `http.server`。

## 2. 性能画像（实测，本机）

| 操作 | 耗时 | 判断 |
|---|---|---|
| CLI 冷启动（解释器+import） | **274 ms** | 每次命令都付，体验差 |
| LBS 顶点蒙皮 ×16 帧（4450 顶点×36 骨） | **851 ms** | 🔴 最大 CPU 热点（~53ms/帧，逐顶点矩阵运算） |
| render GIF（PIL 绘制+编码） | 284–401 ms | 🟠 中热 |
| glTF 导出 | 185 ms | 🟡 低热 |
| build_skeleton_3d | 11 ms | 🟢 低热 |
| pose_3d ×16 帧（表达式求值） | 17 ms | 🟢 低热 |
| render_pose 单帧 | 3.7 ms | 🟢 低热 |

结论：**“慢”主要来自 ① LBS 蒙皮（纯 Python 逐顶点矩阵）、② CLI 冷启动、③ 图像/GIF 编码**；骨架/动作求值本身并不慢。

## 3. Go 迁移可行性

### 适合 Go 的部分（自包含、无动态语言依赖）
- **LBS 蒙皮**（矩阵×向量，天然向量化）→ Go 收益 10–50×
- **表达式求值器**（motion._eval：递归 dict→可映射为 `interface{}`/泛型）→ 收益 5–20×
- **FK 骨骼**（树遍历+矩阵）→ 收益 10×+
- **渲染/图像**（需选图像库：`image/png`、第三方 GIF 编码、调色板量化需自写或引库）→ 收益中
- **HTTP server**（`http.Server` + goroutine 天然并发）→ 收益大（线程→goroutine）
- **单二进制分发**（Go 天生；现用 pyinstaller）→ 更简单

### 不适合/代价大的部分
- **动态 JSON 无类型 DSL**：Go 需 `map[string]interface{}` 大量类型断言，代码密度高、易错（Python 的 dict 即数据）
- **快速数据探索/向导/DSL 拼装**：Python 优势明显，Go 拖慢迭代
- **PIL 图像生态**：Go 标准库 GIF 编码弱（无调色板量化），需第三方（如 `github.com/fogleman/gg`、`gif` 库）或自写
- **测试**：`test_cli.py` 等 Python 测试需重写；E2E（Playwright 前端）不受影响
- **工具链/脚本**（migrate、gen_skin、verify 等）仍留 Python

## 4. 收益 vs 成本

| 维度 | 收益 | 成本 |
|---|---|---|
| 性能 | LBS ~50×、冷启动 ~25×、GIF 中 | 数值热路径重写 |
| 分发 | 单静态二进制、无解释器 | 构建链调整（CI 三平台） |
| 并发 | server 天然并发 | 状态管理重写 |
| 代码量 | — | ~6500 行全量重写 ≈ 1–3 人月 |
| 动态 DSL | — | Go 类型断言繁琐，可维护性下降 |
| 生态 | 部署简单 | 失去 Python 数据/图像生态 |

## 5. 建议路径（按性价比排序）

### 方案 A（推荐，改动最小）：纯 Python 优化热路径
- LBS 蒙皮用 **numpy 向量化**（顶点矩阵×骨矩阵）→ 预计 851ms → <80ms，改 1 个函数
- CLI 冷启动：减少顶层 import（延迟导入）、pyinstaller 已解决分发
- 收益高、风险零、1–2 天

### 方案 B（混合，渐进）：Go 只做计算核心
- 用 Go 重写**纯计算内核**（LBS + FK + DSL 求值 + 渲染），编译为独立二进制
- Python 侧通过 **cgo**（Python C 扩展）或 **JSON 管道/子进程** 调用
- 保留 Python 做 API/CLI/数据/向导编排（动态 DSL 部分）
- 收益：热路径 10–50×；成本：需定义 C 接口或进程协议（~2–4 周）
- 风险可控，可逐步替换

### 方案 C（全量，不推荐一步到位）
- 全部后端/CLI 迁 Go，前端/数据不变
- 收益：性能+分发+并发统一；成本：6500 行重写、DSL 重构、图像库选型、测试重写
- 建议先做 A/B 验证，确有收益再考虑 C

## 6. 结论

- **可行**：核心计算（LBS/FK/DSL/渲染）自包含，适合 Go；数据与前端不受影响。
- **不建议全量一步迁移**：动态 JSON DSL、PIL 图像生态、快速迭代是 Python 优势，Go 全量会显著增加维护成本。
- **优先**：方案 A（numpy 优化 LBS）几乎免费解决最大热点；若需更强性能与并发，走方案 B（Go 计算内核 + Python 编排），风险低、收益可量化。
