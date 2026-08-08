#!/usr/bin/env python3
"""CreatureForge HTTP API server.

组装（composition root）各领域模块，只依赖接口：
  species.py     物种模块（自包含：骨架/默认参数/动作/约束）
  skeleton3d.py  3D 骨架/动作引擎（读物种默认参数渲染）
  motion.py      3D 动作 DSL 求值器
  render.py      3D 绘制原语

API（仅 3D，基于物种默认参数）:
  /api/species            — 物种 (CRUD) + /default 默认参数读写
  /api/skeleton3d/<sp>    — 3D 骨架任意视角 PNG（基于物种默认参数）
  /api/motion3d/<action>  — 3D 动作帧/GIF（基于物种默认参数）

Usage:
  python creatureforge/server.py --port 8765
"""

from __future__ import annotations

import argparse
import json
import sys
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote

# Make 'creatureforge' package importable when run as `python creatureforge/server.py`
_PKG_ROOT = Path(__file__).resolve().parent  # creatureforge/
_REPO_ROOT = _PKG_ROOT.parent
if str(_REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(_REPO_ROOT))

from creatureforge.interfaces import Api
from creatureforge.api import ApiService
from creatureforge.config import DEFAULT_DATA_DIR, ensure_species_seeded

# ---- paths ----
PKG_ROOT = _PKG_ROOT
_BUNDLE = getattr(sys, "_MEIPASS", None)
WEB_DIST = (Path(_BUNDLE) / "web" / "dist") if _BUNDLE else (PKG_ROOT / "web" / "dist")


