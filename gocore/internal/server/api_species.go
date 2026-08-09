package server

// Species + actions CRUD (mirror of server._species_* + species.py).
import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/PeonUnion/creature-forge/gocore/internal/store"
)

// routeSpecies dispatches /api/species* by method.
func (s *Server) routeSpecies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.speciesGet(w, r)
	case http.MethodPost, http.MethodPut:
		s.speciesPost(w, r)
	case http.MethodDelete:
		s.speciesDelete(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// --- GET ------------------------------------------------------------------

func (s *Server) speciesGet(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/species")
	if len(parts) == 0 {
		items, err := s.listSpecies()
		if err != nil {
			s.fail(w, err, http.StatusInternalServerError)
			return
		}
		s.json(w, map[string]any{"species": items}, http.StatusOK)
		return
	}
	id := parts[0]
	if len(parts) >= 2 && parts[1] == "preset_schema" {
		schema, err := s.Store.GetPresetSchema(id)
		if err != nil {
			s.fail(w, err, http.StatusInternalServerError)
			return
		}
		if schema == nil {
			s.json(w, map[string]any{"ok": false, "error": "preset_schema not found: " + id}, http.StatusNotFound)
			return
		}
		s.json(w, schema, http.StatusOK)
		return
	}
	if len(parts) >= 2 && parts[1] == "default" {
		def, err := s.Store.GetDefault(id)
		if err != nil {
			s.fail(w, err, http.StatusNotFound)
			return
		}
		s.json(w, def, http.StatusOK)
		return
	}
	if len(parts) >= 3 && parts[1] == "actions" {
		a, err := s.Store.GetAction(id, parts[2])
		if err != nil {
			s.fail(w, errors.New("action not found: "+id+"/"+parts[2]), http.StatusNotFound)
			return
		}
		s.json(w, a, http.StatusOK)
		return
	}
	// species detail: raw skeleton.json + actions (excluding base.json)
	detail, err := s.speciesDetail(id)
	if err != nil {
		s.fail(w, errors.New("species not found: "+id), http.StatusNotFound)
		return
	}
	s.json(w, detail, http.StatusOK)
}

// listSpecies mirrors SpeciesService.list().
func (s *Server) listSpecies() ([]map[string]any, error) {
	ids, err := s.Store.ListSpecies()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		sp, err := s.Store.GetSpecies(id)
		if err != nil {
			continue
		}
		acts := s.actionSummaries(id)
		motions := make([]string, 0, len(acts))
		for _, a := range acts {
			motions = append(motions, a["id"].(string))
		}
		out = append(out, map[string]any{
			"id":                id,
			"title":             sp.Title,
			"description":       sp.Description,
			"joint_count":       len(sp.Joints),
			"bone_count":        len(sp.Bones3D),
			"chain_count":       len(sp.Chains),
			"param_chain_count": len(sp.ParamChains),
			"motions":           motions,
			"actions":           acts,
		})
	}
	return out, nil
}

// actionSummaries mirrors SpeciesService.list_actions().
func (s *Server) actionSummaries(speciesID string) []map[string]any {
	acts, err := s.Store.ListActions(speciesID)
	if err != nil {
		return nil
	}
	out := make([]map[string]any, 0, len(acts))
	for _, id := range acts {
		if id == "base" {
			continue
		}
		m, err := s.Store.GetAction(speciesID, id)
		if err != nil {
			out = append(out, map[string]any{"id": id, "title": id, "params": map[string]any{}})
			continue
		}
		out = append(out, map[string]any{"id": m.MotionID, "title": m.Title, "params": m.Params})
	}
	return out
}

// speciesDetail mirrors SpeciesService.get() (raw skeleton + actions).
func (s *Server) speciesDetail(id string) (map[string]any, error) {
	b, err := os.ReadFile(s.Store.SpeciesPath(id))
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	actions := make([]any, 0)
	dir := s.Store.ActionsDir(id)
	if entries, err := os.ReadDir(dir); err == nil {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".json" && e.Name() != "base.json" {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			b, err := os.ReadFile(filepath.Join(dir, n))
			if err != nil {
				continue
			}
			var v any
			if json.Unmarshal(b, &v) == nil {
				actions = append(actions, v)
			}
		}
	}
	data["actions"] = actions
	return data, nil
}

// --- POST/PUT -------------------------------------------------------------

func (s *Server) speciesPost(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/species")
	var body map[string]any
	if err := decodeBody(r, &body); err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	if len(parts) == 0 {
		// create species
		sp, err := mapToSpecies(body)
		if err != nil {
			s.fail(w, err, http.StatusBadRequest)
			return
		}
		if _, err := s.Store.GetSpecies(sp.SpeciesID); err == nil {
			s.fail(w, errors.New("species already exists: "+sp.SpeciesID), http.StatusConflict)
			return
		}
		if sp.SpeciesID == "" {
			s.fail(w, errors.New("species_id required"), http.StatusBadRequest)
			return
		}
		if err := s.Store.SaveSpecies(sp); err != nil {
			s.fail(w, err, http.StatusBadRequest)
			return
		}
		s.writePresetSchema(sp)
		s.ok(w, map[string]any{"created": sp.SpeciesID})
		return
	}
	id := parts[0]
	if len(parts) >= 2 && parts[1] == "default" {
		if err := s.saveDefaultRaw(id, body); err != nil {
			s.fail(w, err, http.StatusBadRequest)
			return
		}
		s.ok(w, map[string]any{"saved": id})
		return
	}
	if len(parts) >= 2 && parts[1] == "actions" {
		if len(parts) >= 4 && parts[3] == "extract-params" {
			new, err := s.actionExtractParams(id, parts[2])
			if err != nil {
				s.fail(w, err, http.StatusBadRequest)
				return
			}
			s.json(w, map[string]any{"ok": true, "action": new}, http.StatusOK)
			return
		}
		actionID := ""
		if len(parts) == 2 {
			actionID, _ = body["motion_id"].(string)
			actionID = trimSpace(actionID)
		} else {
			actionID = parts[2]
		}
		if actionID == "" {
			s.fail(w, errors.New("action_id required"), http.StatusBadRequest)
			return
		}
		if err := s.saveActionRaw(id, actionID, body); err != nil {
			s.fail(w, err, http.StatusBadRequest)
			return
		}
		s.ok(w, map[string]any{"saved": actionID})
		return
	}
	// update species
	sp, err := mapToSpecies(body)
	if err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	if err := s.Store.SaveSpecies(sp); err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	s.writePresetSchema(sp)
	s.ok(w, map[string]any{"updated": id})
}

