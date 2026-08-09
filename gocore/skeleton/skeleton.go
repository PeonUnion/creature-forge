// Package skeleton reads species skeleton/default + motion JSON (external
// data) and computes FK poses and LBS vertex skinning. Data-driven — no
// values hardcoded.
package skeleton

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/PeonUnion/creature-forge/gocore/expr"
)

// ---------------------------------------------------------------------------
// Data models (matching the skeleton JSON schemas)
// ---------------------------------------------------------------------------

// CoordParam is a species-level coordinate parameter definition.
type CoordParam struct {
	Default float64 `json:"default"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Step    float64 `json:"step"`
	Label   string  `json:"label"`
}

// SpeciesSkeleton mirrors species/<id>/skeleton.json (subset used by engine).
type SpeciesSkeleton struct {
	SpeciesID string                `json:"species_id"`
	FkTree    map[string]*string    `json:"fk_tree"` // joint → parent (nil = root)
	Bones     [][2]string           `json:"bones_3d,omitempty"`
	Params    map[string]CoordParam `json:"params,omitempty"` // coordinate params
}

// JointPos is one joint's coordinates: either [x,y,z] array or {x,y,z} object
// whose components may be numbers or expressions.
type JointPos struct {
	Array *[3]*expr.ParamValue
	Obj   map[string]*expr.ParamValue
}

// UnmarshalJSON accepts [x,y,z] | {x,y,z}.
func (j *JointPos) UnmarshalJSON(b []byte) error {
	*j = JointPos{}
	var arr []*expr.ParamValue
	if err := json.Unmarshal(b, &arr); err == nil && len(arr) == 3 {
		j.Array = &[3]*expr.ParamValue{arr[0], arr[1], arr[2]}
		return nil
	}
	var obj map[string]*expr.ParamValue
	if err := json.Unmarshal(b, &obj); err != nil {
		return fmt.Errorf("joint pos: cannot parse %s: %w", b, err)
	}
	j.Obj = obj
	return nil
}

// Default mirrors species/<id>/default.json (subset).
type Default struct {
	Positions3d map[string]JointPos `json:"positions_3d"`
	Canvas      struct {
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
		FloorY float64 `json:"floor_y"`
	} `json:"canvas"`
	HeadRadius float64            `json:"head_radius"`
	Body       map[string]float64 `json:"body,omitempty"`
}

// Motion mirrors species/<id>/actions3d/*.json (subset used by FK path).
type Motion struct {
	Schema     string                           `json:"schema"`
	MotionID   string                           `json:"motion_id"`
	FrameCount int                              `json:"frame_count"`
	Params     map[string]ParamSpec             `json:"params"`
	Signals    map[string]*expr.Expr            `json:"signals"`
	Fk3d       *FK3D                            `json:"fk3d,omitempty"`
	Root3d     map[string]*expr.Expr            `json:"root3d,omitempty"`
	Offsets3d  map[string]map[string]*expr.Expr `json:"offsets3d,omitempty"`
}

// ParamSpec is a motion action-param definition.
type ParamSpec struct {
	Default float64 `json:"default"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Step    float64 `json:"step"`
	Label   string  `json:"label"`
}

// FK3D is the fk3d block (FK animation).
type FK3D struct {
	Root        string                           `json:"root"`
	Rotations3d map[string]map[string]*expr.Expr `json:"rotations3d"` // joint → {x_rot,y_rot,z_rot}
}

// Skeleton is the built 3D skeleton.
type Skeleton struct {
	Joints     map[string][3]float64
	FkTree     map[string]*string
	FkLocal    map[string][3]float64
	Bones      [][2]string
	Center     [3]float64
	FloorY     float64
	HeadRadius float64
	Params     map[string]float64 // resolved coordinate params (incl. body overrides)
}

// ---------------------------------------------------------------------------
// Build (coordinate resolution + FK local frame)
// ---------------------------------------------------------------------------

func coordParams(skel *SpeciesSkeleton, body map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(skel.Params))
	for k, v := range skel.Params {
		out[k] = v.Default
	}
	for k, v := range body {
		out[k] = v
	}
	return out
}

