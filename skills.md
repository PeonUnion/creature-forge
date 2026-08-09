# CreatureForge CLI 技能（Skills）

> 数据驱动角色素材管线（物种 → 预设 → 皮肤 → 导出）的命令行工具。
> 数据始终在外部 JSON（`data/species/`、`data/presets/`），CLI 只做引擎处理，不硬编码数据。

---

## 1. 安装：从 GitHub Releases 下载

CLI 以单文件二进制发布到 **GitHub Releases**（仓库 `PeonUnion/creature-forge`），资产命名：

```
creature-forge-cli-<版本>-<平台>         # Linux/macOS
creature-forge-cli-<版本>-<平台>.exe     # Windows
```

平台后缀：`linux-x64` / `linux-arm64` / `windows-x64` / `macos-x64` / `macos-arm64` …

### 方式 A：CLI 自更新（已安装后）

```bash
# 检查是否有新版本（有则退出码 2）
creatureforge upgrade --check

# 从最新正式版 Release 下载并替换自身
creatureforge upgrade

# 跳过确认直接更新；或强制重装当前最新版
creatureforge upgrade -y
creatureforge upgrade -y --force

# 更新通道：含预览版 / 指定更新源仓库
creatureforge upgrade --channel prerelease
creatureforge upgrade --repo PeonUnion/creature-forge

# 查看当前版本
creatureforge --version
```

> 源码运行（`python -m creatureforge.cli`）无法自替换，请用 `git pull`；只有打包二进制才支持 `upgrade` 下载替换。

### 方式 B：手动下载

1. 打开 https://github.com/PeonUnion/creature-forge/releases
2. 下载对应平台资产，如 `creature-forge-cli-0.1.0-rc.1-linux-x64`
3. 赋予执行权限并放入 PATH：

```bash
chmod +x creature-forge-cli-0.1.0-rc.1-linux-x64
sudo mv creature-forge-cli-0.1.0-rc.1-linux-x64 /usr/local/bin/creatureforge
creatureforge --version
```

### 数据目录

默认数据目录为仓库根 `data/`（species 为资产、presets 为运行时用户数据）；可覆盖：

```bash
creatureforge --data-dir /path/to/data species list
```

---

## 2. 快速开始

```bash
# 物种
creatureforge species list                          # 列出物种
creatureforge species show human                    # 物种详情（骨架/动作/参数）
creatureforge species schema human                  # 预设 schema（体型+动作参数）
creatureforge species templates                     # 列形态模板（可作向导起步）

# 预设
creatureforge preset new human                      # 新建空白预设表单（含 schema）
creatureforge preset create --file preset.json      # 用 JSON 创建（自动烘焙）
creatureforge preset list
creatureforge preset bake model_male                # 重新烘焙（脱离物种）

# 渲染
creatureforge render skeleton human --out skel.png
creatureforge render motion walk3d --gif --out walk.gif
creatureforge render preset model_male --action walk3d --out walk.gif
```

---

## 3. 命令参考

### 3.1 `species` — 物种管理（骨架拓扑 + 动作）

| 子命令 | 说明 |
|---|---|
| `list` | 物种列表 |
| `templates` | 列形态模板（可选起步） |
| `show <id>` | 物种详情 |
| `schema <id>` | 预设 schema（体型参数 + 各动作参数） |
| `default <id>` | 物种默认参数 |
| `create/update/delete` | JSON 建改删（`--json` / `--file`） |
| `wizard <species>` | 交互式分步向导（建物种） |
| `edit <species>` | 交互式语义化编辑已有物种 |

**向导分步（非交互，可脚本化）：**

| 子命令 | 说明 |
|---|---|
| `wizard-init <species>` | 初始化草稿（`--template` / `--title` / `--desc`） |
| `joint-add <species> <name>` | 加关节（`--parent` / `--pos x,y,z` / `--sym`） |
| `joint-rm / joint-rename / joint-parent` | 删/改名/改父级 |
| `limb-mirror <species> <source>` | 一键镜像对称肢（`--to-prefix`） |
| `chain-add <species> <name>` | 加关节链（`--joints a,b,c`） |
| `chain-rm` | 删链 |
| `pose-set <species> <joint>` | 设姿态坐标（`--pos x,y,z`） |
| `canvas <species>` | 画布/地面（`--width`/`--height`/`--floor-y`） |
| `param-add <species> <name>` | 加体型参数链（`--joints` / `--anchor` / `--label`） |
| `wizard-commit / wizard-discard` | 落盘 / 放弃草稿 |

**坐标参数化（数值=常量，表达式=计算参数，对称共享）：**

| 子命令 | 说明 |
|---|---|
| `coord-param <species> <name>` | 定义/更新坐标参数（`--label` / `--default` / `--min` / `--max` / `--step`） |
| `coord-expr <species> <joint> <axis>` | 设关节某轴表达式（`--expr`，见 §4 表达式语法） |
| `coord-extract <species>` | 对称参数提取：相同/中心互补/相反 → 共享参数（`--prefix sym`） |
| `coord-apply <species>` | 整体写坐标+参数（`--json`/`--file` 传 `{positions, params}`） |

### 3.2 `action` — 物种动作管理

