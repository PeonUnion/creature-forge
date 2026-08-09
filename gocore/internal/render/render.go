// Package render is the Go mirror of creatureforge/render.py + the 3D
// projection/drawing of skeleton3d.py: orbit camera + pinhole perspective
// projection, ground grid, and skeleton/pose drawing to PNG/GIF — replacing
// Pillow with the standard library (image/draw).
package render

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"math"
	"strconv"
)

// Canvas + camera constants (mirror render.py / skeleton3d.py).
const (
	CanvasW, CanvasH = 960, 600
	FOVDeg           = 45.0
	FitFill          = 0.76
	Near             = 1.0
	PitchLimit       = 89.0
)

// Focal is the fixed vertical FOV focal length in pixels.
var Focal = (CanvasH / 2.0) / math.Tan(FOVDeg*math.Pi/180/2.0)

// Center is the default skeleton lookAt target.
var Center = [3]float64{480.0, 300.0, 0.0}

// Palette colors (mirror render.py).
var (
	BG       = color.RGBA{17, 24, 39, 255}    // 111827
	GUIDE    = color.RGBA{75, 94, 122, 255}   // 4b5e7a
	BONE     = color.RGBA{157, 214, 255, 255} // 9dd6ff
	JOINT    = color.RGBA{255, 241, 168, 255} // fff1a8
	BoneDark = color.RGBA{30, 42, 63, 255}    // 1e3a5f
	HeadDark = color.RGBA{35, 51, 74, 255}    // 23334a
	HeadFill = color.RGBA{45, 60, 90, 255}
)

// SkeletonView is the render input (built skeleton / pose).
type SkeletonView struct {
	Joints     map[string][3]float64
	Bones      [][2]string
	Center     [3]float64
	HeadRadius float64
}

// ---------------------------------------------------------------------------
// Projection (mirror skeleton3d.project3d)
// ---------------------------------------------------------------------------

// Project3d projects world joints (Y-down) to screen pixels via an orbit
// camera + pinhole perspective (fixed FOV). Points behind the camera are
// culled. distance<=0 auto-fits to view.
func Project3d(joints map[string][3]float64, yawDeg, pitchDeg, distance float64,
	center [3]float64, panX, panY float64) map[string][2]float64 {
	pitchDeg = clampF(pitchDeg, -PitchLimit, PitchLimit)
	yaw := yawDeg * math.Pi / 180
	pitch := pitchDeg * math.Pi / 180
	cx0, cy0, cz0 := center[0], center[1], center[2]
	if distance <= 0 {
		distance = FitDistance(joints, center, FitFill)
	}
	cp, sp := math.Cos(pitch), math.Sin(pitch)
	sy, cy := math.Sin(yaw), math.Cos(yaw)
	// camera position (sphere coords; Y-down: positive pitch raises camera)
	camX := cx0 + distance*cp*sy
	camY := cy0 - distance*sp
	camZ := cz0 + distance*cp*cy
	// view basis: forward (camera→target), right, up
	fx, fy, fz := cx0-camX, cy0-camY, cz0-camZ
	fl := math.Sqrt(fx*fx + fy*fy + fz*fz)
	if fl == 0 {
		fl = 1
	}
	fx, fy, fz = fx/fl, fy/fl, fz/fl
	rx, rz := -fz, fx
	rl := math.Hypot(rx, rz)
	if rl == 0 {
		rl = 1
	}
	rx, rz = rx/rl, rz/rl
	ry := 0.0
	ux := ry*fz - rz*fy
	uy := rz*fx - rx*fz
	uz := rx*fy - ry*fx
	halfW, halfH := CanvasW/2.0, CanvasH/2.0
	out := make(map[string][2]float64, len(joints))
	for name, j := range joints {
		vx, vy, vz := j[0]-camX, j[1]-camY, j[2]-camZ
		xc := vx*rx + vy*ry + vz*rz
		yc := vx*ux + vy*uy + vz*uz
		zc := vx*fx + vy*fy + vz*fz
		if zc <= Near {
			continue
		}
		out[name] = [2]float64{halfW + xc*Focal/zc + panX, halfH + yc*Focal/zc + panY}
	}
	return out
}

// FitDistance computes the camera distance so the model occupies `fill` of the
// vertical FOV (mirror skeleton3d._fit_distance).
func FitDistance(joints map[string][3]float64, center [3]float64, fill float64) float64 {
	r := 1.0
	for _, j := range joints {
		d := math.Sqrt((j[0]-center[0])*(j[0]-center[0]) + (j[1]-center[1])*(j[1]-center[1]) + (j[2]-center[2])*(j[2]-center[2]))
		if d > r {
			r = d
		}
	}
	fill = math.Max(fill, 0.05)
	return r / (math.Tan(FOVDeg*math.Pi/180/2.0) * fill)
}