// BuildSkeleton resolves coordinates from default.json + species coordinate
// params (+ optional body overrides), and computes fk_local.
func BuildSkeleton(skel *SpeciesSkeleton, def *Default, body map[string]float64) *Skeleton {
	params := coordParams(skel, body)
	ctx := &expr.Ctx{Params: params, Index: 0, FrameCount: 1, Phase: 0, Signals: map[string]func(*expr.Ctx) float64{}}
	joints := make(map[string][3]float64, len(def.Positions3d))
	for name, jp := range def.Positions3d {
		var v [3]float64
		if jp.Array != nil {
			for i := 0; i < 3; i++ {
				v[i] = jp.Array[i].Eval(ctx)
			}
		} else {
			v[0] = jp.Obj["x"].Eval(ctx)
			v[1] = jp.Obj["y"].Eval(ctx)
			v[2] = jp.Obj["z"].Eval(ctx)
		}
		joints[name] = v
	}
	// fk_local: joint - parent (bind pose offsets)
	fkLocal := make(map[string][3]float64)
	for j, p := range skel.FkTree {
		if p != nil {
			if pj, ok := joints[*p]; ok {
				if cj, ok := joints[j]; ok {
					fkLocal[j] = [3]float64{cj[0] - pj[0], cj[1] - pj[1], cj[2] - pj[2]}
				}
			}
		}
	}
	center := [3]float64{def.Canvas.Width / 2, def.Canvas.Height / 2, 0}
	return &Skeleton{
		Joints:     joints,
		FkTree:     skel.FkTree,
		FkLocal:    fkLocal,
		Bones:      skel.Bones,
		Center:     center,
		FloorY:     def.Canvas.FloorY,
		HeadRadius: def.HeadRadius,
		Params:     params,
	}
}

// ---------------------------------------------------------------------------
// FK math (rotation matrices + forward kinematics)
// ---------------------------------------------------------------------------

type mat3 [3][3]float64

func rotMat(rx, ry, rz float64) mat3 {
	cx, sx := math.Cos(rx), math.Sin(rx)
	cy, sy := math.Cos(ry), math.Sin(ry)
	cz, sz := math.Cos(rz), math.Sin(rz)
	return mat3{
		{cz * cy, cz*sy*sx - sz*cx, cz*sy*cx + sz*sx},
		{sz * cy, sz*sy*sx + cz*cx, sz*sy*cx - cz*sx},
		{-sy, cy * sx, cy * cx},
	}
}

func matVec(m mat3, v [3]float64) [3]float64 {
	return [3]float64{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2],
	}
}

func matMul(a, b mat3) mat3 {
	var o mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			s := 0.0
			for k := 0; k < 3; k++ {
				s += a[i][k] * b[k][j]
			}
			o[i][j] = s
		}
	}
	return o
}

