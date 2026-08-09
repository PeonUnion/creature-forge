package server

// Integration tests exercise the HTTP API against the real repo data
// (data/) via httptest.
import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/PeonUnion/creature-forge/gocore/internal/config"
	"github.com/PeonUnion/creature-forge/gocore/internal/logging"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := config.Default()
	cfg.Data.Root = "../../../data"
	cfg.Server.Dev = true
	log := logging.New(logging.Config{Level: logging.LevelError, Format: logging.FormatConsole})
	srv := New(cfg, log)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func getJSON(t *testing.T, url string, v any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
	return resp.StatusCode
}

func postJSON(t *testing.T, url string, body any, v any) int {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
	return resp.StatusCode
}

func TestSpeciesListAndDetail(t *testing.T) {
	ts := newTestServer(t)
	var list struct {
		Species []map[string]any `json:"species"`
	}
	if code := getJSON(t, ts.URL+"/api/species", &list); code != 200 {
		t.Fatalf("species list: %d", code)
	}
	if len(list.Species) == 0 {
		t.Fatal("no species")
	}
	found := false
	for _, sp := range list.Species {
		if sp["id"] == "human" {
			found = true
			if sp["joint_count"].(float64) != 36 {
				t.Errorf("human joint_count: %v", sp["joint_count"])
			}
		}
	}
	if !found {
		t.Fatal("human not in list")
	}
	var detail map[string]any
	if code := getJSON(t, ts.URL+"/api/species/human", &detail); code != 200 {
		t.Fatalf("species detail: %d", code)
	}
	if acts, ok := detail["actions"].([]any); !ok || len(acts) == 0 {
		t.Fatalf("species actions missing: %v", detail["actions"])
	}
}

func TestMotions3dList(t *testing.T) {
	ts := newTestServer(t)
	var out struct {
		Motions3d []map[string]any `json:"motions3d"`
	}
	if code := getJSON(t, ts.URL+"/api/motions3d", &out); code != 200 {
		t.Fatalf("motions3d: %d", code)
	}
	ids := map[string]bool{}
	for _, m := range out.Motions3d {
		ids[m["id"].(string)] = true
	}
	for _, want := range []string{"walk3d", "run3d", "idle3d"} {
		if !ids[want] {
			t.Errorf("missing motion %s", want)
		}
	}
}

func TestSkeleton3dData(t *testing.T) {
	ts := newTestServer(t)
	var out struct {
		OK     bool                 `json:"ok"`
		Joints map[string][]float64 `json:"joints"`
		Bones  [][]string           `json:"bones"`
		Center []float64            `json:"center"`
	}
	if code := getJSON(t, ts.URL+"/api/skeleton3d/human?data=1", &out); code != 200 {
		t.Fatalf("skeleton3d: %d", code)
	}
	if !out.OK || len(out.Joints) != 36 || len(out.Bones) == 0 {
		t.Fatalf("bad skeleton data: ok=%v joints=%d", out.OK, len(out.Joints))
	}
	// body override should change coordinates
	var over map[string]any
	getJSON(t, ts.URL+"/api/skeleton3d/human?data=1&body=%7B%22head_scale%22%3A1.5%7D", &over)
}

func TestMotion3dData(t *testing.T) {
	ts := newTestServer(t)
	var out struct {
		OK         bool                   `json:"ok"`
		Frames     []map[string][]float64 `json:"frames"`
		FrameCount int                    `json:"frame_count"`
		FPS        int                    `json:"fps"`
	}
	if code := getJSON(t, ts.URL+"/api/motion3d/walk3d?data=1", &out); code != 200 {
		t.Fatalf("motion3d: %d", code)
	}
	if !out.OK || len(out.Frames) == 0 || out.FrameCount != len(out.Frames) {
		t.Fatalf("bad motion data: ok=%v frames=%d count=%d", out.OK, len(out.Frames), out.FrameCount)
	}
	if len(out.Frames[0]) != 36 {
		t.Errorf("frame joints: %d", len(out.Frames[0]))
	}
}

func TestSkin3dData(t *testing.T) {
	ts := newTestServer(t)
	var out struct {
		OK   bool `json:"ok"`
		Mesh struct {
			VertexCount int       `json:"vertex_count"`
			Vertices    []float64 `json:"vertices"`
		} `json:"mesh"`
		Frames     []any `json:"frames"`
		FrameCount int   `json:"frame_count"`
	}
	if code := getJSON(t, ts.URL+"/api/skin3d/walk3d?preset=model_male", &out); code != 200 {
		t.Fatalf("skin3d: %d", code)
	}
	if !out.OK || out.Mesh.VertexCount != 4450 || len(out.Frames) == 0 {
		t.Fatalf("bad skin data: ok=%v verts=%d frames=%d", out.OK, out.Mesh.VertexCount, len(out.Frames))
	}
}

