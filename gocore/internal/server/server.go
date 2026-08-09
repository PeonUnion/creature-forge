package server

// Package server is the Go mirror of creatureforge/server.py + api.py: it
// exposes the full HTTP API (species / presets / skins / actions / 3D data)
// over the store + engine packages, and serves the Vue SPA in production.
//
// JSON contracts match the Python server exactly (front-end unchanged).

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/PeonUnion/creature-forge/gocore/internal/config"
	"github.com/PeonUnion/creature-forge/gocore/internal/logging"
	"github.com/PeonUnion/creature-forge/gocore/internal/store"
)

// Server is the assembled HTTP server (mirror of build_server).
type Server struct {
	Store   *store.Store
	Dev     bool
	WebDist string // production SPA root; empty in dev
	Log     *logging.Logger
}

// New assembles a Server from config (mirror of build_server).
func New(cfg *config.Config, log *logging.Logger) *Server {
	return &Server{
		Store:   store.New(cfg.Data.Root),
		Dev:     cfg.Server.Dev,
		WebDist: "web/dist",
		Log:     log,
	}
}

// ---------------------------------------------------------------------------
// routing
// ---------------------------------------------------------------------------

// Handler returns the root http.Handler with the API + static routes.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Dev {
			// dev mode: permissive CORS for the Vite dev server
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", "*")
			h.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		p := r.URL.Path
		switch {
		case strings.HasPrefix(p, "/api/species"):
			s.routeSpecies(w, r)
		case strings.HasPrefix(p, "/api/preset3d/"):
			s.routePreset3d(w, r)
		case strings.HasPrefix(p, "/api/presets"):
			s.routePresets(w, r)
		case p == "/api/motions3d":
			s.routeMotions3dList(w, r)
		case strings.HasPrefix(p, "/api/motion3d/"):
			s.routeMotion3d(w, r)
		case strings.HasPrefix(p, "/api/skin3d/"):
			s.routeSkin3d(w, r)
		case strings.HasPrefix(p, "/api/skins"):
			s.routeSkins(w, r)
		case strings.HasPrefix(p, "/api/templates"):
			s.routeTemplates(w, r)
		case strings.HasPrefix(p, "/api/wizard/"):
			s.routeWizard(w, r)
		case strings.HasPrefix(p, "/api/skeleton3d/"):
			s.routeSkeleton3d(w, r)
		case strings.HasPrefix(p, "/api/"):
			s.json(w, map[string]any{"ok": false, "error": "api not found"}, http.StatusNotFound)
		default:
			s.serveStatic(w, r)
		}
	})
}

// pathParts splits a path after prefix into unescaped non-empty segments
// (mirror of server._path_parts).
func pathParts(p, prefix string) []string {
	rest := strings.TrimSuffix(strings.TrimPrefix(p, prefix), "/")
	if rest == "" {
		return nil
	}
	raw := strings.Split(rest, "/")
	out := raw[:0]
	for _, seg := range raw {
		if seg == "" {
			continue
		}
		out = append(out, seg)
	}
	return out
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// json writes a JSON response (mirror of server._json).
func (s *Server) json(w http.ResponseWriter, data any, status int) {
	body, err := json.Marshal(data)
	if err != nil {
		body = []byte(`{"ok":false,"error":"marshal failed"}`)
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// ok writes {ok:true,...} merged with extra fields.
func (s *Server) ok(w http.ResponseWriter, extra map[string]any) {
	m := map[string]any{"ok": true}
	for k, v := range extra {
		m[k] = v
	}
	s.json(w, m, http.StatusOK)
}

// fail writes {ok:false,error} with a status (mirror of error mapping).
func (s *Server) fail(w http.ResponseWriter, err error, status int) {
	s.json(w, map[string]any{"ok": false, "error": err.Error()}, status)
}

// decodeBody reads and parses the JSON request body (mirror of _read_body).
func decodeBody(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

// serveStatic serves the Vue SPA from WebDist with index.html fallback.
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if s.WebDist == "" {
		s.json(w, map[string]any{"ok": false, "error": "static not configured (dev mode)"}, http.StatusNotFound)
		return
	}
	clean := filepath.Clean(r.URL.Path)
	if clean == "." || strings.HasPrefix(clean, "..") {
		clean = "index.html"
	}
	fp := filepath.Join(s.WebDist, clean)
	// SPA fallback: unknown non-file routes → index.html
	if r.URL.Path != "/" && filepath.Ext(fp) == "" {
		fp = filepath.Join(s.WebDist, "index.html")
	}
	http.ServeFile(w, r, fp)
}
