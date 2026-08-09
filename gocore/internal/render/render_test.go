package render

// Tests render real repo skeleton/motion data and verify decodable, correctly
// sized PNG/GIF/sprite output.
import (
	"bytes"
	"encoding/base64"
	"image"
	"image/gif"
	_ "image/png"
	"testing"

	"github.com/PeonUnion/creature-forge/gocore/internal/store"
	"github.com/PeonUnion/creature-forge/gocore/skeleton"
)

const dataRoot = "../../../data"

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

func mustB64(t *testing.T, dataURL string) []byte {
	t.Helper()
	raw := dataURL[len("data:image/png;base64,"):]
	if len(dataURL) >= 22 && dataURL[:10] == "data:image" {
		for i := 0; i < len(dataURL); i++ {
			if dataURL[i] == ',' {
				raw = dataURL[i+1:]
				break
			}
		}
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		t.Fatalf("b64: %v", err)
	}
	return b
}

func buildHuman(t *testing.T) *skeleton.Skeleton {
	t.Helper()
	s := store.New(dataRoot)
	sp, err := s.GetSpecies("human")
	if err != nil {
		t.Fatal(err)
	}
	def, err := s.GetDefault("human")
	if err != nil {
		t.Fatal(err)
	}
	engDef, err := def.ToEngineDefault()
	if err != nil {
		t.Fatal(err)
	}
	return skeleton.BuildSkeleton(sp.ToEngineSkeleton(), engDef, nil)
}

func TestRenderSkeletonPNG(t *testing.T) {
	sk := buildHuman(t)
	view := SkeletonView{Joints: sk.Joints, Bones: sk.Bones, Center: sk.Center, HeadRadius: sk.HeadRadius}
	groundY := 0.0
	for _, j := range sk.Joints {
		if j[1] > groundY {
			groundY = j[1]
		}
	}
	gridRad := FitDistance(sk.Joints, sk.Center, FitFill)
	img := RenderPose(view.Joints, view.Bones, 45, 12, 0, view.Center, 0, 0, true, groundY, gridRad, view.HeadRadius)
	b, err := PNGBytes(img)
	if err != nil {
		t.Fatalf("PNGBytes: %v", err)
	}
	dec, _, err := image.DecodeConfig(bytesReader(b))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if dec.Width != CanvasW || dec.Height != CanvasH {
		t.Fatalf("size: %dx%d want %dx%d", dec.Width, dec.Height, CanvasW, CanvasH)
	}
	url, err := PNGDataURL(img)
	if err != nil || len(url) < 100 || url[:22] != "data:image/png;base64," {
		t.Fatalf("bad data url")
	}
}

func TestRenderMotionFrameAndGIF(t *testing.T) {
	sk := buildHuman(t)
	s := store.New(dataRoot)
	m, err := s.GetAction("human", "walk3d")
	if err != nil {
		t.Fatal(err)
	}
	params := map[string]float64{}
	for k, p := range m.Params {
		params[k] = p.Default
	}
	eng := m.ToEngineMotion()
	n := eng.FrameCount
	groundY := 0.0
	for _, j := range sk.Joints {
		if j[1] > groundY {
			groundY = j[1]
		}
	}
	gridRad := FitDistance(sk.Joints, sk.Center, FitFill)
	frames := make([]*image.RGBA, 0, n)
	for i := 0; i < n; i++ {
		pose := skeleton.Pose(sk, eng, i, params)
		frames = append(frames, RenderPose(pose, sk.Bones, 0, 0, 0, sk.Center, 0, 0, true, groundY, gridRad, sk.HeadRadius))
	}
	// single frame PNG
	fb, err := PNGBytes(frames[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(fb) < 1000 {
		t.Fatalf("frame png too small: %d", len(fb))
	}
	// GIF
	gurl, err := GIFDataURL(frames, 640, 400)
	if err != nil {
		t.Fatalf("GIFDataURL: %v", err)
	}
	if len(gurl) < 100 {
		t.Fatalf("gif url too small")
	}
	// sprite
	surl, err := SpriteSheetDataURL(frames)
	if err != nil {
		t.Fatalf("SpriteSheet: %v", err)
	}
	dec, _, err := image.DecodeConfig(bytesReader(mustB64(t, surl)))
	if err != nil {
		t.Fatalf("decode sprite: %v", err)
	}
	if dec.Width != CanvasW*n || dec.Height != CanvasH {
		t.Fatalf("sprite size: %dx%d want %dx%d", dec.Width, dec.Height, CanvasW*n, CanvasH)
	}
}

func TestGIFDecodable(t *testing.T) {
	sk := buildHuman(t)
	s := store.New(dataRoot)
	m, _ := s.GetAction("human", "idle3d")
	params := map[string]float64{}
	for k, p := range m.Params {
		params[k] = p.Default
	}
	eng := m.ToEngineMotion()
	groundY := 0.0
	for _, j := range sk.Joints {
		if j[1] > groundY {
			groundY = j[1]
		}
	}
	gridRad := FitDistance(sk.Joints, sk.Center, FitFill)
	var frames []*image.RGBA
	for i := 0; i < eng.FrameCount; i++ {
		pose := skeleton.Pose(sk, eng, i, params)
		frames = append(frames, RenderPose(pose, sk.Bones, 0, 0, 0, sk.Center, 0, 0, true, groundY, gridRad, sk.HeadRadius))
	}
	url, err := GIFDataURL(frames, 640, 400)
	if err != nil {
		t.Fatal(err)
	}
	raw := mustB64(t, url)
	if _, err := gif.DecodeAll(bytesReader(raw)); err != nil {
		t.Fatalf("decode gif: %v", err)
	}
}
