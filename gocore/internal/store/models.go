// Package store is the data-access layer: it owns the complete domain models
// (mirroring the real JSON files under data/ species|presets|skins) and the
// Store CRUD over the data directory.
//
// Data lives in external JSON — the code never hardcodes any values. The
// engine package (skeleton) stays independent; store only provides raw models
// plus ToEngine* conversion helpers.
package store

import (
	"encoding/json"
	"fmt"

	"github.com/PeonUnion/creature-forge/gocore/expr"
)

// ---------------------------------------------------------------------------
// Canvas
// ---------------------------------------------------------------------------

// Canvas mirrors the canvas block of default.json.
type Canvas struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	FloorY float64 `json:"floor_y"`
}

// ---------------------------------------------------------------------------
// Param / ParamChain (species coordinate params)
// ---------------------------------------------------------------------------

// Param is a generic parameter spec (coordinate or action param share shape).
type Param struct {
	Default float64 `json:"default"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Step    float64 `json:"step"`
	Label   string  `json:"label"`
}

// ParamChain describes how a body param scales a set of joints (structure
// only — the numeric part is baked into species params/default).
type ParamChain struct {
	Joints  []string `json:"joints"`
	Param   string   `json:"param"`
	Anchor  string   `json:"anchor"`
	Label   string   `json:"label"`
	Min     float64  `json:"min"`
	Max     float64  `json:"max"`
	Step    float64  `json:"step"`
	Default float64  `json:"default"`
}

// ---------------------------------------------------------------------------
// Constraints
// ---------------------------------------------------------------------------

// Constraints mirrors the constraints block of skeleton.json.
type Constraints struct {
	Schema      string `json:"schema"`
	Description string `json:"description"`
	Symmetry3D  struct {
		Pairs [][2]string `json:"pairs"`
	} `json:"symmetry3d"`
}

// ---------------------------------------------------------------------------
// Species (species/<id>/skeleton.json)
// ---------------------------------------------------------------------------

// Species is the full species skeleton document.
type Species struct {
	SpeciesID   string                `json:"species_id"`
	Schema      string                `json:"schema"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	Joints      map[string]string     `json:"joints"`       // flat: name → name
	Chains      map[string][]string   `json:"chains"`       // spine/arm_left/...
	ParamChains map[string]ParamChain `json:"param_chains"` // name → chain spec
	Params      map[string]Param      `json:"params"`       // coordinate params
	Bones3D     [][2]string           `json:"bones_3d"`
	FkTree      map[string]*string    `json:"fk_tree"` // joint → parent (nil = root)
	Constraints *Constraints          `json:"constraints,omitempty"`
}

// ---------------------------------------------------------------------------
// SpeciesDefault (species/<id>/default.json)
// ---------------------------------------------------------------------------

// Position3D is a joint coordinate: either [x,y,z] numbers or {x,y,z} whose
// components may be numbers or coordinate-param expressions. Kept raw so the
// engine (skeleton.JointPos) does its own parse on conversion.
type Position3D struct {
	Array *[3]float64
	Obj   map[string]json.RawMessage
}

// UnmarshalJSON accepts [x,y,z] | {x,y,z}.
func (p *Position3D) UnmarshalJSON(b []byte) error {
	*p = Position3D{}
	var arr [3]float64
	if err := json.Unmarshal(b, &arr); err == nil {
		p.Array = &arr
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		return fmt.Errorf("position3d: cannot parse %s: %w", b, err)
	}
	p.Obj = obj
	return nil
}

