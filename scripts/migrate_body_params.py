#!/usr/bin/env python3
"""一次性迁移：体型参数单一来源（骨架 param_chains）。

- 把 default.json 的 params 元数据（label/min/max/step/default）合并进 skeleton.json 的 param_chains
- default.json 删掉 params（降为纯 body 默认值表）
- 为 humanoid/dragon/human 补「整体尺度」全局参数（overall_scale / height）

运行：.venv/bin/python scripts/migrate_body_params.py
"""
from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DATA = ROOT / "data"

DEFAULT_GLOBAL = [
    {"name": "overall", "param": "overall_scale", "global": "all", "joints": [],
     "anchor": "center", "label": "整体大小", "min": 0.5, "max": 2.0, "step": 0.05, "default": 1.0},
    {"name": "height", "param": "height", "global": "y", "joints": [],
     "anchor": "floor", "label": "身高", "min": 0.5, "max": 1.5, "step": 0.05, "default": 1.0},
]


def enrich_param_chains(pc: dict, meta: dict | None = None) -> None:
    """给 param_chains 每条链补 label/min/max/step/default（缺省回退）。"""
    meta = meta or {}
    for chain in pc.values():
        pname = chain.get("param")
        m = (meta.get(pname) or {}) if pname else {}
        chain.setdefault("label", m.get("label", pname or "param"))
        chain.setdefault("min", m.get("min", 0.6))
        chain.setdefault("max", m.get("max", 1.6))
        chain.setdefault("step", m.get("step", 0.05))
        chain.setdefault("default", m.get("default", 1.0))


def add_globals(pc: dict) -> bool:
    changed = False
    for g in DEFAULT_GLOBAL:
        if g["param"] not in {c.get("param") for c in pc.values()}:
            pc[g["name"]] = {k: v for k, v in g.items() if k != "name"}
            changed = True
    return changed


def migrate_species(sid: str) -> None:
    sp_dir = DATA / "species" / sid
    skel_path = sp_dir / "skeleton.json"
    def_path = sp_dir / "default.json"
    if not skel_path.is_file():
        return
    skel = json.loads(skel_path.read_text(encoding="utf-8"))
    default = json.loads(def_path.read_text(encoding="utf-8")) if def_path.is_file() else {}
    pc = skel.setdefault("param_chains", {})
    enrich_param_chains(pc, default.get("params"))
    add_globals(pc)
    skel_path.write_text(json.dumps(skel, ensure_ascii=False, indent=2), encoding="utf-8")
    # default.json：删 params（元数据），保留 body 并补全局默认值
    default.pop("params", None)
    body = default.setdefault("body", {})
    for g in DEFAULT_GLOBAL:
        body.setdefault(g["param"], 1.0)
    if def_path.is_file():
        def_path.write_text(json.dumps(default, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"  ✓ species/{sid}：param_chains 元数据 + 全局参数，default.json 精简")


def migrate_template(morph: str) -> None:
    t_path = DATA / "templates" / f"{morph}.json"
    if not t_path.is_file():
        return
    t = json.loads(t_path.read_text(encoding="utf-8"))
    pc = t.setdefault("param_chains", {})
    enrich_param_chains(pc)
    add_globals(pc)
    t_path.write_text(json.dumps(t, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"  ✓ template/{morph}：param_chains 元数据 + 全局参数")


if __name__ == "__main__":
    print("体型参数单一来源迁移：")
    for sid in ("human", "dragon"):
        migrate_species(sid)
    for morph in ("humanoid", "custom"):
        migrate_template(morph)
    print("完成。")
