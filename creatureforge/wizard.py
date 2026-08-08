# =========================================================================
# CreatureForge — 物种分步向导（Species Wizard）
# =========================================================================
# 目标：把「一股脑填 JSON 维护物种」改为分步、简单化；支持任意形态（不局限人）。
#
# - 模板系统：data/templates/<morph>.json（可选择起步；custom = 从 0 开始空骨架）
# - 分步操作：基本信息 → 选模板/从 0 → 骨架结构（add_joint/mirror_limb/add_chain…）
#             → 默认姿态（set_pose）→ 体型参数（add_param_chain）→ commit
# - 草稿：species/<id>/draft.json（未完成暂存）；commit 落盘正式文件
#   skeleton.json（bones_3d/fk_tree/symmetry3d/chains/param_chains 自动派生）
#   default.json（positions_3d/canvas/params）+ preset_schema.json（SpeciesService 派生）
#
# 引擎兼容：产物字段（joints/bones_3d/fk_tree/chains/constraints/positions_3d）
# 与现有 build_skeleton_3d / pose_3d / verify_motions3d 一致，任意形态可消费。
# =========================================================================

from __future__ import annotations

import json
from pathlib import Path

from .species import SpeciesService

TEMPLATES_DIR_NAME = "templates"
DRAFT_NAME = "draft.json"


# --------------------------------------------------------------------------
# 模板服务（数据驱动：data/templates/*.json，可选择，非硬编码）
# --------------------------------------------------------------------------

class TemplatesService:
    """形态模板：列出/读取 data/templates/<morph>.json（含 custom 从 0 开始）。"""

    def __init__(self, root: Path) -> None:
        # root 为仓库数据根（data/）：模板在 data/templates/
        self._root = root

    def _dir(self) -> Path:
        return self._root / TEMPLATES_DIR_NAME

    def list(self) -> list[dict]:
        items: list[dict] = []
        d = self._dir()
        if not d.is_dir():
            return items
        for pf in sorted(d.glob("*.json")):
            try:
                t = json.loads(pf.read_text(encoding="utf-8"))
            except Exception:
                continue
            items.append({
                "morph_id": t.get("morph_id", pf.stem),
                "title": t.get("title", pf.stem),
                "description": t.get("description", ""),
                "tags": t.get("tags", []),
                "limb_scheme": t.get("limb_scheme", "custom"),
                "symmetry": bool(t.get("symmetry")),
                "root": t.get("root", ""),
                "joint_count": len(t.get("nodes", {})),
                "chain_count": len(t.get("chains", {})),
                "actions": t.get("actions", []),
            })
        return items

    def get(self, morph_id: str) -> dict:
        path = self._dir() / f"{morph_id}.json"
        if not path.is_file():
            raise KeyError(f"template not found: {morph_id}")
        return json.loads(path.read_text(encoding="utf-8"))


# --------------------------------------------------------------------------
# 骨架派生（nodes/chains/param_chains → skeleton.json + default.json）
# --------------------------------------------------------------------------

def derive_skeleton(draft: dict) -> dict:
    """从向导草稿派生 skeleton.json（bones_3d/fk_tree/symmetry3d/chains/param_chains）。"""
    nodes: dict = draft.get("nodes", {})
    bones_3d: list[list[str]] = []
    fk_tree: dict[str, str | None] = {}
    for name, nd in nodes.items():
        parent = nd.get("parent")
        fk_tree[name] = parent
        if parent and parent in nodes:
            bones_3d.append([parent, name])
    sym_pairs: list[list[str]] = []
    seen: set[str] = set()
    for name, nd in nodes.items():
        s = nd.get("sym")
        if s and name not in seen and nodes.get(s, {}).get("sym") == name:
            sym_pairs.append([name, s])
            seen.add(name)
            seen.add(s)
    return {
        "species_id": draft.get("species_id", ""),
        "schema": "creatureforge_species_v1",
        "title": draft.get("title", draft.get("species_id", "")),
        "description": draft.get("description", ""),
        "joints": {name: name for name in nodes},
        "chains": draft.get("chains", {}),
        "param_chains": draft.get("param_chains", {}),
        "bones_3d": bones_3d,
        "fk_tree": fk_tree,
        "constraints": {
            "schema": "creatureforge_constraints_v1",
            "description": "向导自动派生：对称对 + 刚性链（可手动扩展）",
            "symmetry3d": {"pairs": sym_pairs},
            "rigid_chains": {"chains": [list(c) for c in (draft.get("chains", {}) or {}).values()]},
        },
    }


