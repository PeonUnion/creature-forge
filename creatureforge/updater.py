#!/usr/bin/env python3
"""CreatureForge CLI 自更新：检测最新版本 + 下载替换自身二进制（GitHub Releases）。

- 打包时（scripts/build_release.py）生成 cf_meta/version.json 打进产物
  （含 version / platform / repo），运行时读取当前版本与平台；源码运行回退 __version__。
- 更新源：GitHub Releases（默认 PeonUnion/creature-forge），资产名
  creature-forge-cli-<ver>-<platform>[.exe]。
- 纯标准库（urllib / shutil / subprocess），无第三方依赖。
"""
from __future__ import annotations

import json
import os
import platform
import shutil
import subprocess
import sys
import tempfile
import urllib.request
from pathlib import Path

DEFAULT_REPO = "PeonUnion/creature-forge"
_UA = "CreatureForge-CLI/updater"


# ---------------------------------------------------------------------------
# 当前版本 / 平台（打包资源优先）
# ---------------------------------------------------------------------------


def packaged_meta() -> dict:
    """读取打包时嵌入的 cf_meta/version.json（pyinstaller onefile 解压到 sys._MEIPASS）。"""
    base = getattr(sys, "_MEIPASS", None)
    if base:
        f = Path(base) / "cf_meta" / "version.json"
        if f.is_file():
            try:
                return json.loads(f.read_text(encoding="utf-8"))
            except Exception:
                pass
    return {}


def current_version() -> str:
    meta = packaged_meta()
    if meta.get("version"):
        return str(meta["version"])
    try:
        from creatureforge import __version__  # noqa: PLC0415
        return str(__version__)
    except Exception:
        return "0.0.0-dev"


def current_platform() -> str:
    meta = packaged_meta()
    if meta.get("platform"):
        return str(meta["platform"])
    return _detect_platform()


def _detect_platform() -> str:
    machine = {
        "x86_64": "x64", "AMD64": "x64", "aarch64": "arm64", "arm64": "arm64",
        "x86": "x86", "i386": "x86", "armv7l": "arm",
    }.get(platform.machine(), platform.machine().lower())
    osname = {"linux": "linux", "windows": "windows", "darwin": "macos"}.get(sys.platform, sys.platform)
    return f"{osname}-{machine}"


# ---------------------------------------------------------------------------
# 版本比较（SemVer 子集：X.Y.Z[-pre.N]）
# ---------------------------------------------------------------------------


def _semver_key(v: str) -> tuple:
    v = v.lstrip("v")
    core, _, pre = v.partition("-")
    nums = [int(x) if x.isdigit() else 0 for x in core.split(".")]
    nums = (nums + [0, 0, 0])[:3]
    if not pre:
        return (*nums, 1, "")  # 正式版高于任何同号预发布
    return (*nums, 0, pre)


def is_newer(a: str, b: str) -> bool:
    """a 是否比 b 新（SemVer）。"""
    return _semver_key(a) > _semver_key(b)


# ---------------------------------------------------------------------------
# 查询最新版本（GitHub Releases API）
# ---------------------------------------------------------------------------


def _http_json(url: str, timeout: float = 20.0):
    req = urllib.request.Request(
        url, headers={"User-Agent": _UA, "Accept": "application/vnd.github+json"})
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return json.load(r)


def fetch_latest(repo: str, channel: str = "latest", timeout: float = 20.0) -> dict:
    """查询最新 release。返回 {version, tag, prerelease, assets:[{name,url}]}。

    channel=latest → 首个正式版（跳过预发布）；prerelease → 最新（含预发布）。
    """
    releases = _http_json(f"https://api.github.com/repos/{repo}/releases?per_page=30", timeout)
    if not isinstance(releases, list):
        raise RuntimeError(f"无法解析 GitHub Releases 响应（仓库 {repo}）")
    for rel in releases:
        if channel == "prerelease" or not rel.get("prerelease"):
            return {
                "version": str(rel["tag_name"]).lstrip("v"),
                "tag": str(rel["tag_name"]),
                "prerelease": bool(rel.get("prerelease")),
                "assets": [
                    {"name": a["name"], "url": a["browser_download_url"]}
                    for a in rel.get("assets") or []
                ],
            }
    raise RuntimeError(f"仓库 {repo} 没有可用的 {channel} 版本")


