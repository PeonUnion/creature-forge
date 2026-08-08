#!/usr/bin/env python3
"""CreatureForge CLI — 与 HTTP API 同级的命令行工具（直接交互，不启动 server）。

CLI 与 HTTP（server.py）共用同一套 Api 接口（creatureforge.interfaces.Api，
实现为 creatureforge.api.ApiService）→ 两侧行为一致，避免漂移。

用法示例：
  python -m creatureforge.cli species list
  python -m creatureforge.cli species schema human
  python -m creatureforge.cli action list
  python -m creatureforge.cli preset new human
  python -m creatureforge.cli preset create --json '{"preset_id":"m","species":"human","title":"M","body":{"head_scale":1.2}}'
  python -m creatureforge.cli render skeleton human --out skel.png --yaw 45 --body head_scale=1.2,shoulder_width=1.4
  python -m creatureforge.cli render motion walk3d --gif --out walk.gif
  python -m creatureforge.cli render preset m --action walk3d --out walk.gif
"""
from __future__ import annotations

import argparse
import base64
import json
import sys
from pathlib import Path

_PKG_ROOT = Path(__file__).resolve().parent
_REPO_ROOT = _PKG_ROOT.parent
if str(_REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(_REPO_ROOT))

from creatureforge.api import make_api  # noqa: E402
from creatureforge.config import DEFAULT_DATA_DIR, ensure_species_seeded  # noqa: E402
from creatureforge import updater  # noqa: E402

_DATA_DIR: Path | None = None  # --data-dir 覆盖（默认仓库根 data/；打包运行时用户目录）


def api():
    data_dir = ensure_species_seeded(_DATA_DIR or DEFAULT_DATA_DIR)
    return make_api(data_dir / "species", data_dir / "presets")


# -- 输出辅助 --


def _json(data) -> None:
    print(json.dumps(data, ensure_ascii=False, indent=2))


def _save_data_url(out: Path, data_url: str) -> None:
    raw = data_url.split(",", 1)[1] if data_url.startswith("data:") else data_url
    out.write_bytes(base64.b64decode(raw))
    print(f"已写入 {out}")


def _parse_kv(s: str | None) -> dict:
    """解析 'a=1,b=2' → {a:1.0, b:2.0}（体型参数等）。"""
    out: dict = {}
    for pair in (s or "").split(","):
        if "=" not in pair:
            continue
        k, v = pair.split("=", 1)
        try:
            out[k.strip()] = float(v.strip())
        except ValueError:
            out[k.strip()] = v.strip()
    return out


def _parse_pos(s: str | None) -> list[float] | None:
    """解析 'x,y,z' → [x,y,z]（向导关节/姿态坐标）。"""
    if not s:
        return None
    try:
        return [float(x) for x in s.split(",")]
    except ValueError:
        raise ValueError(f"非法坐标: {s}（期望 x,y,z）")


def _parse_list(s: str | None) -> list[str]:
    """解析 'a,b,c' → [a,b,c]（向导链/参数链关节）。"""
    return [x.strip() for x in (s or "").split(",") if x.strip()]


def _load_json_arg(args) -> dict:
    if getattr(args, "json", None):
        return json.loads(args.json)
    if getattr(args, "file", None):
        return json.loads(Path(args.file).read_text(encoding="utf-8"))
    return {}


# -- 命令实现 --


