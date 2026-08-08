# =========================================================================
# CreatureForge — 皮肤模块（Skin）
# =========================================================================
# 皮肤 = 基于预设的外观实例：一组皮肤参数（肤色/体脂/肌肉等）+ 材质参数
# （albedo/roughness/metallic）。预设基于物种（species + body 体型 + actions 动作参数），
# 皮肤基于预设：schema 由预设的物种提供（species/<id>/skin/skin_params.json，含 body_scale
# 体态公式），皮肤只需提供参数值，界面按 schema 渲染参数面板。网格/权重为物种基底
# （skin/mesh.json + skin/weights.json），皮肤定义只存"外观覆盖"，不重复存储网格数据。
#
# 层级：Species → Preset（体型 + 动作参数）→ Skin（材质 + 皮肤参数）→ 导出（蒙皮 + 动作）
#
# 目录结构：
#   skins/<skin_id>.json        — 皮肤定义（值），schema 由预设派生
#   presets/<preset_id>.json    — 预设定义（species + body + actions）
#   species/<id>/skin/          — 物种皮肤基底（mesh/weights/skin_params）
# =========================================================================

from __future__ import annotations

import json
from pathlib import Path

from .models import Skin, SkinSummary
from .presets import PresetService
from .species import SpeciesService

SKIN_SCHEMA = "creatureforge_skin_v1"
DEFAULT_MATERIALS = {"albedo": "#c9a58c", "roughness": 0.6, "metallic": 0.0}


class SkinService:
    """皮肤模块：管理 skins/<id>.json，派生完整 schema（预设物种皮肤参数 + 材质 + 体态）。"""

    def __init__(self, root: Path, species: SpeciesService, presets: PresetService | None = None) -> None:
        self._root = root
        self._species = species
        self._presets = presets

    # -- 内部路径 --

    def _path(self, skin_id: str) -> Path:
        return self._root / f"{skin_id}.json"

    @staticmethod
    def _save(path: Path, data: dict) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")

    def _species_of(self, preset_id: str) -> str:
        """预设 → 物种（皮肤基于预设，schema 由预设的物种皮肤定义派生）。"""
        if not self._presets:
            raise ValueError("preset required (skin is based on a preset)")
        p = self._presets.get(preset_id)
        species = (p or {}).get("species", "")
        if not species:
            raise ValueError(f"preset has no species: {preset_id}")
        return species

    # -- schema（数据驱动：预设物种皮肤参数 + 默认材质 + 体态公式） --

    def build_skin_schema(self, preset_id: str) -> dict:
        """派生皮肤完整 schema（供前端参数面板渲染）。

        - preset: 皮肤基于的预设 ID
        - species: 预设的物种（网格/权重基底）
        - params: 皮肤参数（物种 skin/skin_params.json，如肤色/体脂/肌肉）
        - materials: 默认材质（albedo/roughness/metallic）
        - body_scale: 体态公式（皮肤参数 → 网格 x/z 缩放，数据驱动）
        """
        params: dict = {}
        materials: dict = dict(DEFAULT_MATERIALS)
        body_scale: dict | None = None
        try:
            species_id = self._species_of(preset_id)
        except Exception:
            species_id = ""
        try:
            p = json.loads((self._species._root / species_id / "skin" / "skin_params.json").read_text(encoding="utf-8"))
            params = p.get("params", {}) or {}
            materials = {**materials, **(p.get("materials", {}) or {})}
            body_scale = p.get("body_scale")
        except Exception:
            pass  # 预设物种无皮肤参数 → 空 schema
        return {"preset": preset_id, "species": species_id, "params": params, "materials": materials,
                "body_scale": body_scale}

    @staticmethod
    def body_scale(skin_params: dict | None, bs_schema: dict | None) -> float | None:
        """由皮肤参数 + body_scale 公式派生网格 x/z 缩放因子（数据驱动，不硬编码）。

        scale = base + Σ coef*(value - offset)，clamp [min, max]。
        无公式定义时返回 None（不缩放）。
        """
        if not bs_schema:
            return None
        s = float(bs_schema.get("base", 1.0))
        p = skin_params or {}
        for k, cfg in (bs_schema.get("params") or {}).items():
            v = float(p.get(k, cfg.get("offset", 0.0)))
            s += float(cfg.get("coef", 0.0)) * (v - float(cfg.get("offset", 0.0)))
        return min(float(bs_schema.get("max", 1.6)), max(float(bs_schema.get("min", 0.6)), s))

    # -- CRUD --

    def list(self) -> list[SkinSummary]:
        items: list[SkinSummary] = []
        if not self._root.is_dir():
            return items
        for pf in sorted(self._root.glob("*.json")):
            try:
                d = json.loads(pf.read_text(encoding="utf-8"))
                items.append({
                    "skin_id": d.get("skin_id", pf.stem),
                    "title": d.get("title", pf.stem),
                    "description": d.get("description", ""),
                    "preset": d.get("preset", ""),
                    "species": d.get("species", ""),
                })
            except Exception:
                continue
        return items

    def get(self, skin_id: str) -> dict:
        """皮肤详情 = 皮肤值 + 完整 schema（预设物种皮肤参数 + 材质 + 体态）。"""
        path = self._path(skin_id)
        if not path.is_file():
            raise KeyError(f"skin not found: {skin_id}")
        skin = json.loads(path.read_text(encoding="utf-8"))
        schema = self.build_skin_schema(skin.get("preset", ""))
        return {**skin, "schema_info": schema}

    def new_schema(self, preset_id: str) -> dict:
        """新建皮肤的空白表单：值 = 预设物种默认 + 完整 schema。"""
        schema = self.build_skin_schema(preset_id)
        defaults = {k: v.get("default", 0.0) for k, v in schema["params"].items()}
        return {
            "schema": SKIN_SCHEMA,
            "skin_id": "",
            "preset": preset_id,
            "species": schema["species"],
            "title": "",
            "description": "",
            "materials": dict(schema["materials"]),
            "params": defaults,
            "schema_info": schema,
        }

    def create(self, data: Skin) -> str:
        sid = (data.get("skin_id") or "").strip()
        if not sid:
            raise ValueError("skin_id required")
        if not data.get("preset"):
            raise ValueError("preset required")
        if self._path(sid).exists():
            raise FileExistsError(f"skin already exists: {sid}")
        data = dict(data)
        data.pop("schema_info", None)  # schema 由预设派生，不持久化
        data.setdefault("schema", SKIN_SCHEMA)
        # 补全 species（预设的物种，冗余便于查询/渲染）
        if not data.get("species"):
            data["species"] = self._species_of(data["preset"])
        self._save(self._path(sid), data)
        return sid

    def update(self, skin_id: str, data: Skin) -> str:
        path = self._path(skin_id)
        if not path.is_file():
            raise KeyError(f"skin not found: {skin_id}")
        data = dict(data)
        data.pop("schema_info", None)
        data.setdefault("schema", SKIN_SCHEMA)
        data["skin_id"] = data.get("skin_id") or skin_id
        if data.get("preset") and not data.get("species"):
            data["species"] = self._species_of(data["preset"])
        self._save(path, data)
        return data["skin_id"]

    def delete(self, skin_id: str) -> str:
        path = self._path(skin_id)
        if not path.is_file():
            raise KeyError(f"skin not found: {skin_id}")
        path.unlink()
        return skin_id
