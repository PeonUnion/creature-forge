// Command gocore exposes the Go computation kernel (FK pose + LBS skinning)
// as a standalone binary reading external JSON data — demonstrating the
// "Go core + Python orchestration" integration path (Python can shell out to
// this binary for the hot numeric paths).
//
// Usage:
//
//	gocore --data-dir <root> --species human [--action walk3d] [--frame 0] --task build|pose|lbs
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PeonUnion/creature-forge/gocore/skeleton"
)

func main() {
	var dataDir, species, action, task string
	var frame int
	flag.StringVar(&dataDir, "data-dir", "../../data", "species data root (contains species/)")
	flag.StringVar(&species, "species", "human", "species id")
	flag.StringVar(&action, "action", "", "action id (required for pose/lbs)")
	flag.IntVar(&frame, "frame", 0, "frame index")
	flag.StringVar(&task, "task", "build", "task: build | pose | lbs")
	flag.Parse()

	base := filepath.Join(dataDir, "species", species)
	var skel skeleton.SpeciesSkeleton
	if err := readJSON(filepath.Join(base, "skeleton.json"), &skel); err != nil {
		fatal(err)
	}
	var def skeleton.Default
	if err := readJSON(filepath.Join(base, "default.json"), &def); err != nil {
		fatal(err)
	}
	s := skeleton.BuildSkeleton(&skel, &def, nil)

	switch task {
	case "build":
		emit(map[string]any{"joints": s.Joints, "bones": s.Bones, "fk_tree": s.FkTree})
	case "pose", "lbs":
		if action == "" {
			fatal(fmt.Errorf("--action required for task %s", task))
		}
		var motion skeleton.Motion
		if err := readJSON(filepath.Join(base, "actions3d", action+".json"), &motion); err != nil {
			fatal(err)
		}
		params := make(map[string]float64)
		for name, spec := range motion.Params {
			params[name] = spec.Default
		}
		if task == "pose" {
			pose := skeleton.Pose(s, &motion, frame, params)
			emit(map[string]any{"frame": frame, "pose": pose})
		} else {
			mesh, err := skeleton.LoadMesh(filepath.Join(base, "skin", "mesh.json"))
			if err != nil {
				fatal(err)
			}
			weights, err := skeleton.LoadWeights(filepath.Join(base, "skin", "weights.json"))
			if err != nil {
				fatal(err)
			}
			posMap, rMap := skeleton.FKWorldPose(s, &motion, frame, params)
			verts := skeleton.SkinnedVertices(posMap, rMap, s, mesh, weights, 0)
			emit(map[string]any{"frame": frame, "vertex_count": len(verts) / 3, "vertices": verts})
		}
	default:
		fatal(fmt.Errorf("unknown task %q", task))
	}
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func emit(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gocore:", err)
	os.Exit(1)
}
