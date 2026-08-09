package store

// Store is a thin CRUD layer over the external JSON data directory.
// Everything is read/written as-is (indent 2, trailing newline) — data stays
// the source of truth; this package only locates, loads, lists and saves.
import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNotFound is returned when a requested document does not exist.
var ErrNotFound = errors.New("not found")

// Store locates documents under a data root (contains species/ presets/ skins/).
type Store struct {
	DataDir string
}

// New returns a Store rooted at dataDir.
func New(dataDir string) *Store { return &Store{DataDir: dataDir} }

// --- paths ----------------------------------------------------------------

func (s *Store) SpeciesDir() string { return filepath.Join(s.DataDir, "species") }
func (s *Store) SpeciesPath(id string) string {
	return filepath.Join(s.SpeciesDir(), id, "skeleton.json")
}
func (s *Store) DefaultPath(id string) string {
	return filepath.Join(s.SpeciesDir(), id, "default.json")
}
func (s *Store) PresetSchemaPath(id string) string {
	return filepath.Join(s.SpeciesDir(), id, "preset_schema.json")
}
func (s *Store) ActionsDir(id string) string {
	return filepath.Join(s.SpeciesDir(), id, "actions3d")
}
func (s *Store) ActionPath(id, motion string) string {
	return filepath.Join(s.ActionsDir(id), motion+".json")
}
func (s *Store) PresetsDir() string { return filepath.Join(s.DataDir, "presets") }
func (s *Store) PresetPath(id string) string {
	return filepath.Join(s.PresetsDir(), id+".json")
}
func (s *Store) SkinsDir() string { return filepath.Join(s.DataDir, "skins") }
func (s *Store) SkinPath(id string) string {
	return filepath.Join(s.SkinsDir(), id+".json")
}

// --- io helpers ------------------------------------------------------------

func readJSON[T any](path string) (T, error) {
	var v T
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return v, ErrNotFound
		}
		return v, err
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return v, fmt.Errorf("parse %s: %w", path, err)
	}
	return v, nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

// listIDs returns base names (without extension) of *.json files in dir.
func listIDs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		out = append(out, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(out)
	return out, nil
}

func listSubDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// --- species ---------------------------------------------------------------

// ListSpecies returns species IDs (subdirectories of species/ that contain a
// skeleton.json — draft folders without one are excluded).
func (s *Store) ListSpecies() ([]string, error) {
	dirs, err := listSubDirs(s.SpeciesDir())
	if err != nil {
		return nil, err
	}
	out := dirs[:0]
	for _, d := range dirs {
		if _, err := os.Stat(filepath.Join(s.SpeciesDir(), d, "skeleton.json")); err == nil {
			out = append(out, d)
		}
	}
	return out, nil
}

// GetSpecies loads species/<id>/skeleton.json.
func (s *Store) GetSpecies(id string) (*Species, error) {
	return readJSON[*Species](s.SpeciesPath(id))
}

// GetDefault loads species/<id>/default.json.
func (s *Store) GetDefault(id string) (*SpeciesDefault, error) {
	return readJSON[*SpeciesDefault](s.DefaultPath(id))
}

// SaveSpecies writes species/<id>/skeleton.json.
func (s *Store) SaveSpecies(sp *Species) error {
	return writeJSON(s.SpeciesPath(sp.SpeciesID), sp)
}

// SaveDefault writes species/<id>/default.json.
func (s *Store) SaveDefault(d *SpeciesDefault) error {
	return writeJSON(s.DefaultPath(d.Species), d)
}

// GetPresetSchema loads species/<id>/preset_schema.json (nil if absent).
func (s *Store) GetPresetSchema(id string) (map[string]any, error) {
	b, err := os.ReadFile(s.PresetSchemaPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.PresetSchemaPath(id), err)
	}
	return v, nil
}

// SavePresetSchema writes species/<id>/preset_schema.json.
func (s *Store) SavePresetSchema(id string, v map[string]any) error {
	return writeJSON(s.PresetSchemaPath(id), v)
}

// --- actions ---------------------------------------------------------------

// ListActions returns action IDs of a species (actions3d/*.json).
func (s *Store) ListActions(speciesID string) ([]string, error) {
	return listIDs(s.ActionsDir(speciesID))
}

// GetAction loads species/<id>/actions3d/<motion>.json.
func (s *Store) GetAction(speciesID, motion string) (*Motion, error) {
	return readJSON[*Motion](s.ActionPath(speciesID, motion))
}

// SaveAction writes species/<id>/actions3d/<motion>.json.
func (s *Store) SaveAction(a *Motion) error {
	return writeJSON(s.ActionPath(a.Species, a.MotionID), a)
}

// DeleteAction removes species/<id>/actions3d/<motion>.json.
func (s *Store) DeleteAction(speciesID, motion string) error {
	path := s.ActionPath(speciesID, motion)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrNotFound
	}
	return os.Remove(path)
}

// --- presets ---------------------------------------------------------------

// ListPresets returns preset IDs.
func (s *Store) ListPresets() ([]string, error) { return listIDs(s.PresetsDir()) }

// GetPreset loads presets/<id>.json.
func (s *Store) GetPreset(id string) (*Preset, error) {
	return readJSON[*Preset](s.PresetPath(id))
}

// SavePreset writes presets/<id>.json.
func (s *Store) SavePreset(p *Preset) error {
	return writeJSON(s.PresetPath(p.PresetID), p)
}

// DeletePreset removes presets/<id>.json.
func (s *Store) DeletePreset(id string) error {
	path := s.PresetPath(id)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrNotFound
	}
	return os.Remove(path)
}

// --- skins -----------------------------------------------------------------

// ListSkins returns skin IDs.
func (s *Store) ListSkins() ([]string, error) { return listIDs(s.SkinsDir()) }

// GetSkin loads skins/<id>.json.
func (s *Store) GetSkin(id string) (*Skin, error) {
	return readJSON[*Skin](s.SkinPath(id))
}

// SaveSkin writes skins/<id>.json.
func (s *Store) SaveSkin(sk *Skin) error {
	return writeJSON(s.SkinPath(sk.SkinID), sk)
}

// DeleteSkin removes skins/<id>.json.
func (s *Store) DeleteSkin(id string) error {
	path := s.SkinPath(id)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrNotFound
	}
	return os.Remove(path)
}
