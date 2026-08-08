#!/usr/bin/env python3
"""glTF 2.0 (.glb) 导出器（纯标准库，无第三方依赖）。

从项目数据导出引擎可用资产：骨架 + 蒙皮网格 + 真实动捕动作动画。
数据全部来自外挂 JSON（skeleton/skin/actions3d），不硬编码。

输出 .glb 二进制（GLB 容器：JSON chunk + BIN chunk）：
  - 骨骼层级（fk_tree 拓扑，Y-up）
  - mesh：POSITION/NORMAL/TEXCOORD_0/JOINTS_0/WEIGHTS_0 + indices
  - skin：joints + inverseBindMatrices（绑定姿态相对根）
  - animation：每骨局部旋转 quaternion + 根位移（真实动捕帧）

用法: from creatureforge.gltf import export_glb
"""

from __future__ import annotations

import math
import struct

# glTF 常量
COMP_FLOAT = 5126
COMP_USHORT = 5123
COMP_UBYTE = 5121
T_SCALAR = "SCALAR"
T_VEC2 = "VEC2"
T_VEC3 = "VEC3"
T_VEC4 = "VEC4"
T_MAT4 = "MAT4"
TARGET_ARRAY = 34962


def _flip_y(v3):
    return [v3[0], -v3[1], v3[2]]


def _hex_to_rgb(hex_color: str) -> list[float]:
    """>#rrggbb → [r,g,b]（0-1）。支持简写 #rgb，非法值回退默认肤色。"""
    s = (hex_color or "").lstrip("#")
    if len(s) == 3:
        s = "".join(c * 2 for c in s)
    try:
        if len(s) == 6:
            return [int(s[0:2], 16) / 255.0, int(s[2:4], 16) / 255.0, int(s[4:6], 16) / 255.0]
    except ValueError:
        pass
    return [0.788, 0.647, 0.549]  # 默认肤色 #c9a58c


def _quat_from_euler_xyz(rx, ry, rz):
    """欧拉角(Rz·Ry·Rx 顺序, Three.js Euler XYZ) → 四元数 [x,y,z,w]。"""
    c1, s1 = math.cos(rx / 2), math.sin(rx / 2)
    c2, s2 = math.cos(ry / 2), math.sin(ry / 2)
    c3, s3 = math.cos(rz / 2), math.sin(rz / 2)
    return [s1 * c2 * c3 + c1 * s2 * s3,
            c1 * s2 * c3 - s1 * c2 * s3,
            c1 * c2 * s3 + s1 * s2 * c3,
            c1 * c2 * c3 - s1 * s2 * s3]


class _GLB:
    """GLB 组装：bufferView/accessor 管理 + 二进制打包。"""

    def __init__(self):
        self.json = {
            "asset": {"version": "2.0", "generator": "creatureforge"},
            "scene": 0,
            "scenes": [{"nodes": []}],
            "nodes": [], "meshes": [], "skins": [], "animations": [],
            "accessors": [], "bufferViews": [], "buffers": [{"byteLength": 0}],
            "materials": [], "textures": [], "images": [], "samplers": [],
        }
        self.bin = bytearray()

    def _view(self, data: bytes) -> int:
        """追加数据到 BIN，返回 bufferView 索引（byteOffset 默认按 4 对齐）。"""
        offset = (len(self.bin) + 3) & ~3
        while len(self.bin) < offset:
            self.bin.append(0)
        self.bin += data
        vi = len(self.json["bufferViews"])
        self.json["bufferViews"].append({"buffer": 0, "byteOffset": offset, "byteLength": len(data)})
        return vi

    def accessor(self, values, comp_type, comp_size, typ, count=None, normalized=False, minmax=None):
        """写入 accessor。values 为 python 数值列表（flat），按 comp_type 打包。"""
        fmt = {COMP_FLOAT: "<f", COMP_USHORT: "<H", COMP_UBYTE: "<B"}[comp_type]
        data = b"".join(struct.pack(fmt, v) for v in values)
        vi = self._view(data)
        if count is None:
            count = len(values) // comp_size
        a = {"bufferView": vi, "componentType": comp_type, "count": count, "type": typ}
        if normalized:
            a["normalized"] = True
        if minmax is not None and comp_type == COMP_FLOAT:
            lo, hi = minmax
            a["min"], a["max"] = lo, hi
        self.json["accessors"].append(a)
        return len(self.json["accessors"]) - 1


