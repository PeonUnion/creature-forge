# CreatureForge — 分步向导化物种/动作维护 设计方案

> 目标：把「一股脑填 JSON 维护物种/动作」改为**分步向导（wizard）**，降低门槛；
> 同时**去人类化**数据模型，支持任意幻想生物（龙/四足/蛇形/多足/飞行/水栖），不预设人形。
> 状态：**方案（待评审）**，未实现。

---

## 1. 目标与原则

1. **分步、简单化**：每个步骤只回答一个简单问题，系统生成/校验 JSON，用户不直接碰 JSON。
2. **门槛降低**：默认值智能、可视化反馈（3D 预览）、交互式编辑（拖拽/点选），而非手写结构。
3. **不局限人**：骨架/动作数据模型**去人类化**，任何生物形态都能表达。
4. **CLI 与 Web 同一套语义**：向导分步在 CLI（交互）与 Web（界面）等价实现，共用后端步进逻辑。
5. **兼容演进**：现有 `human` 数据/CMU 动捕/验证工具不破坏；向导产出仍落盘为
   `skeleton.json / default.json / preset_schema.json / actions3d/<id>.json`（引擎继续消费）。

---

## 2. 现状问题

| 环节 | 现状 | 问题 |
|---|---|---|
| 物种编辑（Web） | 7 个 JSON textarea（joints/bones/chains/param_chains/follow_chains/follow_config/default） | 一股脑 JSON，无校验无引导 |
| 物种维护（CLI） | `species create --json '{整块}'` | 同上 |
| 动作维护（CLI） | `action create --json '{整块 fk3d 旋转表}'` | 手写每关节旋转表，几乎不可用 |
| 数据模型 | `chains` 硬编码 `spine/arm_left/leg_left`、`torso_joints`/`upper_joints`、解剖学 `constraints` | 人类专属，无法表达龙/蛇/蜘蛛 |
| 动作语义 | `fk3d.rotations3d` 每关节旋转表 + 信号 DSL | 需高级知识；无「关键帧姿势」概念 |

**已通用（不需改）**：引擎层读 `bones_3d`（骨骼对）、`fk_tree`（父子）、`param_chains`（体型参数链）、
`default.positions_3d`（任意 3D 坐标）、动作 `fk3d.rotations3d`（任意关节旋转）——均不假设人形关节名。

---

## 3. 核心设计

### 3.1 分步向导总览

**物种向导（Species Wizard，5 步）**：基本信息 → 身体形态（模板）→ 骨架结构 → 默认姿态 → 体型参数

**动作向导（Action Wizard，4 步）**：基本信息 → 关键帧姿势 → 幅度参数 → 时序/循环

每一步可"上一步/下一步"，随时 3D 预览，未完成允许暂存草稿（`species/<id>/draft/`）。

### 3.2 物种向导（Web + CLI 等价）

| 步骤 | 做什么 | 用户操作 | 产出（自动生成/校验） |
|---|---|---|---|
| **1 基本信息** | ID / 名称 / 描述 / 形态标签 | 填表单 | 骨架骨架文件头 |
| **2 身体形态（模板）** | **可选形态模板起步**（humanoid/quadruped/dragon/serpent/multi_pod/floating），**或「从 0 开始（custom）」空骨架**（见 §4.4） | 点选模板 → 初始化；或选「从 0 开始」 | 初始拓扑（或空骨架）+ 默认姿态 + 动作集 |
| **3 骨架结构** | 结构化编辑骨架（替代 JSON）；custom 从空骨架构建 | 3D 预览 + 表单：**新增关节**（选父骨+命名）、**新增对称肢**（一键镜像）、**连骨骼**、**建链**；无需手写 JSON | `joints/bones_3d/chains/symmetry` 实时校验 |
| **4 默认姿态** | 摆绑定姿态 + 画布/地面 | 3D 预览里**拖拽关节**摆姿势；设置画布宽高/地面 | `default.json positions_3d` 自动落盘 |
| **5 体型参数** | 定义可调体型参数（哪些部位能调长短胖瘦） | 勾选链 → 填「名称+范围」；向导自动从链派生 | `param_chains` + `preset_schema` |

> **模板可选择，非强制**：任何模板起步后仍可自由增删改骨架；「从 0 开始」不加载任何模板，逐关节构建任意形态。
> 核心：**步骤 3/4 是可视化/表单化的**，用户永远不手写 JSON；JSON 只是落盘产物。
> 已有 JSON 的专家可保留「高级模式」（现有编辑 UI 折叠为高级入口）。

### 3.3 动作向导（Web + CLI 等价）

