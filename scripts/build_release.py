#!/usr/bin/env python3
"""CreatureForge 跨平台二进制构建：server（嵌入 web）+ cli，基于 pyinstaller。

产物（dist/）：
    creature-forge-server[.exe]   — HTTP server（嵌入 Vue 前端 web/dist + 物种数据 data/species）
    creature-forge-cli[.exe]      — 命令行工具（嵌入物种数据）

前置：
    1) 前端已构建：cd creatureforge/web && pnpm build   （生成 creatureforge/web/dist）
    2) 已安装 pyinstaller：pip install pyinstaller

运行：
    python scripts/build_release.py

设计要点：
    - --add-data 用 os.pathsep 分隔（Windows ';' / POSIX ':'）→ 跨平台
    - --onefile：单文件二进制；运行时资源解压到 sys._MEIPASS
    - 嵌入 data/species（只读资产），运行时由 config.ensure_species_seeded()
      首次播种到用户可写目录（presets 持久化）
"""

from __future__ import annotations

import os
import platform
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
WEB_DIST = ROOT / "creatureforge" / "web" / "dist"
DATA_SPECIES = ROOT / "data" / "species"
SEP = ";" if os.name == "nt" else ":"
EXE = ".exe" if os.name == "nt" else ""

# 版本号：优先环境变量 ASSETSLAB_VERSION（CI 从 tag 提取），否则取仓库最近 tag/commit
VERSION = os.environ.get("ASSETSLAB_VERSION", "").strip()
# 平台+架构后缀：CI 传 ASSETSLAB_PLATFORM（如 linux-x64 / windows-x64 / macos-arm64）
PLATFORM = os.environ.get("ASSETSLAB_PLATFORM", "").strip()


def _platform_suffix() -> str:
    """产物平台架构后缀（保证三平台产物名不冲突）：如 -linux-x64。"""
    if PLATFORM:
        return "-" + PLATFORM
    # 本地 fallback：检测当前平台+架构
    machine = {
        "x86_64": "x64", "AMD64": "x64", "aarch64": "arm64", "arm64": "arm64",
        "x86": "x86", "i386": "x86", "armv7l": "arm",
    }.get(platform.machine(), platform.machine().lower())
    osname = {"linux": "linux", "windows": "windows", "darwin": "macos"}.get(sys.platform, sys.platform)
    return f"-{osname}-{machine}"


def _version_suffix() -> str:
    if VERSION:
        return VERSION.lstrip("v")
    # 本地：尝试 git 最近 tag，否则短 commit
    try:
        tag = subprocess.run(
            ["git", "describe", "--tags", "--abbrev=0"],
            capture_output=True, text=True, cwd=ROOT,
        ).stdout.strip()
        if tag:
            return tag.lstrip("v")
    except Exception:
        pass
    return "dev"


def _build(name: str, entry: Path) -> None:
    """pyinstaller --onefile 打包单个入口（嵌入 web + 物种数据）。"""
    cmd = [
        sys.executable, "-m", "PyInstaller",
        "--noconfirm", "--clean",
        "--onefile",
        "--name", name,
        "--add-data", f"{WEB_DIST}{SEP}web/dist",
        "--add-data", f"{DATA_SPECIES}{SEP}data/species",
        str(entry),
    ]
    print(f"\n=== Building {name} ===")
    print("  " + " ".join(cmd))
    subprocess.run(cmd, cwd=ROOT, check=True)


def main() -> None:
    if not WEB_DIST.is_dir() or not (WEB_DIST / "index.html").is_file():
        sys.exit(f"[x] Frontend not built: {WEB_DIST}\n    Run first: cd creatureforge/web && pnpm build")
    if not DATA_SPECIES.is_dir():
        sys.exit(f"[x] Species data not found: {DATA_SPECIES}")

    ver = _version_suffix()
    print(f"CreatureForge version: {ver}")

    dist = ROOT / "dist"
    dist.mkdir(exist_ok=True)

    _build("creature-forge-server", ROOT / "creatureforge" / "server.py")
    _build("creature-forge-cli", ROOT / "creatureforge" / "cli.py")

    # rename to versioned + platform/arch artifacts (for release)
    print("\n=== Artifacts ===")
    for name in ("creature-forge-server", "creature-forge-cli"):
        src = dist / f"{name}{EXE}"
        if src.is_file():
            tagged = dist / f"{name}-{ver}{_platform_suffix()}{EXE}"
            shutil.move(str(src), str(tagged))
            print(f"  {tagged.name}  {tagged.stat().st_size / 1024 / 1024:.1f} MiB")


if __name__ == "__main__":
    main()
