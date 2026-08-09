package server

// Skins CRUD + parts.
import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/PeonUnion/creature-forge/gocore/internal/store"
)

const skinSchema = "creatureforge_skin_v1"

// routeSkins dispatches /api/skins* by method.
func (s *Server) routeSkins(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.skinsGet(w, r)
	case http.MethodPost, http.MethodPut:
		s.skinsPost(w, r)
	case http.MethodDelete:
		s.skinsDelete(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) skinsGet(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/skins")
	if len(parts) == 0 {
		items, err := s.listSkins()
		if err != nil {
			s.fail(w, err, http.StatusInternalServerError)
			return
		}
		s.json(w, map[string]any{"skins": items}, http.StatusOK)
		return
	}
	if parts[0] == "new" {
		pid := r.URL.Query().Get("preset")
		if pid == "" {
			s.fail(w, errors.New("preset required"), http.StatusBadRequest)
			return
		}
		form, err := s.skinNew(pid)
		if err != nil {
			s.fail(w, err, http.StatusBadRequest)
			return
		}
		s.json(w, form, http.StatusOK)
		return
	}
	sk, err := s.Store.GetSkin(parts[0])
	if err != nil {
		s.fail(w, errors.New("skin not found: "+parts[0]), http.StatusNotFound)
		return
	}
	s.json(w, s.skinWithSchema(sk), http.StatusOK)
}

func (s *Server) skinsPost(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/skins")
	var body map[string]any
	if err := decodeBody(r, &body); err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	// part routes
	if len(parts) >= 2 && parts[1] == "parts" {
		skinID, partID := parts[0], ""
		if len(parts) > 2 {
			partID = parts[2]
		}
		// upload mesh
		if partID != "" && len(parts) == 4 && parts[3] == "mesh" {
			data, err := b64decode(body["data_b64"])
			if err != nil {
				s.fail(w, err, http.StatusBadRequest)
				return
			}
			parsed, err := s.skinPartUploadMesh(skinID, partID, str(body["filename"]), data)
			if err != nil {
				s.fail(w, err, http.StatusBadRequest)
				return
			}
			s.ok(w, parsed)
			return
		}
		// upload texture
		if partID != "" && len(parts) == 4 && parts[3] == "texture" {
			data, err := b64decode(body["data_b64"])
			if err != nil {
				s.fail(w, err, http.StatusBadRequest)
				return
			}
			ref, err := s.skinPartUploadTexture(skinID, partID, str(body["field"]), str(body["filename"]), data)
			if err != nil {
				s.fail(w, err, http.StatusBadRequest)
				return
			}
			s.ok(w, map[string]any{"ref": ref})
			return
		}
		// add part (POST) / update part (PUT)
		if partID == "" {
			pid, err := s.skinPartAdd(skinID, body)
			if err != nil {
				s.fail(w, err, statusFor(err))
				return
			}
			s.ok(w, map[string]any{"part": pid})
			return
		}
		if err := s.skinPartUpdate(skinID, partID, body); err != nil {
			s.fail(w, errors.New("part not found: "+partID), http.StatusNotFound)
			return
		}
		s.ok(w, map[string]any{"updated": partID})
		return
	}
	// skin create / update
	delete(body, "schema_info")
	if len(parts) == 0 {
		sk, err := mapToSkin(body)
		if err != nil {
			s.fail(w, err, http.StatusBadRequest)
			return
		}
		if sk.SkinID == "" {
			s.fail(w, errors.New("skin_id required"), http.StatusBadRequest)
			return
		}
		if sk.Preset == "" {
			s.fail(w, errors.New("preset required"), http.StatusBadRequest)
			return
		}
		if _, err := s.Store.GetSkin(sk.SkinID); err == nil {
			s.fail(w, errors.New("skin already exists: "+sk.SkinID), http.StatusConflict)
			return
		}
		if sk.Schema == "" {
			sk.Schema = skinSchema
		}
		if sk.Species == "" {
			sk.Species = s.speciesOfPreset(sk.Preset)
		}
		if err := s.Store.SaveSkin(sk); err != nil {
			s.fail(w, err, http.StatusBadRequest)
			return
		}
		s.ok(w, map[string]any{"created": sk.SkinID})
		return
	}
	id := parts[0]
	if _, err := s.Store.GetSkin(id); err != nil {
		s.fail(w, errors.New("skin not found: "+id), http.StatusNotFound)
		return
	}
	sk, err := mapToSkin(body)
	if err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	if sk.Schema == "" {
		sk.Schema = skinSchema
	}
	if sk.SkinID == "" {
		sk.SkinID = id
	}
	if sk.Preset != "" && sk.Species == "" {
		sk.Species = s.speciesOfPreset(sk.Preset)
	}
	if err := s.Store.SaveSkin(sk); err != nil {
		s.fail(w, err, http.StatusBadRequest)
		return
	}
	s.ok(w, map[string]any{"updated": sk.SkinID})
}

func (s *Server) skinsDelete(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/skins")
	if len(parts) == 0 {
		s.fail(w, errors.New("missing id"), http.StatusBadRequest)
		return
	}
	if len(parts) >= 3 && parts[1] == "parts" {
		if err := s.skinPartDelete(parts[0], parts[2]); err != nil {
			s.fail(w, errors.New("part not found: "+parts[2]), http.StatusNotFound)
			return
		}
		s.ok(w, map[string]any{"deleted": parts[2]})
		return
	}
	if err := s.Store.DeleteSkin(parts[0]); err != nil {
		s.fail(w, errors.New("skin not found: "+parts[0]), http.StatusNotFound)
		return
	}
	_ = os.RemoveAll(filepath.Join(s.Store.SkinsDir(), "assets", parts[0]))
	s.ok(w, map[string]any{"deleted": parts[0]})
}

// --- helpers --------------------------------------------------------------

func (s *Server) listSkins() ([]map[string]any, error) {
	ids, err := s.Store.ListSkins()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		sk, err := s.Store.GetSkin(id)
		if err != nil {
			continue
		}
		out = append(out, map[string]any{
			"skin_id":     sk.SkinID,
			"title":       sk.Title,
			"description": sk.Description,
			"preset":      sk.Preset,
			"species":     sk.Species,
		})
	}
	return out, nil
}