def derive_default(draft: dict) -> dict:
    """从向导草稿派生 default.json（positions_3d/canvas/head_radius + params 派生）。"""
    dp = draft.get("default", {})
    params: dict = {}
    for cname, pc in (draft.get("param_chains", {}) or {}).items():
        pname = pc.get("param") or cname
        params[pname] = {
            "default": 1.0, "min": 0.6, "max": 1.6, "step": 0.05,
            "label": (pc.get("label") or pname),
        }
    return {
        "schema": "creatureforge_default_v1",
        "species": draft.get("species_id", ""),
        "title": draft.get("title", ""),
        "description": "向导生成的默认姿态/体型参数",
        "head_radius": float(dp.get("head_radius", 12.5)),
        "canvas": dp.get("canvas", {"width": 960, "height": 600, "floor_y": 470.0}),
        "positions_3d": dp.get("positions_3d", {}),
        "params": params,
    }


# --------------------------------------------------------------------------
# 物种向导（分步状态机 + 骨架结构操作 + 草稿）
# --------------------------------------------------------------------------

class SpeciesWizard:
    """物种分步向导：模板应用 + 骨架结构操作（add_joint/mirror_limb/add_chain/…）。

    草稿存 species/<id>/draft.json；commit() 派生并落盘 skeleton/default/preset_schema。
    """

    def __init__(self, species_root: Path, templates: TemplatesService | None = None) -> None:
        self._root = species_root
        self._templates = templates or TemplatesService(species_root.parent)

    # -- 路径 --

    def _draft_path(self, species_id: str) -> Path:
        return self._root / species_id / DRAFT_NAME

    def _load_draft(self, species_id: str) -> dict:
        """加载草稿；若无草稿但物种已存在 → 从已有 skeleton/default 加载（语义化编辑模式）。"""
        path = self._draft_path(species_id)
        if path.is_file():
            return json.loads(path.read_text(encoding="utf-8"))
        if (self._root / species_id / "skeleton.json").is_file():
            draft = self._load_existing(species_id)
            self._save_draft(path, draft)
            return draft
        raise KeyError(f"no species draft: {species_id}（先 species wizard init 或编辑已有物种）")

    def _load_existing(self, species_id: str) -> dict:
        """从已有物种（skeleton.json + default.json）加载为向导草稿，供语义化编辑。"""
        from .species import SpeciesService
        species = SpeciesService(self._root)
        skeleton = json.loads((self._root / species_id / "skeleton.json").read_text(encoding="utf-8"))
        default: dict = {}
        dp = self._root / species_id / "default.json"
        if dp.is_file():
            default = json.loads(dp.read_text(encoding="utf-8"))
        return self._draft_from_skeleton(species_id, skeleton, default,
                                         actions=[a.get("id", a.get("motion_id", ""))
                                                  for a in species.list_actions(species_id)])

    @staticmethod
    def _draft_from_skeleton(species_id: str, skeleton: dict, default: dict | None = None,
                             actions: list[str] | None = None) -> dict:
        """skeleton/default dict → 向导草稿（高级 JSON 模式与普通模式共享数据的关键）。

        兼容两种 joints 格式（human 分组列表 / 向导映射）；对称对从 constraints.symmetry3d 恢复；
        parent 关系优先 fk_tree，其次 bones_3d 推断。
        """
        joints = skeleton.get("joints", {}) or {}
        names: list[str] = []
        if joints:
            first = next(iter(joints.values()))
            if isinstance(first, list):
                names = list({j for v in joints.values() if isinstance(v, list) for j in v})
            else:
                names = list(joints)
        sym: dict[str, str] = {}
        for pair in ((skeleton.get("constraints", {}) or {}).get("symmetry3d", {}) or {}).get("pairs", []) or []:
            if len(pair) == 2:
                sym[pair[0]] = pair[1]
                sym[pair[1]] = pair[0]
        # parent：fk_tree 优先，其次 bones_3d 推断
        fk = skeleton.get("fk_tree", {}) or {}
        parent_of: dict[str, str | None] = {n: fk.get(n) for n in names}
        if not any(parent_of.values()):
            for a, b in skeleton.get("bones_3d", []) or []:
                if b in parent_of and parent_of.get(b) is None:
                    parent_of[b] = a
        default = default or {}
        nodes: dict = {}
        for n in names:
            nodes[n] = {"parent": parent_of.get(n), "sym": sym.get(n)}
        return {
            "schema": "creatureforge_wizard_v1",
            "species_id": skeleton.get("species_id", species_id),
            "morph": "existing",
            "title": skeleton.get("title", species_id),
            "description": skeleton.get("description", ""),
            "nodes": nodes,
            "chains": dict(skeleton.get("chains", {}) or {}),
            "param_chains": dict(skeleton.get("param_chains", {}) or {}),
            "default": {
                "positions_3d": dict(default.get("positions_3d", {}) or {}),
                "canvas": default.get("canvas", {"width": 960, "height": 600, "floor_y": 470.0}),
                "head_radius": default.get("head_radius", 12.5),
            },
            "actions": actions or [],
        }

    # -- 高级 JSON 模式（与普通模式共享同一份草稿 draft） --

    def files(self, species_id: str) -> dict:
        """草稿派生的完整文件（skeleton/default），供高级 JSON 页签查看/编辑。"""
        draft = self._load_draft(species_id)
        return {"species_id": species_id, "skeleton": derive_skeleton(draft), "default": derive_default(draft)}

    def save_files(self, species_id: str, skeleton: dict, default: dict | None = None) -> dict:
        """高级 JSON 页签保存：校验骨架 → 重建草稿（普通模式立即同步）。"""
        sk = dict(skeleton or {})
        if not sk.get("species_id"):
            sk["species_id"] = species_id
        if not sk.get("joints") and not sk.get("bones_3d"):
            raise ValueError("skeleton 缺少 joints/bones_3d")
        draft = self._draft_from_skeleton(species_id, sk, default or {})
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    @staticmethod
    def _save_draft(path: Path, draft: dict) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(draft, ensure_ascii=False, indent=2), encoding="utf-8")

    def _commit(self, draft: dict) -> dict:
        """草稿 → 派生 skeleton/default 并写回 draft（供 commit 落盘）。"""
        draft["skeleton"] = derive_skeleton(draft)
        draft["default"] = derive_default(draft)
        return draft

    # -- 分步操作 --

    def init(self, species_id: str, *, morph_id: str = "custom",
             title: str = "", description: str = "") -> dict:
        """第 1-2 步：基本信息 + 选模板（morph_id=custom 表示从 0 开始空骨架）。

        应用模板 → 生成草稿（nodes/chains/param_chains/default/actions）。
        返回草稿（含模板派生信息，供前端向导展示）。
        """
        species_id = (species_id or "").strip()
        if not species_id:
            raise ValueError("species_id required")
        tmpl = self._templates.get(morph_id)
        nodes: dict = {}
        chains: dict = {}
        param_chains: dict = {}
        default = dict(tmpl.get("default_pose", {}))
        actions = list(tmpl.get("actions", []))
        for name, nd in (tmpl.get("nodes", {}) or {}).items():
            nodes[name] = {"parent": nd.get("parent"), "sym": nd.get("sym")}
        chains = dict(tmpl.get("chains", {}) or {})
        for pname, pc in (tmpl.get("param_chains", {}) or {}).items():
            param_chains[pname] = dict(pc)
        draft = {
            "schema": "creatureforge_wizard_v1",
            "species_id": species_id,
            "morph": morph_id,
            "title": title or tmpl.get("title", species_id),
            "description": description or tmpl.get("description", ""),
            "nodes": nodes,
            "chains": chains,
            "param_chains": param_chains,
            "default": default,
            "actions": actions,
        }
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    def add_joint(self, species_id: str, name: str, *, parent: str | None = None,
                  pos: list[float] | None = None, sym: str | None = None) -> dict:
        """第 3 步：新增关节（第一个无 parent 即根关节）。"""
        draft = self._load_draft(species_id)
        name = (name or "").strip()
        if not name:
            raise ValueError("joint name required")
        if name in draft["nodes"]:
            raise FileExistsError(f"joint already exists: {name}")
        if parent and parent not in draft["nodes"]:
            raise KeyError(f"parent joint not found: {parent}")
        if sym and sym not in draft["nodes"]:
            # 声明对称伙伴（可后补）；不强制已存在
            pass
        draft["nodes"][name] = {"parent": parent, "sym": sym}
        draft.setdefault("default", {}).setdefault("positions_3d", {})[name] = \
            list(pos) if pos else [0.0, 0.0, 0.0]
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    def remove_joint(self, species_id: str, name: str) -> dict:
        """删除关节（连同其后代关节、相关骨骼/链/姿态）。"""
        draft = self._load_draft(species_id)
        nodes = draft["nodes"]
        if name not in nodes:
            raise KeyError(f"joint not found: {name}")
        # 收集后代（BFS）
        doomed = {name}
        queue = [name]
        while queue:
            cur = queue.pop(0)
            for n, nd in list(nodes.items()):
                if nd.get("parent") == cur and n not in doomed:
                    doomed.add(n)
                    queue.append(n)
        for n in doomed:
            nodes.pop(n, None)
            draft.setdefault("default", {}).setdefault("positions_3d", {}).pop(n, None)
        # 清理引用（sym/parent/链）
        for nd in nodes.values():
            if nd.get("parent") in doomed:
                nd["parent"] = None
            if nd.get("sym") in doomed:
                nd["sym"] = None
        chains = draft.get("chains", {})
        for cname in list(chains):
            chains[cname] = [j for j in chains[cname] if j not in doomed]
            if not chains[cname]:
                chains.pop(cname)
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    def rename_joint(self, species_id: str, old: str, new: str) -> dict:
        """重命名关节（同步链/参数链/姿态/对称引用）。"""
        draft = self._load_draft(species_id)
        nodes = draft["nodes"]
        if old not in nodes:
            raise KeyError(f"joint not found: {old}")
        if new in nodes:
            raise FileExistsError(f"joint already exists: {new}")
        # 替换所有引用
        new_nodes = {}
        for name, nd in nodes.items():
            if name == old:
                new_nodes[new] = {
                    "parent": (nd["parent"] if nd["parent"] != old else new),
                    "sym": (nd["sym"] if nd["sym"] != old else new),
                }
            else:
                new_nodes[name] = {
                    "parent": (new if nd["parent"] == old else nd["parent"]),
                    "sym": (new if nd["sym"] == old else nd["sym"]),
                }
        draft["nodes"] = new_nodes
        for c in (draft.get("chains", {}) or {}).values():
            for i, j in enumerate(c):
                if j == old:
                    c[i] = new
        for pc in (draft.get("param_chains", {}) or {}).values():
            pc["joints"] = [new if j == old else j for j in pc.get("joints", [])]
            if pc.get("anchor") == old:
                pc["anchor"] = new
        pos3d = draft.setdefault("default", {}).setdefault("positions_3d", {})
        if old in pos3d:
            pos3d[new] = pos3d.pop(old)
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    def set_parent(self, species_id: str, name: str, parent: str | None) -> dict:
        """修改关节父级（重接骨架）。"""
        draft = self._load_draft(species_id)
        nodes = draft["nodes"]
        if name not in nodes:
            raise KeyError(f"joint not found: {name}")
        if parent and parent not in nodes:
            raise KeyError(f"parent joint not found: {parent}")
        if parent == name:
            raise ValueError("parent cannot be itself")
        nodes[name]["parent"] = parent
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    def mirror_limb(self, species_id: str, source: str, *, to_prefix: str | None = None) -> dict:
        """第 3 步：一键镜像整条链（source 起点及其子树）到对称侧。

        对称命名：source 以 _left/_l 结尾 → 目标替换为 _right/_r；否则用 to_prefix。
        自动生成对称关节 + symmetry3d 对 + 姿态镜像（x 轴对称）。
        """
        draft = self._load_draft(species_id)
        nodes = draft["nodes"]
        if source not in nodes:
            raise KeyError(f"joint not found: {source}")
        # 收集 source 子树（自顶向下）
        subtree: list[str] = []
        queue = [source]
        while queue:
            cur = queue.pop(0)
            subtree.append(cur)
            for n, nd in nodes.items():
                if nd.get("parent") == cur and n not in subtree:
                    queue.append(n)
        # 目标命名：识别常见左右后缀（_left/_right/_l/_r/末尾 l/r，如 leg_fl → leg_fr）
        def _mirror_name(nm: str) -> str:
            if nm.endswith("_left"):
                return nm[:-5] + "_right"
            if nm.endswith("_right"):
                return nm[:-6] + "_left"
            if nm.endswith("_l"):
                return nm[:-2] + "_r"
            if nm.endswith("_r"):
                return nm[:-2] + "_l"
            if nm.endswith("l"):
                return nm[:-1] + "r"
            if nm.endswith("r"):
                return nm[:-1] + "l"
            if to_prefix:
                return f"{to_prefix}{nm}"
            return f"{nm}_mirror"
        # 自顶向下镜像（保证父先于子）
        new_nodes = dict(nodes)
        for nm in subtree:
            target = _mirror_name(nm)
            if target in new_nodes:
                raise FileExistsError(f"镜像目标已存在: {target}（先改名/删除）")
            src = nodes[nm]
            t_parent = None
            if src.get("parent"):
                # 父关节的镜像（若父在 subtree 中，用其 target）
                parent_mirror = _mirror_name(src["parent"]) if src["parent"] in subtree else None
                t_parent = parent_mirror if parent_mirror else (src["parent"] if src["parent"] in new_nodes else None)
            new_nodes[target] = {"parent": t_parent, "sym": nm}
            new_nodes[nm]["sym"] = target
        draft["nodes"] = new_nodes
        # 姿态镜像（x 轴对称：项目坐标 x 为左右）
        pos3d = draft.setdefault("default", {}).setdefault("positions_3d", {})
        for nm in subtree:
            target = _mirror_name(nm)
            if nm in pos3d:
                x, y, z = pos3d[nm]
                pos3d[target] = [960.0 - x, y, z]  # 以画布中心 480 对称 → 镜像 x
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    def add_chain(self, species_id: str, name: str, joints: list[str]) -> dict:
        """第 3 步：新增命名链（如 spine/tail/arm_left）。"""
        draft = self._load_draft(species_id)
        name = (name or "").strip()
        if not name:
            raise ValueError("chain name required")
        if name in draft.get("chains", {}):
            raise FileExistsError(f"chain already exists: {name}")
        missing = [j for j in joints if j not in draft["nodes"]]
        if missing:
            raise KeyError(f"joint not found: {missing}")
        draft.setdefault("chains", {})[name] = list(joints)
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    def remove_chain(self, species_id: str, name: str) -> dict:
        """删除命名链。"""
        draft = self._load_draft(species_id)
        if name not in draft.get("chains", {}):
            raise KeyError(f"chain not found: {name}")
        draft["chains"].pop(name)
        draft.get("param_chains", {}).pop(name, None)
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    def set_pose(self, species_id: str, name: str, pos: list[float]) -> dict:
        """第 4 步：设关节默认姿态坐标（3D 预览拖拽落点）。"""
        draft = self._load_draft(species_id)
        if name not in draft["nodes"]:
            raise KeyError(f"joint not found: {name}")
        if len(pos) != 3:
            raise ValueError("pos must be [x, y, z]")
        draft.setdefault("default", {}).setdefault("positions_3d", {})[name] = \
            [float(v) for v in pos]
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    # -- 姿态快速操作（旋转 / 平移，避免笔直朝天等姿势问题） --

    @staticmethod
    def _subtree(draft: dict, name: str) -> list[str]:
        """关节及其所有后代（BFS，自顶向下）。"""
        nodes = draft["nodes"]
        out: list[str] = []
        queue = [name]
        while queue:
            cur = queue.pop(0)
            out.append(cur)
            for n, nd in nodes.items():
                if nd.get("parent") == cur and n not in out:
                    queue.append(n)
        return out

    def rotate(self, species_id: str, *, axis: str = "z", angle: float = 90,
               joint: str | None = None) -> dict:
        """第 4 步快速操作：绕中心旋转姿态。

        joint 给定时旋转该关节及其子树（绕该关节当前位置）；
        否则整体旋转（绕画布中心/质心）。axis ∈ x/y/z，angle 度。
        """
        import math
        draft = self._load_draft(species_id)
        pos3d = draft.setdefault("default", {}).setdefault("positions_3d", {})
        if not pos3d:
            raise ValueError("暂无姿态坐标")
        axis = (axis or "z").lower()
        rad = math.radians(float(angle))
        c, s = math.cos(rad), math.sin(rad)
        if joint:
            if joint not in draft["nodes"]:
                raise KeyError(f"joint not found: {joint}")
            names = self._subtree(draft, joint)
            cx, cy, cz = pos3d.get(joint, [0.0, 0.0, 0.0])
        else:
            names = list(pos3d)
            cx = sum(v[0] for v in pos3d.values()) / len(pos3d)
            cy = sum(v[1] for v in pos3d.values()) / len(pos3d)
            cz = sum(v[2] for v in pos3d.values()) / len(pos3d)
        for n in names:
            x, y, z = pos3d[n]
            dx, dy, dz = x - cx, y - cy, z - cz
            if axis == "z":
                nx, ny, nz = dx * c - dy * s + cx, dx * s + dy * c + cy, z
            elif axis == "y":
                nx, ny, nz = dx * c + dz * s + cx, y, -dx * s + dz * c + cz
            elif axis == "x":
                nx, ny, nz = x, dy * c - dz * s + cy, dy * s + dz * c + cz
            else:
                raise ValueError(f"axis must be x/y/z: {axis}")
            pos3d[n] = [round(nx, 2), round(ny, 2), round(nz, 2)]
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    def translate(self, species_id: str, *, dx: float = 0, dy: float = 0,
                  dz: float = 0, joint: str | None = None) -> dict:
        """第 4 步快速操作：平移姿态（joint 给定时平移该关节及其子树，否则整体）。"""
        draft = self._load_draft(species_id)
        pos3d = draft.setdefault("default", {}).setdefault("positions_3d", {})
        names = self._subtree(draft, joint) if joint else list(pos3d)
        for n in names:
            if n not in pos3d:
                continue
            x, y, z = pos3d[n]
            pos3d[n] = [round(x + float(dx), 2), round(y + float(dy), 2), round(z + float(dz), 2)]
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    def set_canvas(self, species_id: str, *, width: float | None = None,
                   height: float | None = None, floor_y: float | None = None) -> dict:
        """第 4 步：画布/地面设置。"""
        draft = self._load_draft(species_id)
        canvas = draft.setdefault("default", {}).setdefault("canvas", {})
        if width is not None:
            canvas["width"] = float(width)
        if height is not None:
            canvas["height"] = float(height)
        if floor_y is not None:
            canvas["floor_y"] = float(floor_y)
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    def add_param_chain(self, species_id: str, name: str, joints: list[str], *,
                        anchor: str | None = None, label: str | None = None) -> dict:
        """第 5 步：新增体型参数链（可调部位）。"""
        draft = self._load_draft(species_id)
        name = (name or "").strip()
        if not name:
            raise ValueError("param chain name required")
        missing = [j for j in joints if j not in draft["nodes"]]
        if missing:
            raise KeyError(f"joint not found: {missing}")
        draft.setdefault("param_chains", {})[name] = {
            "param": name, "joints": list(joints), "anchor": anchor or joints[-1], "label": label or name,
        }
        self._save_draft(self._draft_path(species_id), draft)
        return self._view(draft)

    def commit(self, species_id: str) -> str:
        """完成向导：草稿 → 落盘 skeleton.json + default.json + preset_schema.json。"""
        draft = self._load_draft(species_id)
        if not draft.get("nodes"):
            raise ValueError("骨架为空：请先新增至少一个关节（species wizard joint add）")
        skeleton = derive_skeleton(draft)
        default = derive_default(draft)
        species = SpeciesService(self._root)
        # update 对新物种/旧物种均有效（mkdir + 写 skeleton + 派生 preset_schema + actions3d/）
        species.update(species_id, skeleton)
        species.save_default(species_id, default)
        self._draft_path(species_id).unlink(missing_ok=True)
        return species_id

    def discard(self, species_id: str) -> str:
        """放弃向导草稿（不落盘）。"""
        self._draft_path(species_id).unlink(missing_ok=True)
        return species_id

    # -- 视图 --

    def _view(self, draft: dict) -> dict:
        """草稿视图（供前端/CLI 展示当前状态）。"""
        nodes = draft.get("nodes", {})
        return {
            "schema": draft.get("schema"),
            "species_id": draft.get("species_id", ""),
            "morph": draft.get("morph", ""),
            "title": draft.get("title", ""),
            "description": draft.get("description", ""),
            "joint_count": len(nodes),
            "bone_count": sum(1 for nd in nodes.values() if nd.get("parent")),
            "chain_count": len(draft.get("chains", {})),
            "param_chain_count": len(draft.get("param_chains", {})),
            "nodes": {n: dict(nd) for n, nd in nodes.items()},
            "chains": draft.get("chains", {}),
            "param_chains": draft.get("param_chains", {}),
            "positions_3d": draft.get("default", {}).get("positions_3d", {}),
            "canvas": draft.get("default", {}).get("canvas", {}),
            "head_radius": draft.get("default", {}).get("head_radius", 12.5),
            "actions": draft.get("actions", []),
            "steps": {
                "1_info": bool(draft.get("species_id")),
                "2_morph": bool(draft.get("morph")),
                "3_skeleton": bool(nodes),
                "4_pose": bool(draft.get("default", {}).get("positions_3d")),
                "5_params": bool(draft.get("param_chains")),
            },
        }

    def get(self, species_id: str) -> dict:
        return self._view(self._load_draft(species_id))