func TestPresetCRUDViaAPI(t *testing.T) {
	ts := newTestServer(t)
	body := map[string]any{
		"preset_id": "api_test_preset", "species": "human", "title": "API测试",
		"body":    map[string]float64{"head_scale": 1.2},
		"actions": map[string]map[string]float64{"walk3d": {"intensity": 1.1}},
	}
	var created map[string]any
	if code := postJSON(t, ts.URL+"/api/presets", body, &created); code != 200 {
		t.Fatalf("create preset: %d", code)
	}
	defer func() {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/presets/api_test_preset", nil)
		_, _ = http.DefaultClient.Do(req)
	}()
	var got map[string]any
	if code := getJSON(t, ts.URL+"/api/presets/api_test_preset", &got); code != 200 {
		t.Fatalf("get preset: %d", code)
	}
	baked, _ := got["baked"].(map[string]any)
	skel3d, _ := baked["skel3d"].(map[string]any)
	if joints, ok := skel3d["joints"].(map[string]any); !ok || len(joints) != 36 {
		t.Fatalf("baked joints missing: %v", skel3d)
	}
	// new preset form
	var form map[string]any
	if code := getJSON(t, ts.URL+"/api/presets/new?species=human", &form); code != 200 {
		t.Fatalf("preset new: %d", code)
	}
	if _, ok := form["schema_info"].(map[string]any); !ok {
		t.Fatal("new preset missing schema_info")
	}
}

func TestMotionLive(t *testing.T) {
	ts := newTestServer(t)
	reqBody := map[string]any{
		"species": "human", "index": 0,
		"action": map[string]any{"motion_id": "t", "frame_count": 8,
			"fk3d": map[string]any{"root": "pelvis", "rotations3d": map[string]any{}}},
	}
	var out struct {
		OK     bool                 `json:"ok"`
		Joints map[string][]float64 `json:"joints"`
		Index  int                  `json:"index"`
	}
	if code := postJSON(t, ts.URL+"/api/motion3d/live", reqBody, &out); code != 200 {
		t.Fatalf("motion3d live: %d", code)
	}
	if !out.OK || len(out.Joints) != 36 {
		t.Fatalf("bad live: ok=%v joints=%d", out.OK, len(out.Joints))
	}
}

func TestTemplates(t *testing.T) {
	ts := newTestServer(t)
	var out struct {
		Templates []map[string]any `json:"templates"`
	}
	if code := getJSON(t, ts.URL+"/api/templates", &out); code != 200 {
		t.Fatalf("templates: %d", code)
	}
	ids := map[string]bool{}
	for _, tp := range out.Templates {
		ids[tp["morph_id"].(string)] = true
	}
	if !ids["custom"] || !ids["humanoid"] {
		t.Fatalf("templates missing: %v", ids)
	}
}

// TestEmbedStatic verifies the embedded SPA is served (index.html, hashed
// assets, SPA history fallback) and that missing real assets return 404.
func TestEmbedStatic(t *testing.T) {
	ts := newTestServer(t)
	// GET / → index.html
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !bytes.Contains(body, []byte("<!doctype html")) {
		t.Fatalf("GET /: %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("GET / content-type: %s", ct)
	}
	// extract a hashed JS asset reference from index.html and fetch it
	re := regexp.MustCompile(`src="\.?/?([^"]+\.js)"`)
	m := re.FindStringSubmatch(string(body))
	if len(m) < 2 {
		t.Fatalf("no js asset in index.html")
	}
	asset := strings.TrimPrefix(m[1], "./")
	resp2, err := http.Get(ts.URL + "/" + asset)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("GET %s: %d", asset, resp2.StatusCode)
	}
	if ct := resp2.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("asset content-type: %s", ct)
	}
	// SPA history fallback → index.html
	resp3, err := http.Get(ts.URL + "/presets/model_male")
	if err != nil {
		t.Fatal(err)
	}
	body3, _ := io.ReadAll(resp3.Body)
	resp3.Body.Close()
	if resp3.StatusCode != 200 || !bytes.Contains(body3, []byte("<!doctype html")) {
		t.Fatalf("SPA fallback: %d", resp3.StatusCode)
	}
	// missing real asset → 404
	resp4, err := http.Get(ts.URL + "/assets/definitely-missing-12345.js")
	if err != nil {
		t.Fatal(err)
	}
	resp4.Body.Close()
	if resp4.StatusCode != 404 {
		t.Fatalf("missing asset: %d", resp4.StatusCode)
	}
}