| 子命令 | 说明 |
|---|---|
| `list` | 跨物种动作列表（含参数） |
| `show <species> <id>` | 动作详情 |
| `create/update/delete` | JSON 建改删 |
| `extract-params <species> <id>` | **动作参数提取**：单一 `intensity` → 整体+部位/维度多参数（摆臂/腿部/躯干/步长/起伏），写回动作 JSON |

### 3.3 `preset` — 预设（基于物种参数生成独立数据）

| 子命令 | 说明 |
|---|---|
| `list` | 预设列表 |
| `new <species>` | 新建空白表单（actions 为空，从物种动作**选择添加**） |
| `show <id>` | 预设详情（含 schema） |
| `create/update/delete` | JSON 建改删（保存时**自动烘焙**：基于物种参数生成固化骨架，脱离物种） |
| `bake <id>` | 手动重新烘焙 |

### 3.4 `skin` — 皮肤管理（基于预设）

| 子命令 | 说明 |
|---|---|
| `list / new <preset> / show / create / update / delete` | 皮肤 CRUD |
| `parts <id>` | 列出皮肤部件 |
| `part-add <id>` | 加部件（`--json`/`--file`） |
| `part-del <id> <part>` | 删部件 |

### 3.5 `render` — 3D 渲染到文件

| 子命令 | 说明 |
|---|---|
| `render skeleton <species>` | 渲染骨架（`--body a=1,b=2`） |
| `render motion <action>` | 渲染动作（`--species` 限定，`--gif` 输出动画） |
| `render preset <preset>` | 渲染预设（`--action` 指定动作） |
| `render live` | 未保存实时预览（`--species` / `--body` / `--actions` / `--action`） |
| `render skin <action>` | 蒙皮 glTF 导出（`--preset` 提供体型+动作，`--skin` 应用材质/体态） |

渲染通用参数：`--yaw` / `--pitch` / `--dist` / `--pan-x` / `--pan-y` / `--no-grid` / `--out`。

`render live --actions` 支持表达式动作参数：

```bash
creatureforge render live --species human --action walk3d \
  --body overall_scale=1.5 \
  --actions 'walk3d=intensity=mul:overall_scale*1.2' \
  --gif --out live.gif
```

### 3.6 `upgrade` — 从 GitHub Releases 自更新

见 §1「从 release 下载」。

---

## 4. 表达式语法（参数化交互）

坐标参数与动作参数值可为**数值（常量）**或**表达式（dict）**，复用同一 DSL：

| 语法 | 含义 | 表达式 |
|---|---|---|
| `const:v` | 常量 | `v` |
| `param:p` | 直接引用参数 | `{"param":"p"}` |
| `neg:p` | 取负 | `{"neg":{"param":"p"}}` |
| `mul:p*k` | 倍数 | `{"mul":[{"param":"p"},{"const":k}]}` |
| `add:p+k` | 偏移 | `{"add":[{"param":"p"},{"const":k}]}` |
| `{JSON}` | 任意表达式（如中心互补） | `{"add":[{"const":480},{"neg":{"param":"p"}}]}` |

示例：

```bash
# 骨架关节轴绑定坐标参数（对称共享）
creatureforge species coord-expr human shoulder_left x --expr 'add:shoulder_width+480'
creatureforge species coord-extract human          # 一键提取对称参数
creatureforge species coord-param human shoulder_width --label 肩宽 --default 60

# 动作参数提取 + 预设中配置表达式
creatureforge action extract-params human walk3d
# 预设 JSON 里动作参数可引用体型参数：
#   "actions": {"walk3d": {"intensity": {"mul":[{"param":"overall_scale"},{"const":1.2}]}}}
```

---

## 5. 端到端工作流示例

```bash
# 1) 建物种（向导：human 模板起步）
creatureforge species wizard-init human --template human --title 人类 --desc "基础人形"
creatureforge species joint-add human pelvis
# ... 补关节/链/姿态/参数链 ...
creatureforge species coord-extract human        # 骨架坐标参数化（对称共享）
creatureforge species wizard-commit human

# 2) 动作参数提取（单一幅度 → 部位/维度多参数）
creatureforge action extract-params human walk3d

# 3) 制作预设（基于公开人体测量数据）
creatureforge preset create --file model_male.json
creatureforge preset bake model_male

# 4) 渲染/导出
creatureforge render preset model_male --action walk3d --gif --out model_male_walk.gif
creatureforge render skin walk3d --preset model_male --skin model_male_skin --out model.glb

# 5) 更新到最新版
creatureforge upgrade
```

---

## 6. 关键约定

- **数据驱动**：所有参数/预设值在外部 JSON，CLI 只做引擎处理，不硬编码数据。
- **预设脱离物种**：保存时自动烘焙（`baked.skel3d` 固化骨架 + body/actions 数值），物种后续修改不影响已有预设。
- **预设动作独立**：预设从物种动作**选择添加**（`actions` 非默认全部），与骨骼参数 `body` 相对独立。
- **草稿不入库**：`data/species/*/draft.json` 为向导临时草稿（gitignore），`wizard-commit` 后才落盘正式文件。
- **坐标语义**：x 左右 / y 上下（高度） / z 前后（纵深）。