| 步骤 | 做什么 | 用户操作 | 产出 |
|---|---|---|---|
| **1 基本信息** | 动作 ID / 名称 / 用途（走/跑/跳/飞/游泳/攻击/待机/爬行…） | 填表单 + 选动作模板 | 动作文件头 + 由形态模板提供的骨架动作集 |
| **2 关键帧姿势** | 在 3D 预览**摆关键帧姿势**（起点/中间/终点），引擎插值生成每帧旋转 | 拖拽关节摆姿势 + 设关键帧 | `fk3d.rotations3d` 每帧旋转表（自动插值，含根位移） |
| **3 幅度参数** | 定义可调幅度（动作强度） | 勾选关节/幅度 → 填 label/min/max | `params`（intensity 等） |
| **4 时序/循环** | 帧数 / 循环类型 / 速度 / 是否可过渡 | 填表单 | `frame_count` + `signals`（循环相位） |

> 关键帧插值引擎复用现有信号 DSL（`motion.py` table/phase），向导生成 `table` 关键帧数据。

### 3.4 CLI 分步命令（镜像向导，交互式）

```bash
# 物种向导：交互式分步（逐步提示，回车确认）
creatureforge species wizard
#   1> 物种 ID: dragon
#   2> 形态模板 [dual_quadruped/serpent/multi_pod/...]: dragon   （回车默认）
#   3> 骨架：joint add chest <- spine / joint add wing_l <- shoulder_l ...
#       （或 joint list / joint rename / limb mirror left->right）
#   4> 姿态：pose set head [x,y,z] ...（或直接接受模板默认姿态）
#   5> 体型参数：chain add head 可调[head_scale 0.6-1.6] ...
#   done → 生成 skeleton.json / default.json / preset_schema.json

# 动作向导：交互式分步
creatureforge action wizard --species dragon
#   1> 动作 ID: fly3d | 用途: 飞行
#   2> 关键帧：keyframe add 0 姿势A / keyframe add 8 姿势B（或载入模板关键帧）
#   3> 幅度参数：param add wing_flap 0.5-1.5
#   4> 帧数 16 / 循环 loop
#   done → 生成 actions3d/fly3d.json

# 分步子命令也支持非交互单步（脚本化）
creatureforge species joint add dragon chest --parent spine --pos 0,-20,0
creatureforge species limb mirror dragon wing_l --to wing_r
creatureforge action keyframe set dragon fly3d 0 --pose '{"wing_l":[0.5,0,0],...}'

# 高级模式保留（兼容专家/脚本）
creatureforge species create --json '...'   # 原样保留
```

> CLI 与 Web 共用同一个**步进器（WizardService）**：`species.wizard.stepN(...)` /
> `action.wizard.stepN(...)`，两侧只是输入形态不同（终端提示 vs 表单）。避免两套逻辑漂移。

---

## 4. 幻想生物支持（去人类化）

### 4.1 形态模板（数据驱动，非硬编码）

模板存 `data/templates/<morph>.json`（不在代码里写死，可扩展任意幻想生物）：

```jsonc
{
  "morph_id": "dragon",
  "title": "龙形（四足 + 翼）",
  "tags": ["fantasy", "flying", "quadruped"],
  "limb_scheme": "quadruped + wings",       // 肢体方案
  "symmetry": true,                          // 是否左右对称
  "default_topology": {                       // 骨架模板：通用节点（见 §4.2）
    "root": "pelvis",
    "nodes": {
      "pelvis": {"parent": null},
      "chest": {"parent": "pelvis"},
      "neck": {"parent": "chest"},
      "head": {"parent": "neck"},
      "horn_l": {"parent": "head", "sym": "horn_r"},
      "wing_l": {"parent": "chest", "sym": "wing_r"},
      "leg_fl": {"parent": "pelvis", "sym": "leg_fr"},  // 前腿
      "leg_hl": {"parent": "pelvis", "sym": "leg_hr"},  // 后腿
      "tail": {"parent": "pelvis"}
    },
    "chains": {"spine": ["head","neck","chest","pelvis"], "tail": ["tail..."]}
  },
  "default_pose": { "positions_3d": {...} },  // 模板默认姿态
  "param_chains": { "head_scale": {...} },
  "actions": ["fly3d", "walk3d", "roar3d", "idle3d"]  // 该形态可用动作集
}
```

### 4.2 骨架数据模型去人类化

`skeleton.json` 通用化（**不破坏现有字段**，新增/语义放宽）：

| 字段 | 现状（人类导向） | 通用化 |
|---|---|---|
| `chains` | `spine/arm_left/leg_left` | 任意命名链（`spine/tail/leg_fl/leg_fr/wing_l...`），无固定集合 |
| 对称 | `symmetry3d`（已通用） | 保留；向导「新增对称肢」自动写 `symmetry3d` 镜像对 |
| `torso_joints`/`upper_joints` | 人类解剖分组 | 移除/废弃（引擎不依赖）；改由 `chains` + `symmetry` 表达 |
| `constraints` | 解剖（bone_length/joint_direction 人形弯膝） | 通用化：`symmetry`（对称对）+ `rigid_chains`（刚性链）保留；`joint_direction` 变为可选（非人形用任意"弯曲基准方向"） |
| `param_chains` | 派生自链 | 保留（向导第 5 步生成，任意链可定义体型参数） |
| `default.positions_3d` | 任意 3D 坐标 | 保留（向导第 4 步拖拽生成，天然支持任意形态） |
| 动作 `fk3d.rotations3d` | 任意关节旋转 | 保留（向导关键帧插值生成） |