func (s *Server) speciesOfPreset(presetID string) string {
	p, err := s.Store.GetPreset(presetID)
	if err != nil {
		return ""
	}
	return p.Species
}

// buildSkinSchema reads species/<id>/skin/skin_params.json to derive the
// skin schema (params / materials / body_scale).
func (s *Server) buildSkinSchema(presetID string) map[string]any {
	params := map[string]any{}
	materials := map[string]any{"albedo": "#c9a58c", "roughness": 0.6, "metallic": 0.0}
	var bodyScale any
	speciesID := s.speciesOfPreset(presetID)
	if speciesID != "" {
		path := filepath.Join(s.Store.SpeciesDir(), speciesID, "skin", "skin_params.json")
		if b, err := os.ReadFile(path); err == nil {
			var p struct {
				Params    map[string]any `json:"params"`
				Materials map[string]any `json:"materials"`
				BodyScale any            `json:"body_scale"`
			}
			if json.Unmarshal(b, &p) == nil {
				if p.Params != nil {
					params = p.Params
				}
				for k, v := range p.Materials {
					materials[k] = v
				}
				bodyScale = p.BodyScale
			}
		}
	}
	return map[string]any{
		"preset": presetID, "species": speciesID, "params": params,
		"materials": materials, "body_scale": bodyScale,
	}
}

// skinWithSchema returns skin values + derived schema_info.
func (s *Server) skinWithSchema(sk *store.Skin) map[string]any {
	return map[string]any{
		"schema": sk.Schema, "skin_id": sk.SkinID, "title": sk.Title,
		"description": sk.Description, "preset": sk.Preset, "species": sk.Species,
		"materials": sk.Materials, "params": sk.Params, "parts": sk.Parts,
		"schema_info": s.buildSkinSchema(sk.Preset),
	}
}

// skinNew returns a blank form plus its schema.
func (s *Server) skinNew(presetID string) (map[string]any, error) {
	schema := s.buildSkinSchema(presetID)
	params := map[string]any{}
	if ps, ok := schema["params"].(map[string]any); ok {
		for k, v := range ps {
			if m, ok := v.(map[string]any); ok {
				params[k] = m["default"]
			}
		}
	}
	speciesID, _ := schema["species"].(string)
	materials, _ := schema["materials"].(map[string]any)
	return map[string]any{
		"schema": skinSchema, "skin_id": "", "preset": presetID,
		"species": speciesID, "title": "", "description": "",
		"materials": materials, "params": params, "schema_info": schema,
	}, nil
}

func mapToSkin(m map[string]any) (*store.Skin, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var sk store.Skin
	if err := json.Unmarshal(b, &sk); err != nil {
		return nil, err
	}
	return &sk, nil
}

// --- parts ----------------------------------------------------------------

func (s *Server) skinPartAdd(skinID string, part map[string]any) (string, error) {
	sk, err := s.Store.GetSkin(skinID)
	if err != nil {
		return "", err
	}
	pid := trimSpace(str(part["part_id"]))
	if pid == "" {
		return "", errors.New("part_id required")
	}
	for _, p := range sk.Parts {
		if p.PartID == pid {
			return "", errors.New("part already exists: " + pid)
		}
	}
	np := store.SkinPart{
		PartID: pid, Kind: "bone", Bone: "",
		Transform: map[string][]float64{"position": {0, 0, 0}, "rotation": {0, 0, 0}, "scale": {1, 1, 1}},
		Textures:  map[string]string{}, Materials: map[string]any{},
	}
	if v, ok := part["title"].(string); ok {
		np.Title = v
	}
	if v, ok := part["kind"].(string); ok && v != "" {
		np.Kind = v
	}
	if v, ok := part["bone"].(string); ok {
		np.Bone = v
	}
	if v, ok := part["transform"].(map[string]any); ok {
		if b, err := json.Marshal(v); err == nil {
			_ = json.Unmarshal(b, &np.Transform)
		}
	}
	if v, ok := part["mesh_file"].(string); ok {
		np.MeshFile = &v
	}
	if v, ok := part["mesh"].(map[string]any); ok {
		np.Mesh = v
	}
	if v, ok := part["textures"].(map[string]any); ok {
		for k, val := range v {
			np.Textures[k] = str(val)
		}
	}
	if v, ok := part["materials"].(map[string]any); ok {
		np.Materials = v
	}
	if v, ok := part["weights"].(map[string]any); ok {
		np.Weights = v
	}
	sk.Parts = append(sk.Parts, np)
	if err := s.Store.SaveSkin(sk); err != nil {
		return "", err
	}
	return pid, nil
}