func clampF(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

// ---------------------------------------------------------------------------
// Drawing primitives (mirror render.py)
// ---------------------------------------------------------------------------

func setPixel(dst *image.RGBA, x, y int, c color.Color) {
	if x >= 0 && x < CanvasW && y >= 0 && y < CanvasH {
		dst.Set(x, y, c)
	}
}

// thickLine draws a line of the given width (square brush), mirroring the
// two-pass bone draw (dark wide underlay + colored thin line).
func thickLine(dst *image.RGBA, x0, y0, x1, y1, width int, c color.Color) {
	if width < 1 {
		width = 1
	}
	r := (width - 1) / 2
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := 1
	if x0 >= x1 {
		sx = -1
	}
	sy := 1
	if y0 >= y1 {
		sy = -1
	}
	err := dx + dy
	for {
		for oy := -r; oy <= r; oy++ {
			for ox := -r; ox <= r; ox++ {
				setPixel(dst, x0+ox, y0+oy, c)
			}
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// ellipse fills a (possibly tall) ellipse centered at (cx,cy) with radii rx,ry.
func ellipse(dst *image.RGBA, cx, cy, rx, ry int, fill, outline color.Color, width int) {
	rx = abs(rx)
	ry = abs(ry)
	fx, fy := float64(rx), float64(ry)
	// interior
	for y := cy - ry; y <= cy+ry; y++ {
		for x := cx - rx; x <= cx+rx; x++ {
			dx := (float64(x-cx) + 0.5) / fx
			dy := (float64(y-cy) + 0.5) / fy
			if dx*dx+dy*dy <= 1 {
				setPixel(dst, x, y, fill)
			}
		}
	}
	// outline: scan for boundary
	if width > 0 {
		for a := 0; a < 360; a++ {
			rad := float64(a) * math.Pi / 180
			px := int(float64(cx) + math.Cos(rad)*fx)
			py := int(float64(cy) + math.Sin(rad)*fy)
			thickLine(dst, px, py, px, py, width, outline)
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// ---------------------------------------------------------------------------
// Scene rendering (mirror skeleton3d.render_pose / render_view)
// ---------------------------------------------------------------------------

// RenderPose draws a single pose/skeleton frame to an RGBA image. Grid overlay
// uses ground_y (world Y of the ground) and gridRad (fixed skeleton radius).
func RenderPose(pose map[string][3]float64, bones [][2]string,
	yawDeg, pitchDeg, distance float64, center [3]float64,
	panX, panY float64, grid bool, groundY float64, gridRad float64,
	headRadius float64) *image.RGBA {
	if distance <= 0 {
		distance = FitDistance(pose, center, FitFill)
	}
	dst := image.NewRGBA(image.Rect(0, 0, CanvasW, CanvasH))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: BG}, image.Point{}, draw.Src)
	if grid {
		drawGroundGrid(dst, pose, yawDeg, pitchDeg, distance, center, panX, panY, groundY, gridRad)
	}
	pts := Project3d(pose, yawDeg, pitchDeg, distance, center, panX, panY)
	// heads on top of bones
	for _, hk := range []string{"head", "head_left", "head_right"} {
		if p, ok := pts[hk]; ok {
			r := int(headRadius)
			ellipse(dst, int(p[0]), int(p[1]), int(float64(r)*0.78), r, HeadFill, BONE, 3)
		}
	}
	for _, b := range bones {
		a, aok := pts[b[0]]
		c, cok := pts[b[1]]
		if !aok || !cok {
			continue
		}
		thickLine(dst, int(a[0]), int(a[1]), int(c[0]), int(c[1]), 13, BoneDark)
		thickLine(dst, int(a[0]), int(a[1]), int(c[0]), int(c[1]), 7, BONE)
	}
	for _, p := range pts {
		ellipse(dst, int(p[0]), int(p[1]), 7, 7, JOINT, HeadDark, 2)
	}
	return dst
}

// drawGroundGrid mirrors skeleton3d._draw_ground_grid (world XZ plane at
// ground_y, projected with the same camera).
func drawGroundGrid(dst *image.RGBA, pose map[string][3]float64,
	yawDeg, pitchDeg, distance float64, center [3]float64,
	panX, panY float64, groundY float64, gridRad float64) {
	cx, cy, cz := center[0], center[1], center[2]
	rad := gridRad
	if rad <= 0 {
		rad = 100.0
		for _, j := range pose {
			d := math.Sqrt((j[0]-cx)*(j[0]-cx) + (j[1]-cy)*(j[1]-cy) + (j[2]-cz)*(j[2]-cz))
			if d > rad {
				rad = d
			}
		}
	}
	gy := groundY
	if gy == 0 {
		for _, j := range pose {
			if j[1] > gy {
				gy = j[1]
			}
		}
	}
	extent := rad * 1.4
	step := math.Max(extent/6.0, 1.0)
	n := int(math.Round(extent / step))
	pts := map[string][3]float64{}
	var lines [][2]string
	for i := -n; i <= n; i++ {
		v := float64(i) * step
		a := fmtKey("gx", i, "a")
		b := fmtKey("gx", i, "b")
		pts[a] = [3]float64{cx + v, gy, cz - extent}
		pts[b] = [3]float64{cx + v, gy, cz + extent}
		lines = append(lines, [2]string{a, b})
		c := fmtKey("gz", i, "a")
		d := fmtKey("gz", i, "b")
		pts[c] = [3]float64{cx - extent, gy, cz + v}
		pts[d] = [3]float64{cx + extent, gy, cz + v}
		lines = append(lines, [2]string{c, d})
	}
	proj := Project3d(pts, yawDeg, pitchDeg, distance, center, panX, panY)
	for _, l := range lines {
		pa, ok1 := proj[l[0]]
		pb, ok2 := proj[l[1]]
		if ok1 && ok2 {
			thickLine(dst, int(pa[0]), int(pa[1]), int(pb[0]), int(pb[1]), 1, GUIDE)
		}
	}
}

func fmtKey(prefix string, i int, suffix string) string {
	if i >= 0 {
		return prefix + strconv.Itoa(i) + suffix
	}
	return prefix + "m" + strconv.Itoa(-i) + suffix
}

// ---------------------------------------------------------------------------
// Encoding
// ---------------------------------------------------------------------------

// PNGBytes encodes an RGBA frame to PNG bytes.
func PNGBytes(img *image.RGBA) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// PNGDataURL returns a data:image/png;base64 URL (mirror image_to_data_url).
func PNGDataURL(img *image.RGBA) (string, error) {
	b, err := PNGBytes(img)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b), nil
}

// gifPalette is the fixed palette used for GIF frames.
var gifPalette = color.Palette{
	BG, GUIDE, BONE, JOINT, BoneDark, HeadDark, HeadFill,
	color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255},
}

func nearestPalette(c color.Color) color.Color {
	r, g, b, _ := c.RGBA()
	best := gifPalette[0]
	bestD := uint32(1<<31 - 1)
	for _, p := range gifPalette {
		pr, pg, pb, _ := p.RGBA()
		dr := int64(r) - int64(pr)
		dg := int64(g) - int64(pg)
		db := int64(b) - int64(pb)
		d := dr*dr + dg*dg + db*db
		if uint32(d) < bestD {
			bestD = uint32(d)
			best = p
		}
	}
	return best
}

func toPaletted(img *image.RGBA) *image.Paletted {
	out := image.NewPaletted(img.Bounds(), gifPalette)
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			out.Set(x, y, nearestPalette(img.At(x, y)))
		}
	}
	return out
}

// GIFDataURL encodes frames (resized to w×h) as an animated GIF data URL
// (duration 180ms/frame, loop forever — mirror of the Python render).
func GIFDataURL(frames []*image.RGBA, w, h int) (string, error) {
	var buf bytes.Buffer
	g := &gif.GIF{LoopCount: 0}
	for _, fr := range frames {
		scaled := scaleRGBA(fr, w, h)
		g.Image = append(g.Image, toPaletted(scaled))
		g.Delay = append(g.Delay, 18) // 180ms
		g.Disposal = append(g.Disposal, gif.DisposalBackground)
	}
	if err := gif.EncodeAll(&buf, g); err != nil {
		return "", err
	}
	return "data:image/gif;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// SpriteSheetDataURL pastes frames horizontally (mirror sprite sheet).
func SpriteSheetDataURL(frames []*image.RGBA) (string, error) {
	if len(frames) == 0 {
		return "", nil
	}
	w, h := frames[0].Bounds().Dx(), frames[0].Bounds().Dy()
	sheet := image.NewRGBA(image.Rect(0, 0, w*len(frames), h))
	for i, fr := range frames {
		draw.Draw(sheet, image.Rect(i*w, 0, (i+1)*w, h), fr, image.Point{}, draw.Src)
	}
	return PNGDataURL(sheet)
}

// scaleRGBA nearest-neighbor resizes a frame to w×h.
func scaleRGBA(src *image.RGBA, w, h int) *image.RGBA {
	if w <= 0 {
		w = src.Bounds().Dx()
	}
	if h <= 0 {
		h = src.Bounds().Dy()
	}
	if w == src.Bounds().Dx() && h == src.Bounds().Dy() {
		return src
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	sw, sh := float64(src.Bounds().Dx()), float64(src.Bounds().Dy())
	for y := 0; y < h; y++ {
		sy := int(float64(y) * sh / float64(h))
		for x := 0; x < w; x++ {
			sx := int(float64(x) * sw / float64(w))
			out.Set(x, y, src.At(sx, sy))
		}
	}
	return out
}