**不变量**：任何模板产出都能被现有引擎（`build_skeleton_3d` / `pose_3d` / `skinned_vertices` /
`verify_motions3d`）消费；`verify_motions3d` 的检查项改为从通用 `constraints`（对称/刚性/弯曲基准）读取。

### 4.3 初始模板集（首批，全部数据驱动可扩展）

- `humanoid`（双足人形，= 现有人类模板）
- `quadruped`（四足兽形：四肢+尾）
- `dragon`（龙形：四足+翼+角+尾）
- `serpent`（蛇形：无肢，链式躯干+头）
- `multi_pod`（多足：N 肢，如蜘蛛/蜈蚣）
- `floating`（无肢浮空/触手：如水母/章鱼）
- `custom`（**从 0 开始**：不加载任何模板，向导逐步构建任意形态，见 §4.4）

### 4.4 「从 0 开始」（custom）向导流程

模板**可选择、非强制**；`custom` 是一等能力（不是普通模板）：

1. 步骤 2 选「从 0 开始」→ 空骨架（无关节/链/姿态/动作集）。
2. 步骤 3 骨架结构：**第一步新增「根关节」**（无父骨，如 `root`/`body`），随后逐级：
   - `add_joint <name> --parent <joint> --pos x,y,z`（指定父骨挂接）
   - `add_joint <name> --sym <partner>`（声明对称关节）
   - `mirror_limb <src> --to <prefix>`（整条链一键镜像，如 `mirror_limb arm_l --to arm_r` 自动生成 `_r` 对称 + `symmetry3d`）
   - `add_chain <name> --joints a,b,c`（命名链）
3. 步骤 4 默认姿态：从 0 开始所有关节初始在原点，拖拽/`pose set` 逐个摆开。
4. 步骤 5 体型参数：`chain add <name> 可调 [param 0.6-1.6]` 派生。

> custom 与模板共享同一套骨架操作（add_joint/mirror_limb/add_chain/pose set），
> 因此「从 0 开始」与「模板起步」的编辑能力完全一致，只是起点不同。

---

## 5. 后端支撑（WizardService）

新增 `creatureforge/wizard.py`：物种/动作分步状态机 + 模板应用 + 校验。
- `SpeciesWizard`：持步骤状态，`step(species_id, n, payload)` 逐层构建/校验 skeleton/default/schema。
- `ActionWizard`：关键帧姿势 → 插值生成 `fk3d`（复用 `motion.py` 信号 DSL 生成 table）+ 根位移。
- `templates.py` 或并入 wizard：加载 `data/templates/*.json`（模板即数据，不硬编码）。
- 暴露到 Api/HTTP：`/api/species/wizard/<step>`、`/api/action/wizard/...`（与 CLI 共用）。

---

## 6. 落地阶段

| 阶段 | 内容 | 交付 |
|---|---|---|
| **A** | 形态模板系统（数据驱动）+ **物种向导**（Web 分步 + CLI 分步交互）+ 骨架结构化编辑（去 JSON）+ 对称肢/关节/链表单 + 3D 预览 | `wizard.py` + `templates/*.json` + SpeciesView 改向导 + CLI `species wizard` |
| **B** | **动作向导**：关键帧姿势 → 插值生成 fk3d + 幅度参数 + 时序 | `ActionWizard` + SpeciesView 动作向导 + CLI `action wizard` |
| **C** | 幻想生物模板集（dragon/quadruped/serpent/multi_pod/floating）+ `verify_motions3d` 通用化 + constraints 去人类化 + 文档 | 首批非人形物种可跑通（骨架/默认姿态/动作/预设/蒙皮/导出） |

> A/B 可与现有 human 数据并行（human 保留为 humanoid 模板实例）；C 完成即"不局限人"落地。

---

## 7. 待确认决策点（已定案）

1. **模板可选 + 从 0 开始**：✅ 定案 — 模板可选择（非强制），`custom` 支持从空骨架构建任意形态（§4.4）。
2. **动作生成方式**：关键帧姿势 → 引擎插值（推荐）；BVH/动捕文件导入作为后续增强。
3. **高级 JSON 模式**：保留为折叠的「高级模式」（兼容专家/脚本）。
4. **首批模板范围**：humanoid + custom（本轮）；quadruped/dragon/serpent 后续。
5. **数据位置**：`data/templates/` 与 species 平级。
