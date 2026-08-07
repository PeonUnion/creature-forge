# Changelog

本项目遵循 **Keep a Changelog**（[1.1.0](https://keepachangelog.com/zh-CN/1.1.0/)）
与 **Semantic Versioning**（[2.0.0](https://semver.org/lang/zh-CN/)）。

版本格式：`主版本号.次版本号.修订号`（`MAJOR.MINOR.PATCH`）

- **主版本号（MAJOR）**：不兼容的 API 变更
- **次版本号（MINOR）**：向后兼容的功能新增（小版本）
- **修订号（PATCH）**：向后兼容的缺陷修复（fix 版本）
- **预发布**：`-alpha.N` / `-beta.N` / `-rc.N` 后缀，用于正式发布前的预览版

变更日志**有取舍**：每个版本只汇总用户可见的重要变更（按 Added / Changed / Fixed / Removed 分类），
不逐条罗列提交记录。

## [Unreleased]

## [0.1.0-rc.2] - 2026-08-08

### Added
- **多动作真实动捕补全**：新增 `run3d`（跑步，subject16 16_55）、`jump3d`（跳跃，subject16 16_01）、`crawl3d`（爬行，subject111 111_03）、`idle3d`（待机呼吸，subject140 140_06）——全部来自 CMU MoCap 公开数据库真实 BVH，全关节每帧真实旋转 + 真实根位移，无任何猜测/硬编码数据
- **通用 BVH→动作转换**：`rebuild_skeleton_cmu.py --convert <id> <bvh> <cycle|jump|idle> <N>`（下包络着地检测 / 跳跃腾空窗口 / 待机呼吸段三种策略；root3d 按 CMU 世界高度基准对齐骨架）
- **动作切换过渡机制**：`motion3d_data` 支持 `transition_from`——上一动作尾帧 → 本动作首帧 逐关节线性插值生成过渡段（数据来自两个动作 JSON，非硬编码）；前端动作切换（预设预览 / 动作编辑器）自动拼接过渡帧
- **验证适配真实动捕**：`verify_motions3d` 的贴地约束改用脚趾着地 + 自然离地容忍（walk/run/crawl/idle 全 PASS），FK 动作按数据声明跳过关节方向检查（真实数据关节方向由动作决定）

### Changed
- **walk3d 统一生成流程**：与新增动作共用 `--convert`（root3d 基准修正后与其余动作一致）
- **E2E 扩至 11 用例**：新增多动作预览 + 切换过渡段测试（帧计数含过渡帧）

## [0.1.0] - 2026-08-07

### Added
- **3D 骨架引擎**：`skeleton3d.py`（FK 正向运动学 + 3D IK + 透视投影 `project3d`），任意视角渲染 PNG/GIF
- **真实 CMU 动捕数据**：subject16 `16_15.bvh` 重建骨骼与 walk 动作（骨长比例精确一致、全关节旋转照搬）
- **物种 / 预设系统**：数据驱动（schema 派生）——物种（骨架/默认参数/动作）+ 预设（体型 + 动作幅度实例）
- **统一 Api**：`interfaces.Api`（Protocol）声明全部操作，CLI 与 HTTP 共享同一实现（`api.ApiService`）
- **HTTP API**：`server.py`（物种 CRUD + 3D 骨架/动作渲染端点）
- **CLI**：`creatureforge.cli`（物种/动作/预设管理 + 渲染命令）
- **Vue 3 前端**：物种/预设管理、动作预览（播放 + GIF 导出）、3D 轨道相机
- **动作验证**：`verify_motions3d.py`（8 项检查：骨长/贴地/平滑/对称/关节/肘/坐标/参数，数据驱动）
- **全量测试**：CLI 流程化测试（`scripts/test_cli.py`）+ 前端 E2E（10 用例）
- **跨平台发布**：pyinstaller 二进制（`creature-forge-server` 嵌入 web + `creature-forge-cli`）+ GitHub Actions（`v*` tag，含预览版）

### Changed
- **数据目录可配置**：`--data-dir`（默认仓库根 `data/`），测试隔离 `test-data/`；打包运行时物种从 bundle 播种到用户目录

### Fixed
- 前端 E2E：el-select 下拉弹层在 720 视口下超界导致的点击失败（改 DOM click）
