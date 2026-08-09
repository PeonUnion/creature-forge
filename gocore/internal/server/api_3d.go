package server

// 3D data endpoints (skeleton3d / motion3d / motion3d_live / skin3d_data).
import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/PeonUnion/creature-forge/gocore/internal/store"
	"github.com/PeonUnion/creature-forge/gocore/skeleton"
)

// routeMotions3dList handles GET /api/motions3d.
func (s *Server) routeMotions3dList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items, err := s.actionsListAll()
	if err != nil {
		s.fail(w, err, http.StatusInternalServerError)
		return
	}
	s.json(w, map[string]any{"motions3d": items}, http.StatusOK)
}

// actionsListAll lists all actions across species.
func (s *Server) actionsListAll() ([]map[string]any, error) {
	ids, err := s.Store.ListSpecies()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	for _, sid := range ids {
		acts, err := s.Store.ListActions(sid)
		if err != nil {
			continue
		}
		for _, a := range acts {
			m, err := s.Store.GetAction(sid, a)
			if err != nil {
				continue
			}
			out = append(out, map[string]any{
				"id":          m.MotionID,
				"title":       orTitle(m.Title, m.MotionID),
				"description": m.Description,
				"species":     sid,
				"params":      m.Params,
				"has_ik":      false,
			})
		}
	}
	return out, nil
}