def cmd_species(args) -> None:
    svc = api()
    if args.sub == "list":
        items = svc.species_list()
        if not items:
            print("(无物种)")
            return
        for it in items:
            print(f"  {it['id']:16s} {it.get('title',''):24s} 关节{it.get('joint_count','-')} 动作{len(it.get('actions') or [])}")
    elif args.sub == "show":
        _json(svc.species_get(args.id))
    elif args.sub == "schema":
        _json(svc.species_preset_schema(args.id))
    elif args.sub == "default":
        _json(svc.species_default(args.id))
    elif args.sub == "create":
        data = _load_json_arg(args)
        print("created:", svc.species_create(data))
    elif args.sub == "update":
        print("updated:", svc.species_update(args.id, _load_json_arg(args)))
    elif args.sub == "delete":
        print("deleted:", svc.species_delete(args.id))
    # -- 物种分步向导（模板可选择 + custom 从 0 开始） --
    elif args.sub == "templates":
        for t in svc.templates_list():
            print(f"  {t['morph_id']:12s} {t['title']:18s} 关节{t['joint_count']:3d} 链{t['chain_count']:3d} 动作{len(t['actions'])} — {t['description'][:44]}")
    elif args.sub == "wizard":
        cmd_wizard(svc, args.species)
    elif args.sub == "edit":
        cmd_edit(svc, args.species)
    elif args.sub == "wizard-init":
        _json(svc.wizard_init(args.species, morph_id=args.template, title=args.title, description=args.desc))
    elif args.sub == "joint-add":
        _json(svc.wizard_add_joint(args.species, args.name, parent=args.parent, pos=_parse_pos(args.pos), sym=args.sym))
    elif args.sub == "joint-rm":
        _json(svc.wizard_remove_joint(args.species, args.name))
    elif args.sub == "joint-rename":
        _json(svc.wizard_rename_joint(args.species, args.old, args.new))
    elif args.sub == "joint-parent":
        _json(svc.wizard_set_parent(args.species, args.name, args.parent))
    elif args.sub == "limb-mirror":
        _json(svc.wizard_mirror_limb(args.species, args.source, to_prefix=args.to_prefix))
    elif args.sub == "chain-add":
        _json(svc.wizard_add_chain(args.species, args.name, _parse_list(args.joints)))
    elif args.sub == "chain-rm":
        _json(svc.wizard_remove_chain(args.species, args.name))
    elif args.sub == "pose-set":
        _json(svc.wizard_set_pose(args.species, args.name, _parse_pos(args.pos)))
    elif args.sub == "canvas":
        _json(svc.wizard_set_canvas(args.species, width=args.width, height=args.height, floor_y=args.floor_y))
    elif args.sub == "param-add":
        _json(svc.wizard_add_param_chain(args.species, args.name, _parse_list(args.joints), anchor=args.anchor, label=args.label))
    elif args.sub == "wizard-commit":
        print("created:", svc.wizard_commit(args.species))
    elif args.sub == "wizard-discard":
        print("discarded:", svc.wizard_discard(args.species))


def cmd_wizard(svc, species_id: str) -> None:
    """交互式物种向导：选模板/从 0 → 逐步补全骨架 → commit。"""
    print(f"== CreatureForge 物种向导：{species_id} ==")
    tmpls = svc.templates_list()
    print("可选形态模板（输入编号；0 = 从 0 开始空骨架）：")
    for i, t in enumerate(tmpls, 1):
        print(f"  [{i}] {t['title']}（{t['morph_id']}）· 关节{t['joint_count']} · 动作 {len(t['actions'])}")
    try:
        choice = input("选择模板 [0=从 0 开始]: ").strip()
    except (EOFError, KeyboardInterrupt):
        return
    morph = "custom"
    if choice.isdigit() and 0 < int(choice) <= len(tmpls):
        morph = tmpls[int(choice) - 1]["morph_id"]
    try:
        title = input(f"名称 [{species_id}]: ").strip() or species_id
        desc = input("描述: ").strip()
    except (EOFError, KeyboardInterrupt):
        return
    _json(svc.wizard_init(species_id, morph_id=morph, title=title, description=desc))
    print(f"✓ 已初始化（模板 {morph}）。逐步补全骨架，指令示例：")
    print("    joint add pelvis                 ← 第一个关节（根，无 parent）")
    print("    joint add head --parent neck --pos 480,115,-4")
    print("    mirror arm_left                  ← 一键镜像对称肢")
    print("    chain add spine --joints head,neck,chest,pelvis")
    print("    pose set head --pos 480,115,-4")
    print("    param add head_scale --joints head --anchor neck --label 头大小")
    print("    list · done（提交）· quit（放弃）")
    while True:
        try:
            line = input("骨架> ").strip()
        except (EOFError, KeyboardInterrupt):
            print()
            break
        if not line:
            continue
        if line in ("done", "commit", "next"):
            break
        if line in ("quit", "exit", "discard"):
            svc.wizard_discard(species_id)
            print("已放弃")
            return
        try:
            _wizard_cmd(svc, species_id, line)
        except Exception as e:
            print(f"✗ {e}")
    try:
        svc.wizard_commit(species_id)
        print(f"✓ 物种 {species_id} 已创建（骨架 + 默认姿态 + 预设 schema）")
        print(f"  下一步：creatureforge action wizard --species {species_id}")
    except Exception as e:
        print(f"✗ 提交失败: {e}")


