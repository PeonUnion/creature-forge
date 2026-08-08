# =========================================================================
# CreatureForge — 统一 API 服务（CLI 与 HTTP 共享的同一套接口实现）
# =========================================================================
# 唯一的 API 实现：物种 + 动作 + 预设 + 3D 渲染。
# - HTTP（server.py）的 handler 只依赖这里的 ApiService（薄路由层，无业务逻辑）
# - CLI（python -m creatureforge.cli）直接实例化 ApiService 交互，不启动 server
# - 硬约束：ApiService 必须满足 interfaces.Api（Protocol，@runtime_checkable），
#   任何新增操作先在 interfaces.Api 声明，两侧（CLI/HTTP）自动一致。
# =========================================================================

from __future__ import annotations

import base64
import io
import json
import math
from pathlib import Path

from .config import DEFAULT_DATA_DIR
from .interfaces import Api
from .models import Motion, MotionListItem, Preset, PresetSummary, SpeciesDetail, SpeciesListItem, SpeciesSkeleton
from .presets import PresetService
from .species import SpeciesService


def image_to_data_url(img) -> str:
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return "data:image/png;base64," + base64.b64encode(buf.getvalue()).decode()


class ApiService:
    """统一 API 实现：组合物种 + 预设服务，并承担 3D 渲染（CLI/HTTP 共用）。"""

    def __init__(self, species_root: Path, presets_root: Path) -> None:
        self.species = SpeciesService(species_root)
        self.presets = PresetService(presets_root, self.species)

    # ------------------------------------------------------------------
    # 物种
    # ------------------------------------------------------------------

    def species_list(self) -> list[SpeciesListItem]:
        return self.species.list()

    def species_get(self, species_id: str) -> SpeciesDetail:
        return self.species.get(species_id)

    def species_create(self, data: SpeciesSkeleton) -> str:
        return self.species.create(data)

    def species_update(self, species_id: str, data: SpeciesSkeleton) -> str:
        return self.species.update(species_id, data)

    def species_delete(self, species_id: str) -> str:
        return self.species.delete(species_id)

    def species_preset_schema(self, species_id: str) -> dict | None:
        return self.species.get_preset_schema(species_id)

    def species_default(self, species_id: str) -> dict | None:
        return self.species.get_default(species_id)

    def species_save_default(self, species_id: str, data: dict) -> str:
        return self.species.save_default(species_id, data)

    # ------------------------------------------------------------------
    # 动作
    # ------------------------------------------------------------------

    def actions_list_all(self) -> list[MotionListItem]:
        return self.species.list_actions_all()

    def action_get(self, species_id: str, action_id: str) -> Motion:
        return self.species.get_action(species_id, action_id)

    def action_create(self, species_id: str, data: Motion) -> str:
        return self.species.save_action(species_id, data.get("motion_id", ""), data)

    def action_update(self, species_id: str, action_id: str, data: Motion) -> str:
        return self.species.save_action(species_id, action_id, data)

    def action_delete(self, species_id: str, action_id: str) -> str:
        return self.species.delete_action(species_id, action_id)

    # ------------------------------------------------------------------
    # 预设
    # ------------------------------------------------------------------

    def presets_list(self) -> list[PresetSummary]:
        return self.presets.list()

    def preset_get(self, preset_id: str) -> dict:
        return self.presets.get(preset_id)

    def preset_new(self, species_id: str) -> dict:
        return self.presets.new_schema(species_id)

    def preset_create(self, data: Preset) -> str:
        return self.presets.create(data)

    def preset_update(self, preset_id: str, data: Preset) -> str:
        return self.presets.update(preset_id, data)

    def preset_delete(self, preset_id: str) -> str:
        return self.presets.delete(preset_id)

    # ------------------------------------------------------------------
    # 3D 渲染
    # ------------------------------------------------------------------

    def skeleton3d_data(self, species_id: str, *, body: dict | None = None) -> dict:
        """返回应用体型后的骨架 3D 数据（前端 WebGL 实时渲染用，不渲染 PNG）。

        joints 为 {关节名: [x,y,z]}（Y-down 项目坐标），bones 为连接对，center 为观察中心。
        """
        from .skeleton3d import build_skeleton_3d
        skel3d = build_skeleton_3d(species_id, body=body, species_root=self.species._root)
        return {"ok": True,
                "joints": {k: list(v) for k, v in skel3d["joints"].items()},
                "bones": [list(b) for b in skel3d["bones"]],
                "center": list(skel3d.get("center", (480.0, 300.0, 0.0))),
                "head_radius": float(skel3d.get("head_radius", 22.0))}

    def motion3d_data(self, action_id: str, *, species: str | None = None,
                      body: dict | None = None, params: dict | None = None,
                      transition_from: str | None = None,
                      transition_frames: int = 6) -> dict:
        """返回动作每帧 3D 关节数据（前端 WebGL 动画播放用，不渲染 PNG）。

        body 为体型参数、params 为动作参数（均应用）；frames 为每帧 {关节名: [x,y,z]}
        （Y-down 项目坐标），bones 连接对，frame_count/fps。

        transition_from：可选，上一动作 id。切换动作时在其帧前拼接过渡段
        （上一动作尾帧 → 本动作首帧 逐关节线性插值），数据全部来自两个动作 JSON，
        过渡帧数为 transition_frames（默认 6）。
        """
        from .skeleton3d import build_skeleton_3d, pose_3d
        if species:
            motion = self.species.get_action(species, action_id)
            species_id = species
        else:
            found = self.species.find_action(action_id)
            if not found:
                raise KeyError(f"3D action not found: {action_id}")
            species_id, motion = found
        skel3d = build_skeleton_3d(species_id, body=body, species_root=self.species._root)
        n = int(motion.get("frame_count", 8))
        p = params or {}
        frames = [pose_3d(skel3d, motion, i, params=p) for i in range(n)]
        # 过渡段：上一动作尾帧 → 本动作首帧 逐关节插值（不硬编码，数值来自两动作真实 JSON）
        n_trans = 0
        if transition_from and transition_from != action_id:
            try:
                if species:
                    fm = self.species.get_action(species, transition_from)
                else:
                    found = self.species.find_action(transition_from)
                    fm = found[1] if found else None
                if fm is not None:
                    fn = int(fm.get("frame_count", 8))
                    tail = pose_3d(skel3d, fm, max(0, fn - 1))
                    head = frames[0]
                    n_trans = max(1, min(int(transition_frames), n))
                    trans = []
                    for k in range(1, n_trans + 1):
                        t = k / (n_trans + 1)
                        trans.append({j: [tail[j][a] + (head[j][a] - tail[j][a]) * t
                                          for a in range(3)] for j in head})
                    frames = trans + frames
            except Exception:
                n_trans = 0  # 过渡数据不可用则忽略，仅返回动作本身
        return {"ok": True,
                "bones": [list(b) for b in skel3d["bones"]],
                "frames": frames,
                "frame_count": n + n_trans,
                "transition_frames": n_trans,
                "fps": int(motion.get("fps", 6)) or 6,
                "center": list(skel3d.get("center", (480.0, 300.0, 0.0))),
                "head_radius": float(skel3d.get("head_radius", 22.0))}

    # ------------------------------------------------------------------
    # 蒙皮（顶点蒙皮预览：网格 + 权重外挂，前端每帧更新顶点）
    # ------------------------------------------------------------------

    def skin3d_data(self, action_id: str, *, species: str | None = None,
                    body: dict | None = None, params: dict | None = None) -> dict:
        """返回蒙皮网格 + 动作每帧变形顶点（前端 WebGL 蒙皮预览，不渲染 PNG）。

        mesh 为绑定姿态网格（indices/uvs/normals/vertex_count/materials），
        frames 为每帧 flat 顶点列表（[x,y,z,...]，Y-down 项目坐标，由 LBS 计算）；
        boneNames/bindJoints 供前端叠加骨骼显示；数据全部外挂 skin/。
        """
        from .skeleton3d import build_skeleton_3d, skinned_vertices, per_frame_trs
        if species:
            motion = self.species.get_action(species, action_id)
            species_id = species
        else:
            found = self.species.find_action(action_id)
            if not found:
                raise KeyError(f"3D action not found: {action_id}")
            species_id, motion = found
        skel3d = build_skeleton_3d(species_id, body=body, species_root=self.species._root)
        skin = self._load_skin(species_id)
        n = int(motion.get("frame_count", 8))
        p = params or {}
        frames = [skinned_vertices(skel3d, motion, i, skin, params=p) for i in range(n)]
        mesh = skin["mesh"]
        return {"ok": True,
                "mesh": {"indices": mesh["indices"], "uvs": mesh["uvs"],
                         "normals": mesh["normals"], "vertex_count": mesh["vertex_count"],
                         "materials": mesh.get("materials", {})},
                "boneNames": skin["weights"]["boneNames"],
                "weights": skin["weights"]["perVertex"],
                "bindJoints": {j: list(v) for j, v in skel3d["joints"].items()},
                "fk_tree": {j: p for j, p in (skel3d.get("fk_tree") or {}).items()},
                "bones": [list(b) for b in skel3d["bones"]],
                "frames": frames,
                "trs": per_frame_trs(motion, params=p),  # 每帧骨骼 TRS（导出 glTF 动画）
                "frame_count": n,
                "fps": int(motion.get("fps", 6)) or 6,
                "center": list(skel3d.get("center", (480.0, 300.0, 0.0)))}

    def _load_skin(self, species_id: str) -> dict:
        """加载外挂蒙皮数据（skin/mesh.json + skin/weights.json）。"""
        root = self.species._root / species_id / "skin"
        mesh = json.loads((root / "mesh.json").read_text(encoding="utf-8"))
        weights = json.loads((root / "weights.json").read_text(encoding="utf-8"))
        return {"mesh": mesh, "weights": weights}

    def render_skeleton3d(self, species_id: str, *, yaw: float = 0, pitch: float = 0,
                          dist: float = 1.0, pan_x: float = 0, pan_y: float = 0,
                          grid: bool = True, body: dict | None = None) -> str:
        """3D 骨架渲染（应用体型参数 body），返回 PNG data_url。

        dist 为距离倍数（相对自动适配基准：1=模型占垂直视野 76%，>1 拉远、<1 拉近）；
        grid 是否绘制地面辅助网格（默认开）。
        """
        from .skeleton3d import build_skeleton_3d, render_pose, _fit_distance
        skel3d = build_skeleton_3d(species_id, body=body, species_root=self.species._root)
        center = tuple(skel3d.get("center", (480.0, 300.0, 0.0)))
        hr = float(skel3d.get("head_radius", 22.0))
        base = {j: list(v) for j, v in skel3d["joints"].items()}
        ground_y = max(v[1] for v in skel3d["joints"].values())
        grid_rad = max((math.hypot(x - center[0], y - center[1], z - center[2])
                        for x, y, z in skel3d["joints"].values()), default=100.0)
        dist_abs = _fit_distance(base, center) * max(dist, 0.01)
        img = render_pose(base, skel3d["bones"], yaw, pitch, dist_abs, center, pan_x, pan_y,
                          grid=grid, grid_y=ground_y, grid_rad=grid_rad, head_radius=hr)
        return image_to_data_url(img)

    def render_motion3d(self, action_id: str, *, species: str | None = None, yaw: float = 0,
                        pitch: float = 0, dist: float = 1.0, pan_x: float = 0,
                        pan_y: float = 0, grid: bool = True, frame: int = 0,
                        gif: bool = False, frames: bool = False, sprite: bool = False) -> dict:
        """3D 动作渲染。返回 {'data_url'} | {'frames':[...],'frame_count'} | {'gif':...} | {'sprite':...}。

        dist 为距离倍数（相对自动适配基准，1=模型占垂直视野 76%）；grid 是否绘制网格。
        sprite=1 返回横向拼接大图（一次请求/解码，前端 CSS 动画逐帧播放，性能最优）。
        """
        from .skeleton3d import build_skeleton_3d, pose_3d, render_pose, _fit_distance
        from PIL import Image
        if species:
            motion = self.species.get_action(species, action_id)
            species_id = species
        else:
            found = self.species.find_action(action_id)
            if not found:
                raise KeyError(f"3D action not found: {action_id}")
            species_id, motion = found
        skel3d = build_skeleton_3d(species_id, species_root=self.species._root)
        center = tuple(skel3d.get("center", (480.0, 300.0, 0.0)))
        hr = float(skel3d.get("head_radius", 22.0))
        ground_y = max(v[1] for v in skel3d["joints"].values())
        # 网格覆盖半径用骨架静态尺寸（不随动画帧姿势伸缩，避免网格“呼吸”）
        grid_rad = max((math.hypot(x - center[0], y - center[1], z - center[2])
                        for x, y, z in skel3d["joints"].values()), default=100.0)
        n = int(motion.get("frame_count", 8))
        base_pose = pose_3d(skel3d, motion, 0)
        dist_abs = _fit_distance(base_pose, center) * max(dist, 0.01)
        if sprite:
            imgs = []
            for i in range(n):
                p = pose_3d(skel3d, motion, i)
                imgs.append(render_pose(p, skel3d["bones"], yaw, pitch, dist_abs, center, pan_x, pan_y,
                                        grid=grid, grid_y=ground_y, grid_rad=grid_rad, head_radius=hr))
            w, h = imgs[0].size
            sheet = Image.new("RGB", (w * n, h))
            for i, im in enumerate(imgs):
                sheet.paste(im, (i * w, 0))
            fps = int(motion.get("fps", 6)) or 6
            return {"ok": True, "sprite": image_to_data_url(sheet), "frame_count": n,
                    "frame_w": w, "frame_h": h, "fps": fps, "species": species_id}
        if frames:
            urls = []
            for i in range(n):
                p = pose_3d(skel3d, motion, i)
                urls.append(image_to_data_url(
                    render_pose(p, skel3d["bones"], yaw, pitch, dist_abs, center, pan_x, pan_y,
                                grid=grid, grid_y=ground_y, grid_rad=grid_rad, head_radius=hr)))
            return {"ok": True, "frames": urls, "frame_count": n, "species": species_id}
        if gif:
            imgs = []
            for i in range(n):
                p = pose_3d(skel3d, motion, i)
                imgs.append(render_pose(p, skel3d["bones"], yaw, pitch, dist_abs, center, pan_x, pan_y,
                                        grid=grid, grid_y=ground_y, grid_rad=grid_rad, head_radius=hr)
                             .resize((640, 400), Image.Resampling.NEAREST))
            buf = io.BytesIO()
            imgs[0].save(buf, format="GIF", save_all=True, append_images=imgs[1:],
                         duration=180, loop=0, disposal=2)
            return {"ok": True, "gif": "data:image/gif;base64," + base64.b64encode(buf.getvalue()).decode(),
                    "species": species_id}
        p = pose_3d(skel3d, motion, frame)
        img = render_pose(p, skel3d["bones"], yaw, pitch, dist_abs, center, pan_x, pan_y,
                          grid=grid, grid_y=ground_y, grid_rad=grid_rad, head_radius=hr)
        return {"ok": True, "data_url": image_to_data_url(img), "species": species_id}

    def render_preset3d(self, preset_ref: str, *, species: str | None = None,
                        body: dict | None = None, actions: dict | None = None,
                        action_id: str | None = None, yaw: float = 0, pitch: float = 0,
                        dist: float = 1.0, pan_x: float = 0, pan_y: float = 0,
                        grid: bool = True, frame: int = 0, gif: bool = False,
                        frames: bool = False) -> dict:
        """3D 预设渲染（应用体型 body + 动作参数 actions）。

        preset_ref='live' 用传参（未保存实时预览），否则读 presets/<id>.json。
        有 action_id → 动作帧/GIF；无 → 骨架。dist 为距离倍数；grid 是否绘制网格。
        """
        from .skeleton3d import build_skeleton_3d, pose_3d, render_pose, _fit_distance
        from PIL import Image
        if preset_ref == "live":
            if not species:
                raise ValueError("live preset requires species")
            species_id = species
            b = body or {}
            ac = actions or {}
        else:
            preset = self.presets.get(preset_ref)
            species_id = preset.get("species", "")
            b = preset.get("body") or {}
            ac = preset.get("actions") or {}
        skel3d = build_skeleton_3d(species_id, body=b, species_root=self.species._root)
        center = tuple(skel3d.get("center", (480.0, 300.0, 0.0)))
        hr = float(skel3d.get("head_radius", 22.0))
        ground_y = max(v[1] for v in skel3d["joints"].values())
        grid_rad = max((math.hypot(x - center[0], y - center[1], z - center[2])
                        for x, y, z in skel3d["joints"].values()), default=100.0)
        if action_id:
            motion = self.species.get_action(species_id, action_id)
            n = int(motion.get("frame_count", 8))
            params = (ac or {}).get(action_id, {})
            base_pose = pose_3d(skel3d, motion, 0, params=params)
            dist_abs = _fit_distance(base_pose, center) * max(dist, 0.01)
            if frames:
                urls = []
                for i in range(n):
                    p = pose_3d(skel3d, motion, i, params=params)
                    urls.append(image_to_data_url(
                        render_pose(p, skel3d["bones"], yaw, pitch, dist_abs, center, pan_x, pan_y,
                                    grid=grid, grid_y=ground_y, grid_rad=grid_rad, head_radius=hr)))
                return {"ok": True, "frames": urls, "frame_count": n}
            if gif:
                imgs = []
                for i in range(n):
                    p = pose_3d(skel3d, motion, i, params=params)
                    imgs.append(render_pose(p, skel3d["bones"], yaw, pitch, dist_abs, center, pan_x, pan_y,
                                            grid=grid, grid_y=ground_y, grid_rad=grid_rad, head_radius=hr)
                                 .resize((640, 400), Image.Resampling.NEAREST))
                buf = io.BytesIO()
                imgs[0].save(buf, format="GIF", save_all=True, append_images=imgs[1:],
                             duration=180, loop=0, disposal=2)
                return {"ok": True, "gif": "data:image/gif;base64," + base64.b64encode(buf.getvalue()).decode()}
            p = pose_3d(skel3d, motion, frame, params=params)
            img = render_pose(p, skel3d["bones"], yaw, pitch, dist_abs, center, pan_x, pan_y,
                              grid=grid, grid_y=ground_y, grid_rad=grid_rad, head_radius=hr)
            return {"ok": True, "data_url": image_to_data_url(img)}
        # 骨架渲染（应用体型）
        base = {j: list(v) for j, v in skel3d["joints"].items()}
        dist_abs = _fit_distance(base, center) * max(dist, 0.01)
        img = render_pose(base, skel3d["bones"], yaw, pitch, dist_abs, center, pan_x, pan_y,
                          grid=grid, grid_y=ground_y, grid_rad=grid_rad, head_radius=hr)
        return {"ok": True, "data_url": image_to_data_url(img)}


# 硬约束：ApiService 必须实现 interfaces.Api 声明的全部操作（运行时校验）
def make_api(species_root: Path, presets_root: Path) -> Api:
    service = ApiService(species_root, presets_root)
    assert isinstance(service, Api), "ApiService 未实现 interfaces.Api 契约（CLI 与 HTTP 将不一致）"
    return service
