# =========================================================================
# CreatureForge — Go 计算内核桥接（可选加速）
# =========================================================================
# 对 LBS 顶点蒙皮 / FK pose 等数值热路径，若存在 gocore 二进制（Go 内核，
# gocore/cmd/gocore），则把「内存对象 + 批量帧」通过 stdin JSON 一次性转发，
# 由 Go 批量计算后返回——避免逐帧子进程开销。
#
# 无 gocore 时自动回退纯 Python（纯标准库，无需 Go）。数据仍全部来自外部
# JSON；本模块只是引擎调度，不硬编码任何数据。
#
# gocore 二进制来源（优先级）：
#   1) 环境变量 GOCORE_BIN
#   2) creatureforge.config.GOCORE_BIN
#   3) PATH 中的 `gocore`
# =========================================================================

from __future__ import annotations

import json
import os
import shutil
import subprocess

from . import config

_GOCORE_TIMEOUT = 120.0


def gocore_bin() -> str | None:
    """返回可用的 gocore 二进制路径；无则 None。"""
    env = os.environ.get("GOCORE_BIN")
    if env and os.path.isfile(env) and os.access(env, os.X_OK):
        return env
    cfg = config.GOCORE_BIN
    if cfg and os.path.isfile(cfg) and os.access(cfg, os.X_OK):
        return cfg
    return shutil.which("gocore")


def _call(req: dict) -> dict | None:
    """调 gocore --stdin 批量计算；失败/无二进制返回 None。"""
    binary = gocore_bin()
    if not binary:
        return None
    try:
        r = subprocess.run(
            [binary, "--stdin"],
            input=json.dumps(req, ensure_ascii=False),
            capture_output=True, text=True, timeout=_GOCORE_TIMEOUT,
        )
    except Exception:
        return None
    if r.returncode != 0:
        return None
    try:
        out = json.loads(r.stdout)
    except Exception:
        return None
    return out if isinstance(out, dict) and out.get("ok") else None


def _resolved(skel3d: dict, motion: dict, params: dict | None) -> dict:
    """解析动作参数为数值（与 Python _resolve_params 一致，refs=骨架坐标参数）。"""
    from .motion import _resolve_params
    return _resolve_params(motion, params or {}, refs=skel3d.get("params") or {})


def batch_pose(skel3d: dict, motion: dict, frames: list[int],
               params: dict) -> dict | None:
    """批量 FK pose（Go 加速）：返回 {frame: {joint: [x,y,z]}}；无 gocore 返回 None。

    skel3d 为 build_skeleton_3d 结果（joints/fk_tree/fk_local），motion 为动作 dict。
    """
    resolved = _resolved(skel3d, motion, params)
    req = {
        "task": "pose",
        "joints": skel3d.get("joints"),
        "fk_tree": skel3d.get("fk_tree"),
        "fk_local": skel3d.get("fk_local"),
        "motion": motion,
        "frames": list(frames),
        "params": resolved,
    }
    res = _call(req)
    if res is None:
        return None
    return {f["frame"]: f["pose"] for f in res.get("frames", [])}


def batch_lbs(skel3d: dict, motion: dict, frames: list[int],
              mesh: dict, weights: dict, params: dict,
              body_scale: float = 0.0) -> dict | None:
    """批量 LBS 顶点蒙皮（Go 加速）：返回 {frame: flat_vertices}；无 gocore 返回 None。"""
    resolved = _resolved(skel3d, motion, params)
    req = {
        "task": "lbs",
        "joints": skel3d.get("joints"),
        "fk_tree": skel3d.get("fk_tree"),
        "fk_local": skel3d.get("fk_local"),
        "motion": motion,
        "frames": list(frames),
        "params": resolved,
        "mesh": mesh,
        "weights": weights,
        "body_scale": float(body_scale or 0.0),
    }
    res = _call(req)
    if res is None:
        return None
    return {f["frame"]: f["vertices"] for f in res.get("frames", [])}
