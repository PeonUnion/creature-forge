# =========================================================================
# CreatureForge — 皮肤部件网格解析（游戏皮肤式：上传画师导出文件 → 部件）
# =========================================================================
# 支持文件格式：
#   .glb/.gltf — glTF 2.0（Blender/Substance 直接导出，含网格 + 材质 + 贴图）
#   .obj       — Wavefront OBJ（顶点/面/uv/法线，Y-up 惯例）
#   .json      — 项目网格格式 {vertices, indices, normals, uvs, vertex_count}（Y-down）
#
# 坐标系约定：项目内部 Y-down。GLB/OBJ 常为 Y-up（画师工具），导入时翻转顶点/法线 Y
# 到 Y-down（与基底网格一致），导出 glTF 时再翻回 Y-up。
# =========================================================================

from __future__ import annotations

import json
import struct


def parse_mesh_file(filename: str, data: bytes) -> dict:
    """按扩展名解析网格文件，返回 {mesh, materials, textures}。

    mesh: {vertices, indices, normals, uvs, vertex_count}（Y-down flat）
    materials: {albedo, roughness, metallic}（默认肤色）
    textures: {albedo: bytes, ...}（内嵌贴图，如 GLB base64 image）
    """
    name = (filename or "").lower()
    if name.endswith(".glb"):
        return _parse_glb(data)
    if name.endswith(".gltf"):
        return _parse_gltf(data)
    if name.endswith(".obj"):
        return _parse_obj(data.decode("utf-8", errors="replace"))
    if name.endswith(".json"):
        return _parse_json(data)
    raise ValueError(f"不支持的网格文件格式: {filename}（支持 .glb/.gltf/.obj/.json）")


def _flip_mesh_y(mesh: dict) -> dict:
    """顶点/法线翻转 Y（Y-up → Y-down，与项目基底一致）。"""
    m = dict(mesh)
    verts = list(m["vertices"])
    for k in range(1, len(verts), 3):
        verts[k] = -verts[k]
    m["vertices"] = verts
    if m.get("normals"):
        nrm = list(m["normals"])
        for k in range(1, len(nrm), 3):
            nrm[k] = -nrm[k]
        m["normals"] = nrm
    return m


# --------------------------------------------------------------------------
# GLB 解析（纯标准库：header + JSON chunk + BIN chunk）
# --------------------------------------------------------------------------

_COMP = {5126: ("<f", 4), 5125: ("<I", 4), 5123: ("<H", 2), 5121: ("<B", 1)}
_TYPC = {"SCALAR": 1, "VEC2": 2, "VEC3": 3, "VEC4": 4}


def _read_accessor(doc, bin_data, acc_idx):
    a = doc["accessors"][acc_idx]
    bv = doc["bufferViews"][a["bufferView"]]
    comp_type = a["componentType"]
    cnt = _TYPC[a["type"]]
    n = a["count"]
    fmt, sz = _COMP.get(comp_type, ("<f", 4))
    base = int(bv.get("byteOffset", 0) or 0) + int(a.get("byteOffset", 0) or 0)
    stride = bv.get("byteStride")
    vals = []
    if stride:
        for i in range(n):
            for k in range(cnt):
                vals.append(struct.unpack_from(fmt, bin_data, base + i * stride + k * sz)[0])
    else:
        for i in range(n * cnt):
            vals.append(struct.unpack_from(fmt, bin_data, base + i * sz)[0])
    return vals, a.get("min"), a.get("max")


def _split_glb(data: bytes) -> tuple[dict, bytes]:
    """GLB → (json doc, bin bytes)。"""
    if len(data) < 12 or data[:4] != b"glTF":
        raise ValueError("不是有效的 GLB 文件")
    off = 12
    json_len = struct.unpack_from("<I", data, off)[0]
    off += 4
    if data[off:off + 4] != b"JSON":
        raise ValueError("GLB 缺少 JSON chunk")
    off += 4
    doc = json.loads(data[off:off + json_len].decode("utf-8"))
    off += json_len
    bin_data = b""
    if off < len(data):
        bin_len = struct.unpack_from("<I", data, off)[0]
        off += 4
        if data[off:off + 4] == b"BIN\x00":
            off += 4
            bin_data = data[off:off + bin_len]
    return doc, bin_data


