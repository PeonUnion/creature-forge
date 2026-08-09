# 体型参数与骨骼结构的关系 — 设计评审与修正

> 背景：用户反馈「骨骼结构和体型参数有一定重复；体型参数应该是骨骼长度等、影响姿态或整体大小的参数，而非独立」。
> 本文件：现状澄清 + 业界标准/开源对照 + 修正结论 + 落地方案。
> 状态：**方案（待评审）**，未实现。

---

## 1. 现状：体型参数 = 骨骼长度缩放（其实已实现）

当前 `apply_proportions_3d()` 用 `param_chains`（定义在 `skeleton.json`）对绑定姿态 `positions_3d` 做缩放：

- `head_scale` → 把 `head` 骨沿生长方向拉长（`anchor=bottom`）
- `neck_length` → 把 `neck`→`head` 段缩放（`anchor=neck`）
- `arm_length` / `leg_length` → 肢体相对锚点 3 维缩放（`anchor=shoulder_left/hip_left`）
- `shoulder_width` → 绕脊柱中线 X 缩放整体肩宽（`anchor=center`）

**结论：体型参数在语义上就是"对绑定姿态骨骼长度的乘数"**——这正是你说的「骨骼长度」。问题不在概念，而在**数据表达的冗余/割裂**：

| 数据位置 | 存了什么 | 问题 |
|---|---|---|
| `skeleton.json` 的 `param_chains` | 参数影响哪些骨（joints）+ anchor 语义 + param 名 | 骨架内（合理） |
| `default.json` 的 `params` | 每个参数的 **label/min/max/default 又定义一遍** | ❌ 与骨架重复 |
| `preset_schema.json` | 由骨架派生的 `body_params`（再生成） | 派生（合理，但依赖前两处） |

即：**同一参数的定义（label/范围/作用骨）在 2~3 处重复**，才是"感觉重复"的根源。

---

## 2. 业界对照（标准与开源）

| 方案 | 体型如何表达 | 是否"独立于骨骼" | 参考 |
|---|---|---|---|
| **glTF 2.0**（Khronos） | **无"体型参数"标准**；体型要么是**骨骼 scale 动画**，要么是**网格 morph（BlendShape）** | 不独立（落在骨骼 scale 或网格） | glTF 规范：骨架=joints+skin，无比例概念 |
| **Roblox Avatar** | `BodyScale`（Height/Width/Depth/HeadScale/BodyTypeScale）**直接是骨骼 scale 属性** | ❌ 不独立（参数=骨骼缩放） | Roblox 文档 BodyScale |
| **Daz 3D Genesis** | 参数化体型通过**骨骼比例（proportions）**（骨长缩放 + 围度）；"Fit" 贴合不同骨长 | ❌ 不独立（骨长即参数） | Daz 参数化骨架 |
| **MB-Lab / Blender**（开源） | **宏观参数 → 修改骨骼长度**（骨长驱动），再驱动网格 | ❌ 不独立（参数=骨长） | MB-Lab 参数化人体 |
| **MakeHuman**（开源） | 宏观参数（身高/体重/肌肉…）→ **网格 Morph**（非骨骼） | ✅ 独立于骨骼（做在网格上） | MakeHuman Macro 参数 |
| **Unreal / Unity** | 体型通常 = 骨骼 scale 或 mesh morph | 视实现 | 引擎惯例 |

**关键事实**：
1. **glTF 没有"体型参数"标准**——体型在行业里要么落在**骨骼 scale**，要么落在**网格 morph**。
2. **主流"参数化角色"（Roblox/Daz/MB-Lab）都是"参数 = 骨骼长度/比例缩放"**，与你的直觉一致；参数**定义在骨架上**（骨骼的可调 scale 属性），不是独立体系。
3. **MakeHuman** 走另一条路：参数驱动**网格变形**（适合"胖瘦/肌肉"这类非骨长外观）。

---

## 3. 修正与补充结论

**你的方向基本正确**，需精确化两点：

### 3.1 不是"骨架结构里有东西是参数"，而是"结构 vs 可调语义"两个正交维度
- **骨架结构** = 拓扑（关节/父子/对称/链）+ **中性绑定姿态（默认骨长）**。它回答「有哪些骨、默认多长」。
- **体型参数** = 作用于**绑定姿态骨长**的**可调乘数**。它回答「哪些骨能一起变、怎么变」。
- 两者**不重复**：结构不含可调语义，参数不定义拓扑。**重复的是当前数据里参数的 label/min/max 被存了多份** → 统一单一来源即可。

### 3.2 体型参数 = 「骨骼长度 / 整体大小」类参数（你的补充是对的）
建议把体型参数明确为两类（对应你提的"影响姿态或整体大小"）：

| 类别 | 语义 | 例 | 现有支持 |
|---|---|---|---|
| **局部骨长** | 缩放特定骨链（长度/围度） | `head_scale` / `neck_length` / `arm_length` / `shoulder_width` | ✅ `apply_proportions` anchor 语义 |
| **整体尺度** | 整体身高 / 整体大小 / 拉伸 | `height`（整体 y 缩放）/ `overall_scale`（整体 xyz） | ⚠️ 缺"整体 scale"类型，需补 |

> 补充：这两类都**定义在骨架数据上**（骨骼/链的 scale 属性），与业界（Roblox/Daz/MB-Lab）一致。

---

## 4. 落地方案（消除重复 + 对齐业界）

### 4.1 统一体型参数的**单一来源**（消除三处重复）
- **参数定义只存一处**：`skeleton.json` 的 `param_chains`（作用骨 + anchor + param 名）+ **新增参数元数据**（label/min/max/step/default）**也放骨架**（或骨架侧派生）。
- `default.json` **不再重复存** label/min/max，只存「物种的当前默认值」（`params: {head_scale: 1.0}`），缺省由骨架派生。
- `preset_schema.json` 恒由骨架派生（现状已如此，无冗余）。
- 迁移：现有 `default.json` 的完整 params 定义 → 合并进 `param_chains`（骨架），`default.json` 降为纯值表。

### 4.2 补「整体尺度」参数类型
- `apply_proportions_3d` 增加 `anchor == "global"`（整体）语义：作用于所有关节 × 全局系数（整体 scale / 整体 y 拉伸身高）。
- 模板（humanoid/dragon）加 `overall_scale`、`height` 两个全局参数。

### 4.3 导出对齐（glTF）：体型 = 骨骼 scale（可选演进）
- 当前体型在骨架构建时算进 `positions_3d`（等价于**烘焙**），Godot/Unity 导入即可用。
- 更"标准"的替代：导出时把体型参数写成**骨架节点 scale 通道**（对应 Roblox/Daz 的骨骼 scale），运行时引擎直接缩放——但这会改变绑定姿态语义，建议保留烘焙方案（简单可靠），文档注明。

### 4.4 皮肤参数保持「网格级 Morph」路线（与骨长参数互补）
- 骨长参数（体型）：管**骨骼比例**。
- 皮肤参数（fat/muscle → `body_scale` 网格 x/z 缩放）：管**外观体态**（对应 MakeHuman 的网格 Morph 路线）。
- 两者分工清晰，**不冲突**：一个在骨上，一个在网格上。

---

## 5. 决策点

1. 是否采纳「参数定义单一来源（移到骨架 param_chains）」的迁移？（推荐 ✅，消除重复）
2. 是否补「整体尺度」参数类型（overall_scale / height）？（推荐 ✅，覆盖"整体大小/姿态"）
3. 皮肤参数（fat/muscle）是否维持网格 Morph 路线（不并入骨长参数）？（推荐 ✅，分工清晰）