func (s *Server) skinPartUpdate(skinID, partID string, patch map[string]any) error {
	sk, err := s.Store.GetSkin(skinID)
	if err != nil {
		return err
	}
	for i, p := range sk.Parts {
		if p.PartID == partID {
			b, _ := json.Marshal(patch)
			var updated store.SkinPart
			if err := json.Unmarshal(b, &updated); err != nil {
				return err
			}
			updated.PartID = partID
			sk.Parts[i] = updated
			return s.Store.SaveSkin(sk)
		}
	}
	return errors.New("part not found: " + partID)
}

func (s *Server) skinPartDelete(skinID, partID string) error {
	sk, err := s.Store.GetSkin(skinID)
	if err != nil {
		return err
	}
	keep := sk.Parts[:0]
	found := false
	for _, p := range sk.Parts {
		if p.PartID == partID {
			found = true
			continue
		}
		keep = append(keep, p)
	}
	if !found {
		return errors.New("part not found: " + partID)
	}
	sk.Parts = keep
	if err := s.Store.SaveSkin(sk); err != nil {
		return err
	}
	_ = os.RemoveAll(filepath.Join(s.Store.SkinsDir(), "assets", skinID, partID))
	return nil
}

// skinPartUploadMesh writes the mesh file + updates the part (full mesh
// parsing lives with the glTF module; here we store the raw file + refs).
func (s *Server) skinPartUploadMesh(skinID, partID, filename string, data []byte) (map[string]any, error) {
	sk, err := s.Store.GetSkin(skinID)
	if err != nil {
		return nil, err
	}
	if !partExists(sk, partID) {
		return nil, errors.New("part not found: " + partID)
	}
	dir := filepath.Join(s.Store.SkinsDir(), "assets", skinID, partID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".glb"
	}
	meshName := "mesh" + ext
	if err := os.WriteFile(filepath.Join(dir, meshName), data, 0o644); err != nil {
		return nil, err
	}
	ref := partID + "/" + meshName
	if err := s.skinPartUpdate(skinID, partID, map[string]any{
		"mesh_file": ref, "mesh": nil, "materials": map[string]any{}, "textures": map[string]any{},
	}); err != nil {
		return nil, err
	}
	return map[string]any{"mesh_file": ref, "mesh": nil, "materials": map[string]any{}, "textures": map[string]any{}}, nil
}

// skinPartUploadTexture writes the texture file + updates part.textures.
func (s *Server) skinPartUploadTexture(skinID, partID, field, filename string, data []byte) (string, error) {
	sk, err := s.Store.GetSkin(skinID)
	if err != nil {
		return "", err
	}
	if !partExists(sk, partID) {
		return "", errors.New("part not found: " + partID)
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if ext == "" {
		ext = "png"
	}
	if ext != "png" && ext != "jpg" && ext != "jpeg" && ext != "webp" {
		return "", errors.New("不支持的贴图格式: " + filename + "（支持 png/jpg/webp）")
	}
	if field == "" {
		field = "albedo"
	}
	dir := filepath.Join(s.Store.SkinsDir(), "assets", skinID, partID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tfile := field + "." + ext
	if err := os.WriteFile(filepath.Join(dir, tfile), data, 0o644); err != nil {
		return "", err
	}
	ref := "skin://" + partID + "/" + tfile
	for i, p := range sk.Parts {
		if p.PartID == partID {
			if p.Textures == nil {
				p.Textures = map[string]string{}
			}
			p.Textures[field] = ref
			sk.Parts[i] = p
			if err := s.Store.SaveSkin(sk); err != nil {
				return "", err
			}
			return ref, nil
		}
	}
	return "", errors.New("part not found: " + partID)
}

func partExists(sk *store.Skin, partID string) bool {
	for _, p := range sk.Parts {
		if p.PartID == partID {
			return true
		}
	}
	return false
}

// --- utils ----------------------------------------------------------------

func b64decode(v any) ([]byte, error) {
	s := strings.TrimSpace(str(v))
	if i := strings.Index(s, ","); i >= 0 && strings.HasPrefix(s, "data:") {
		s = s[i+1:]
	}
	return base64.StdEncoding.DecodeString(s)
}

func str(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func statusFor(err error) int {
	if strings.Contains(err.Error(), "already exists") {
		return http.StatusConflict
	}
	return http.StatusBadRequest
}