def _extract_images(doc, bin_data) -> dict[str, bytes]:
    """提取 glTF 内嵌贴图（image.bufferView / base64 data URI）→ {name: bytes}。"""
    out: dict[str, bytes] = {}
    for i, img in enumerate(doc.get("images", []) or []):
        name = (img.get("name") or f"texture_{i}").split(".")[0]
        if "bufferView" in img:
            bv = doc["bufferViews"][img["bufferView"]]
            base = int(bv.get("byteOffset", 0) or 0)
            out[name] = bin_data[base:base + bv["byteLength"]]
        elif img.get("uri", "").startswith("data:"):
            import base64
            b64 = img["uri"].split(",", 1)[1]
            out[name] = base64.b64decode(b64)
    return out


def _parse_glb(data: bytes) -> dict:
    doc, bin_data = _split_glb(data)
    meshes = doc.get("meshes") or []
    if not meshes:
        raise ValueError("GLB 中没有网格")
    prim = meshes[0]["primitives"][0]
    attrs = prim.get("attributes", {})
    if "POSITION" not in attrs:
        raise ValueError("GLB 网格缺少 POSITION")

    def acc(idx, flip_y=False):
        vals, _mn, _mx = _read_accessor(doc, bin_data, idx)
        if flip_y:
            for k in range(1, len(vals), 3):
                vals[k] = -vals[k]
        return vals

    vertices = acc(attrs["POSITION"], flip_y=True)
    normals = acc(attrs["NORMAL"], flip_y=True) if "NORMAL" in attrs else []
    uvs = acc(attrs["TEXCOORD_0"]) if "TEXCOORD_0" in attrs else []
    indices = acc(prim.get("indices")) if "indices" in prim else list(range(len(vertices) // 3))
    vertex_count = len(vertices) // 3

    # 材质（PBR）：baseColorFactor / metallicFactor / roughnessFactor
    materials = {"albedo": "#c9a58c", "roughness": 0.6, "metallic": 0.0}
    if "material" in prim and prim["material"] is not None:
        m = (doc.get("materials") or [])[prim["material"]]
        pbr = m.get("pbrMetallicRoughness", {}) or {}
        bcf = pbr.get("baseColorFactor") or [0.8, 0.7, 0.6, 1.0]
        materials = {
            "albedo": "#%02x%02x%02x" % tuple(max(0, min(255, int(c * 255))) for c in bcf[:3]),
            "roughness": float(pbr.get("roughnessFactor", 0.6)),
            "metallic": float(pbr.get("metallicFactor", 0.0)),
        }
    textures = _extract_images(doc, bin_data)
    return {
        "mesh": {"vertices": vertices, "indices": indices, "normals": normals,
                 "uvs": uvs, "vertex_count": vertex_count},
        "materials": materials,
        "textures": textures,
    }


def _parse_gltf(data: bytes) -> dict:
    """.gltf（JSON 文本，bin 需为 data URI 内嵌）→ 同 GLB 结果。"""
    doc = json.loads(data.decode("utf-8"))
    bin_data = b""
    if doc.get("buffers"):
        uri = (doc["buffers"][0].get("uri") or "")
        if uri.startswith("data:"):
            import base64
            bin_data = base64.b64decode(uri.split(",", 1)[1])
    # 复用 GLB 解析逻辑（doc 结构一致）
    return _from_doc(doc, bin_data)


def _from_doc(doc, bin_data):
    meshes = doc.get("meshes") or []
    if not meshes:
        raise ValueError("glTF 中没有网格")
    prim = meshes[0]["primitives"][0]
    attrs = prim.get("attributes", {})

    def acc(idx, flip_y=False):
        vals, _mn, _mx = _read_accessor(doc, bin_data, idx)
        if flip_y:
            for k in range(1, len(vals), 3):
                vals[k] = -vals[k]
        return vals

    vertices = acc(attrs["POSITION"], flip_y=True)
    normals = acc(attrs["NORMAL"], flip_y=True) if "NORMAL" in attrs else []
    uvs = acc(attrs["TEXCOORD_0"]) if "TEXCOORD_0" in attrs else []
    indices = acc(prim.get("indices")) if "indices" in prim else list(range(len(vertices) // 3))
    materials = {"albedo": "#c9a58c", "roughness": 0.6, "metallic": 0.0}
    if "material" in prim and prim["material"] is not None:
        m = (doc.get("materials") or [])[prim["material"]]
        pbr = m.get("pbrMetallicRoughness", {}) or {}
        bcf = pbr.get("baseColorFactor") or [0.8, 0.7, 0.6, 1.0]
        materials = {
            "albedo": "#%02x%02x%02x" % tuple(max(0, min(255, int(c * 255))) for c in bcf[:3]),
            "roughness": float(pbr.get("roughnessFactor", 0.6)),
            "metallic": float(pbr.get("metallicFactor", 0.0)),
        }
    textures = _extract_images(doc, bin_data)
    return {
        "mesh": {"vertices": vertices, "indices": indices, "normals": normals,
                 "uvs": uvs, "vertex_count": len(vertices) // 3},
        "materials": materials,
        "textures": textures,
    }


# --------------------------------------------------------------------------
# OBJ 解析（顶点/面/uv/法线，Y-up → Y-down）
# --------------------------------------------------------------------------

def _parse_obj(text: str) -> dict:
    verts, uvs, normals, faces = [], [], [], []
    for raw in text.splitlines():
        parts = raw.split()
        if not parts:
            continue
        if parts[0] == "v":
            verts.append([float(parts[1]), float(parts[2]), float(parts[3])])
        elif parts[0] == "vt":
            uvs.append([float(parts[1]), float(parts[2])])
        elif parts[0] == "vn":
            normals.append([float(parts[1]), float(parts[2]), float(parts[3])])
        elif parts[0] == "f":
            faces.append(parts[1:])

    out_verts: list[float] = []
    out_uvs: list[float] = []
    out_nrm: list[float] = []
    out_idx: list[int] = []
    cache: dict[tuple, int] = {}
    for face in faces:
        for tok in face:
            bits = tok.split("/")
            vi = int(bits[0]) - 1
            key = tuple(bits)
            if key not in cache:
                v = verts[vi]
                cache[key] = len(out_verts) // 3
                out_verts += [v[0], -v[1], v[2]]  # Y-up → Y-down
                if len(bits) > 1 and bits[1] and int(bits[1]) <= len(uvs):
                    uv = uvs[int(bits[1]) - 1]
                    out_uvs += [uv[0], uv[1]]
                if len(bits) > 2 and bits[2] and int(bits[2]) <= len(normals):
                    n = normals[int(bits[2]) - 1]
                    out_nrm += [n[0], -n[1], n[2]]
            out_idx.append(cache[key])
    return {
        "mesh": {"vertices": out_verts, "indices": out_idx, "normals": out_nrm,
                 "uvs": out_uvs, "vertex_count": len(out_verts) // 3},
        "materials": {"albedo": "#c9a58c", "roughness": 0.6, "metallic": 0.0},
        "textures": {},
    }


# --------------------------------------------------------------------------
# JSON 网格（项目格式，Y-down）
# --------------------------------------------------------------------------

def _parse_json(data: bytes) -> dict:
    d = json.loads(data.decode("utf-8"))
    if "mesh" in d:
        d = d["mesh"]
    mesh = {
        "vertices": [float(v) for v in d.get("vertices", [])],
        "indices": [int(v) for v in d.get("indices", [])],
        "normals": [float(v) for v in d.get("normals", [])],
        "uvs": [float(v) for v in d.get("uvs", [])],
        "vertex_count": int(d.get("vertex_count", len(d.get("vertices", [])) // 3)),
    }
    materials = dict(d.get("materials", {"albedo": "#c9a58c", "roughness": 0.6, "metallic": 0.0}))
    return {"mesh": mesh, "materials": materials, "textures": {}}
