#!/usr/bin/env python3
"""自动从 git 历史生成 CHANGELOG（Conventional Commits + tag 区间，零第三方依赖）。

用法:
  python scripts/gen_changelog.py                    # 生成全部版本节（stdout）
  python scripts/gen_changelog.py --tag v0.1.0-rc.2  # 只生成该 tag 区间
  python scripts/gen_changelog.py --write            # 写回 CHANGELOG.md（替换版本节，保留文件头说明）
  python scripts/gen_changelog.py --detail           # 展开 commit body 详情（默认只用 subject）
  python scripts/gen_changelog.py --write --release v0.1.0-rc.3  # 发布：生成并固化 [Unreleased] 为新版本

发布流程（全自动，无需手动维护）:
  1) 完成功能提交（feat:/fix:/refactor: ...）
  2) python scripts/gen_changelog.py --write --release v0.1.0-rc.3
  3) git commit -m "chore: 更新 CHANGELOG (自动生成)" && git tag v0.1.0-rc.3 && git push
  4) CI（release.yml）自动从 CHANGELOG 提取该版本块作为 Release Notes

设计:
  - 版本区间 = tag 边界（lastTag..HEAD 为 [Unreleased]；相邻 tag 之间为各版本）
  - 提交按 Conventional Commits 类型自动分组：feat→Added / fix→Fixed /
    refactor/perf→Changed / breaking(! 或 BREAKING CHANGE)→Breaking / 其余→Chore
  - 不依赖提交者素质之外的任何人工维护；不硬编码版本号/条目（全部来自 git 数据）
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from datetime import datetime
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

# Conventional Commits 类型 → Keep a Changelog 小节
SECTION_MAP: dict[str, str] = {
    "feat": "Added",
    "feature": "Added",
    "fix": "Fixed",
    "bugfix": "Fixed",
    "refactor": "Changed",
    "perf": "Changed",
    "build": "Changed",
    "style": "Changed",
    "breaking": "Breaking",
}
# 这些类型不进 Changelog（维护噪音），除非带 breaking
IGNORE_TYPES = {"docs", "test", "tests", "ci", "chore", "misc", "wip", "merge", "revert"}

COMMIT_RE = re.compile(
    r"^(?P<type>[a-zA-Z][a-zA-Z0-9_-]*)"          # feat
    r"(?:\((?P<scope>[^)]*)\))?"                   # (scope)
    r"(?P<breaking>!)?:"                           # !:
    r"\s*(?P<subject>.+)$"                          # subject
)
BREAKING_RE = re.compile(r"BREAKING[ -]CHANGE:?\s*(.*)", re.IGNORECASE)


def _git(args: list[str]) -> str:
    r = subprocess.run(["git", *args], capture_output=True, text=True, cwd=ROOT)
    if r.returncode != 0:
        raise RuntimeError(f"git {' '.join(args)} 失败: {r.stderr.strip()}")
    return r.stdout


def _tags() -> list[str]:
    """语义版本排序的 tag 列表（旧→新）。"""
    out = _git(["tag", "--list", "--sort=v:refname"])
    return [t for t in out.splitlines() if re.match(r"^v?\d", t)]


def _tag_date(tag: str) -> str:
    d = _git(["log", "-1", "--format=%cs", tag]).strip()
    return d or datetime.now().strftime("%Y-%m-%d")


def _commits(rng: str) -> list[dict]:
    """区间内提交（旧→新）：{hash, type, scope, breaking, subject, body}。"""
    if not rng:
        return []
    raw = _git(["log", "--reverse", "--format=%x1e%H%x1f%s%x1f%b", rng])
    out = []
    for rec in raw.split("\x1e"):
        rec = rec.strip()
        if not rec:
            continue
        h, subj, body = (rec.split("\x1f", 2) + ["", ""])[:3]
        m = COMMIT_RE.match(subj)
        if not m:
            continue  # 非 Conventional 提交跳过（不猜测）
        breaking = bool(m.group("breaking"))
        if not breaking and BREAKING_RE.search(body):
            breaking = True
        out.append({
            "hash": h[:7],
            "type": m.group("type").lower(),
            "scope": m.group("scope"),
            "breaking": breaking,
            "subject": m.group("subject").strip(),
            "body": body.strip(),
        })
    return out


def _fmt_item(c: dict, detail: bool) -> str:
    scope = f"**{c['scope']}**: " if c.get("scope") else ""
    line = f"- {scope}{c['subject']}"
    if detail and c.get("body"):
        for bl in c["body"].splitlines():
            bl = bl.strip().lstrip("-")
            if bl:
                line += f"\n  - {bl}"
    return line


def _sections(commits: list[dict], detail: bool) -> str:
    """按类型分组输出 Markdown 小节。"""
    groups: dict[str, list[dict]] = {}
    for c in commits:
        key = "Breaking" if c["breaking"] else SECTION_MAP.get(c["type"], "")
        if not key and c["type"] in IGNORE_TYPES:
            continue
        if not key:
            key = "Chore"  # 未知但有意义的类型兜底
        groups.setdefault(key, []).append(c)
    out = []
    for section in ("Added", "Fixed", "Changed", "Breaking", "Chore"):
        items = groups.get(section)
        if not items:
            continue
        out.append(f"### {section}\n")
        for c in items:
            out.append(_fmt_item(c, detail))
        out.append("")
    return "\n".join(out).rstrip() + "\n"


def gen_version(tag: str | None, prev_tag: str | None, detail: bool) -> str:
    """生成单个版本节。tag=None 表示 [Unreleased]（HEAD 到 prev_tag）。"""
    if tag is None:
        rng = f"HEAD" if not prev_tag else f"{prev_tag}..HEAD"
        title = "## [Unreleased]\n"
        commits = _commits(rng)
    else:
        rng = tag if not prev_tag else f"{prev_tag}..{tag}"
        title = f"## [{tag.lstrip('v')}] - {_tag_date(tag)}\n"
        commits = _commits(rng)
    body = _sections(commits, detail)
    if not body:
        body = "_无面向用户的可见变更。_\n"
    return f"{title}\n{body}\n"


def gen_all(detail: bool, only_tag: str | None = None, release: str | None = None) -> str:
    tags = _tags()
    if only_tag and only_tag not in tags:
        raise SystemExit(f"tag 不存在: {only_tag}（现有: {', '.join(tags)}）")
    out = []
    # [Unreleased]（HEAD → 最新 tag）；--release 时固化为正式版本节
    latest = tags[-1] if tags else None
    if release:
        ver = release.lstrip("v")
        if release in tags:
            raise SystemExit(f"版本已存在: {release}（勿用已发布的 tag 作 --release）")
        head_date = _git(["log", "-1", "--format=%cs", "HEAD"]).strip() or datetime.now().strftime("%Y-%m-%d")
        rng = f"HEAD" if not latest else f"{latest}..HEAD"
        commits = _commits(rng)
        body = _sections(commits, detail) or "_无面向用户的可见变更。_\n"
        out.append(f"## [{ver}] - {head_date}\n\n{body}\n")
        latest = release  # 固化后，该版本成为新的最新 tag（避免重复进下面循环）
    else:
        out.append(gen_version(None, latest, detail))
    # 各版本节（新→旧）
    if only_tag:
        i = tags.index(only_tag)
        prev = tags[i - 1] if i > 0 else None
        out.append(gen_version(only_tag, prev, detail))
    else:
        for i in range(len(tags) - 1, -1, -1):
            prev = tags[i - 1] if i > 0 else None
            out.append(gen_version(tags[i], prev, detail))
    return "\n".join(out)


def write_changelog(generated: str) -> Path:
    """写回 CHANGELOG.md：保留文件头说明，替换全部版本节。"""
    p = ROOT / "CHANGELOG.md"
    text = p.read_text(encoding="utf-8")
    # 保留首个 "## [" 之前的内容（说明/规范）
    m = re.search(r"^## \[", text, re.M)
    header = text[: m.start()] if m else text
    p.write_text(header.rstrip() + "\n\n" + generated, encoding="utf-8")
    return p


def main() -> None:
    ap = argparse.ArgumentParser(description="自动生成 CHANGELOG（Conventional Commits + tag 区间）")
    ap.add_argument("--tag", help="只生成指定 tag 的版本节")
    ap.add_argument("--release", help="将 [Unreleased] 固化为指定版本（如 v0.1.0-rc.3）")
    ap.add_argument("--raw", action="store_true", help="仅输出 --tag 区间节体（无版本标题），供 CI Release Notes 用")
    ap.add_argument("--write", action="store_true", help="写回 CHANGELOG.md（默认只打印 stdout）")
    ap.add_argument("--detail", action="store_true", help="展开 commit body 详情")
    args = ap.parse_args()
    if args.raw:
        if not args.tag:
            ap.error("--raw 需要 --tag")
        tags = _tags()
        if args.tag not in tags:
            raise SystemExit(f"tag 不存在: {args.tag}（现有: {', '.join(tags)}）")
        i = tags.index(args.tag)
        prev = tags[i - 1] if i > 0 else None
        rng = args.tag if not prev else f"{prev}..{args.tag}"
        body = _sections(_commits(rng), args.detail)
        print(body or "_无面向用户的可见变更。_", end="")
        return
    gen = gen_all(args.detail, args.tag, args.release)
    if args.write:
        p = write_changelog(gen)
        print(f"已写回 {p.relative_to(ROOT)}")
    else:
        print(gen, end="")


if __name__ == "__main__":
    main()
