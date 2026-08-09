#!/usr/bin/env python3
"""CreatureForge — 3D 动作 DSL 求值器（数据驱动）。

3D 动作（species/<id>/actions3d/*.json）用 offsets3d / root3d / ik3d + signals
描述每帧关节运动。本模块只提供通用表达式求值（_eval）与参数解析
（_build_signals / _resolve_params），供 skeleton3d.pose_3d 与
verify_motions3d 使用。所有定义均来自动作 JSON 数据，无任何硬编码。

3D 姿势求值（pose_3d / IK / 层级跟随 / 刚性传播）在 creatureforge/skeleton3d.py。
"""
from __future__ import annotations

import math


class MotionError(Exception):
    """Raised for invalid motion data or expressions."""


def _eval(expr, ctx: dict):
    """Evaluate a motion expression against a context dict.

    ``ctx`` keys: params, index, frame_count, phase, signals (name -> fn(ctx)).
    """
    if isinstance(expr, bool):
        return 1.0 if expr else 0.0
    if isinstance(expr, (int, float)):
        return float(expr)
    if isinstance(expr, str):
        return ctx["signals"][expr](ctx)
    if isinstance(expr, dict):
        if len(expr) != 1:
            raise MotionError(f"expression must be a single-op dict: {expr!r}")
        op, arg = next(iter(expr.items()))
        if op == "param":
            return float(ctx["params"][arg])
        if op == "phase":
            return ctx["phase"]
        if op == "index":
            return float(ctx["index"])
        if op == "frame_count":
            return float(ctx["frame_count"])
        if op == "const":
            return float(arg)
        if op == "signal":
            return ctx["signals"][arg](ctx)
        if op == "sin":
            return math.sin(_eval(arg, ctx))
        if op == "cos":
            return math.cos(_eval(arg, ctx))
        if op == "neg":
            return -_eval(arg, ctx)
        if op == "rect":
            return max(0.0, _eval(arg, ctx))
        if op == "abs":
            return abs(_eval(arg, ctx))
        if op == "add":
            return sum(_eval(a, ctx) for a in arg)
        if op == "sub":
            return _eval(arg[0], ctx) - _eval(arg[1], ctx)
        if op == "mul":
            out = 1.0
            for a in arg:
                out *= _eval(a, ctx)
            return out
        if op == "table":
            return float(arg[ctx["index"] % len(arg)])
        raise MotionError(f"unknown expression op: {op!r}")
    raise MotionError(f"cannot evaluate: {expr!r}")


def _build_signals(motion: dict) -> dict:
    """Return {signal_name: fn(ctx)} for every named signal in the preset."""
    defined = motion.get("signals", {})
    return {name: (lambda expr: (lambda c: _eval(expr, c)))(expr)
            for name, expr in defined.items()}


def _resolve_params(motion: dict, overrides: dict, refs: dict | None = None) -> dict:
    """解析动作参数：值可为数值（常量）或表达式（dict，复用 _eval DSL）。

    只接受动作 params 里定义的名字（数据驱动，无白名单）。``refs`` 提供额外
    命名空间（体型/坐标参数），动作参数表达式可引用它以及动作参数自身——
    例如 preset.actions[id].params = {"intensity": {"mul": [{"param":"head_scale"},
    {"const":1.2}]}} → 渲染时按当前体型参数求值。
    """
    defaults = {name: spec.get("default", 0.0)
                for name, spec in motion.get("params", {}).items()}
    merged = dict(defaults)
    for key, value in (overrides or {}).items():
        if key not in defaults:
            raise MotionError(f"unknown motion param: {key}")
        if isinstance(value, dict):
            ctx = {"params": {**merged, **(refs or {})},
                   "index": 0, "frame_count": 1, "phase": 0.0, "signals": {}}
            merged[key] = float(_eval(value, ctx))
        else:
            merged[key] = float(value)
    return merged


# ---------------------------------------------------------------------------
# 动作参数提取：按部位/维度把单一 intensity 拆分为多个独立可调参数
# ---------------------------------------------------------------------------
# 数据驱动：提取逻辑是引擎（遍历动作 JSON 的旋转/位移表达式），分组用关节名
# 语义归类（arm/leg/body），参数数值不硬编码——全部来自动作 JSON 自身。
# 默认各参数=1.0 时与原动作完全等价；预设/渲染可分别调节摆臂/腿部/躯干/步长/起伏。

# 关节 → 部位参数 的语义分组（关键词匹配关节名，前缀含即归类；其余归躯干）
_ACTION_GROUP_KEYS = {
    "arm_swing": ("shoulder", "elbow", "wrist", "palm", "finger", "clavicle"),
    "leg_swing": ("hip", "knee", "ankle", "toe", "heel", "foot"),
}
# 提取出的部位参数定义（label / default / min / max / step）——写入动作 JSON params
_ACTION_EXTRACT_DEFS = {
    "arm_swing": ("摆臂幅度", 1.0, 0.0, 2.0, 0.05),
    "leg_swing": ("腿部摆动", 1.0, 0.0, 2.0, 0.05),
    "body_sway": ("躯干摇摆", 1.0, 0.0, 2.0, 0.05),
    "stride": ("步长/位移", 1.0, 0.0, 2.0, 0.05),
    "bounce": ("起伏/弹跳", 1.0, 0.0, 2.0, 0.05),
}


def _group_param(joint: str) -> str:
    low = (joint or "").lower()
    for pname, keys in _ACTION_GROUP_KEYS.items():
        if any(k in low for k in keys):
            return pname
    return "body_sway"


def _mul_intensity(expr, pname: str):
    """把表达式树中所有引用 ``intensity`` 的因子替换为 ``intensity × pname``。

    动作各旋转/位移表达式形如 {"mul":[{"table":[...]},{"param":"intensity"}]}，
    替换后 → {"mul":[{"table":[...]},{"mul":[{"param":"intensity"},{"param":pname}]}]}。
    """
    if isinstance(expr, dict):
        if expr.get("param") == "intensity":
            return {"mul": [{"param": "intensity"}, {"param": pname}]}
        return {k: _mul_intensity(v, pname) for k, v in expr.items()}
    if isinstance(expr, list):
        return [_mul_intensity(v, pname) for v in expr]
    return expr


def extract_params(motion: dict) -> dict:
    """把动作的单一 ``intensity`` 提取为「整体 + 部位/维度」多参数（数据驱动）。

    - 保留 intensity（整体幅度）
    - fk3d 旋转按关节归组：arm_swing / leg_swing / body_sway
    - root3d 位移：x → stride（步长），y → bounce（起伏）
    默认各参数=1.0 时结果与原动作等价；写回前请 deep copy。
    """
    import copy
    m = copy.deepcopy(motion)
    fk = m.get("fk3d", {}).get("rotations3d", {}) or {}
    for joint, comp in fk.items():
        pname = _group_param(joint)
        for ax in ("x_rot", "y_rot", "z_rot"):
            e = comp.get(ax) if isinstance(comp, dict) else None
            if e is not None and "intensity" in str(e):
                comp[ax] = _mul_intensity(e, pname)
    root = m.get("root3d", {}) or {}
    for ax, pname in (("x", "stride"), ("y", "bounce")):
        e = root.get(ax)
        if e is not None and "intensity" in str(e):
            root[ax] = _mul_intensity(e, pname)
    params = m.setdefault("params", {})
    for pname, (label, default, mn, mx, step) in _ACTION_EXTRACT_DEFS.items():
        if pname not in params:
            params[pname] = {
                "label": label, "default": default,
                "min": mn, "max": mx, "step": step,
            }
    return m