class CreatureForgeHandler(SimpleHTTPRequestHandler):
    """HTTP 处理器。依赖注入：统一 Api 服务（CLI 与 HTTP 共用同一套接口）。"""

    server_version = "CreatureForge/2.0"

    # 注入的依赖（由 build_server 设置）
    api: Api = None        # type: ignore[assignment]
    dev_mode: bool = False

    def end_headers(self) -> None:
        """开发模式（--dev）下追加 CORS 头，供前端 Vite dev / proxy 跨域。"""
        if self.dev_mode:
            self.send_header("Access-Control-Allow-Origin", "*")
            self.send_header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
            self.send_header("Access-Control-Allow-Headers", "Content-Type")
        super().end_headers()

    def do_OPTIONS(self) -> None:
        self.send_response(204)
        self.end_headers()

    def log_message(self, format: str, *args: object) -> None:
        return  # silent

    # -- routing -------------------------------------------------------

    def do_GET(self) -> None:
        p = self.path
        if p.startswith("/api/species"):
            return self._species_get()
        if p.startswith("/api/preset3d/"):
            return self._preset3d_get()
        if p.startswith("/api/presets"):
            return self._presets_get()
        if p == "/api/motions3d":
            return self._motions3d_list()
        if p.startswith("/api/motion3d/"):
            return self._motion3d_get()
        if p.startswith("/api/skin3d/"):
            return self._skin3d_get()
        if p.startswith("/api/skins"):
            return self._skins_get()
        if p.startswith("/api/templates"):
            return self._templates_get()
        if p.startswith("/api/wizard/"):
            return self._wizard_get()
        if p.startswith("/api/skeleton3d/"):
            return self._skeleton3d_get()
        if p.startswith("/api/"):
            return self._json({"ok": False, "error": "api not found"}, 404)
        return self._serve_static()

    def do_POST(self) -> None:
        p = self.path
        if p.startswith("/api/species"):
            return self._species_post()
        if p.startswith("/api/presets"):
            return self._presets_post()
        if p.startswith("/api/skins"):
            return self._skins_post()
        if p.startswith("/api/wizard/"):
            return self._wizard_post()
        self.send_error(404)

    def do_PUT(self) -> None:
        if p := self.path:
            if p.startswith("/api/species/"):
                return self._species_post()
            if p.startswith("/api/presets"):
                return self._presets_post()
            if p.startswith("/api/skins"):
                return self._skins_post()
        self.send_error(404)

    def do_DELETE(self) -> None:
        if self.path.startswith("/api/species/"):
            return self._species_delete()
        if self.path.startswith("/api/presets"):
            return self._presets_delete()
        if self.path.startswith("/api/skins"):
            return self._skins_delete()

    # -- 3D API ---------------------------------------------------------
    # 阶段 1/2：3D 骨架 + 3D 动作，任意视角（yaw）正交投影。

    def _motions3d_list(self) -> None:
        """GET /api/motions3d — 列出所有物种的 3D 动作（含 params，供前端参数滑块）。"""
        return self._json({"motions3d": self.api.actions_list_all()})

    def _skeleton3d_get(self) -> None:
        """GET /api/skeleton3d/<species_id>?yaw=45&pitch=12&dist=1&<body 参数> — 3D 骨架任意角度/距离 PNG（基于物种默认参数，dist 为距离倍数）。"""
        from urllib.parse import parse_qs, urlparse
        path_only = urlparse(self.path).path
        parts = _path_parts(path_only, "/api/skeleton3d")
        if len(parts) != 1:
            self.send_error(404)
            return
        qs = parse_qs(urlparse(self.path).query)
        yaw = float(qs.get("yaw", ["0"])[0])
        pitch = float(qs.get("pitch", ["0"])[0])
        dist = float(qs.get("dist", ["1"])[0])
        pan_x = float(qs.get("pan_x", ["0"])[0])
        pan_y = float(qs.get("pan_y", ["0"])[0])
        grid = qs.get("grid", ["1"])[0] not in ("0", "false")
        data = qs.get("data", ["0"])[0] in ("1", "true")
        # body JSON（骨架体型参数，WebGL 数据用）与其余浮点体型参数（param_chains 驱动）合并
        cam_keys = {"yaw", "pitch", "dist", "pan_x", "pan_y", "grid", "data", "body"}
        body = {k: float(v[0]) for k, v in qs.items() if k not in cam_keys}
        try:
            body_json = json.loads(qs.get("body", ["{}"])[0])
            if isinstance(body_json, dict):
                body = {**body, **body_json}
        except Exception:
            pass
        species_id = parts[0]
        try:
            if data:
                # WebGL 实时渲染：返回骨架 3D 数据（不走 Pillow PNG）
                return self._json(self.api.skeleton3d_data(species_id, body=body or None))
            data_url = self.api.render_skeleton3d(
                species_id, yaw=yaw, pitch=pitch, dist=dist,
                pan_x=pan_x, pan_y=pan_y, grid=grid,
                body=body or None)
            return self._json({"ok": True, "data_url": data_url})
        except Exception as e:
            return self._json({"ok": False, "error": str(e)}, 500)

    def _motion3d_get(self) -> None:
        """GET /api/motion3d/<action_id>?yaw=45&pitch=12&dist=600&frame=0[&gif=1] — 3D 动作帧/GIF。"""
        from urllib.parse import parse_qs, urlparse
        path_only = urlparse(self.path).path
        parts = _path_parts(path_only, "/api/motion3d")
        if len(parts) != 1:
            self.send_error(404)
            return
        qs = parse_qs(urlparse(self.path).query)
        yaw = float(qs.get("yaw", ["0"])[0])
        pitch = float(qs.get("pitch", ["0"])[0])
        dist = float(qs.get("dist", ["1"])[0])
        pan_x = float(qs.get("pan_x", ["0"])[0])
        pan_y = float(qs.get("pan_y", ["0"])[0])
        grid = qs.get("grid", ["1"])[0] not in ("0", "false")
        frame = int(qs.get("frame", ["0"])[0])
        gif = qs.get("gif", ["0"])[0] in ("1", "true")
        sprite = qs.get("sprite", ["0"])[0] in ("1", "true")
        species_q = qs.get("species", [None])[0]
        # data=1 → 返回动作每帧 3D 关节数据（前端 WebGL 动画播放）
        if qs.get("data", ["0"])[0] in ("1", "true"):
            body = json.loads(qs.get("body", ["{}"])[0])
            params = json.loads(qs.get("params", ["{}"])[0])
            transition_from = qs.get("transition_from", [None])[0]
            transition_frames = int(qs.get("transition_frames", ["6"])[0])
            try:
                return self._json(self.api.motion3d_data(
                    parts[0], species=species_q, body=body, params=params,
                    transition_from=transition_from, transition_frames=transition_frames))
            except KeyError as e:
                return self._json({"ok": False, "error": str(e)}, 404)
            except Exception as e:
                return self._json({"ok": False, "error": str(e)}, 500)
        try:
            result = self.api.render_motion3d(
                parts[0], species=species_q, yaw=yaw, pitch=pitch, dist=dist,
                pan_x=pan_x, pan_y=pan_y, grid=grid,
                frame=frame, gif=gif, sprite=sprite,
                frames=qs.get("frames", ["0"])[0] in ("1", "true"))
            return self._json(result)
        except KeyError as e:
            return self._json({"ok": False, "error": str(e)}, 404)
        except Exception as e:
            return self._json({"ok": False, "error": str(e)}, 500)

    def _skin3d_get(self) -> None:
        """GET /api/skin3d/<action>?preset=&skin_id=&species=&body=&params= — 蒙皮网格 + 每帧变形顶点。
        GET /api/skin3d/export/<action>?preset=&skin_id= — 导出 .glb（骨骼 + 蒙皮 + 动作动画）。
        """
        from urllib.parse import parse_qs, urlparse
        path_only = urlparse(self.path).path
        parts = _path_parts(path_only, "/api/skin3d")
        qs = parse_qs(urlparse(self.path).query)
        # 导出 .glb：预设（物种+体型+动作）+ 皮肤（材质+体态）→ 蒙皮 → 最终导出素材
        if parts and parts[0] == "export" and len(parts) == 2:
            return self._skin3d_export(parts[1], qs)
        if len(parts) != 1:
            self.send_error(404)
            return
        species_q = qs.get("species", [None])[0]
        preset_q = qs.get("preset", [None])[0]
        skin_q = qs.get("skin_id", [None])[0]
        body = json.loads(qs.get("body", ["{}"])[0])
        params = json.loads(qs.get("params", ["{}"])[0])
        try:
            return self._json(self.api.skin3d_data(
                parts[0], species=species_q, preset=preset_q, skin_id=skin_q,
                body=body, params=params))
        except KeyError as e:
            return self._json({"ok": False, "error": str(e)}, 404)
        except Exception as e:
            return self._json({"ok": False, "error": str(e)}, 500)

    def _skin3d_export(self, action_id: str, qs: dict) -> None:
        """导出 .glb 二进制（预设 + 皮肤 → 蒙皮 → 动画）。"""
        species_q = qs.get("species", [None])[0]
        preset_q = qs.get("preset", [None])[0]
        skin_q = qs.get("skin_id", [None])[0]
        try:
            glb = self.api.export_glb(action_id, species=species_q, preset=preset_q, skin_id=skin_q)
            if isinstance(glb, dict):
                raise RuntimeError(glb.get("error", "export failed"))
            self.send_response(200)
            self.send_header("Content-Type", "model/gltf-binary")
            self.send_header("Content-Disposition", f'attachment; filename="{action_id}.glb"')
            self.send_header("Content-Length", str(len(glb)))
            self.end_headers()
            self.wfile.write(glb)
        except Exception as e:
            return self._json({"ok": False, "error": str(e)}, 500)

    # -- static files --------------------------------------------------

    def _serve_static(self) -> None:
        """Serve the Vue SPA: try WEB_DIST, fallback to index.html."""
        path = self.path.lstrip("/")
        if not path:
            path = "index.html"
        file_path = WEB_DIST / path
        if file_path.is_file():
            content_type = _mime(path)
            self.send_response(200)
            self.send_header("Content-Type", content_type)
            self.send_header("Content-Length", str(file_path.stat().st_size))
            # HTML 始终不缓存（前端构建后刷新即生效）；带 hash 的静态资源保持缓存
            if file_path.suffix == ".html":
                self.send_header("Cache-Control", "no-cache")
            self.end_headers()
            self.wfile.write(file_path.read_bytes())
            return
        # SPA fallback
        index = WEB_DIST / "index.html"
        if index.is_file():
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(index.stat().st_size))
            self.send_header("Cache-Control", "no-cache")
            self.end_headers()
            self.wfile.write(index.read_bytes())
        else:
            self.send_error(404, "Frontend not built. Run: cd creatureforge/web && npm run build")

    # -- species API ---------------------------------------------------

    def _species_get(self) -> None:
        """GET /api/species — list; /api/species/<id> — detail; .../actions/<aid> — action."""
        parts = _path_parts(self.path, "/api/species")
        if not parts:
            return self._json({"species": self.api.species_list()})

        sp_id = parts[0]
        # 预设 schema：GET /api/species/<id>/preset_schema
        if len(parts) >= 2 and parts[1] == "preset_schema":
            schema = self.api.species_preset_schema(sp_id)
            if schema is None:
                return self._json({"ok": False, "error": f"preset_schema not found: {sp_id}"}, 404)
            return self._json(schema)
        # 默认参数：GET /api/species/<id>/default
        if len(parts) >= 2 and parts[1] == "default":
            try:
                return self._json(self.api.species_default(sp_id))
            except KeyError:
                return self._json({"ok": False, "error": f"default not found: {sp_id}"}, 404)
        # 动作详情：GET /api/species/<id>/actions/<action_id>
        if len(parts) >= 3 and parts[1] == "actions":
            action_id = parts[2]
            try:
                return self._json(self.api.action_get(sp_id, action_id))
            except KeyError:
                return self._json({"ok": False, "error": f"action not found: {sp_id}/{action_id}"}, 404)

        try:
            return self._json(self.api.species_get(sp_id))
        except KeyError:
            return self._json({"ok": False, "error": f"species not found: {sp_id}"}, 404)

    def _species_post(self) -> None:
        """POST /api/species — create; PUT /api/species/<id> — update;
        POST .../actions — create action; PUT .../actions/<aid> — update action."""
        parts = _path_parts(self.path, "/api/species")
        try:
            body = self._read_body()
        except Exception as e:
            return self._json({"ok": False, "error": str(e)}, 400)

        if not parts:
            # 创建物种
            try:
                sp_id = self.api.species_create(body)
            except FileExistsError as e:
                return self._json({"ok": False, "error": str(e)}, 409)
            except (ValueError, KeyError) as e:
                return self._json({"ok": False, "error": str(e)}, 400)
            return self._json({"ok": True, "created": sp_id})

        sp_id = parts[0]
        # 默认参数保存：POST/PUT /api/species/<id>/default
        if len(parts) >= 2 and parts[1] == "default":
            try:
                self.api.species_save_default(sp_id, body)
            except Exception as e:
                return self._json({"ok": False, "error": str(e)}, 400)
            return self._json({"ok": True, "saved": sp_id})
        # 动作路由：POST /api/species/<id>/actions 或 PUT .../actions/<aid>
        if len(parts) >= 2 and parts[1] == "actions":
            action_id = body.get("motion_id", "").strip() if len(parts) == 2 else parts[2]
            if not action_id:
                return self._json({"ok": False, "error": "action_id required"}, 400)
            try:
                saved = self.api.action_create(sp_id, body) if len(parts) == 2 else self.api.action_update(sp_id, action_id, body)
            except Exception as e:
                return self._json({"ok": False, "error": str(e)}, 400)
            return self._json({"ok": True, "saved": saved})

        # 更新物种
        try:
            sp_id = self.api.species_update(sp_id, body)
        except Exception as e:
            return self._json({"ok": False, "error": str(e)}, 400)
        return self._json({"ok": True, "updated": sp_id})

    def _species_delete(self) -> None:
        """DELETE /api/species/<id> — delete species; .../actions/<aid> — delete action."""
        parts = _path_parts(self.path, "/api/species")
        if not parts:
            return self._json({"ok": False, "error": "missing id"}, 400)

        sp_id = parts[0]
        # 删除动作：DELETE /api/species/<id>/actions/<action_id>
        if len(parts) >= 3 and parts[1] == "actions":
            action_id = parts[2]
            try:
                self.api.action_delete(sp_id, action_id)
            except KeyError:
                return self._json({"ok": False, "error": f"action not found: {sp_id}/{action_id}"}, 404)
            return self._json({"ok": True, "deleted": action_id})

        # 删除物种
        try:
            sp_id = self.api.species_delete(sp_id)
        except KeyError:
            return self._json({"ok": False, "error": f"species not found: {parts[0]}"}, 404)
        return self._json({"ok": True, "deleted": sp_id})

    # -- helpers -------------------------------------------------------

    # -- presets API ---------------------------------------------------

    def _presets_get(self) -> None:
        """GET /api/presets — list; /api/presets/new?species= — 新建空白表单; /api/presets/<id> — 详情。"""
        from urllib.parse import urlparse
        parts = _path_parts(urlparse(self.path).path, "/api/presets")
        if not parts:
            return self._json({"presets": self.api.presets_list()})
        if parts[0] == "new":
            from urllib.parse import parse_qs, urlparse
            qs = parse_qs(urlparse(self.path).query)
            sp = qs.get("species", ["human"])[0]
            try:
                return self._json(self.api.preset_new(sp))
            except Exception as e:
                return self._json({"ok": False, "error": str(e)}, 400)
        try:
            return self._json(self.api.preset_get(parts[0]))
        except KeyError:
            return self._json({"ok": False, "error": f"preset not found: {parts[0]}"}, 404)

    def _presets_post(self) -> None:
        """POST /api/presets — create; PUT /api/presets/<id> — update。"""
        from urllib.parse import urlparse
        parts = _path_parts(urlparse(self.path).path, "/api/presets")
        try:
            body = self._read_body()
        except Exception as e:
            return self._json({"ok": False, "error": str(e)}, 400)
        if not parts:
            try:
                pid = self.api.preset_create(body)
            except FileExistsError as e:
                return self._json({"ok": False, "error": str(e)}, 409)
            except (ValueError, KeyError) as e:
                return self._json({"ok": False, "error": str(e)}, 400)
            return self._json({"ok": True, "created": pid})
        try:
            pid = self.api.preset_update(parts[0], body)
        except KeyError:
            return self._json({"ok": False, "error": f"preset not found: {parts[0]}"}, 404)
        except Exception as e:
            return self._json({"ok": False, "error": str(e)}, 400)
        return self._json({"ok": True, "updated": pid})

    def _presets_delete(self) -> None:
        """DELETE /api/presets/<id> — delete preset。"""
        from urllib.parse import urlparse
        parts = _path_parts(urlparse(self.path).path, "/api/presets")
        if not parts:
            return self._json({"ok": False, "error": "missing id"}, 400)
        try:
            self.api.preset_delete(parts[0])
        except KeyError:
            return self._json({"ok": False, "error": f"preset not found: {parts[0]}"}, 404)
        return self._json({"ok": True, "deleted": parts[0]})

    # -- skins API -----------------------------------------------------

    def _skins_get(self) -> None:
        """GET /api/skins — list; /api/skins/new?preset= — 新建空白表单; /api/skins/<id> — 详情。"""
        from urllib.parse import urlparse
        parts = _path_parts(urlparse(self.path).path, "/api/skins")
        if not parts:
            return self._json({"skins": self.api.skins_list()})
        if parts[0] == "new":
            from urllib.parse import parse_qs, urlparse
            qs = parse_qs(urlparse(self.path).query)
            pid = qs.get("preset", [""])[0]
            if not pid:
                return self._json({"ok": False, "error": "preset required"}, 400)
            try:
                return self._json(self.api.skin_new(pid))
            except Exception as e:
                return self._json({"ok": False, "error": str(e)}, 400)
        try:
            return self._json(self.api.skin_get(parts[0]))
        except KeyError:
            return self._json({"ok": False, "error": f"skin not found: {parts[0]}"}, 404)

    def _skins_post(self) -> None:
        """POST/PUT /api/skins 路由：
        - POST /api/skins                  → 创建皮肤
        - POST /api/skins/<id>/parts       → 添加部件（body=part）
        - POST /api/skins/<id>/parts/<p>/mesh   → 上传部件网格（body={filename, data_b64}）
        - POST /api/skins/<id>/parts/<p>/texture → 上传部件贴图（body={field, filename, data_b64}）
        - PUT  /api/skins/<id>             → 更新皮肤
        - PUT  /api/skins/<id>/parts/<p>   → 更新部件（body=patch）
        """
        from urllib.parse import urlparse
        parts = _path_parts(urlparse(self.path).path, "/api/skins")
        try:
            body = self._read_body()
        except Exception as e:
            return self._json({"ok": False, "error": str(e)}, 400)
        # -- 部件路由 --
        if len(parts) >= 2 and parts[1] == "parts":
            skin_id, part_id = parts[0], (parts[2] if len(parts) > 2 else None)
            # 上传部件网格
            if part_id and len(parts) == 4 and parts[3] == "mesh":
                try:
                    data = _b64decode(body.get("data_b64", ""))
                    parsed = self.api.skin_part_upload_mesh(skin_id, part_id,
                                                            body.get("filename", ""), data)
                except Exception as e:
                    return self._json({"ok": False, "error": str(e)}, 400)
                return self._json({"ok": True, **parsed})
            # 上传部件贴图
            if part_id and len(parts) == 4 and parts[3] == "texture":
                try:
                    data = _b64decode(body.get("data_b64", ""))
                    ref = self.api.skin_part_upload_texture(skin_id, part_id,
                                                            body.get("field", "albedo"),
                                                            body.get("filename", ""), data)
                except Exception as e:
                    return self._json({"ok": False, "error": str(e)}, 400)
                return self._json({"ok": True, "ref": ref})
            # 添加部件（POST）
            if not part_id:
                try:
                    pid = self.api.skin_part_add(skin_id, body)
                except FileExistsError as e:
                    return self._json({"ok": False, "error": str(e)}, 409)
                except (ValueError, KeyError) as e:
                    return self._json({"ok": False, "error": str(e)}, 400)
                return self._json({"ok": True, "part": pid})
            # 更新部件（PUT）
            try:
                self.api.skin_part_update(skin_id, part_id, body)
            except KeyError:
                return self._json({"ok": False, "error": f"part not found: {part_id}"}, 404)
            except Exception as e:
                return self._json({"ok": False, "error": str(e)}, 400)
            return self._json({"ok": True, "updated": part_id})
        # -- 皮肤创建 / 更新 --
        if not parts:
            try:
                sid = self.api.skin_create(body)
            except FileExistsError as e:
                return self._json({"ok": False, "error": str(e)}, 409)
            except (ValueError, KeyError) as e:
                return self._json({"ok": False, "error": str(e)}, 400)
            return self._json({"ok": True, "created": sid})
        try:
            sid = self.api.skin_update(parts[0], body)
        except KeyError:
            return self._json({"ok": False, "error": f"skin not found: {parts[0]}"}, 404)
        except Exception as e:
            return self._json({"ok": False, "error": str(e)}, 400)
        return self._json({"ok": True, "updated": sid})

    def _skins_delete(self) -> None:
        """DELETE /api/skins/<id> — 删除皮肤; /api/skins/<id>/parts/<p> — 删除部件。"""
        from urllib.parse import urlparse
        parts = _path_parts(urlparse(self.path).path, "/api/skins")
        if not parts:
            return self._json({"ok": False, "error": "missing id"}, 400)
        if len(parts) >= 3 and parts[1] == "parts":
            try:
                self.api.skin_part_delete(parts[0], parts[2])
            except KeyError:
                return self._json({"ok": False, "error": f"part not found: {parts[2]}"}, 404)
            return self._json({"ok": True, "deleted": parts[2]})
        try:
            self.api.skin_delete(parts[0])
        except KeyError:
            return self._json({"ok": False, "error": f"skin not found: {parts[0]}"}, 404)
        return self._json({"ok": True, "deleted": parts[0]})

    # -- 物种分步向导（模板可选择 + custom 从 0 开始） -------------------

    def _templates_get(self) -> None:
        """GET /api/templates — 形态模板列表（数据驱动，含 custom 从 0 开始）。"""
        return self._json({"templates": self.api.templates_list()})

    def _wizard_get(self) -> None:
        """GET /api/wizard/<species_id> — 草稿视图; /files — 派生 skeleton/default（高级 JSON）。"""
        from urllib.parse import urlparse
        parts = _path_parts(urlparse(self.path).path, "/api/wizard")
        if len(parts) >= 1:
            if len(parts) == 2 and parts[1] == "files":
                try:
                    return self._json(self.api.wizard_files(parts[0]))
                except KeyError as e:
                    return self._json({"ok": False, "error": str(e)}, 404)
            if len(parts) == 1:
                try:
                    return self._json(self.api.wizard_get(parts[0]))
                except KeyError as e:
                    return self._json({"ok": False, "error": str(e)}, 404)
        self.send_error(404)

    def _wizard_post(self) -> None:
        """POST /api/wizard/<species_id>/<action> — 分步操作（init/joint/limb/chain/pose/canvas/param/commit/discard）。"""
        from urllib.parse import urlparse
        parts = _path_parts(urlparse(self.path).path, "/api/wizard")
        if len(parts) < 1:
            return self._json({"ok": False, "error": "missing species"}, 400)
        sp = parts[0]
        act = parts[1:]
        try:
            body = self._read_body()
        except Exception as e:
            return self._json({"ok": False, "error": str(e)}, 400)
        try:
            if act == ["init"]:
                return self._json(self.api.wizard_init(
                    sp, morph_id=body.get("morph_id", "custom"),
                    title=body.get("title", ""), description=body.get("description", "")))
            if act == ["joint", "add"]:
                return self._json(self.api.wizard_add_joint(
                    sp, body.get("name", ""), parent=body.get("parent"),
                    pos=body.get("pos"), sym=body.get("sym")))
            if act == ["joint", "rm"]:
                return self._json(self.api.wizard_remove_joint(sp, body.get("name", "")))
            if act == ["joint", "rename"]:
                return self._json(self.api.wizard_rename_joint(sp, body.get("old", ""), body.get("new", "")))
            if act == ["joint", "parent"]:
                return self._json(self.api.wizard_set_parent(sp, body.get("name", ""), body.get("parent")))
            if act == ["limb", "mirror"]:
                return self._json(self.api.wizard_mirror_limb(sp, body.get("source", ""), to_prefix=body.get("to_prefix")))
            if act == ["chain", "add"]:
                return self._json(self.api.wizard_add_chain(sp, body.get("name", ""), body.get("joints", []) or []))
            if act == ["chain", "rm"]:
                return self._json(self.api.wizard_remove_chain(sp, body.get("name", "")))
            if act == ["pose", "set"]:
                return self._json(self.api.wizard_set_pose(sp, body.get("name", ""), body.get("pos", []) or []))
            if act == ["apply_pose"]:
                return self._json(self.api.wizard_apply_pose(sp, body.get("positions") or {}))
            if act == ["rotate"]:
                return self._json(self.api.wizard_rotate(
                    sp, axis=body.get("axis", "z"), angle=float(body.get("angle", 90)), joint=body.get("joint")))
            if act == ["translate"]:
                return self._json(self.api.wizard_translate(
                    sp, dx=float(body.get("dx", 0)), dy=float(body.get("dy", 0)),
                    dz=float(body.get("dz", 0)), joint=body.get("joint")))
            if act == ["canvas"]:
                return self._json(self.api.wizard_set_canvas(
                    sp, width=body.get("width"), height=body.get("height"), floor_y=body.get("floor_y")))
            if act == ["param", "add"]:
                return self._json(self.api.wizard_add_param_chain(
                    sp, body.get("name", ""), body.get("joints", []) or [],
                    anchor=body.get("anchor"), label=body.get("label")))
            if act == ["commit"]:
                return self._json({"ok": True, "created": self.api.wizard_commit(sp)})
            if act == ["discard"]:
                return self._json({"ok": True, "discarded": self.api.wizard_discard(sp)})
            if act == ["files"]:
                return self._json(self.api.wizard_save_files(
                    sp, body.get("skeleton") or {}, body.get("default")))
        except (ValueError, KeyError, FileExistsError) as e:
            return self._json({"ok": False, "error": str(e)}, 400)
        return self._json({"ok": False, "error": f"unknown wizard action: {'/'.join(act)}"}, 404)

    def _preset3d_get(self) -> None:
        """GET /api/preset3d/<id> 或 /api/preset3d/live — 预设渲染（骨架/动作）。

        - <id>: 读 presets/<id>.json（body 体型参数 + actions 动作参数）
        - live: 用 query 直接传参渲染（未保存的编辑实时预览）
          ?species=human&body=<json>&actions=<json>[&action=walk3d]
        不传 action → 渲染骨架（应用体型参数）。
        """
        from urllib.parse import parse_qs, urlparse
        parts = _path_parts(urlparse(self.path).path, "/api/preset3d")
        if len(parts) != 1:
            return self._json({"ok": False, "error": "preset id or 'live' required"}, 400)
        qs = parse_qs(urlparse(self.path).query)
        yaw = float(qs.get("yaw", ["0"])[0])
        pitch = float(qs.get("pitch", ["0"])[0])
        dist = float(qs.get("dist", ["1"])[0])
        pan_x = float(qs.get("pan_x", ["0"])[0])
        pan_y = float(qs.get("pan_y", ["0"])[0])
        grid = qs.get("grid", ["1"])[0] not in ("0", "false")
        frame = int(qs.get("frame", ["0"])[0])
        gif = qs.get("gif", ["0"])[0] in ("1", "true")
        frames = qs.get("frames", ["0"])[0] in ("1", "true")
        action_id = qs.get("action", [None])[0]
        try:
            if parts[0] == "live":
                species_id = qs.get("species", [None])[0]
                if not species_id:
                    return self._json({"ok": False, "error": "live preset requires species"}, 400)
                body = json.loads(qs.get("body", ["{}"])[0])
                actions = json.loads(qs.get("actions", ["{}"])[0])
            else:
                species_id = None
                body = None
                actions = None
        except json.JSONDecodeError as e:
            return self._json({"ok": False, "error": str(e)}, 400)
        try:
            result = self.api.render_preset3d(
                parts[0], species=species_id, body=body, actions=actions,
                action_id=action_id, yaw=yaw, pitch=pitch, dist=dist,
                pan_x=pan_x, pan_y=pan_y, grid=grid,
                frame=frame, gif=gif, frames=frames)
            return self._json(result)
        except KeyError as e:
            return self._json({"ok": False, "error": str(e)}, 404)
        except Exception as e:
            return self._json({"ok": False, "error": str(e)}, 500)

    def _json(self, data: dict, status: int = 200) -> None:
        body = json.dumps(data, ensure_ascii=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _read_body(self) -> dict:
        length = int(self.headers.get("Content-Length", "0"))
        if length == 0:
            return {}
        return json.loads(self.rfile.read(length))


# ---- utility ------------------------------------------------------------


def _path_parts(path: str, prefix: str) -> list[str]:
    return [unquote(p) for p in path[len(prefix):].rstrip("/").split("/") if p]


def _b64decode(data_b64: str) -> bytes:
    """上传文件 base64 解码（兼容 data URI 前缀）。"""
    import base64
    s = (data_b64 or "").strip()
    if "," in s and s.startswith("data:"):
        s = s.split(",", 1)[1]
    return base64.b64decode(s)


def _float_map(body: dict, keys: tuple[str, ...]) -> dict[str, float]:
    out: dict[str, float] = {}
    for k in keys:
        if body.get(k) is not None:
            out[k] = float(body[k])
    return out


_MIME_MAP = {
    ".html": "text/html; charset=utf-8",
    ".css": "text/css; charset=utf-8",
    ".js": "application/javascript; charset=utf-8",
    ".json": "application/json; charset=utf-8",
    ".png": "image/png",
    ".jpg": "image/jpeg",
    ".svg": "image/svg+xml",
    ".ico": "image/x-icon",
    ".woff2": "font/woff2",
}


def _mime(path: str) -> str:
    return _MIME_MAP.get(Path(path).suffix.lower(), "application/octet-stream")


# ---- main ---------------------------------------------------------------


def build_server(port: int = 8765, host: str = "0.0.0.0", dev: bool = False,
                 data_dir: Path | None = None):
    """组装服务器：依赖注入统一 Api 服务。

    唯一的组装根：在这里实例化 ApiService（满足 interfaces.Api 契约）注入到 Handler。
    其余代码一律只依赖 Api 接口（与 CLI 相同）。
    dev=True：开发模式，追加 CORS 头（前端 Vite dev / proxy）。
    data_dir：数据目录（默认仓库根 data/，测试用 test-data/；打包运行时从 bundle 播种）。
    """
    data_dir = ensure_species_seeded(data_dir or DEFAULT_DATA_DIR)
    api = ApiService(data_dir / "species", data_dir / "presets")
    handler = type(
        "InjectedHandler",
        (CreatureForgeHandler,),
        {"api": api, "dev_mode": dev},
    )
    return ThreadingHTTPServer((host, port), handler)


def main():
    parser = argparse.ArgumentParser(description="CreatureForge API Server")
    parser.add_argument("--port", type=int, default=8765)
    parser.add_argument("--host", default="0.0.0.0")
    parser.add_argument("--dev", action="store_true",
                        help="开发模式：追加 CORS 头，配合前端 Vite dev (pnpm run dev + proxy) 使用")
    parser.add_argument("--data-dir", default=None,
                        help="数据目录（默认仓库根 data/，测试用 test-data/）")
    args = parser.parse_args()

    if not args.dev and not (WEB_DIST / "index.html").is_file():
        print("Warning: web/dist not found. Run: cd creatureforge/web && npm run build", file=sys.stderr)

    data_dir = Path(args.data_dir) if args.data_dir else None
    server = build_server(args.port, args.host, dev=args.dev, data_dir=data_dir)
    mode = "dev" if args.dev else "prod"
    print(f"CreatureForge server [{mode}]: http://{args.host}:{args.port}")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        server.shutdown()


if __name__ == "__main__":
    raise SystemExit(main())
