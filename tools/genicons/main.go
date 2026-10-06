// Command genicons renders the tray icons into assets/light.ico and assets/dark.ico.
//
// Run from the repository root: go run ./tools/genicons
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

var sizes = []int{16, 20, 24, 32, 40, 48, 64, 256}

const supersample = 4

// shape reports whether a point in the unit square [0,1]x[0,1] is inside the glyph.
type shape func(x, y float64) bool

func main() {
	icons := []struct {
		file  string
		shape shape
		color color.NRGBA
	}{
		// Shown on the light taskbar, so it needs a saturated, darker fill.
		{"light.ico", sun, color.NRGBA{R: 0xF0, G: 0x8A, B: 0x00, A: 0xFF}},
		// Shown on the dark taskbar.
		{"dark.ico", moon, color.NRGBA{R: 0xF2, G: 0xF4, B: 0xF8, A: 0xFF}},
	}

	if err := os.MkdirAll("assets", 0o755); err != nil {
		log.Fatal(err)
	}
	for _, ic := range icons {
		data, err := buildICO(ic.shape, ic.color)
		if err != nil {
			log.Fatalf("%s: %v", ic.file, err)
		}
		path := filepath.Join("assets", ic.file)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			log.Fatal(err)
		}
		log.Printf("wrote %s (%d bytes)", path, len(data))
	}
}

func sun(x, y float64) bool {
	dx, dy := x-0.5, y-0.5
	if math.Hypot(dx, dy) <= 0.21 {
		return true
	}
	for i := range 8 {
		a := float64(i) * math.Pi / 4
		ux, uy := math.Cos(a), math.Sin(a)
		if distToSegment(dx, dy, ux*0.32, uy*0.32, ux*0.45, uy*0.45) <= 0.048 {
			return true
		}
	}
	return false
}

func moon(x, y float64) bool {
	inDisc := math.Hypot(x-0.47, y-0.53) <= 0.40
	inCut := math.Hypot(x-0.62, y-0.38) <= 0.35
	return inDisc && !inCut
}

func distToSegment(px, py, ax, ay, bx, by float64) float64 {
	vx, vy := bx-ax, by-ay
	t := ((px-ax)*vx + (py-ay)*vy) / (vx*vx + vy*vy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(px-(ax+t*vx), py-(ay+t*vy))
}

func render(s shape, c color.NRGBA, size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const samples = supersample * supersample
	for py := range size {
		for px := range size {
			hits := 0
			for sy := range supersample {
				for sx := range supersample {
					x := (float64(px) + (float64(sx)+0.5)/supersample) / float64(size)
					y := (float64(py) + (float64(sy)+0.5)/supersample) / float64(size)
					if s(x, y) {
						hits++
					}
				}
			}
			if hits > 0 {
				p := c
				p.A = uint8(int(c.A) * hits / samples)
				img.SetNRGBA(px, py, p)
			}
		}
	}
	return img
}

// buildICO writes a multi-resolution ICO with PNG-compressed entries
// (supported by Windows Vista and later).
func buildICO(s shape, c color.NRGBA) ([]byte, error) {
	images := make([][]byte, len(sizes))
	for i, size := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, render(s, c, size)); err != nil {
			return nil, err
		}
		images[i] = buf.Bytes()
	}

	var out bytes.Buffer
	le := binary.LittleEndian
	binary.Write(&out, le, [3]uint16{0, 1, uint16(len(sizes))}) // reserved, type=icon, count

	offset := 6 + 16*len(sizes)
	for i, size := range sizes {
		dim := uint8(size)
		if size >= 256 {
			dim = 0
		}
		binary.Write(&out, le, struct {
			Width, Height, Colors, Reserved uint8
			Planes, BitCount                uint16
			BytesInRes, ImageOffset         uint32
		}{dim, dim, 0, 0, 1, 32, uint32(len(images[i])), uint32(offset)})
		offset += len(images[i])
	}
	for _, img := range images {
		out.Write(img)
	}
	return out.Bytes(), nil
}
