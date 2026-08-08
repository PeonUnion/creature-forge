#!/usr/bin/env python3
"""从绑定姿态骨架生成人形蒙皮网格 + 权重（数据全外挂，不硬编码顶点数值）。

产物（data/species/human/skin/）：
    mesh.json     — 绑定姿态网格：vertices/indices/uvs/normals + 材质参数
    weights.json  — 蒙皮权重：boneNames（骨索引→关节名）+ 每顶点 {indices[≤4], weights[]}
                    （每顶点绑最近 4 根骨，点到线段距离倒数归一化）

网格 = 每根骨一个胶囊（圆柱+两端半球）+ 关节处球覆盖 + 头部球；
绑定姿态 = skeleton.json fk_tree 层级 + default.json positions_3d（Y-down 项目坐标）。
权重可被预设参数化（如皮肤胖瘦 → 半径缩放），生成参数 RADII 为生成器配置。

用法: python scripts/gen_skin.py [--species human]
"""
from __future__ import annotations

import argparse
import json
import math
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

# 关节名 → 骨半径（px；按绑定姿态骨架尺度，生成器配置）
RADII: dict[str, float] = {
    "head": 30.0, "jaw": 11.0, "neck": 13.0,
    "chest": 30.0, "sternum": 28.0, "waist": 25.0, "abdomen": 23.0,
    "clavicle": 8.0, "shoulder": 12.0, "elbow": 10.0, "wrist": 7.0,
    "palm": 6.0, "finger": 4.0,
    "hip": 15.0, "knee": 13.0, "ankle": 9.0, "heel": 7.0, "foot": 6.0, "toe": 5.0,
    "pelvis": 17.0,
}


def _norm(v):
    L = math.sqrt(sum(a * a for a in v)) or 1e-9
    return [a / L for a in v]


def _cross(a, b):
    return [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]]


def _sub(a, b):
    return [a[0] - b[0], a[1] - b[1], a[2] - b[2]]


def _add(a, b):
    return [a[0] + b[0], a[1] + b[1], a[2] + b[2]]


def _scale(v, s):
    return [a * s for a in v]


