package skeleton

import (
	"encoding/json"
	"fmt"
	"os"
)

// ---------------------------------------------------------------------------
// Skinned mesh data (mirror skin/mesh.json + skin/weights.json)
// ---------------------------------------------------------------------------

// Mesh is the bind-pose triangle mesh.
type Mesh struct {
	Vertices    []float64 `json:"vertices"`
	Indices     []int     `json:"indices"`
	Normals     []float64 `json:"normals"`
	UVs         []float64 `json:"uvs,omitempty"`
	VertexCount int       `json:"vertex_count"`
}

// Weights is the per-vertex bone weights (≤4 bones each).
type Weights struct {
	BoneNames []string        `json:"boneNames"`
	PerVertex []VertexWeights `json:"perVertex"`
}

// VertexWeights binds up to 4 bones with weights.
type VertexWeights struct {
	Indices []int     `json:"indices"`
	Weights []float64 `json:"weights"`
}

// LoadMesh / LoadWeights read from external JSON files.
func LoadMesh(path string) (*Mesh, error) {
	var m Mesh
	if err := readJSON(path, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func LoadWeights(path string) (*Weights, error) {
	var w Weights
	if err := readJSON(path, &w); err != nil {
		return nil, err
	}
	return &w, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// LBS vertex skinning (mirror skinned_vertices)
// ---------------------------------------------------------------------------

// SkinnedVertices computes one frame's deformed vertices (flat [x,y,z,...],
// Y-down project coords) via linear blend skinning. bodyScale optionally
// scales the bind-pose x/z (horizontal) for fat/muscle.
func SkinnedVertices(posMap map[string][3]float64, rMap map[string]mat3,
	skel *Skeleton, mesh *Mesh, weights *Weights, bodyScale float64) []float64 {
	nv := mesh.VertexCount
	if nv == 0 {
		nv = len(mesh.Vertices) / 3
	}
	verts := mesh.Vertices
	per := weights.PerVertex
	bn := weights.BoneNames
	bind := skel.Joints
	out := make([]float64, nv*3)
	sx := bodyScale
	if sx == 0 {
		sx = 1
	}
	I := mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	for vi := 0; vi < nv; vi++ {
		vx, vy, vz := verts[3*vi], verts[3*vi+1], verts[3*vi+2]
		if bodyScale != 0 {
			vx *= sx
			vz *= sx
		}
		var ax, ay, az float64
		pw := per[vi]
		for k := range pw.Indices {
			bi := pw.Indices[k]
			wt := pw.Weights[k]
			jn := bn[bi]
			bj, ok := bind[jn]
			if !ok {
				ax += wt * vx
				ay += wt * vy
				az += wt * vz
				continue
			}
			Rm := I
			if rm, ok := rMap[jn]; ok {
				Rm = rm
			}
			rv := matVec(Rm, [3]float64{vx - bj[0], vy - bj[1], vz - bj[2]})
			pj, _ := posMap[jn]
			ax += wt * (rv[0] + pj[0])
			ay += wt * (rv[1] + pj[1])
			az += wt * (rv[2] + pj[2])
		}
		out[3*vi] = round3(ax)
		out[3*vi+1] = round3(ay)
		out[3*vi+2] = round3(az)
	}
	return out
}

func round3(v float64) float64 {
	return mathRound(v*1000) / 1000
}

func mathRound(v float64) float64 {
	if v >= 0 {
		return float64(int(v + 0.5))
	}
	return float64(int(v - 0.5))
}