def cmd_edit(svc, species_id: str) -> None:
    """交互式语义化编辑已有物种（加载草稿 → 骨架操作 → commit 覆盖，非 JSON）。"""
    print(f"== CreatureForge 物种编辑：{species_id} ==")
    try:
        v = svc.wizard_get(species_id)  # 自动加载已有物种为草稿（无草稿则从 skeleton 加载）
    except Exception as e:
        print(f"✗ {e}")
        return
    print(f"✓ 已加载：{v['title']}（关节{v['joint_count']} 骨{v['bone_count']} 链{v['chain_count']} 参数{v['param_chain_count']}）")
    print("  指令：joint add/rm/rename/parent · mirror · chain add/rm · pose · param · list · done(提交) · quit(放弃)")
    while True:
        try:
            line = input("编辑> ").strip()
        except (EOFError, KeyboardInterrupt):
            print()
            break
        if not line:
            continue
        if line in ("done", "commit", "next"):
            break
        if line in ("quit", "exit"):
            print("已放弃（草稿保留，可再编辑）")
            return
        try:
            _wizard_cmd(svc, species_id, line)
        except Exception as e:
            print(f"✗ {e}")
    try:
        svc.wizard_commit(species_id)
        print(f"✓ 物种 {species_id} 已更新")
    except Exception as e:
        print(f"✗ 提交失败: {e}")


def _wizard_cmd(svc, species_id: str, line: str) -> None:
    """解析向导交互行（骨架操作子命令）。"""
    toks = line.split()
    if not toks:
        return
    cmd = toks[0]
    args_ = toks[1:]

    def opt(name: str) -> str | None:
        for i, t in enumerate(args_):
            if t == name:
                return args_[i + 1] if i + 1 < len(args_) else None
        return None

    def status():
        v = svc.wizard_get(species_id)
        print(f"  → 关节{v['joint_count']} 骨{v['bone_count']} 链{v['chain_count']} 参数链{v['param_chain_count']}")

    if cmd in ("joint", "j"):
        sub = args_[0] if args_ else "add"
        if sub == "add":
            name = args_[1] if len(args_) > 1 else opt("--name")
            svc.wizard_add_joint(species_id, name, parent=opt("--parent"),
                                 pos=_parse_pos(opt("--pos")), sym=opt("--sym"))
            print(f"  ✓ 加关节 {name}")
        elif sub == "rm" and len(args_) > 1:
            svc.wizard_remove_joint(species_id, args_[1])
            print(f"  ✓ 删关节 {args_[1]}")
        elif sub == "rename" and len(args_) > 2:
            svc.wizard_rename_joint(species_id, args_[1], args_[2])
            print("  ✓ 重命名")
        elif sub == "parent" and len(args_) > 1:
            svc.wizard_set_parent(species_id, args_[1], opt("--parent"))
            print("  ✓ 改父级")
        else:
            print("  joint add <name> [--parent p] [--pos x,y,z] [--sym s] | joint rm/rename/parent")
            return
        status()
    elif cmd in ("mirror", "m"):
        svc.wizard_mirror_limb(species_id, args_[0], to_prefix=opt("--to-prefix"))
        print(f"  ✓ 镜像 {args_[0]}")
        status()
    elif cmd in ("chain", "c"):
        sub = args_[0] if args_ else "list"
        if sub == "add":
            name = args_[1] if len(args_) > 1 else opt("--name")
            svc.wizard_add_chain(species_id, name, _parse_list(opt("--joints")))
            print(f"  ✓ 链 {name}")
            status()
        elif sub == "rm" and len(args_) > 1:
            svc.wizard_remove_chain(species_id, args_[1])
            print(f"  ✓ 删链 {args_[1]}")
            status()
        else:
            print("  chains:", svc.wizard_get(species_id)["chains"])
    elif cmd in ("pose", "p"):
        svc.wizard_set_pose(species_id, args_[0], _parse_pos(opt("--pos")))
        print(f"  ✓ 姿态 {args_[0]}")
    elif cmd in ("param", "param-add"):
        name = args_[0] if args_ else opt("--name")
        svc.wizard_add_param_chain(species_id, name, _parse_list(opt("--joints")),
                                   anchor=opt("--anchor"), label=opt("--label"))
        print(f"  ✓ 参数链 {name}")
        status()
    elif cmd in ("list", "show", "status"):
        v = svc.wizard_get(species_id)
        print(f"  关节: {list(v['nodes'])}")
        print(f"  链: {v['chains']}")
        print(f"  参数链: {list(v['param_chains'])}")
    else:
        print("  ✗ 未知指令。支持：joint add/rm/rename/parent · mirror · chain add/rm · pose · param · list")


