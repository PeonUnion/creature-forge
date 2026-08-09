package skeleton

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

// 用真实数据（data/species/human）验证引擎输出与基准数据一致
// （基准：gocore/skeleton/testdata/golden_human.json）。

const dataRoot = "../../data/species/human"

func load(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}

func closeEnough(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestBuildSkeletonMatchesGolden(t *testing.T) {
	var skel SpeciesSkeleton
	load(t, dataRoot+"/skeleton.json", &skel)
	var def Default
	load(t, dataRoot+"/default.json", &def)
	s := BuildSkeleton(&skel, &def, nil)

	var golden struct {
		Joints map[string][3]float64 `json:"joints"`
	}
	load(t, "testdata/golden_human.json", &golden)
	if len(s.Joints) != len(golden.Joints) {
		t.Fatalf("joints count: got %d want %d", len(s.Joints), len(golden.Joints))
	}
	for name, want := range golden.Joints {
		got, ok := s.Joints[name]
		if !ok {
			t.Errorf("joint %s missing", name)
			continue
		}
		for i := 0; i < 3; i++ {
			if !closeEnough(got[i], want[i]) {
				t.Errorf("joint %s[%d]: got %v want %v", name, i, got[i], want[i])
			}
		}
	}
	// 关键关节抽查
	if !closeEnough(s.Joints["head"][0], golden.Joints["head"][0]) ||
		!closeEnough(s.Joints["head"][1], golden.Joints["head"][1]) {
		t.Errorf("head mismatch: got %v want %v", s.Joints["head"], golden.Joints["head"])
	}
}

func TestPoseMatchesGolden(t *testing.T) {
	var skel SpeciesSkeleton
	load(t, dataRoot+"/skeleton.json", &skel)
	var def Default
	load(t, dataRoot+"/default.json", &def)
	var motion Motion
	load(t, dataRoot+"/actions3d/walk3d.json", &motion)
	s := BuildSkeleton(&skel, &def, nil)

	params := make(map[string]float64)
	for name, spec := range motion.Params {
		params[name] = spec.Default
	}

	var golden struct {
		Pose0 map[string][3]float64 `json:"pose_frame_0"`
		Pose5 map[string][3]float64 `json:"pose_frame_5"`
	}
	load(t, "testdata/golden_human.json", &golden)

	for idx, want := range map[int]map[string][3]float64{0: golden.Pose0, 5: golden.Pose5} {
		got := Pose(s, &motion, idx, params)
		if len(got) != len(want) {
			t.Fatalf("pose%d joints: got %d want %d", idx, len(got), len(want))
		}
		for name, w := range want {
			g, ok := got[name]
			if !ok {
				t.Errorf("pose%d joint %s missing", idx, name)
				continue
			}
			for i := 0; i < 3; i++ {
				if !closeEnough(g[i], w[i]) {
					t.Errorf("pose%d %s[%d]: got %v want %v", idx, name, i, g[i], w[i])
				}
			}
		}
	}
}

func TestLBSMatchesGolden(t *testing.T) {
	var skel SpeciesSkeleton
	load(t, dataRoot+"/skeleton.json", &skel)
	var def Default
	load(t, dataRoot+"/default.json", &def)
	var motion Motion
	load(t, dataRoot+"/actions3d/walk3d.json", &motion)
	mesh, err := LoadMesh(dataRoot + "/skin/mesh.json")
	if err != nil {
		t.Fatal(err)
	}
	weights, err := LoadWeights(dataRoot + "/skin/weights.json")
	if err != nil {
		t.Fatal(err)
	}
	s := BuildSkeleton(&skel, &def, nil)

	params := make(map[string]float64)
	for name, spec := range motion.Params {
		params[name] = spec.Default
	}
	posMap, rMap := FKWorldPose(s, &motion, 0, params)
	got := SkinnedVertices(posMap, rMap, s, mesh, weights, 0)

	var golden struct {
		LBS []float64 `json:"lbs_frame0"`
	}
	load(t, "testdata/golden_human.json", &golden)
	if len(got) != len(golden.LBS) {
		t.Fatalf("lbs len: got %d want %d", len(got), len(golden.LBS))
	}
	mismatch := 0
	for i := range got {
		if math.Abs(got[i]-golden.LBS[i]) > 1e-6 {
			mismatch++
			if mismatch <= 5 {
				t.Errorf("lbs[%d]: got %v want %v", i, got[i], golden.LBS[i])
			}
		}
	}
	if mismatch > 0 {
		t.Errorf("lbs mismatch count: %d / %d", mismatch, len(got))
	}
}
