package server

// Templates, wizard (pending), preset3d render (pending) and action param
// extraction (pending) endpoints.
import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// routeTemplates handles GET /api/templates.
func (s *Server) routeTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items, err := s.templatesList()
	if err != nil {
		s.fail(w, err, http.StatusInternalServerError)
		return
	}
	s.json(w, map[string]any{"templates": items}, http.StatusOK)
}

// templatesList lists data/templates/*.json morph templates.
func (s *Server) templatesList() ([]map[string]any, error) {
	dir := filepath.Join(s.Store.DataDir, "templates")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	names := make([]string, 0)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	out := make([]map[string]any, 0, len(names))
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			continue
		}
		var t struct {
			MorphID     string         `json:"morph_id"`
			Title       string         `json:"title"`
			Description string         `json:"description"`
			Tags        []string       `json:"tags"`
			LimbScheme  string         `json:"limb_scheme"`
			Symmetry    bool           `json:"symmetry"`
			Root        string         `json:"root"`
			Nodes       map[string]any `json:"nodes"`
			Chains      map[string]any `json:"chains"`
			Actions     []string       `json:"actions"`
		}
		if json.Unmarshal(b, &t) != nil {
			continue
		}
		out = append(out, map[string]any{
			"morph_id":    t.MorphID,
			"title":       t.Title,
			"description": t.Description,
			"tags":        t.Tags,
			"limb_scheme": t.LimbScheme,
			"symmetry":    t.Symmetry,
			"root":        t.Root,
			"joint_count": len(t.Nodes),
			"chain_count": len(t.Chains),
			"actions":     t.Actions,
		})
	}
	return out, nil
}

// routeWizard — the species wizard (init/joint/limb/chain/pose/coord/commit).
// Pending: species wizard (init/joint/coord/commit).
func (s *Server) routeWizard(w http.ResponseWriter, r *http.Request) {
	s.fail(w, errors.New("species wizard not implemented yet"), http.StatusNotImplemented)
}

// routePreset3d — preset rendering (skeleton or action frame/GIF).
func (s *Server) routePreset3d(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	parts := pathParts(r.URL.Path, "/api/preset3d")
	if len(parts) != 1 {
		s.fail(w, errors.New("preset id or 'live' required"), http.StatusBadRequest)
		return
	}
	q := r.URL.Query()
	cam := parseCamQuery(r)
	var body, actions map[string]float64
	if parts[0] == "live" {
		species := q.Get("species")
		if species == "" {
			s.fail(w, errors.New("live preset requires species"), http.StatusBadRequest)
			return
		}
		_ = json.Unmarshal([]byte(q.Get("body")), &body)
		_ = json.Unmarshal([]byte(q.Get("actions")), &actions)
	}
	actionID := q.Get("action")
	result, err := s.renderPreset3d(parts[0], q.Get("species"), body, actions, actionID, cam)
	if err != nil {
		s.fail(w, err, http.StatusInternalServerError)
		return
	}
	s.json(w, result, http.StatusOK)
}

// actionExtractParams — split a single intensity into per-part/dimension
// params. Pending: action parameter extraction.
func (s *Server) actionExtractParams(speciesID, actionID string) (map[string]any, error) {
	return nil, errors.New("action param extraction not implemented yet")
}
