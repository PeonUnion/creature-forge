package skeleton

import (
	"encoding/json"
	"os"
	"testing"
)

// Benchmark16Frames 模拟真实场景：16 帧 LBS 蒙皮耗时。
func Benchmark16Frames(b *testing.B) {
	var skel SpeciesSkeleton
	loadBench(b, dataRoot+"/skeleton.json", &skel)
	var def Default
	loadBench(b, dataRoot+"/default.json", &def)
	var motion Motion
	loadBench(b, dataRoot+"/actions3d/walk3d.json", &motion)
	mesh, _ := LoadMesh(dataRoot + "/skin/mesh.json")
	weights, _ := LoadWeights(dataRoot + "/skin/weights.json")
	s := BuildSkeleton(&skel, &def, nil)
	params := make(map[string]float64)
	for name, spec := range motion.Params {
		params[name] = spec.Default
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < 16; i++ {
			posMap, rMap := FKWorldPose(s, &motion, i, params)
			_ = SkinnedVertices(posMap, rMap, s, mesh, weights, 0)
		}
	}
}

func loadBench(b *testing.B, path string, v any) {
	b.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		b.Fatal(err)
	}
}
