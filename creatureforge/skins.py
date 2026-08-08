# =========================================================================
# CreatureForge — 皮肤模块（Skin）
# =========================================================================
# 皮肤 = 基于物种的外观实例：一组皮肤参数（肤色/体脂/肌肉等）+ 材质参数
# （albedo/roughness/metallic）。物种提供皮肤参数 schema（species/<id>/skin/skin_params.json），
# 皮肤只需提供参数值，界面按 schema 渲染参数面板。网格/权重为物种基底（skin/mesh.json +
# skin/weights.json），皮肤定义只存"外观覆盖"，不重复存储网格数据。
#
# 目录结构：
#   skins/<skin_id>.json        — 皮肤定义（值），schema 由物种派生
#   species/<id>/skin/          — 物种皮肤基底（mesh/weights/skin_params）
# =========================================================================

from __future__ import annotations

import json
from pathlib import Path

from .models import Skin, SkinSummary
from .species import SpeciesService

SKIN_SCHEMA = "creatureforge_skin_v1"
DEFAULT_MATERIALS = {"albedo": "#c9a58c", "roughness": 0.6, "metallic": 0.0}


class SkinService:
    """皮肤模块：管理 skins/<id>.json，派生完整 schema（物种皮肤参数 + 材质）。"""

    def __init__(self, root: Path, species: SpeciesService) -> None:
        self._root = root
        self._species = species

    # -- 内部路径 --

    def _path(self, skin_id: str) -> Path:
        return self._root / f"{skin_id}.json"

    @staticmethod
    def _save(path: Path, data: dict) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")

    # -- schema（数据驱动：物种皮肤参数 + 默认材质） --

    def build_skin_schema(self, species_id: str) -> dict:
        """派生皮肤完整 schema（供前端参数面板渲染）。

        - params: 皮肤参数（物种 skin/skin_params.json，如肤色/体脂/肌肉）
        - materials: 默认材质（albedo/roughness/metallic）
        """
        params: dict = {}
        materials: dict = dict(DEFAULT_MATERIALS)
        try:
            p = json.loads((self._species._root / species_id / "skin" / "skin_params.json").read_text(encoding="utf-8"))
            params = p.get("params", {}) or {}
            materials = {**materials, **(p.get("materials", {}) or {})}
        except Exception:
            pass  # 物种无皮肤参数 → 空 schema
        return {"species": species_id, "params": params, "materials": materials}

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
                    "species": d.get("species", ""),
                })
            except Exception:
                continue
        return items

    def get(self, skin_id: str) -> dict:
        """皮肤详情 = 皮肤值 + 完整 schema（物种皮肤参数 + 材质）。"""
        path = self._path(skin_id)
        if not path.is_file():
            raise KeyError(f"skin not found: {skin_id}")
        skin = json.loads(path.read_text(encoding="utf-8"))
        schema = self.build_skin_schema(skin.get("species", ""))
        return {**skin, "schema_info": schema}

    def new_schema(self, species_id: str) -> dict:
        """新建皮肤的空白表单：值 = 物种默认 + 完整 schema。"""
        schema = self.build_skin_schema(species_id)
        defaults = {k: v.get("default", 0.0) for k, v in schema["params"].items()}
        return {
            "schema": SKIN_SCHEMA,
            "skin_id": "",
            "species": species_id,
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
        if not data.get("species"):
            raise ValueError("species required")
        if self._path(sid).exists():
            raise FileExistsError(f"skin already exists: {sid}")
        data = dict(data)
        data.pop("schema_info", None)  # schema 由物种派生，不持久化
        data.setdefault("schema", SKIN_SCHEMA)
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
        self._save(path, data)
        return data["skin_id"]

    def delete(self, skin_id: str) -> str:
        path = self._path(skin_id)
        if not path.is_file():
            raise KeyError(f"skin not found: {skin_id}")
        path.unlink()
        return skin_id