def cmd_action(args) -> None:
    svc = api()
    if args.sub == "list":
        for a in svc.actions_list_all():
            print(f"  {a['id']:16s} {a.get('title',''):28s} [{a['species']}] params={list(a.get('params') or {})}")
    elif args.sub == "show":
        _json(svc.action_get(args.species, args.id))
    elif args.sub == "create":
        print("saved:", svc.action_create(args.species, _load_json_arg(args)))
    elif args.sub == "update":
        print("saved:", svc.action_update(args.species, args.id, _load_json_arg(args)))
    elif args.sub == "delete":
        print("deleted:", svc.action_delete(args.species, args.id))


def cmd_preset(args) -> None:
    svc = api()
    if args.sub == "list":
        for p in svc.presets_list():
            print(f"  {p['preset_id']:20s} {p.get('title',''):24s} [{p['species']}]")
    elif args.sub == "new":
        _json(svc.preset_new(args.species))
    elif args.sub == "show":
        _json(svc.preset_get(args.id))
    elif args.sub == "create":
        print("created:", svc.preset_create(_load_json_arg(args)))
    elif args.sub == "update":
        print("updated:", svc.preset_update(args.id, _load_json_arg(args)))
    elif args.sub == "delete":
        print("deleted:", svc.preset_delete(args.id))


def cmd_skin(args) -> None:
    svc = api()
    if args.sub == "list":
        for s in svc.skins_list():
            print(f"  {s['skin_id']:20s} {s.get('title',''):24s} [预设:{s.get('preset','')} 物种:{s['species']}]", file=sys.stdout)
    elif args.sub == "new":
        _json(svc.skin_new(args.preset))
    elif args.sub == "show":
        _json(svc.skin_get(args.id))
    elif args.sub == "create":
        print("created:", svc.skin_create(_load_json_arg(args)))
    elif args.sub == "update":
        print("updated:", svc.skin_update(args.id, _load_json_arg(args)))
    elif args.sub == "delete":
        print("deleted:", svc.skin_delete(args.id))
    elif args.sub == "parts":
        skin = svc.skin_get(args.id)
        for p in skin.get("parts", []):
            print(f"  {p['part_id']:20s} {p.get('title',''):20s} [{p.get('kind')}] bone={p.get('bone')} mesh={p.get('mesh_file') or '内嵌'}", file=sys.stdout)
    elif args.sub == "part-add":
        print("part:", svc.skin_part_add(args.id, _load_json_arg(args)))
    elif args.sub == "part-del":
        print("deleted:", svc.skin_part_delete(args.id, args.part))


