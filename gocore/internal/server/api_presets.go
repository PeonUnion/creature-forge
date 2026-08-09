package server

// Presets CRUD + bake (mirror of server._presets_* + presets.py).
import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/PeonUnion/creature-forge/gocore/expr"
	"github.com/PeonUnion/creature-forge/gocore/internal/store"
	"github.com/PeonUnion/creature-forge/gocore/skeleton"
)

// routePresets dispatches /api/presets* by method.
func (s *Server) routePresets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.presetsGet(w, r)
	case http.MethodPost, http.MethodPut:
		s.presetsPost(w, r)
	case http.MethodDelete:
		s.presetsDelete(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) presetsGet(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/presets")
	if len(parts) == 0 {
		items, err := s.listPresets()
		if err != nil {
			s.fail(w, err, http.StatusInternalServerError)
			return
		}
		s.json(w, map[string]any{"presets": items}, http.StatusOK)
		return
	}
	if parts[0] == "new" {
		sp := r.URL.Query().Get("species")
		if sp == "" {
			sp = "human"
		}
		form, err := s.presetNew(sp)
		if err != nil {
			s.fail(w, err, http.StatusBadRequest)
			return
		}
		s.json(w, form, http.StatusOK)
		return
	}
	p, err := s.Store.GetPreset(parts[0])
	if err != nil {
		s.fail(w, errors.New("preset not found: "+parts[0]), http.StatusNotFound)
		return
	}
	s.json(w, s.presetWithSchema(p), http.StatusOK)
}

func (s *Server) presetsPost(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/presets")
	var body map[string]any
	if err := decodeBody(r, &body); err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	// strip schema_info before persisting (derived from species)
	delete(body, "schema_info")
	delete(body, "baked")
	if len(parts) == 0 {
		p, err := mapToPreset(body)
		if err != nil {
			s.fail(w, err, http.StatusBadRequest)
			return
		}
		if p.PresetID == "" {
			s.fail(w, errors.New("preset_id required"), http.StatusBadRequest)
			return
		}
		if p.Species == "" {
			s.fail(w, errors.New("species required"), http.StatusBadRequest)
			return
		}
		if _, err := s.Store.GetPreset(p.PresetID); err == nil {
			s.fail(w, errors.New("preset already exists: "+p.PresetID), http.StatusConflict)
			return
		}
		if p.Schema == "" {
			p.Schema = "creatureforge_preset_v1"
		}
		if err := s.Store.SavePreset(p); err != nil {
			s.fail(w, err, http.StatusBadRequest)
			return
		}
		if err := s.bakePreset(p.PresetID); err != nil {
			s.fail(w, err, http.StatusBadRequest)
			return
		}
		s.ok(w, map[string]any{"created": p.PresetID})
		return
	}
	id := parts[0]
	existing, err := s.Store.GetPreset(id)
	if err != nil {
		s.fail(w, errors.New("preset not found: "+id), http.StatusNotFound)
		return
	}
	// merge: keep existing fields, incoming overrides (mirror of update)
	p, err := mapToPreset(body)
	if err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	merged := mergePreset(existing, p, id)
	if err := s.Store.SavePreset(merged); err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	if err := s.bakePreset(merged.PresetID); err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	s.ok(w, map[string]any{"updated": merged.PresetID})
}

func (s *Server) presetsDelete(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/presets")
	if len(parts) == 0 {
		s.fail(w, errors.New("missing id"), http.StatusBadRequest)
		return
	}
	if err := s.Store.DeletePreset(parts[0]); err != nil {
		s.fail(w, errors.New("preset not found: "+parts[0]), http.StatusNotFound)
		return
	}
	s.ok(w, map[string]any{"deleted": parts[0]})
}

// --- helpers --------------------------------------------------------------

func (s *Server) listPresets() ([]map[string]any, error) {
	ids, err := s.Store.ListPresets()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		p, err := s.Store.GetPreset(id)
		if err != nil {
			continue
		}
		out = append(out, map[string]any{
			"preset_id":   p.PresetID,
			"title":       p.Title,
			"description": p.Description,
			"species":     p.Species,
		})
	}
	return out, nil
}

// presetSchemaInfo builds the schema_info block (mirror of
// PresetService.build_preset_schema).
func (s *Server) presetSchemaInfo(speciesID string) map[string]any {
	bodyParams := map[string]any{}
	if ps, err := s.Store.GetPresetSchema(speciesID); err == nil && ps != nil {
		if p, ok := ps["params"].(map[string]any); ok {
			bodyParams = p
		}
	}
	defaultBody := map[string]any{}
	if d, err := s.Store.GetDefault(speciesID); err == nil {
		if d.Body != nil {
			for k, v := range d.Body {
				defaultBody[k] = v
			}
		}
	}
	if len(defaultBody) == 0 {
		for k := range bodyParams {
			defaultBody[k] = 1.0
		}
	}
	actions := map[string]any{}
	for _, a := range s.actionSummaries(speciesID) {
		id, _ := a["id"].(string)
		title, _ := a["title"].(string)
		actions[id] = map[string]any{"title": title, "params": a["params"]}
	}
	return map[string]any{
		"species":      speciesID,
		"body_params":  bodyParams,
		"default_body": defaultBody,
		"actions":      actions,
	}
}