// SolveFK propagates rotations down the FK tree (position uses parent
// accumulated rotation; own rotation only affects children).
func SolveFK(rootPos [3]float64, fkTree map[string]*string, fkLocal map[string][3]float64,
	rotations map[string][3]float64) map[string][3]float64 {
	// BFS topological order
	var root string
	for j, p := range fkTree {
		if p == nil {
			root = j
			break
		}
	}
	if root == "" {
		panic("fk_tree missing root")
	}
	order := []string{root}
	seen := map[string]bool{root: true}
	for i := 0; i < len(order); i++ {
		j := order[i]
		for c, p := range fkTree {
			if p != nil && *p == j && !seen[c] {
				seen[c] = true
				order = append(order, c)
			}
		}
	}
	var I mat3 = mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	out := make(map[string][3]float64, len(order))
	R := make(map[string]mat3, len(order))
	for _, j := range order {
		rj := rotations[j]
		Mj := rotMat(rj[0], rj[1], rj[2])
		p := fkTree[j]
		if p == nil {
			out[j] = rootPos
			R[j] = Mj
		} else {
			Rp := I
			if rp, ok := R[*p]; ok {
				Rp = rp
			}
			v := fkLocal[j]
			dv := matVec(Rp, v)
			pp := out[*p]
			out[j] = [3]float64{pp[0] + dv[0], pp[1] + dv[1], pp[2] + dv[2]}
			R[j] = matMul(Rp, Mj)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Pose (FK branch)
// ---------------------------------------------------------------------------

// Pose computes one frame pose in 3D space via FK.
// Returns {joint: [x,y,z]}.
func Pose(skel *Skeleton, motion *Motion, index int, params map[string]float64) map[string][3]float64 {
	frameCount := motion.FrameCount
	if frameCount <= 0 {
		frameCount = 8
	}
	phase := 2 * math.Pi * float64(index%frameCount) / float64(frameCount)
	signals := expr.BuildSignals(motion.Signals)
	ctx := &expr.Ctx{Params: params, Index: index, FrameCount: frameCount, Phase: phase, Signals: signals}

	out := make(map[string][3]float64, len(skel.Joints))
	for n, v := range skel.Joints {
		out[n] = v
	}
	root3d := motion.Root3d
	rdx := evalOr(root3d["x"], ctx)
	rdy := evalOr(root3d["y"], ctx)
	rdz := evalOr(root3d["z"], ctx)
	offsets := motion.Offsets3d

	fk3d := motion.Fk3d
	fkTree := skel.FkTree
	if fk3d != nil && len(fkTree) > 0 {
		root := fk3d.Root
		if _, ok := out[root]; !ok {
			root = ""
			for j, p := range fkTree {
				if p == nil {
					root = j
					break
				}
			}
		}
		rcomp := offsets[root]
		rootPos := [3]float64{
			out[root][0] + rdx + evalOr(rcomp["x"], ctx),
			out[root][1] + rdy + evalOr(rcomp["y"], ctx),
			out[root][2] + rdz + evalOr(rcomp["z"], ctx),
		}
		rotations := make(map[string][3]float64)
		for j, comp := range fk3d.Rotations3d {
			if _, ok := fkTree[j]; ok {
				rotations[j] = [3]float64{
					evalOr(comp["x_rot"], ctx),
					evalOr(comp["y_rot"], ctx),
					evalOr(comp["z_rot"], ctx),
				}
			}
		}
		out = SolveFK(rootPos, fkTree, skel.FkLocal, rotations)
		// FK add-on: explicit world offsets for non-root joints
		for j, comp := range offsets {
			if _, ok := out[j]; ok && j != root {
				v := out[j]
				v[0] += evalOr(comp["x"], ctx)
				v[1] += evalOr(comp["y"], ctx)
				v[2] += evalOr(comp["z"], ctx)
				out[j] = v
			}
		}
	}
	return out
}

func evalOr(e *expr.Expr, ctx *expr.Ctx) float64 {
	if e == nil {
		return 0
	}
	return e.Eval(ctx)
}

// FKWorldPose returns per-joint world position + world rotation matrices
// used by LBS vertex skinning.
func FKWorldPose(skel *Skeleton, motion *Motion, index int, params map[string]float64) (map[string][3]float64, map[string]mat3) {
	frameCount := motion.FrameCount
	if frameCount <= 0 {
		frameCount = 8
	}
	phase := 2 * math.Pi * float64(index%frameCount) / float64(frameCount)
	signals := expr.BuildSignals(motion.Signals)
	ctx := &expr.Ctx{Params: params, Index: index, FrameCount: frameCount, Phase: phase, Signals: signals}

	out := make(map[string][3]float64, len(skel.Joints))
	for n, v := range skel.Joints {
		out[n] = v
	}
	root3d := motion.Root3d
	rdx := evalOr(root3d["x"], ctx)
	rdy := evalOr(root3d["y"], ctx)
	rdz := evalOr(root3d["z"], ctx)
	offsets := motion.Offsets3d

	fk3d := motion.Fk3d
	fkTree := skel.FkTree
	root := ""
	if fk3d != nil {
		root = fk3d.Root
	}
	if _, ok := out[root]; !ok {
		root = ""
		for j, p := range fkTree {
			if p == nil {
				root = j
				break
			}
		}
	}
	rcomp := offsets[root]
	rootPos := [3]float64{
		out[root][0] + rdx + evalOr(rcomp["x"], ctx),
		out[root][1] + rdy + evalOr(rcomp["y"], ctx),
		out[root][2] + rdz + evalOr(rcomp["z"], ctx),
	}
	rotations := make(map[string][3]float64)
	if fk3d != nil {
		for j, comp := range fk3d.Rotations3d {
			if _, ok := fkTree[j]; ok {
				rotations[j] = [3]float64{
					evalOr(comp["x_rot"], ctx),
					evalOr(comp["y_rot"], ctx),
					evalOr(comp["z_rot"], ctx),
				}
			}
		}
	}
	// FK accumulation (same as SolveFK but also returns R_map)
	order := []string{root}
	seen := map[string]bool{root: true}
	for i := 0; i < len(order); i++ {
		j := order[i]
		for c, p := range fkTree {
			if p != nil && *p == j && !seen[c] {
				seen[c] = true
				order = append(order, c)
			}
		}
	}
	var I mat3 = mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	posMap := make(map[string][3]float64, len(order))
	RMap := make(map[string]mat3, len(order))
	for _, j := range order {
		rj := rotations[j]
		Mj := rotMat(rj[0], rj[1], rj[2])
		p := fkTree[j]
		if p == nil {
			posMap[j] = rootPos
			RMap[j] = Mj
		} else {
			Rp := I
			if rp, ok := RMap[*p]; ok {
				Rp = rp
			}
			v := skel.FkLocal[j]
			dv := matVec(Rp, v)
			pp := posMap[*p]
			posMap[j] = [3]float64{pp[0] + dv[0], pp[1] + dv[1], pp[2] + dv[2]}
			RMap[j] = matMul(Rp, Mj)
		}
	}
	return posMap, RMap
}