func orTitle(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// --- camera/query helpers ----------------------------

// camQuery parses the shared camera + grid flags from query params.
type camQuery struct {
	Yaw, Pitch, Dist, PanX, PanY float64
	Grid, Data                   bool
	Frame                        int
	Gif, Sprite, Frames          bool
}

func parseCamQuery(r *http.Request) camQuery {
	q := r.URL.Query()
	c := camQuery{}
	c.Yaw = qf(q.Get("yaw"))
	c.Pitch = qf(q.Get("pitch"))
	c.Dist = qf(q.Get("dist"))
	if c.Dist == 0 {
		c.Dist = 1
	}
	c.PanX = qf(q.Get("pan_x"))
	c.PanY = qf(q.Get("pan_y"))
	c.Grid = q.Get("grid") != "0" && q.Get("grid") != "false"
	c.Data = q.Get("data") == "1" || q.Get("data") == "true"
	c.Frame, _ = strconv.Atoi(q.Get("frame"))
	c.Gif = q.Get("gif") == "1" || q.Get("gif") == "true"
	c.Sprite = q.Get("sprite") == "1" || q.Get("sprite") == "true"
	c.Frames = q.Get("frames") == "1" || q.Get("frames") == "true"
	return c
}

func qf(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// bodyFromQuery extracts body params from query (all non-camera float params
// merged with the optional body JSON).
func bodyFromQuery(r *http.Request) map[string]float64 {
	q := r.URL.Query()
	camKeys := map[string]bool{
		"yaw": true, "pitch": true, "dist": true, "pan_x": true, "pan_y": true,
		"grid": true, "data": true, "body": true, "frame": true, "gif": true,
		"sprite": true, "frames": true, "species": true, "params": true,
		"preset": true, "skin_id": true, "transition_from": true,
		"transition_frames": true, "action": true, "actions": true,
	}
	body := map[string]float64{}
	for k, vs := range q {
		if camKeys[k] || len(vs) == 0 {
			continue
		}
		if f, err := strconv.ParseFloat(vs[0], 64); err == nil {
			body[k] = f
		}
	}
	if bj := q.Get("body"); bj != "" {
		var m map[string]float64
		if json.Unmarshal([]byte(bj), &m) == nil {
			for k, v := range m {
				body[k] = v
			}
		}
	}
	return body
}

// --- skeleton3d -----------------------------------------------------------

// routeSkeleton3d handles GET /api/skeleton3d/<species_id>.
func (s *Server) routeSkeleton3d(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	parts := pathParts(r.URL.Path, "/api/skeleton3d")
	if len(parts) != 1 {
		s.fail(w, errors.New("bad path"), http.StatusNotFound)
		return
	}
	cam := parseCamQuery(r)
	body := bodyFromQuery(r)
	if cam.Data {
		data, err := s.skeleton3dData(parts[0], body)
		if err != nil {
			s.fail(w, err, http.StatusInternalServerError)
			return
		}
		s.json(w, data, http.StatusOK)
		return
	}
	// PNG render (skeleton preview)
	url, err := s.renderSkeleton3d(parts[0], cam, body)
	if err != nil {
		s.fail(w, err, http.StatusInternalServerError)
		return
	}
	s.json(w, map[string]any{"ok": true, "data_url": url}, http.StatusOK)
}

// skeleton3dData returns WebGL-ready joint data.
func (s *Server) skeleton3dData(speciesID string, body map[string]float64) (map[string]any, error) {
	sk, err := s.buildSpeciesSkeleton(speciesID, body)
	if err != nil {
		return nil, err
	}
	joints := map[string][]float64{}
	for k, v := range sk.Joints {
		joints[k] = []float64{v[0], v[1], v[2]}
	}
	return map[string]any{
		"ok":          true,
		"joints":      joints,
		"bones":       bones2d(sk.Bones),
		"center":      vec3(sk.Center),
		"head_radius": sk.HeadRadius,
	}, nil
}

// buildSpeciesSkeleton loads + builds a skeleton for a species with body overrides.
func (s *Server) buildSpeciesSkeleton(speciesID string, body map[string]float64) (*skeleton.Skeleton, error) {
	sp, err := s.Store.GetSpecies(speciesID)
	if err != nil {
		return nil, err
	}
	def, err := s.Store.GetDefault(speciesID)
	if err != nil {
		return nil, err
	}
	engDef, err := def.ToEngineDefault()
	if err != nil {
		return nil, err
	}
	var b map[string]float64
	if len(body) > 0 {
		b = body
	}
	return skeleton.BuildSkeleton(sp.ToEngineSkeleton(), engDef, b), nil
}

func bones2d(bones [][2]string) [][]string {
	out := make([][]string, 0, len(bones))
	for _, b := range bones {
		out = append(out, []string{b[0], b[1]})
	}
	return out
}

func vec3(v [3]float64) []float64 { return []float64{v[0], v[1], v[2]} }

// --- motion3d -------------------------------------------------------------

// routeMotion3d handles GET /api/motion3d/<id> and POST /api/motion3d/live.
func (s *Server) routeMotion3d(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/api/motion3d/live" {
		s.motion3dLive(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	parts := pathParts(r.URL.Path, "/api/motion3d")
	if len(parts) != 1 {
		s.fail(w, errors.New("bad path"), http.StatusNotFound)
		return
	}
	q := r.URL.Query()
	if q.Get("data") == "1" || q.Get("data") == "true" {
		body := map[string]float64{}
		if bj := q.Get("body"); bj != "" {
			_ = json.Unmarshal([]byte(bj), &body)
		}
		params := map[string]float64{}
		if pj := q.Get("params"); pj != "" {
			_ = json.Unmarshal([]byte(pj), &params)
		}
		tf, _ := strconv.Atoi(q.Get("transition_frames"))
		if tf <= 0 {
			tf = 6
		}
		data, err := s.motion3dData(parts[0], q.Get("species"), body, params, tf)
		if err != nil {
			s.fail(w, err, http.StatusInternalServerError)
			return
		}
		s.json(w, data, http.StatusOK)
		return
	}
	// PNG/GIF/frames/sprite render
	cam := parseCamQuery(r)
	result, err := s.renderMotion3d(parts[0], q.Get("species"), cam)
	if err != nil {
		s.fail(w, err, http.StatusInternalServerError)
		return
	}
	s.json(w, result, http.StatusOK)
}

// motion3dData returns per-frame joint data.
func (s *Server) motion3dData(actionID, species string, body, params map[string]float64, transitionFrames int) (map[string]any, error) {
	speciesID, motion, err := s.resolveMotion(actionID, species)
	if err != nil {
		return nil, err
	}
	sk, err := s.buildSpeciesSkeleton(speciesID, body)
	if err != nil {
		return nil, err
	}
	eng := motion.ToEngineMotion()
	n := eng.FrameCount
	if n <= 0 {
		n = 8
	}
	params = s.resolveActionParams(motion, params)
	frames := make([]map[string][]float64, 0, n)
	for i := 0; i < n; i++ {
		pose := skeleton.Pose(sk, eng, i, params)
		frames = append(frames, poseVecs(pose))
	}
	fps := motion.Fps
	if fps <= 0 {
		fps = 6
	}
	return map[string]any{
		"ok":                true,
		"bones":             bones2d(sk.Bones),
		"frames":            frames,
		"frame_count":       n,
		"transition_frames": 0,
		"fps":               fps,
		"center":            vec3(sk.Center),
		"head_radius":       sk.HeadRadius,
	}, nil
}

// resolveMotion finds a motion by id (optionally scoped to a species).
func (s *Server) resolveMotion(actionID, species string) (string, *store.Motion, error) {
	if species != "" {
		m, err := s.Store.GetAction(species, actionID)
		if err != nil {
			return "", nil, err
		}
		return species, m, nil
	}
	ids, err := s.Store.ListSpecies()
	if err != nil {
		return "", nil, err
	}
	for _, sid := range ids {
		if m, err := s.Store.GetAction(sid, actionID); err == nil {
			return sid, m, nil
		}
	}
	return "", nil, errors.New("3D action not found: " + actionID)
}

// resolveActionParams merges motion param defaults with overrides.
func (s *Server) resolveActionParams(m *store.Motion, overrides map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(m.Params))
	for k, p := range m.Params {
		out[k] = p.Default
	}
	for k, v := range overrides {
		out[k] = v
	}
	return out
}

func poseVecs(pose map[string][3]float64) map[string][]float64 {
	out := make(map[string][]float64, len(pose))
	for k, v := range pose {
		out[k] = vec3(v)
	}
	return out
}

// motion3dLive computes one live frame (POST /api/motion3d/live).
func (s *Server) motion3dLive(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Action  map[string]any `json:"action"`
		Species string         `json:"species"`
		Index   int            `json:"index"`
	}
	if err := decodeBody(r, &body); err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	if body.Action == nil || body.Species == "" {
		s.fail(w, errors.New("action/species required"), http.StatusBadRequest)
		return
	}
	sk, err := s.buildSpeciesSkeleton(body.Species, nil)
	if err != nil {
		s.fail(w, err, http.StatusInternalServerError)
		return
	}
	b, _ := json.Marshal(body.Action)
	var motion store.Motion
	if err := json.Unmarshal(b, &motion); err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	eng := motion.ToEngineMotion()
	n := eng.FrameCount
	if n <= 0 {
		n = 8
	}
	idx := body.Index
	if idx < 0 {
		idx = 0
	}
	if idx > n-1 {
		idx = n - 1
	}
	pose := skeleton.Pose(sk, eng, idx, nil)
	s.json(w, map[string]any{
		"ok":          true,
		"joints":      poseVecs(pose),
		"bones":       bones2d(sk.Bones),
		"center":      vec3(sk.Center),
		"head_radius": sk.HeadRadius,
		"frame_count": n,
		"index":       idx,
	}, http.StatusOK)
}

// --- skin3d ---------------------------------------------------------------

// routeSkin3d handles GET /api/skin3d/<id> and export (pending glTF).
func (s *Server) routeSkin3d(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	parts := pathParts(r.URL.Path, "/api/skin3d")
	q := r.URL.Query()
	if len(parts) >= 2 && parts[0] == "export" {
		s.fail(w, errors.New("GLB export not implemented yet (Go glTF package pending)"), http.StatusNotImplemented)
		return
	}
	if len(parts) != 1 {
		s.fail(w, errors.New("bad path"), http.StatusNotFound)
		return
	}
	body := map[string]float64{}
	if bj := q.Get("body"); bj != "" {
		_ = json.Unmarshal([]byte(bj), &body)
	}
	params := map[string]float64{}
	if pj := q.Get("params"); pj != "" {
		_ = json.Unmarshal([]byte(pj), &params)
	}
	data, err := s.skin3dData(parts[0], q.Get("species"), q.Get("preset"), q.Get("skin_id"), body, params)
	if err != nil {
		s.fail(w, err, http.StatusInternalServerError)
		return
	}
	s.json(w, data, http.StatusOK)
}

// skin3dData returns the skinned mesh + per-frame vertices.
func (s *Server) skin3dData(actionID, species, presetID, skinID string, body, params map[string]float64) (map[string]any, error) {
	// resolve source: preset (body/params) → species
	speciesID := species
	var presetBody, presetParams map[string]float64
	if presetID != "" {
		p, err := s.Store.GetPreset(presetID)
		if err != nil {
			return nil, err
		}
		speciesID = p.Species
		presetBody = p.Body
		presetParams = p.Actions[actionID]
	}
	speciesID, motion, err := s.resolveMotion(actionID, speciesID)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		body = presetBody
	}
	if len(params) == 0 {
		params = presetParams
	}
	sk, err := s.buildSpeciesSkeleton(speciesID, body)
	if err != nil {
		return nil, err
	}
	mesh, err := skeleton.LoadMesh(filepath.Join(s.Store.SpeciesDir(), speciesID, "skin", "mesh.json"))
	if err != nil {
		return nil, err
	}
	weights, err := skeleton.LoadWeights(filepath.Join(s.Store.SpeciesDir(), speciesID, "skin", "weights.json"))
	if err != nil {
		return nil, err
	}
	eng := motion.ToEngineMotion()
	n := eng.FrameCount
	if n <= 0 {
		n = 8
	}
	params = s.resolveActionParams(motion, params)

	// 皮肤部件：先加载资产（mesh/weights），帧循环内计算跟随数据
	//   bone 件   → part_bone_frames[part_id] = 每帧绑定骨世界位置（位置跟随）
	//   skinned 件 → part_skin_frames[part_id] = 每帧 LBS 变形顶点（贴合 + 随骨架变形）
	parts := []any{}
	partBoneFrames := map[string][][]float64{}
	partSkinFrames := map[string][][]float64{}
	partEngMesh := map[string]*skeleton.Mesh{}
	partEngWeight := map[string]*skeleton.Weights{}
	partBone := map[string]string{}
	if skinID != "" {
		if skd, err := s.Store.GetSkin(skinID); err == nil {
			for _, p := range skd.Parts {
				if p.MeshFile != nil {
					if m, err := s.loadPartAsset(skinID, *p.MeshFile); err == nil {
						p.Mesh = m
					}
				}
				if p.Kind == "skinned" {
					if p.WeightsFile != nil {
						if w, err := s.loadPartAsset(skinID, *p.WeightsFile); err == nil {
							p.Weights = w
						}
					}
					if p.Mesh != nil && p.Weights != nil {
						if pm, err1 := mapToEngineMesh(p.Mesh); err1 == nil {
							if pw, err2 := mapToEngineWeights(p.Weights); err2 == nil {
								partEngMesh[p.PartID] = pm
								partEngWeight[p.PartID] = pw
							}
						}
					}
				} else if p.Bone != "" {
					partBone[p.PartID] = p.Bone
				}
				parts = append(parts, p)
			}
		}
	}

	frames := make([][]float64, 0, n)
	for i := 0; i < n; i++ {
		posMap, rMap := skeleton.FKWorldPose(sk, eng, i, params)
		frames = append(frames, skeleton.SkinnedVertices(posMap, rMap, sk, mesh, weights, 0))
		// skinned 部件：每帧 LBS 变形
		for pid, pm := range partEngMesh {
			partSkinFrames[pid] = append(partSkinFrames[pid],
				skeleton.SkinnedVertices(posMap, rMap, sk, pm, partEngWeight[pid], 0))
		}
		// bone 部件：每帧绑定骨世界位置
		for pid, bone := range partBone {
			if bpos, ok := posMap[bone]; ok {
				partBoneFrames[pid] = append(partBoneFrames[pid], vec3(bpos))
			} else {
				partBoneFrames[pid] = append(partBoneFrames[pid], nil)
			}
		}
	}
	bindJoints := map[string][]float64{}
	for k, v := range sk.Joints {
		bindJoints[k] = vec3(v)
	}
	fkTree := map[string]any{}
	for k, v := range sk.FkTree {
		if v == nil {
			fkTree[k] = nil
		} else {
			fkTree[k] = *v
		}
	}
	fps := motion.Fps
	if fps <= 0 {
		fps = 6
	}
	return map[string]any{
		"ok": true,
		"mesh": map[string]any{
			"indices": mesh.Indices, "uvs": mesh.UVs, "normals": mesh.Normals,
			"vertex_count": mesh.VertexCount, "vertices": mesh.Vertices,
			"materials": map[string]any{},
		},
		"boneNames":        weights.BoneNames,
		"weights":          weights.PerVertex,
		"bindJoints":       bindJoints,
		"fk_tree":          fkTree,
		"bones":            bones2d(sk.Bones),
		"frames":           frames,
		"trs":              map[string]any{},
		"frame_count":      n,
		"fps":              fps,
		"center":           vec3(sk.Center),
		"parts":            parts,
		"part_bone_frames": partBoneFrames,
		"part_skin_frames": partSkinFrames,
	}, nil
}

// loadPartAsset reads a skin part asset (mesh/weights JSON) from
// data/skins/assets/<skin_id>/<ref>.
func (s *Server) loadPartAsset(skinID, ref string) (map[string]any, error) {
	b, err := os.ReadFile(filepath.Join(s.Store.SkinsDir(), "assets", skinID, filepath.Clean(ref)))
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func mapToEngineMesh(m map[string]any) (*skeleton.Mesh, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var mesh skeleton.Mesh
	if err := json.Unmarshal(b, &mesh); err != nil {
		return nil, err
	}
	return &mesh, nil
}

func mapToEngineWeights(w map[string]any) (*skeleton.Weights, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return nil, err
	}
	var wg skeleton.Weights
	if err := json.Unmarshal(b, &wg); err != nil {
		return nil, err
	}
	return &wg, nil
}