// presetWithSchema returns preset values + derived schema_info.
func (s *Server) presetWithSchema(p *store.Preset) map[string]any {
	return map[string]any{
		"schema":      p.Schema,
		"preset_id":   p.PresetID,
		"title":       p.Title,
		"description": p.Description,
		"species":     p.Species,
		"body":        p.Body,
		"actions":     p.Actions,
		"baked":       p.Baked,
		"schema_info": s.presetSchemaInfo(p.Species),
	}
}

// presetNew returns a blank form + schema (mirror of new_schema).
func (s *Server) presetNew(speciesID string) (map[string]any, error) {
	info := s.presetSchemaInfo(speciesID)
	body := map[string]any{}
	if db, ok := info["default_body"].(map[string]any); ok {
		body = db
	}
	return map[string]any{
		"schema":      "creatureforge_preset_v1",
		"preset_id":   "",
		"species":     speciesID,
		"title":       "",
		"description": "",
		"body":        body,
		"actions":     map[string]any{},
		"schema_info": info,
	}, nil
}

func mapToPreset(m map[string]any) (*store.Preset, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var p store.Preset
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// mergePreset keeps existing fields, applies incoming overrides (mirror of
// PresetService.update).
func mergePreset(existing, incoming *store.Preset, id string) *store.Preset {
	out := *existing
	if incoming.PresetID != "" {
		out.PresetID = incoming.PresetID
	} else {
		out.PresetID = id
	}
	if incoming.Title != "" {
		out.Title = incoming.Title
	}
	if incoming.Description != "" {
		out.Description = incoming.Description
	}
	if incoming.Body != nil {
		out.Body = incoming.Body
	}
	if incoming.Actions != nil {
		out.Actions = incoming.Actions
	}
	if incoming.Schema != "" {
		out.Schema = incoming.Schema
	} else if out.Schema == "" {
		out.Schema = "creatureforge_preset_v1"
	}
	return &out
}

// bakePreset generates the baked block (mirror of PresetService.bake):
// frozen skel3d from body params + resolved action params.
func (s *Server) bakePreset(presetID string) error {
	p, err := s.Store.GetPreset(presetID)
	if err != nil {
		return err
	}
	if p.Species == "" {
		return errors.New("preset has no species: " + presetID)
	}
	sp, err := s.Store.GetSpecies(p.Species)
	if err != nil {
		return err
	}
	def, err := s.Store.GetDefault(p.Species)
	if err != nil {
		return err
	}
	engDef, err := def.ToEngineDefault()
	if err != nil {
		return err
	}
	sk := skeleton.BuildSkeleton(sp.ToEngineSkeleton(), engDef, p.Body)

	// action param values: resolve preset.actions overrides against motion
	// defaults with skel3d coord params as refs (mirror of motion._resolve_params).
	actionVals := map[string]map[string]float64{}
	for aid, ap := range p.Actions {
		m, err := s.Store.GetAction(p.Species, aid)
		if err != nil {
			actionVals[aid] = ap
			continue
		}
		defaults := map[string]float64{}
		for k, spec := range m.Params {
			defaults[k] = spec.Default
		}
		overrides := make(map[string]*expr.ParamValue, len(ap))
		for k, v := range ap {
			v := v
			overrides[k] = &expr.ParamValue{Num: &v}
		}
		resolved, rerr := expr.ResolveParams(defaults, overrides, sk.Params)
		if rerr != nil {
			actionVals[aid] = ap
			continue
		}
		actionVals[aid] = resolved
	}

	baked := &store.Baked{
		Schema:  "creatureforge_preset_baked_v1",
		Species: p.Species,
		Skel3D: &store.BakedSkel3D{
			SpeciesID:   sp.SpeciesID,
			Joints:      sk.Joints,
			Bones:       sk.Bones,
			FkTree:      sk.FkTree,
			Center:      sk.Center,
			FloorY:      sk.FloorY,
			HeadRadius:  sk.HeadRadius,
			Params:      sk.Params,
			RigidChains: nil,
		},
		Body:    p.Body,
		Actions: actionVals,
	}
	p.Baked = baked
	return s.Store.SavePreset(p)
}