// SpeciesDefault mirrors species/<id>/default.json.
type SpeciesDefault struct {
	Schema      string                `json:"schema"`
	Species     string                `json:"species"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	HeadRadius  float64               `json:"head_radius"`
	Canvas      Canvas                `json:"canvas"`
	Positions3D map[string]Position3D `json:"positions_3d"`
	Body        map[string]float64    `json:"body,omitempty"`
}

// ---------------------------------------------------------------------------
// Preset (presets/<id>.json)
// ---------------------------------------------------------------------------

// BakedSkel3D is the skel3d block of a baked preset (a frozen skeleton
// snapshot independent of the species — produced by the bake step).
type BakedSkel3D struct {
	SpeciesID    string                `json:"species_id"`
	Joints       map[string][3]float64 `json:"joints"`
	Bones        [][2]string           `json:"bones"`
	Chains       map[string][]string   `json:"chains"`
	Center       [3]float64            `json:"center"`
	FloorY       float64               `json:"floor_y"`
	RigidChains  [][]string            `json:"rigid_chains"`
	HeadRadius   float64               `json:"head_radius"`
	FollowChains map[string][]string   `json:"follow_chains"`
	FollowConfig map[string]any        `json:"follow_config"`
	FkTree       map[string]*string    `json:"fk_tree"`
	FkLocal      map[string][3]float64 `json:"fk_local"`
	Constraints  json.RawMessage       `json:"constraints"`
	Params       map[string]float64    `json:"params"`
}

// Baked is the baked block of a preset (schema creatureforge_preset_baked_v1).
type Baked struct {
	Schema  string                        `json:"schema"`
	Species string                        `json:"species"`
	Skel3D  *BakedSkel3D                  `json:"skel3d,omitempty"`
	Body    map[string]float64            `json:"body,omitempty"`
	Actions map[string]map[string]float64 `json:"actions,omitempty"`
}

// Preset is a species instance: body params + per-action param overrides.
type Preset struct {
	Schema      string                        `json:"schema"`
	PresetID    string                        `json:"preset_id"`
	Title       string                        `json:"title"`
	Description string                        `json:"description"`
	Species     string                        `json:"species"`
	Body        map[string]float64            `json:"body"`
	Actions     map[string]map[string]float64 `json:"actions"`
	Baked       *Baked                        `json:"baked,omitempty"`
}

// PresetSummary is the list view of a preset.
type PresetSummary struct {
	PresetID    string `json:"preset_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Species     string `json:"species"`
}

// ---------------------------------------------------------------------------
// Skin (skins/<id>.json)
// ---------------------------------------------------------------------------

// SkinPart is a game-style skin part attached to a bone or skinned via LBS.
type SkinPart struct {
	PartID    string               `json:"part_id"`
	Title     string               `json:"title"`
	Kind      string               `json:"kind"` // bone | skinned
	Bone      string               `json:"bone"`
	Transform map[string][]float64 `json:"transform"` // position/rotation/scale
	MeshFile  *string              `json:"mesh_file,omitempty"`
	Mesh      map[string]any       `json:"mesh,omitempty"`
	Textures  map[string]string    `json:"textures"`
	Materials map[string]any       `json:"materials"`
	Weights   map[string]any       `json:"weights,omitempty"`
}

// Skin is a preset appearance instance: base params + parts collection.
type Skin struct {
	Schema      string             `json:"schema"`
	SkinID      string             `json:"skin_id"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Preset      string             `json:"preset"`
	Species     string             `json:"species"`
	Materials   map[string]any     `json:"materials"`
	Params      map[string]float64 `json:"params"`
	Parts       []SkinPart         `json:"parts"`
}

// SkinSummary is the list view of a skin.
type SkinSummary struct {
	SkinID      string `json:"skin_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Preset      string `json:"preset"`
	Species     string `json:"species"`
}

// ---------------------------------------------------------------------------
// Motion / Action (species/<id>/actions3d/<motion_id>.json)
// ---------------------------------------------------------------------------

// FK3D is the fk3d animation block.
type FK3D struct {
	Root        string                           `json:"root"`
	Rotations3d map[string]map[string]*expr.Expr `json:"rotations3d"` // joint → {x_rot,y_rot,z_rot}
}

// Motion is a 3D action document.
type Motion struct {
	Schema      string                           `json:"schema"`
	MotionID    string                           `json:"motion_id"`
	Title       string                           `json:"title"`
	Description string                           `json:"description"`
	Species     string                           `json:"species"`
	FrameCount  int                              `json:"frame_count"`
	Fps         int                              `json:"fps,omitempty"`
	Params      map[string]Param                 `json:"params"`
	Signals     map[string]*expr.Expr            `json:"signals"`
	Fk3d        *FK3D                            `json:"fk3d,omitempty"`
	Root3d      map[string]*expr.Expr            `json:"root3d,omitempty"`
	Offsets3d   map[string]map[string]*expr.Expr `json:"offsets3d,omitempty"`
}

// ActionSummary is the list view of an action.
type ActionSummary struct {
	ID     string           `json:"id"`
	Title  string           `json:"title"`
	Params map[string]Param `json:"params"`
}