// --- DELETE ---------------------------------------------------------------

func (s *Server) speciesDelete(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/species")
	if len(parts) == 0 {
		s.fail(w, errors.New("missing id"), http.StatusBadRequest)
		return
	}
	id := parts[0]
	if len(parts) >= 3 && parts[1] == "actions" {
		if err := s.Store.DeleteAction(id, parts[2]); err != nil {
			s.fail(w, errors.New("action not found: "+id+"/"+parts[2]), http.StatusNotFound)
			return
		}
		s.ok(w, map[string]any{"deleted": parts[2]})
		return
	}
	if err := os.RemoveAll(s.Store.SpeciesPath(id)); err != nil {
		s.fail(w, errors.New("species not found: "+id), http.StatusNotFound)
		return
	}
	_ = os.RemoveAll(filepath.Join(s.Store.SpeciesDir(), id))
	s.ok(w, map[string]any{"deleted": id})
}

// --- helpers --------------------------------------------------------------

// mapToSpecies converts a request body (map) into a typed Species for saving.
func mapToSpecies(m map[string]any) (*store.Species, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var sp store.Species
	if err := json.Unmarshal(b, &sp); err != nil {
		return nil, err
	}
	return &sp, nil
}

// saveDefaultRaw writes default.json with schema/species defaults set
// (mirror of SpeciesService.save_default).
func (s *Server) saveDefaultRaw(speciesID string, body map[string]any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	var d store.SpeciesDefault
	if err := json.Unmarshal(b, &d); err != nil {
		return err
	}
	if d.Species == "" {
		d.Species = speciesID
	}
	if d.Schema == "" {
		d.Schema = "creatureforge_default_v1"
	}
	return s.Store.SaveDefault(&d)
}

// saveActionRaw writes an action JSON preserving any extra fields.
func (s *Server) saveActionRaw(speciesID, actionID string, body map[string]any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	var m store.Motion
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if m.Species == "" {
		m.Species = speciesID
	}
	if m.MotionID == "" {
		m.MotionID = actionID
	}
	return s.Store.SaveAction(&m)
}

// writePresetSchema derives and writes species/<id>/preset_schema.json
// (mirror of SpeciesService.build_preset_schema + _write_preset_schema).
func (s *Server) writePresetSchema(sp *store.Species) {
	old := map[string]any{}
	if prev, err := s.Store.GetPresetSchema(sp.SpeciesID); err == nil && prev != nil {
		if p, ok := prev["params"].(map[string]any); ok {
			old = p
		}
	}
	schema := buildPresetSchema(sp, old)
	_ = s.Store.SavePresetSchema(sp.SpeciesID, schema)
}

// buildPresetSchema mirrors SpeciesService.build_preset_schema.
func buildPresetSchema(sp *store.Species, existing map[string]any) map[string]any {
	seen := map[string]bool{}
	joints3d := make([]string, 0)
	for _, bone := range sp.Bones3D {
		for _, j := range bone {
			if !seen[j] {
				seen[j] = true
				joints3d = append(joints3d, j)
			}
		}
	}
	params := map[string]any{}
	addParam := func(name string, def, min, max, step float64, label string) {
		if name == "" || params[name] != nil {
			return
		}
		base := map[string]any{
			"default": def, "min": min, "max": max, "step": step, "label": label,
		}
		if old, ok := existing[name].(map[string]any); ok {
			for k, v := range old {
				base[k] = v
			}
		}
		params[name] = base
	}
	// body params from param_chains
	for _, chain := range sp.ParamChains {
		addParam(chain.Param, chain.Default, chain.Min, chain.Max, chain.Step, orLabel(chain.Label, chain.Param))
	}
	// coordinate params from skeleton top-level params
	for name, p := range sp.Params {
		if params[name] != nil {
			continue
		}
		addParam(name, p.Default, p.Min, p.Max, p.Step, orLabel(p.Label, name))
	}
	bodyDefault := map[string]any{}
	for k, v := range params {
		if m, ok := v.(map[string]any); ok {
			bodyDefault[k] = m["default"]
		}
	}
	return map[string]any{
		"schema":          "creatureforge_preset_schema_v1",
		"species":         sp.SpeciesID,
		"description":     "预设 schema（随物种骨架自动派生）：预设只需提供 body 体型参数 + actions 动作参数值。",
		"required_fields": []string{"preset_id", "schema", "title", "description", "species", "body", "actions"},
		"joints_3d":       joints3d,
		"params":          params,
		"canvas":          map[string]any{"width": 960, "height": 600, "floor_y": 470},
		"head_radius":     24,
		"body_default":    bodyDefault,
	}
}

func orLabel(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func trimSpace(s string) string {
	out := ""
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			out += string(r)
		}
	}
	return out
}