def cmd_render(args) -> None:
    svc = api()
    out = Path(args.out)
    if args.mode == "skeleton":
        data_url = svc.render_skeleton3d(
            args.id, yaw=args.yaw, pitch=args.pitch, dist=args.dist,
            pan_x=args.pan_x, pan_y=args.pan_y, grid=args.grid,
            body=_parse_kv(args.body) or None)
        _save_data_url(out, data_url)
    elif args.mode == "motion":
        result = svc.render_motion3d(
            args.id, species=args.species, yaw=args.yaw, pitch=args.pitch, dist=args.dist,
            pan_x=args.pan_x, pan_y=args.pan_y, grid=args.grid,
            gif=args.gif, frames=False)
        if "gif" in result:
            _save_data_url(out, result["gif"])
        else:
            _save_data_url(out, result.get("data_url", ""))
    elif args.mode == "preset":
        result = svc.render_preset3d(
            args.id, action_id=args.action, yaw=args.yaw, pitch=args.pitch, dist=args.dist,
            pan_x=args.pan_x, pan_y=args.pan_y, grid=args.grid, gif=args.gif)
        if "gif" in result:
            _save_data_url(out, result["gif"])
        else:
            _save_data_url(out, result.get("data_url", ""))
    elif args.mode == "live":
        result = svc.render_preset3d(
            "live", species=args.species, body=_parse_kv(args.body) or None,
            actions=_parse_kv(args.actions) or None, action_id=args.action,
            yaw=args.yaw, pitch=args.pitch, dist=args.dist,
            pan_x=args.pan_x, pan_y=args.pan_y, grid=args.grid, gif=args.gif)
        if "gif" in result:
            _save_data_url(out, result["gif"])
        else:
            _save_data_url(out, result.get("data_url", ""))
    elif args.mode == "skin":
        # 蒙皮 glTF 导出：骨骼 + 蒙皮网格 + 真实动捕动作动画 → .glb（Godot/Unity/Blender）
        # 预设（--preset）+ 皮肤（--skin）可选：应用体型/动作参数 + 皮肤材质/体态
        result = svc.export_glb(args.id, species=args.species, preset=args.preset,
                                skin_id=args.skin, out=args.out)
        _json(result)


# -- 参数解析 --