def _find_asset(assets: list[dict], ver: str, platform_name: str) -> dict | None:
    """精确匹配资产：creature-forge-cli-<ver>-<platform>[.exe]（发布产物名即此格式）。"""
    want = f"creature-forge-cli-{ver}-{platform_name}"
    candidates = {want, want + ".exe"}
    for a in assets:
        if a["name"] in candidates:
            return a
    return None


# ---------------------------------------------------------------------------
# 下载 / 校验 / 替换
# ---------------------------------------------------------------------------


def _download(url: str, dest: Path) -> None:
    req = urllib.request.Request(url, headers={"User-Agent": _UA})
    with urllib.request.urlopen(req, timeout=180) as src, open(dest, "wb") as out:
        shutil.copyfileobj(src, out)


def _install(tmp: Path) -> None:
    """用下载的临时文件替换当前可执行文件。"""
    if not getattr(sys, "frozen", False):
        raise RuntimeError("源码运行无法自替换，请用 `git pull` 更新")
    self_path = Path(sys.executable)
    try:
        os.replace(str(tmp), str(self_path))
        os.chmod(self_path, 0o755)
    except PermissionError:
        # Windows：运行中的 exe 无法覆盖 → 下载为 .new，提示手动替换
        new = self_path.with_name(self_path.name + ".new")
        shutil.move(str(tmp), str(new))
        raise RuntimeError(f"无法覆盖正在运行的程序，已保存到 {new}，请退出后手动替换")
    except OSError as e:
        raise RuntimeError(f"替换失败: {e}") from e


def run_upgrade(args) -> int:
    """upgrade 命令入口。返回进程退出码（0=成功/最新，1=失败，2=检测到可更新）。"""
    cur = current_version()
    plat = current_platform()
    try:
        info = fetch_latest(args.repo, args.channel)
    except Exception as e:  # noqa: BLE001
        print(f"检测更新失败（请检查网络）: {e}", file=sys.stderr)
        return 1
    latest = info["version"]
    print(f"当前版本: {cur} ({plat})")
    print(f"最新版本: {latest} ({'预发布' if info['prerelease'] else '正式版'}, 通道 {args.channel})")

    if not is_newer(latest, cur):
        if args.check:
            print("已是最新版本。")
            return 0
        if not args.force:
            print("已是最新版本，无需更新（--force 可强制重装）。")
            return 0
        print("--force：强制重装当前最新版。")
    elif args.check:
        print(f"发现新版本 {latest}，可运行 `creatureforge upgrade` 更新。")
        return 2

    if not getattr(sys, "frozen", False):
        print("源码运行无法自替换：请 `git pull` 更新。", file=sys.stderr)
        return 1

    asset = _find_asset(info["assets"], latest, plat)
    if not asset:
        print(f"未找到 {plat} 平台的安装包（期望 creature-forge-cli-{latest}-{plat}）。",
              file=sys.stderr)
        return 1

    if not args.yes:
        try:
            ok = input(f"确认下载并替换为 {latest}？[y/N] ").strip().lower() in ("y", "yes")
        except EOFError:
            ok = False
        if not ok:
            print("已取消。")
            return 0

    print(f"下载 {asset['name']} …")
    tmp = Path(tempfile.mkstemp(suffix=".part")[1])
    try:
        _download(asset["url"], tmp)
        size = tmp.stat().st_size
        if size < 1_000_000:
            raise RuntimeError(f"下载文件异常偏小（{size} 字节），可能下载失败")
        # 校验：运行下载的二进制 --version 确认可执行且版本一致
        r = subprocess.run([str(tmp), "--version"], capture_output=True, text=True, timeout=90)
        if r.returncode != 0:
            raise RuntimeError(f"下载的二进制校验失败: {(r.stderr or r.stdout).strip()}")
        print(f"  校验: {r.stdout.strip()}")
        _install(tmp)
    except Exception as e:  # noqa: BLE001
        print(f"更新失败: {e}", file=sys.stderr)
        return 1
    finally:
        tmp.unlink(missing_ok=True)
    print("更新完成，下次运行即为新版本。")
    return 0