def export_glb(skel3d: dict, skin: dict, motion3d: dict, params: dict | None = None,
               body_scale: float | None = None, materials: dict | None = None,
               parts: list[dict] | None = None) -> bytes:
    """导出 .glb（骨骼 + 蒙皮网格 + 皮肤部件 + 动作动画）。

    skel3d: build_skeleton_3d 输出（joints/fk_tree/fk_local/bones）
    skin:  {"mesh": mesh.json, "weights": weights.json}
    motion3d: 动作 JSON（fk3d 旋转表 + root3d）
    body_scale: 可选，绑定姿态顶点 x/z 缩放（皮肤体态，如 fat/muscle → 胖瘦）
    materials: 可选，基底材质覆盖 {albedo, roughness, metallic}
    parts: 可选，皮肤部件 [{bone, transform, mesh, materials, textures:{name:(ext,bytes)}}]
           — bone 装饰件：mesh 挂在绑定骨骼 node 下，跟随骨骼变换。
    """
    from .skeleton3d import per_frame_trs, _flip_euler

    mesh = skin["mesh"]
    weights = skin["weights"]
    bn = weights["boneNames"]
    per = weights["perVertex"]
    bind = skel3d["joints"]
    fk_tree = skel3d.get("fk_tree") or {}
    root_name = next((j for j, p in fk_tree.items() if p is None), bn[0] if bn else None)
    if root_name not in bn:
        bn = [root_name] + [b for b in bn if b != root_name]
    idx_of = {n: i for i, n in enumerate(bn)}
    nv = mesh["vertex_count"]
    n_bones = len(bn)
    sx = body_scale if body_scale else 1.0

    g = _GLB()
    # ---- 节点：骨骼（Y-up，根在原点，子=绝对坐标差）+ mesh node ----
    by_parent = {}
    for n in bn:
        p = fk_tree.get(n)
        if p is not None:
            by_parent.setdefault(p, []).append(n)
    bone_nodes = list(range(n_bones))  # 骨骼 node 索引 = boneNames 顺序
    node_names = [""] * n_bones
    for j, name in enumerate(bn):
        node_names[j] = name
        if name == root_name:
            g.json["nodes"].append({"name": name, "translation": [0.0, 0.0, 0.0]})
        else:
            parent = fk_tree[name]
            pp, cp = bind[parent], bind[name]
            g.json["nodes"].append({"name": name,
                                    "translation": [cp[0] - pp[0], -(cp[1] - pp[1]), cp[2] - pp[2]]})
    for name in bn:
        kids = by_parent.get(name, [])
        if kids:
            g.json["nodes"][idx_of[name]]["children"] = [idx_of[k] for k in kids]
    # ---- 网格（绑定姿态顶点，Y-up）----
    pos_min = [1e18] * 3
    pos_max = [-1e18] * 3
    pos_flat = []
    for i in range(nv):
        x, y, z = mesh["vertices"][3 * i], mesh["vertices"][3 * i + 1], mesh["vertices"][3 * i + 2]
        if body_scale:
            x *= sx
            z *= sx
        pos_flat += [x, -y, z]
        for a in range(3):
            v = [x, -y, z][a]
            pos_min[a] = min(pos_min[a], v)
            pos_max[a] = max(pos_max[a], v)
    nrm_flat = []
    for i in range(nv):
        x, y, z = mesh["normals"][3 * i], mesh["normals"][3 * i + 1], mesh["normals"][3 * i + 2]
        nrm_flat += [x, -y, z]
    acc_pos = g.accessor(pos_flat, COMP_FLOAT, 3, T_VEC3, minmax=(pos_min, pos_max))
    acc_nrm = g.accessor(nrm_flat, COMP_FLOAT, 3, T_VEC3)
    acc_uv = g.accessor(list(mesh["uvs"]), COMP_FLOAT, 2, T_VEC2)
    # 蒙皮权重（JOINTS 用 ubyte：36 骨 < 256）
    joints_flat = []
    wght_flat = []
    for i in range(nv):
        wi = per[i]
        for k in range(4):
            joints_flat.append(wi["indices"][k] if k < len(wi["indices"]) else 0)
            wght_flat.append(wi["weights"][k] if k < len(wi["weights"]) else 0.0)
    acc_joints = g.accessor(joints_flat, COMP_UBYTE, 4, T_VEC4)
    acc_wght = g.accessor(wght_flat, COMP_FLOAT, 4, T_VEC4)
    acc_idx = g.accessor(list(mesh["indices"]), COMP_USHORT, 1, T_SCALAR)
    g.json["meshes"].append({
        "primitives": [{"attributes": {
            "POSITION": acc_pos, "NORMAL": acc_nrm, "TEXCOORD_0": acc_uv,
            "JOINTS_0": acc_joints, "WEIGHTS_0": acc_wght},
            "indices": acc_idx, "material": 0}],
        "name": "creatureforge_skin"})
    # 材质：皮肤覆盖（albedo / roughness / metallic）优先，否则默认肤色
    mat = materials or {}
    bc = _hex_to_rgb(str(mat.get("albedo", "#c9a58c")))
    rough = float(mat.get("roughness", 0.6))
    metal = float(mat.get("metallic", 0.0))
    g.json["materials"].append({
        "pbrMetallicRoughness": {
            "baseColorFactor": [bc[0], bc[1], bc[2], 1.0],
            "metallicFactor": metal, "roughnessFactor": rough},
        "doubleSided": True, "name": "skin"})
    # ---- 皮肤部件（bone 装饰件：mesh 挂在绑定骨骼 node 下，跟随骨骼）----
    for part in parts or []:
        pmesh = part.get("mesh") or {}
        if not pmesh.get("vertices"):
            continue
        bone_name = part.get("bone", "")
        if bone_name not in idx_of:
            continue  # 绑定骨骼不存在 → 跳过
        parent_node = idx_of[bone_name]
        pv = pmesh["vertices"]
        pn = pmesh.get("normals") or []
        pu = pmesh.get("uvs") or []
        pi = pmesh.get("indices") or list(range(len(pv) // 3))
        # 部件网格顶点 Y-down → Y-up
        pos_flat = []
        for k in range(0, len(pv), 3):
            pos_flat += [pv[k], -pv[k + 1], pv[k + 2]]
        nrm_flat = []
        for k in range(0, len(pn), 3):
            nrm_flat += [pn[k], -pn[k + 1], pn[k + 2]]
        p_acc_pos = g.accessor(pos_flat, COMP_FLOAT, 3, T_VEC3)
        attrs = {"POSITION": p_acc_pos}
        if nrm_flat:
            attrs["NORMAL"] = g.accessor(nrm_flat, COMP_FLOAT, 3, T_VEC3)
        if pu:
            attrs["TEXCOORD_0"] = g.accessor(list(pu), COMP_FLOAT, 2, T_VEC2)
        p_acc_idx = g.accessor(list(pi), COMP_USHORT, 1, T_SCALAR)
        mat_idx = _part_material(g, part.get("materials") or {}, part.get("textures") or {})
        g.json["meshes"].append({
            "primitives": [{"attributes": attrs, "indices": p_acc_idx, "material": mat_idx}],
            "name": part.get("part_id", "part")})
        mesh_i = len(g.json["meshes"]) - 1
        # 节点变换：position Y-down→Y-up；rotation 欧拉→四元数；scale 不变
        tr = part.get("transform") or {}
        pos = tr.get("position") or [0.0, 0.0, 0.0]
        rot = tr.get("rotation") or [0.0, 0.0, 0.0]
        scl = tr.get("scale") or [1.0, 1.0, 1.0]
        try:
            rot_up = _flip_euler(float(rot[0]), float(rot[1]), float(rot[2]))
        except Exception:
            rot_up = [0.0, 0.0, 0.0]
        pnode = len(g.json["nodes"])
        g.json["nodes"].append({
            "name": part.get("part_id", "part"),
            "mesh": mesh_i,
            "translation": [float(pos[0]), -float(pos[1]), float(pos[2])],
            "rotation": _quat_from_euler_xyz(rot_up[0], rot_up[1], rot_up[2]),
            "scale": [float(scl[0]), float(scl[1]), float(scl[2])],
        })
        g.json["nodes"][parent_node].setdefault("children", []).append(pnode)
    # ---- skin：joints + inverseBindMatrices（绑定姿态相对根）----
    ibm = []
    for name in bn:
        # 绑定姿态相对根的平移 = bind[name] - bind[root]（Y-up）
        t = [bind[name][0] - bind[root_name][0],
             -(bind[name][1] - bind[root_name][1]),
             bind[name][2] - bind[root_name][2]]
        # 逆矩阵 = 平移 -t（列主序 4x4）
        ibm += [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, -t[0], -t[1], -t[2], 1]
    acc_ibm = g.accessor(ibm, COMP_FLOAT, 16, T_MAT4, count=n_bones)
    g.json["skins"].append({"joints": bone_nodes, "inverseBindMatrices": acc_ibm, "name": "skin"})
    # mesh node（引用 skin，放在骨骼节点之后）
    mesh_node = len(g.json["nodes"])
    g.json["nodes"].append({"name": "creatureforge", "mesh": 0, "skin": 0, "children": [bone_nodes[0]]})
    # ---- 动画（每骨局部旋转 + 根位移）----
    trs = per_frame_trs(motion3d, params)
    if trs:
        nf = len(trs)
        times = []
        for i in range(nf):
            times.append(i / (int(motion3d.get("fps", 6)) or 6))
        acc_time = g.accessor(times, COMP_FLOAT, 1, T_SCALAR)
        anim = {"name": motion3d.get("motion_id", "anim"), "samplers": [], "channels": []}
        for j, name in enumerate(bn):
            rot_flat = []
            for fr in trs:
                r = fr["rot"].get(name, [0.0, 0.0, 0.0])
                rot_flat += _quat_from_euler_xyz(r[0], r[1], r[2])
            acc_rot = g.accessor(rot_flat, COMP_FLOAT, 4, T_VEC4)
            anim["samplers"].append({"input": acc_time, "output": acc_rot, "interpolation": "LINEAR"})
            anim["channels"].append({"sampler": len(anim["samplers"]) - 1,
                                     "target": {"node": j, "path": "rotation"}})
        # 根位移（相对绑定根）
        root_track_flat = []
        for fr in trs:
            rt = fr["root"]  # 已是 Y-up
            root_track_flat += [bind[root_name][0] + rt[0],
                                -bind[root_name][1] + rt[1],
                                bind[root_name][2] + rt[2]]
        acc_root = g.accessor(root_track_flat, COMP_FLOAT, 3, T_VEC3)
        anim["samplers"].append({"input": acc_time, "output": acc_root, "interpolation": "LINEAR"})
        anim["channels"].append({"sampler": len(anim["samplers"]) - 1,
                                 "target": {"node": bone_nodes[0], "path": "translation"}})
        g.json["animations"].append(anim)
    g.json["scenes"][0]["nodes"] = [mesh_node]
    # ---- GLB 打包 ----
    g.json["buffers"][0]["byteLength"] = len(g.bin)
    json_bytes = _json_bytes(g.json)
    bin_bytes = bytes(g.bin)
    total = 12 + 8 + len(json_bytes) + 8 + len(bin_bytes)
    header = struct.pack("<4sII", b"glTF", 2, total)
    json_chunk = struct.pack("<I4s", len(json_bytes), b"JSON") + json_bytes
    bin_chunk = struct.pack("<I4s", len(bin_bytes), b"BIN\0") + bin_bytes
    return header + json_chunk + bin_chunk


def _json_bytes(obj: dict) -> bytes:
    import json
    b = json.dumps(obj, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    pad = (4 - len(b) % 4) % 4
    return b + b" " * pad


def _add_image(g: "_GLB", ext: str, data: bytes) -> int:
    """贴图字节 → glTF image（写入 BIN），返回 image 索引。"""
    vi = g._view(data)
    mime = {"png": "image/png", "jpg": "image/jpeg", "jpeg": "image/jpeg",
            "webp": "image/webp"}.get(ext, "image/png")
    img_i = len(g.json["images"])
    g.json["images"].append({"bufferView": vi, "mimeType": mime})
    return img_i


def _part_material(g: "_GLB", pmat: dict, tex: dict) -> int:
    """部件材质（albedo/roughness/metallic + 贴图 → glTF material），返回 material 索引。"""
    bc = _hex_to_rgb(str(pmat.get("albedo", "#c9a58c")))
    rough = float(pmat.get("roughness", 0.6))
    metal = float(pmat.get("metallic", 0.0))
    pbr = {"baseColorFactor": [bc[0], bc[1], bc[2], 1.0],
           "metallicFactor": metal, "roughnessFactor": rough}
    m = {"pbrMetallicRoughness": pbr, "doubleSided": True, "name": "skin_part"}
    if "albedo" in tex:
        ext, tdata = tex["albedo"]
        img_i = _add_image(g, ext, tdata)
        tex_idx = len(g.json["textures"])
        g.json["textures"].append({"source": img_i})
        pbr["baseColorTexture"] = {"index": tex_idx}
        pbr["baseColorFactor"] = [1.0, 1.0, 1.0, 1.0]  # 贴图本色
    if "normal" in tex:
        ext, tdata = tex["normal"]
        img_i = _add_image(g, ext, tdata)
        tex_idx = len(g.json["textures"])
        g.json["textures"].append({"source": img_i})
        m["normalTexture"] = {"index": tex_idx}
    if "metallic_roughness" in tex:
        ext, tdata = tex["metallic_roughness"]
        img_i = _add_image(g, ext, tdata)
        tex_idx = len(g.json["textures"])
        g.json["textures"].append({"source": img_i})
        pbr["metallicRoughnessTexture"] = {"index": tex_idx}
    g.json["materials"].append(m)
    return len(g.json["materials"]) - 1
