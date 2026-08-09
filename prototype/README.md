# AssetsLab Minimal Prototype（Godot 4.7）

数据驱动角色素材管线的 **Godot 运行时验证 demo**：四方向移动 + 八帧走路动画 + 炸弹玩法。UI-free，以 headless 命令行验证为主。

## 玩法

- 四方向移动（WASD / 方向键）
- 八帧走路动画（骨架优先管线）
- 与竞技场墙碰撞
- 一颗炸弹（短引信 + 爆炸反馈）

## 运行

```bash
# 直接运行（回退到内置 runtime 资产）
godot --path prototype

# 指定导出的 demo 制品包（atlas + runtime_manifest.json + walk gif）
godot --path prototype -- --artifacts dist/orc

# 皮肤演示模式
godot --path prototype -- --skin-mode --skin-pack=<name>
```

`prototype/scripts/player.gd` 启动时读取 `runtime_manifest.json` 与
`atlas/<layer>/walk_row<row>_frame<frame>.png`（成功打印
`ARTIFACTS_LOADED dir=…`）；无 `--artifacts` 时回退到
`assets/characters/rebuild_atlas_v1_runtime/male/` 内置运行时。

## 分层运行时

独立层级栈：`Feet` + `LowerBody` + `Arms` + `Torso` + `Ear` + 男女 `Head` + `Face`。
第一版外观无鼻/口；头发与衣物为后续层。外观种子决定前脸耳朵/眼/腮红的确定性选择。

## 测试

`prototype/tests/*.gd` 为 Godot 单测（骨架管线各阶段：front/side/back 的
skeleton→legs→pelvis→arms），以 Godot 测试框架运行。

测试产物（PNG 帧 + GIF）写入 `prototype/test_output/`（gitignore）。

## 与主管线关系

`prototype/` 是纯 Godot 预览 demo：由 `creatureforge` 前端/后端生成的
`dist/<workflow_id>/` 制品包（layered atlas + manifest + gif）或烘焙皮肤 demo
驱动，用于验证骨架管线与皮肤体系在真实引擎中的表现。
