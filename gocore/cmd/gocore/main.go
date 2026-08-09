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
	"io"
	"os"
	"path/filepath"

	"github.com/PeonUnion/creature-forge/gocore/internal/config"
	"github.com/PeonUnion/creature-forge/gocore/internal/logging"
	"github.com/PeonUnion/creature-forge/gocore/skeleton"
)

func main() {
	var dataDir, species, action, task, configPath string
	var frame int
	var useStdin bool
	flag.StringVar(&dataDir, "data-dir", "../../data", "species data root (contains species/)")
	flag.StringVar(&species, "species", "human", "species id")
	flag.StringVar(&action, "action", "", "action id (required for pose/lbs)")
	flag.IntVar(&frame, "frame", 0, "frame index")
	flag.StringVar(&task, "task", "build", "task: build | pose | lbs")
	flag.StringVar(&configPath, "config", "", "config.yaml path (optional; viper)")
	flag.BoolVar(&useStdin, "stdin", false, "read JSON request from stdin (Python bridge, batch frames)")
	flag.Parse()

	applyConfig(configPath, task, species)
	defer logging.Default().Sync()

	if useStdin {
		runStdin()
		return
	}

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

// Req is the stdin request payload (Python bridge).
type Req struct {
	Task      string                `json:"task"` // pose | lbs
	Joints    map[string][3]float64 `json:"joints"`
	FkTree    map[string]*string    `json:"fk_tree"`
	FkLocal   map[string][3]float64 `json:"fk_local"`
	Motion    *skeleton.Motion      `json:"motion"`
	Frames    []int                 `json:"frames"`
	Params    map[string]float64    `json:"params"`
	Mesh      *skeleton.Mesh        `json:"mesh,omitempty"`
	Weights   *skeleton.Weights     `json:"weights,omitempty"`
	BodyScale float64               `json:"body_scale"`
}

// runStdin reads a JSON request on stdin and emits batch frame results.
func runStdin() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatal(err)
	}
	var req Req
	if err := json.Unmarshal(data, &req); err != nil {
		fatal(fmt.Errorf("stdin request: %w", err))
	}
	if req.Motion == nil {
		fatal(fmt.Errorf("request missing motion"))
	}
	skel := &skeleton.Skeleton{Joints: req.Joints, FkTree: req.FkTree, FkLocal: req.FkLocal}
	frames := req.Frames
	if len(frames) == 0 {
		frames = []int{0}
	}
	out := make([]map[string]any, 0, len(frames))
	for _, idx := range frames {
		switch req.Task {
		case "lbs":
			if req.Mesh == nil || req.Weights == nil {
				fatal(fmt.Errorf("lbs requires mesh+weights"))
			}
			posMap, rMap := skeleton.FKWorldPose(skel, req.Motion, idx, req.Params)
			verts := skeleton.SkinnedVertices(posMap, rMap, skel, req.Mesh, req.Weights, req.BodyScale)
			out = append(out, map[string]any{"frame": idx, "vertices": verts})
		case "pose":
			pose := skeleton.Pose(skel, req.Motion, idx, req.Params)
			out = append(out, map[string]any{"frame": idx, "pose": pose})
		default:
			fatal(fmt.Errorf("unknown task %q", req.Task))
		}
	}
	emit(map[string]any{"ok": true, "task": req.Task, "frames": out})
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// applyConfig loads config.yaml (if provided) to configure the logging package
// (zap wrapper) with viper-read values; falls back to defaults otherwise.
func applyConfig(path, task, species string) {
	logging.SetDefault(logging.New(logging.Config{Level: logging.LevelInfo}))
	if path == "" {
		return
	}
	cfg, err := config.Load(path)
	if err != nil {
		logging.Warn("config load failed, use defaults", "path", path, "error", err)
		return
	}
	logging.SetDefault(logging.New(logging.Config{
		Level:  logging.Level(cfg.Log.Level),
		Format: logging.Format(cfg.Log.Format),
		Output: cfg.Log.Output,
	}))
	logging.Info("config loaded", "path", path, "server", cfg.Server.Port, "data_root", cfg.Data.Root)
	logging.Info("gocore start", "task", task, "species", species)
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
