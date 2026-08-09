package store

// Conversion helpers: full store models → engine (skeleton) input types.
// The engine stays independent of the store package; adapters live here.
import (
	"encoding/json"
	"fmt"

	"github.com/PeonUnion/creature-forge/gocore/expr"
	"github.com/PeonUnion/creature-forge/gocore/skeleton"
)

// ToEngineSkeleton converts a full Species to the engine's SpeciesSkeleton
// subset used by BuildSkeleton.
func (s *Species) ToEngineSkeleton() *skeleton.SpeciesSkeleton {
	return &skeleton.SpeciesSkeleton{
		SpeciesID: s.SpeciesID,
		FkTree:    s.FkTree,
		Bones:     s.Bones3D,
		Params:    toEngineCoordParams(s.Params),
	}
}

// ToEngineDefault converts SpeciesDefault → skeleton.Default, parsing each
// Position3D into a skeleton.JointPos (numbers or expressions).
func (d *SpeciesDefault) ToEngineDefault() (*skeleton.Default, error) {
	out := &skeleton.Default{
		HeadRadius: d.HeadRadius,
		Body:       d.Body,
		Canvas: struct {
			Width  float64 `json:"width"`
			Height float64 `json:"height"`
			FloorY float64 `json:"floor_y"`
		}{Width: d.Canvas.Width, Height: d.Canvas.Height, FloorY: d.Canvas.FloorY},
		Positions3d: make(map[string]skeleton.JointPos, len(d.Positions3D)),
	}
	for name, pos := range d.Positions3D {
		jp := skeleton.JointPos{}
		if pos.Array != nil {
			var arr [3]*expr.ParamValue
			for i, v := range *pos.Array {
				v := v
				arr[i] = &expr.ParamValue{Num: &v}
			}
			jp.Array = &arr
		} else if pos.Obj != nil {
			obj := make(map[string]*expr.ParamValue, len(pos.Obj))
			for k, raw := range pos.Obj {
				var pv expr.ParamValue
				if err := json.Unmarshal(raw, &pv); err != nil {
					return nil, fmt.Errorf("joint %s %s: %w", name, k, err)
				}
				obj[k] = &pv
			}
			jp.Obj = obj
		}
		out.Positions3d[name] = jp
	}
	return out, nil
}

// ToEngineMotion converts a full Motion to the engine's Motion subset.
func (m *Motion) ToEngineMotion() *skeleton.Motion {
	return &skeleton.Motion{
		Schema:     m.Schema,
		MotionID:   m.MotionID,
		FrameCount: m.FrameCount,
		Params:     toEngineParams(m.Params),
		Signals:    m.Signals,
		Fk3d:       toEngineFK3D(m.Fk3d),
		Root3d:     m.Root3d,
		Offsets3d:  m.Offsets3d,
	}
}

// toEngineCoordParams maps coordinate params (full Param) → engine CoordParam.
func toEngineCoordParams(src map[string]Param) map[string]skeleton.CoordParam {
	out := make(map[string]skeleton.CoordParam, len(src))
	for k, v := range src {
		out[k] = skeleton.CoordParam{
			Default: v.Default, Min: v.Min, Max: v.Max, Step: v.Step, Label: v.Label,
		}
	}
	return out
}

// toEngineParams maps action params (full Param) → engine ParamSpec.
func toEngineParams(src map[string]Param) map[string]skeleton.ParamSpec {
	out := make(map[string]skeleton.ParamSpec, len(src))
	for k, v := range src {
		out[k] = skeleton.ParamSpec{
			Default: v.Default, Min: v.Min, Max: v.Max, Step: v.Step, Label: v.Label,
		}
	}
	return out
}

// toEngineFK3D maps the FK3D block.
func toEngineFK3D(src *FK3D) *skeleton.FK3D {
	if src == nil {
		return nil
	}
	return &skeleton.FK3D{Root: src.Root, Rotations3d: src.Rotations3d}
}