class SkinBuilder:
    """组装网格：顶点/索引/UV/法线（绑定姿态，Y-down）。"""

    def __init__(self):
        self.verts: list[list[float]] = []
        self.uvs: list[list[float]] = []
        self.nrm: list[list[float]] = []
        self.idx: list[int] = []

    def _emit(self, p, n, u=(0.0, 0.0)):
        self.verts.append(p)
        self.nrm.append(n)
        self.uvs.append(list(u))
        return len(self.verts) - 1

    def add_capsule(self, pa, pb, r, rings=8, segs=3):
        """圆柱 + 两端半球（胶囊），沿 pa→pb。"""
        d = _sub(pb, pa)
        L = math.sqrt(sum(x * x for x in d)) or 1e-6
        u = _norm(d)
        aux = [1.0, 0.0, 0.0] if abs(u[0]) < 0.9 else [0.0, 1.0, 0.0]
        v = _norm(_cross(u, aux))
        w = _cross(u, v)
        # 圆柱
        first = len(self.verts)
        for i in range(segs + 1):
            t = i / segs
            pc = _add(pa, _scale(u, t * L))
            for k in range(rings):
                a = k / rings * math.tau
                off = _add(_scale(v, math.cos(a) * r), _scale(w, math.sin(a) * r))
                self._emit(_add(pc, off), _norm(off), (k / rings, t))
        for i in range(segs):
            for k in range(rings):
                k2 = (k + 1) % rings
                a = first + i * rings + k
                b = first + i * rings + k2
                c = first + (i + 1) * rings + k
                d = first + (i + 1) * rings + k2
                self.idx += [a, c, b, b, c, d]
        # 半球（pa 端 + pb 端）
        for (c0, sign) in ((pa, -1.0), (pb, 1.0)):
            for j in range(1, rings // 2):
                phi = j / (rings // 2) * math.pi / 2
                rr = math.sin(phi) * r
                yh = math.cos(phi) * r
                base = len(self.verts)
                for k in range(rings):
                    a = k / rings * math.tau
                    off = _add(_scale(v, math.cos(a) * rr), _scale(w, math.sin(a) * rr))
                    n = _add(_scale(u, sign * yh), _norm(off))
                    p = _add(c0, _add(_scale(u, sign * yh), off))
                    self._emit(p, _norm(n))
                # 顶部圆环到顶点的三角形
                top_idx = len(self.verts)
                self._emit(_add(c0, _scale(u, sign * r)), _scale(u, sign))
                for k in range(rings):
                    a = base + k
                    b = base + (k + 1) % rings
                    if sign < 0:
                        self.idx += [a, b, top_idx]
                    else:
                        self.idx += [a, top_idx, b]
                # 环之间
                for _ in range(rings // 2 - 2):
                    pass
        return L

    def add_sphere(self, c, r, rings=8):
        base = len(self.verts)
        for j in range(rings // 2 + 1):
            phi = j / (rings // 2) * math.pi
            yc = math.cos(phi)
            rr = math.sin(phi) * r
            for k in range(rings):
                a = k / rings * math.tau
                p = [c[0] + math.cos(a) * rr, c[1] + yc * r, c[2] + math.sin(a) * rr]
                n = _norm([p[0] - c[0], p[1] - c[1], p[2] - c[2]])
                self._emit(p, n, (k / rings, j / (rings // 2)))
        for j in range(rings // 2):
            for k in range(rings):
                k2 = (k + 1) % rings
                a = base + j * rings + k
                b = base + j * rings + k2
                c = base + (j + 1) * rings + k
                d = base + (j + 1) * rings + k2
                self.idx += [a, c, b, b, c, d]


def build_mesh(species_id: str, root: Path) -> SkinBuilder:
    sk = json.loads((root / "data" / "species" / species_id / "skeleton.json").read_text(encoding="utf-8"))
    df = json.loads((root / "data" / "species" / species_id / "default.json").read_text(encoding="utf-8"))
    fk_tree = sk.get("fk_tree", {})
    pos = df["positions_3d"]
    bones = [(c, p) for c, p in fk_tree.items() if p is not None]  # (child, parent)
    sb = SkinBuilder()
    # 每根骨一个胶囊
    for child, parent in bones:
        pa, pb = pos[parent], pos[child]
        r = RADII.get(child.split("_")[0], 10.0)
        sb.add_capsule(pa, pb, r)
    # 关节球覆盖（避免拼接缝；头单独球）
    for j, p in pos.items():
        if j == "head":
            sb.add_sphere(p, RADII["head"] * 0.62)
        elif j == "pelvis":
            sb.add_sphere(p, 12.0)
        else:
            key = j.split("_")[0]
            sb.add_sphere(p, RADII.get(key, 10.0) * 0.55)
    return sb, pos, bones


def compute_weights(verts, pos, bones, root_name):
    """每顶点绑最近 ≤4 根骨：点到线段距离倒数归一化。

    boneNames 为完整拓扑序（[root] + 各 child 骨），权重索引对齐该列表
    （根索引 0 不作为权重骨；child 骨索引从 1 起）。
    """
    bone_names = [root_name] + [c for c, _ in bones]
    # 预计算骨段（仅边骨，索引 i → boneNames[i+1]）
    segs = [(pos[a], pos[b]) for a, b in bones]
    out = []
    for p in verts:
        ds = []
        for (pa, pb) in segs:
            ab = _sub(pb, pa)
            L2 = sum(x * x for x in ab)
            t = 0.0 if L2 < 1e-9 else max(0.0, min(1.0, sum((p[i] - pa[i]) * ab[i] for i in range(3)) / L2))
            q = _add(pa, _scale(ab, t))
            d = math.sqrt(sum((p[i] - q[i]) ** 2 for i in range(3)))
            ds.append(d)
        order = sorted(range(len(ds)), key=lambda i: ds[i])[:4]
        order = [i + 1 for i in order]  # 偏移：boneNames[0]=root
        wsum = sum(1.0 / (ds[i - 1] + 0.5) for i in order)
        weights = [(1.0 / (ds[i - 1] + 0.5)) / wsum for i in order]
        out.append({"indices": order, "weights": [round(w, 5) for w in weights]})
    return bone_names, out


def main() -> None:
    ap = argparse.ArgumentParser(description="生成人形蒙皮网格 + 权重（外挂到 skin/）")
    ap.add_argument("--species", default="human")
    args = ap.parse_args()
    outdir = ROOT / "data" / "species" / args.species / "skin"
    outdir.mkdir(parents=True, exist_ok=True)
    sb, pos, bones = build_mesh(args.species, ROOT)
    root_name = "pelvis"  # human 骨架根关节（fk_tree parent=None）
    bone_names, weights = compute_weights(sb.verts, pos, bones, root_name)
    mesh = {
        "schema": "creatureforge_skin_v1",
        "species": args.species,
        "bind_pose": "skeleton fk_tree + default.json positions_3d",
        "vertex_count": len(sb.verts),
        "vertices": [round(x, 3) for v in sb.verts for x in v],
        "indices": sb.idx,
        "uvs": [round(u, 4) for uv in sb.uvs for u in uv],
        "normals": [round(x, 5) for n in sb.nrm for x in n],
        "materials": {"albedo": "#c9a58c", "roughness": 0.6, "metallic": 0.0},
    }
    (outdir / "mesh.json").write_text(json.dumps(mesh, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    wdata = {"schema": "creatureforge_skin_weights_v1", "species": args.species,
             "boneNames": bone_names, "perVertex": weights}
    (outdir / "weights.json").write_text(json.dumps(wdata, ensure_ascii=False) + "\n", encoding="utf-8")
    print(f"顶点 {len(sb.verts)}  三角形 {len(sb.idx)//3}  骨 {len(bone_names)}")
    print(f"已写 {outdir/'mesh.json'} + weights.json（外挂蒙皮数据）")


if __name__ == "__main__":
    main()
