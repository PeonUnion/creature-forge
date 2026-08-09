package server

// Render endpoints (mirror of api.py render_skeleton3d / render_motion3d /
// render_preset3d): PNG / GIF / frames / sprite via the render package.
import (
	"errors"
	"image"
	"math"

	"github.com/PeonUnion/creature-forge/gocore/internal/render"
	"github.com/PeonUnion/creature-forge/gocore/internal/store"
	"github.com/PeonUnion/creature-forge/gocore/skeleton"
)

// camView resolves the shared render inputs from a camQuery.
type camView struct {
	skel    *skeleton.Skeleton
	center  [3]float64
	headRad float64
	groundY float64
	gridRad float64
	distAbs float64
}

// makeCamView computes camera/frame params for rendering a skeleton (mirror of
// the shared block in api.py render methods).
func makeCamView(sk *skeleton.Skeleton, cam camQuery) camView {
	v := camView{skel: sk, center: sk.Center, headRad: sk.HeadRadius}
	for _, j := range sk.Joints {
		if j[1] > v.groundY {
			v.groundY = j[1]
		}
	}
	v.gridRad = render.FitDistance(sk.Joints, v.center, render.FitFill)
	v.distAbs = render.FitDistance(sk.Joints, v.center, render.FitFill) * math.Max(cam.Dist, 0.01)
	return v
}

// frame renders one pose via the view's camera.
func (v camView) frame(pose map[string][3]float64, yaw, pitch, panX, panY float64, grid bool) *image.RGBA {
	return render.RenderPose(pose, v.skel.Bones, yaw, pitch, v.distAbs, v.center,
		panX, panY, grid, v.groundY, v.gridRad, v.headRad)
}

// --- skeleton3d render ----------------------------------------------------

// renderSkeleton3d mirrors api.render_skeleton3d.
func (s *Server) renderSkeleton3d(speciesID string, cam camQuery, body map[string]float64) (string, error) {
	sk, err := s.buildSpeciesSkeleton(speciesID, body)
	if err != nil {
		return "", err
	}
	v := makeCamView(sk, cam)
	img := v.frame(sk.Joints, cam.Yaw, cam.Pitch, cam.PanX, cam.PanY, cam.Grid)
	return render.PNGDataURL(img)
}

// --- motion3d render ------------------------------------------------------

// renderMotion3d mirrors api.render_motion3d: single frame / frames / gif / sprite.
func (s *Server) renderMotion3d(actionID, species string, cam camQuery) (map[string]any, error) {
	speciesID, motion, err := s.resolveMotion(actionID, species)
	if err != nil {
		return nil, err
	}
	sk, err := s.buildSpeciesSkeleton(speciesID, nil)
	if err != nil {
		return nil, err
	}
	v := makeCamView(sk, cam)
	eng := motion.ToEngineMotion()
	n := eng.FrameCount
	if n <= 0 {
		n = 8
	}
	frames := make([]*image.RGBA, 0, n)
	for i := 0; i < n; i++ {
		pose := skeleton.Pose(sk, eng, i, nil)
		frames = append(frames, v.frame(pose, cam.Yaw, cam.Pitch, cam.PanX, cam.PanY, cam.Grid))
	}
	fps := motion.Fps
	if fps <= 0 {
		fps = 6
	}
	if cam.Sprite {
		url, err := render.SpriteSheetDataURL(frames)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "sprite": url, "frame_count": n,
			"frame_w": render.CanvasW, "frame_h": render.CanvasH, "fps": fps, "species": speciesID}, nil
	}
	if cam.Frames {
		urls := make([]string, 0, n)
		for _, fr := range frames {
			u, err := render.PNGDataURL(fr)
			if err != nil {
				return nil, err
			}
			urls = append(urls, u)
		}
		return map[string]any{"ok": true, "frames": urls, "frame_count": n, "species": speciesID}, nil
	}
	if cam.Gif {
		url, err := render.GIFDataURL(frames, 640, 400)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "gif": url, "species": speciesID}, nil
	}
	idx := cam.Frame
	if idx < 0 || idx >= n {
		idx = 0
	}
	url, err := render.PNGDataURL(frames[idx])
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "data_url": url, "species": speciesID}, nil
}

// --- preset3d render ------------------------------------------------------

// renderPreset3d mirrors api.render_preset3d (preset id or "live").
func (s *Server) renderPreset3d(presetRef, species string, body, actions map[string]float64,
	actionID string, cam camQuery) (map[string]any, error) {
	var sk *skeleton.Skeleton
	var ac map[string]float64
	var speciesID string
	if presetRef == "live" {
		if species == "" {
			return nil, errors.New("live preset requires species")
		}
		speciesID = species
		sk, _ = s.buildSpeciesSkeleton(species, body)
		ac = actions
	} else {
		p, err := s.Store.GetPreset(presetRef)
		if err != nil {
			return nil, errors.New("preset not found: " + presetRef)
		}
		speciesID = p.Species
		ac = p.Actions[actionID]
		if p.Baked != nil && p.Baked.Skel3D != nil && len(body) == 0 {
			sk = bakedToSkeleton(p.Baked.Skel3D)
		} else {
			sk, err = s.buildSpeciesSkeleton(speciesID, p.Body)
			if err != nil {
				return nil, err
			}
		}
		if p.Baked != nil && p.Baked.Actions != nil {
			if ba, ok := p.Baked.Actions[actionID]; ok {
				ac = ba
			}
		}
	}
	if sk == nil {
		return nil, errors.New("cannot build skeleton")
	}
	v := makeCamView(sk, cam)
	if actionID != "" {
		motion, err := s.Store.GetAction(speciesID, actionID)
		if err != nil {
			return nil, err
		}
		eng := motion.ToEngineMotion()
		n := eng.FrameCount
		if n <= 0 {
			n = 8
		}
		frames := make([]*image.RGBA, 0, n)
		for i := 0; i < n; i++ {
			pose := skeleton.Pose(sk, eng, i, ac)
			frames = append(frames, v.frame(pose, cam.Yaw, cam.Pitch, cam.PanX, cam.PanY, cam.Grid))
		}
		if cam.Frames {
			urls := make([]string, 0, n)
			for _, fr := range frames {
				u, err := render.PNGDataURL(fr)
				if err != nil {
					return nil, err
				}
				urls = append(urls, u)
			}
			return map[string]any{"ok": true, "frames": urls, "frame_count": n}, nil
		}
		if cam.Gif {
			url, err := render.GIFDataURL(frames, 640, 400)
			if err != nil {
				return nil, err
			}
			return map[string]any{"ok": true, "gif": url}, nil
		}
		idx := cam.Frame
		if idx < 0 || idx >= n {
			idx = 0
		}
		url, err := render.PNGDataURL(frames[idx])
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "data_url": url}, nil
	}
	url, err := render.PNGDataURL(v.frame(sk.Joints, cam.Yaw, cam.Pitch, cam.PanX, cam.PanY, cam.Grid))
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "data_url": url}, nil
}

// bakedToSkeleton converts a baked skel3d snapshot back into an engine
// Skeleton (mirror of _baked_or_build → preset baked path).
func bakedToSkeleton(b *store.BakedSkel3D) *skeleton.Skeleton {
	return &skeleton.Skeleton{
		Joints:     b.Joints,
		FkTree:     b.FkTree,
		FkLocal:    b.FkLocal,
		Bones:      b.Bones,
		Center:     b.Center,
		FloorY:     b.FloorY,
		HeadRadius: b.HeadRadius,
		Params:     b.Params,
	}
}