def _add_render_opts(sp) -> None:
    sp.add_argument("--yaw", type=float, default=0, help="水平角 0-360")
    sp.add_argument("--pitch", type=float, default=0, help="俯仰角 -89..89")
    sp.add_argument("--dist", type=float, default=1, help="距离倍数（1=自动适配，大于 1 拉远、小于 1 拉近）")
    sp.add_argument("--no-grid", action="store_false", dest="grid", help="不绘制地面辅助网格")
    sp.add_argument("--pan-x", type=float, default=0, dest="pan_x")
    sp.add_argument("--pan-y", type=float, default=0, dest="pan_y")
    sp.add_argument("--out", required=True, help="输出文件（.png / .gif）")
    sp.add_argument("--gif", action="store_true", help="输出 GIF")


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="creatureforge",
        description="CreatureForge CLI — 与 HTTP API 同级，共用同一套接口（不启动 server）")
    p.add_argument("--data-dir", default=None, help="数据目录（默认仓库根 data/）")
    p.add_argument("--version", "-V", action="version",
                   version=f"CreatureForge CLI {updater.current_version()} ({updater.current_platform()})")
    sub = p.add_subparsers(dest="cmd", required=True)

    # species（含分步向导：模板可选择 + custom 从 0 开始）
    sp = sub.add_parser("species", help="物种管理（含分步向导）")
    ssp = sp.add_subparsers(dest="sub", required=True)
    ssp.add_parser("list", help="物种列表")
    ssp.add_parser("templates", help="列形态模板（可选起步，含从 0 开始）")
    wiz = ssp.add_parser("wizard", help="交互式分步向导（建物种）")
    wiz.add_argument("species", help="新物种 id")
    ed = ssp.add_parser("edit", help="交互式语义化编辑已有物种（非 JSON）")
    ed.add_argument("species")
    s1 = ssp.add_parser("show"); s1.add_argument("id")
    s1 = ssp.add_parser("schema"); s1.add_argument("id", help="预设 schema")
    s1 = ssp.add_parser("default"); s1.add_argument("id")
    c = ssp.add_parser("create"); c.add_argument("--json"); c.add_argument("--file")
    c = ssp.add_parser("update"); c.add_argument("id"); c.add_argument("--json"); c.add_argument("--file")
    d = ssp.add_parser("delete"); d.add_argument("id")
    # -- 向导分步（非交互，可脚本化；与 Web/HTTP 同一套 WizardService） --
    w1 = ssp.add_parser("wizard-init"); w1.add_argument("species")
    w1.add_argument("--template", default="custom", help="形态模板 id（默认 custom=从 0 开始）")
    w1.add_argument("--title", default=""); w1.add_argument("--desc", default="")
    w1 = ssp.add_parser("joint-add"); w1.add_argument("species"); w1.add_argument("name")
    w1.add_argument("--parent"); w1.add_argument("--pos", help="x,y,z"); w1.add_argument("--sym")
    w1 = ssp.add_parser("joint-rm"); w1.add_argument("species"); w1.add_argument("name")
    w1 = ssp.add_parser("joint-rename"); w1.add_argument("species"); w1.add_argument("old"); w1.add_argument("new")
    w1 = ssp.add_parser("joint-parent"); w1.add_argument("species"); w1.add_argument("name"); w1.add_argument("--parent")
    w1 = ssp.add_parser("limb-mirror"); w1.add_argument("species"); w1.add_argument("source"); w1.add_argument("--to-prefix")
    w1 = ssp.add_parser("chain-add"); w1.add_argument("species"); w1.add_argument("name"); w1.add_argument("--joints", required=True, help="逗号分隔关节")
    w1 = ssp.add_parser("chain-rm"); w1.add_argument("species"); w1.add_argument("name")
    w1 = ssp.add_parser("pose-set"); w1.add_argument("species"); w1.add_argument("name"); w1.add_argument("--pos", required=True, help="x,y,z")
    w1 = ssp.add_parser("canvas"); w1.add_argument("species")
    w1.add_argument("--width", type=float); w1.add_argument("--height", type=float); w1.add_argument("--floor-y", type=float)
    w1 = ssp.add_parser("param-add"); w1.add_argument("species"); w1.add_argument("name")
    w1.add_argument("--joints", required=True, help="逗号分隔关节"); w1.add_argument("--anchor"); w1.add_argument("--label")
    w1 = ssp.add_parser("wizard-commit"); w1.add_argument("species")
    w1 = ssp.add_parser("wizard-discard"); w1.add_argument("species")

    # action
    ap = sub.add_parser("action", help="动作管理")
    asp = ap.add_subparsers(dest="sub", required=True)
    asp.add_parser("list", help="跨物种动作列表")
    a1 = asp.add_parser("show"); a1.add_argument("species"); a1.add_argument("id")
    a1 = asp.add_parser("create"); a1.add_argument("species"); a1.add_argument("--json"); a1.add_argument("--file")
    a1 = asp.add_parser("update"); a1.add_argument("species"); a1.add_argument("id"); a1.add_argument("--json"); a1.add_argument("--file")
    a1 = asp.add_parser("delete"); a1.add_argument("species"); a1.add_argument("id")

    # preset
    pp = sub.add_parser("preset", help="预设管理（独立入口，调体型 + 动作幅度）")
    psp = pp.add_subparsers(dest="sub", required=True)
    psp.add_parser("list")
    p1 = psp.add_parser("new"); p1.add_argument("species", help="新建空白表单（含 schema）")
    p1 = psp.add_parser("show"); p1.add_argument("id")
    p1 = psp.add_parser("create"); p1.add_argument("--json"); p1.add_argument("--file")
    p1 = psp.add_parser("update"); p1.add_argument("id"); p1.add_argument("--json"); p1.add_argument("--file")
    p1 = psp.add_parser("delete"); p1.add_argument("id")

    # skin
    sp_ = sub.add_parser("skin", help="皮肤管理（独立入口，基于物种，可多实例）")
    ssp_ = sp_.add_subparsers(dest="sub", required=True)
    ssp_.add_parser("list")
    s1_ = ssp_.add_parser("new"); s1_.add_argument("preset", help="基于的预设 id（皮肤基于预设）")
    s1_ = ssp_.add_parser("show"); s1_.add_argument("id")
    s1_ = ssp_.add_parser("create"); s1_.add_argument("--json"); s1_.add_argument("--file")
    s1_ = ssp_.add_parser("update"); s1_.add_argument("id"); s1_.add_argument("--json"); s1_.add_argument("--file")
    s1_ = ssp_.add_parser("delete"); s1_.add_argument("id")
    pp_ = ssp_.add_parser("parts"); pp_.add_argument("id", help="皮肤 id（列出部件）")
    pa_ = ssp_.add_parser("part-add"); pa_.add_argument("id"); pa_.add_argument("--json"); pa_.add_argument("--file")
    pd_ = ssp_.add_parser("part-del"); pd_.add_argument("id"); pd_.add_argument("part")

    # render
    rp = sub.add_parser("render", help="3D 渲染到文件")
    rsub = rp.add_subparsers(dest="mode", required=True)
    r1 = rsub.add_parser("skeleton"); r1.add_argument("id", help="species id")
    r1.add_argument("--body", help="体型参数 a=1,b=2"); _add_render_opts(r1)
    r1 = rsub.add_parser("motion"); r1.add_argument("id", help="action id")
    r1.add_argument("--species", help="限定物种"); _add_render_opts(r1)
    r1 = rsub.add_parser("preset"); r1.add_argument("id", help="preset id")
    r1.add_argument("--action", help="动作 id（省略渲染骨架）"); _add_render_opts(r1)
    r1 = rsub.add_parser("live"); r1.add_argument("--species", required=True)
    r1.add_argument("--body", help="体型参数 a=1,b=2"); r1.add_argument("--actions", help="动作参数 walk3d=intensity=1.2")
    r1.add_argument("--action", help="动作 id"); _add_render_opts(r1)
    r1 = rsub.add_parser("skin"); r1.add_argument("id", help="action id")
    r1.add_argument("--species", help="限定物种")
    r1.add_argument("--preset", help="预设 id（提供物种+体型+动作参数）")
    r1.add_argument("--skin", dest="skin", help="皮肤 id（应用材质 + 体态）"); _add_render_opts(r1)

    # self-update
    up = sub.add_parser("upgrade", help="检查并更新到最新版（GitHub Releases）")
    up.add_argument("--check", action="store_true", help="仅检测是否有新版本（有则退出码 2）")
    up.add_argument("--yes", "-y", action="store_true", help="跳过确认直接更新")
    up.add_argument("--force", action="store_true", help="即使版本相同也重新安装")
    up.add_argument("--repo", default=updater.DEFAULT_REPO, help=f"更新源仓库（默认 {updater.DEFAULT_REPO}）")
    up.add_argument("--channel", default="latest", choices=["latest", "prerelease"],
                    help="更新通道：latest=正式版（默认），prerelease=含预览版")

    return p


def main(argv: list[str] | None = None) -> int:
    global _DATA_DIR
    args = build_parser().parse_args(argv)
    if args.data_dir:
        _DATA_DIR = Path(args.data_dir)
    try:
        if args.cmd == "species":
            cmd_species(args)
        elif args.cmd == "action":
            cmd_action(args)
        elif args.cmd == "preset":
            cmd_preset(args)
        elif args.cmd == "skin":
            cmd_skin(args)
        elif args.cmd == "render":
            cmd_render(args)
        elif args.cmd == "upgrade":
            return updater.run_upgrade(args)
        else:
            build_parser().print_help()
    except KeyError as e:
        print(f"错误: {e}", file=sys.stderr)
        return 1
    except Exception as e:  # noqa: BLE001
        print(f"错误: {e}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
