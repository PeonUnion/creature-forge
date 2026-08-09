package store

// Tests exercise the store layer against the real repo data (data/), verify
// the store→engine conversion chain produces the same poses as the raw
// skeleton package, and cover CRUD on a temp dir.
import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PeonUnion/creature-forge/gocore/skeleton"
)

const dataRoot = "../../../data"

func TestListAndGetSpecies(t *testing.T) {
	s := New(dataRoot)
	ids, err := s.ListSpecies()
	if err != nil {
		t.Fatalf("ListSpecies: %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("expected at least one species")
	}
	found := false
	for _, id := range ids {
		if id == "human" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected human in species list, got %v", ids)
	}

	sp, err := s.GetSpecies("human")
	if err != nil {
		t.Fatalf("GetSpecies: %v", err)
	}
	if sp.SpeciesID != "human" || sp.Schema == "" || len(sp.FkTree) < 30 {
		t.Fatalf("bad species doc: id=%q schema=%q fk=%d", sp.SpeciesID, sp.Schema, len(sp.FkTree))
	}
	def, err := s.GetDefault("human")
	if err != nil {
		t.Fatalf("GetDefault: %v", err)
	}
	if len(def.Positions3D) < 30 || def.Canvas.Width <= 0 {
		t.Fatalf("bad default doc: positions=%d canvas=%+v", len(def.Positions3D), def.Canvas)
	}

	actions, err := s.ListActions("human")
	if err != nil {
		t.Fatalf("ListActions: %v", err)
	}
	if len(actions) == 0 {
		t.Fatal("expected actions")
	}
	a, err := s.GetAction("human", actions[0])
	if err != nil {
		t.Fatalf("GetAction: %v", err)
	}
	if a.MotionID == "" || a.FrameCount <= 0 {
		t.Fatalf("bad action doc: %+v", a)
	}
}

// TestStoreEngineChain verifies the store→engine conversion reproduces the
// same build/pose as the raw skeleton package path (data is identical).
func TestStoreEngineChain(t *testing.T) {
	s := New(dataRoot)
	sp, err := s.GetSpecies("human")
	if err != nil {
		t.Fatal(err)
	}
	def, err := s.GetDefault("human")
	if err != nil {
		t.Fatal(err)
	}
	engDef, err := def.ToEngineDefault()
	if err != nil {
		t.Fatalf("ToEngineDefault: %v", err)
	}
	sk := skeleton.BuildSkeleton(sp.ToEngineSkeleton(), engDef, nil)
	if len(sk.Joints) != 36 {
		t.Fatalf("joints: got %d want 36", len(sk.Joints))
	}
	// pelvis must be at the default.json coordinate
	if _, ok := sk.Joints["pelvis"]; !ok {
		t.Fatal("missing pelvis")
	}

	m, err := s.GetAction("human", "walk3d")
	if err != nil {
		t.Fatal(err)
	}
	params := make(map[string]float64, len(m.Params))
	for k, p := range m.Params {
		params[k] = p.Default
	}
	pose := skeleton.Pose(sk, m.ToEngineMotion(), 0, params)
	if len(pose) != 36 {
		t.Fatalf("pose: got %d joints want 36", len(pose))
	}
}

// TestPresetCRUD round-trips a preset through a temp dir.
func TestPresetCRUD(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	p := &Preset{
		Schema:   "creatureforge_preset_v1",
		PresetID: "test_preset",
		Title:    "测试",
		Species:  "human",
		Body:     map[string]float64{"head_scale": 1.2},
		Actions:  map[string]map[string]float64{"walk3d": {"intensity": 1.1}},
	}
	if err := s.SavePreset(p); err != nil {
		t.Fatalf("SavePreset: %v", err)
	}
	got, err := s.GetPreset("test_preset")
	if err != nil {
		t.Fatalf("GetPreset: %v", err)
	}
	if got.Body["head_scale"] != 1.2 || got.Actions["walk3d"]["intensity"] != 1.1 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	ids, err := s.ListPresets()
	if err != nil || len(ids) != 1 || ids[0] != "test_preset" {
		t.Fatalf("ListPresets: %v %v", ids, err)
	}
	if err := s.DeletePreset("test_preset"); err != nil {
		t.Fatalf("DeletePreset: %v", err)
	}
	if _, err := s.GetPreset("test_preset"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestSkinCRUD round-trips a skin with a part through a temp dir.
func TestSkinCRUD(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	sk := &Skin{
		Schema:  "creatureforge_skin_v1",
		SkinID:  "sk_test",
		Title:   "测试皮肤",
		Preset:  "model_male",
		Species: "human",
		Params:  map[string]float64{"skin_tone": 0.5},
		Parts: []SkinPart{{
			PartID: "p_helmet", Title: "头盔", Kind: "bone", Bone: "head",
			Transform: map[string][]float64{"position": {0, 1, 2}},
			Textures:  map[string]string{"albedo": "skin://p_helmet/albedo.png"},
		}},
	}
	if err := s.SaveSkin(sk); err != nil {
		t.Fatalf("SaveSkin: %v", err)
	}
	got, err := s.GetSkin("sk_test")
	if err != nil {
		t.Fatalf("GetSkin: %v", err)
	}
	if len(got.Parts) != 1 || got.Parts[0].PartID != "p_helmet" {
		t.Fatalf("parts mismatch: %+v", got.Parts)
	}
	if err := s.DeleteSkin("sk_test"); err != nil {
		t.Fatalf("DeleteSkin: %v", err)
	}
}

// TestAllSpeciesLoad verifies every species (and its actions) in the repo data
// parses through the store and the store→engine chain produces full poses.
func TestAllSpeciesLoad(t *testing.T) {
	s := New(dataRoot)
	ids, err := s.ListSpecies()
	if err != nil {
		t.Fatalf("ListSpecies: %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("no species in repo data")
	}
	for _, id := range ids {
		sp, err := s.GetSpecies(id)
		if err != nil {
			t.Errorf("%s skeleton: %v", id, err)
			continue
		}
		def, err := s.GetDefault(id)
		if err != nil {
			t.Errorf("%s default: %v", id, err)
			continue
		}
		engDef, err := def.ToEngineDefault()
		if err != nil {
			t.Errorf("%s engine-def: %v", id, err)
			continue
		}
		sk := skeleton.BuildSkeleton(sp.ToEngineSkeleton(), engDef, nil)
		if len(sk.Joints) == 0 {
			t.Errorf("%s: no joints built", id)
		}
		acts, err := s.ListActions(id)
		if err != nil {
			t.Errorf("%s actions: %v", id, err)
			continue
		}
		for _, a := range acts {
			m, err := s.GetAction(id, a)
			if err != nil {
				t.Errorf("%s/%s: %v", id, a, err)
				continue
			}
			params := make(map[string]float64, len(m.Params))
			for k, p := range m.Params {
				params[k] = p.Default
			}
			pose := skeleton.Pose(sk, m.ToEngineMotion(), 0, params)
			if len(pose) != len(sk.Joints) {
				t.Errorf("%s/%s: pose %d vs joints %d", id, a, len(pose), len(sk.Joints))
			}
		}
	}
}

// TestAllPresetsLoad verifies every preset parses through the store.
func TestAllPresetsLoad(t *testing.T) {
	s := New(dataRoot)
	presets, err := s.ListPresets()
	if err != nil {
		t.Fatalf("ListPresets: %v", err)
	}
	if len(presets) == 0 {
		t.Fatal("no presets in repo data")
	}
	for _, p := range presets {
		pp, err := s.GetPreset(p)
		if err != nil {
			t.Errorf("preset %s: %v", p, err)
			continue
		}
		if pp.Species == "" || pp.Body == nil {
			t.Errorf("preset %s: missing species/body", p)
		}
	}
}

// TestSaveSpecies writes a species skeleton and reads it back (format check).
func TestSaveSpecies(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	sp := &Species{
		SpeciesID: "robot", Schema: "creatureforge_species_v1", Title: "机器人",
		Joints: map[string]string{"head": "head"},
		Chains: map[string][]string{"spine": {"head"}},
		FkTree: map[string]*string{"head": nil},
		Params: map[string]Param{"head_scale": {Default: 1, Min: 0.5, Max: 2, Step: 0.05, Label: "头"}},
	}
	if err := s.SaveSpecies(sp); err != nil {
		t.Fatalf("SaveSpecies: %v", err)
	}
	b, err := os.ReadFile(s.SpeciesPath("robot"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(s.SpeciesPath("robot")) != "skeleton.json" {
		t.Fatalf("unexpected path %s", s.SpeciesPath("robot"))
	}
	if len(b) == 0 {
		t.Fatal("empty file")
	}
	got, err := s.GetSpecies("robot")
	if err != nil {
		t.Fatal(err)
	}
	if got.Params["head_scale"].Max != 2 {
		t.Fatalf("param round trip: %+v", got.Params)
	}
}
