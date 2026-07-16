// Generates assets/icon.ico: a link being switched across tracks, rendered
// resolution-independently at each icon size (PNG-compressed ICO entries).
//
//	go run ./tools/genicon
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

var (
	bg     = color.NRGBA{0x1b, 0x20, 0x30, 0xff} // panel
	track  = color.NRGBA{0x3a, 0x41, 0x55, 0xff} // dim tracks
	route  = color.NRGBA{0x6e, 0xa8, 0xfe, 0xff} // accent route
	target = color.NRGBA{0x4a, 0xde, 0x80, 0xff} // destination dot
)

func main() {
	sizes := []int{16, 24, 32, 48, 64, 256}
	var blobs [][]byte
	for _, s := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, render(s)); err != nil {
			panic(err)
		}
		blobs = append(blobs, buf.Bytes())
	}

	out := &bytes.Buffer{}
	binary.Write(out, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(out, binary.LittleEndian, uint16(1)) // 1 = icon
	binary.Write(out, binary.LittleEndian, uint16(len(sizes)))
	offset := 6 + 16*len(sizes)
	for i, s := range sizes {
		dim := byte(s)
		if s >= 256 {
			dim = 0 // 0 means 256 in ICO directory entries
		}
		out.Write([]byte{dim, dim, 0, 0})
		binary.Write(out, binary.LittleEndian, uint16(1))  // planes
		binary.Write(out, binary.LittleEndian, uint16(32)) // bpp
		binary.Write(out, binary.LittleEndian, uint32(len(blobs[i])))
		binary.Write(out, binary.LittleEndian, uint32(offset))
		offset += len(blobs[i])
	}
	for _, b := range blobs {
		out.Write(b)
	}
	if err := os.MkdirAll("assets", 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile("assets/icon.ico", out.Bytes(), 0o644); err != nil {
		panic(err)
	}

	// PNG icons for the companion extension.
	for _, s := range []int{16, 48, 128} {
		var buf bytes.Buffer
		if err := png.Encode(&buf, render(s)); err != nil {
			panic(err)
		}
		p := fmt.Sprintf("internal/companion/ext/icon%d.png", s)
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			panic(err)
		}
	}
}

// render draws the icon at size px using normalized [0,1] coordinates with
// 3x3 supersampling, so every size is crisp rather than a blurry downscale.
func render(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 3
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					u := (float64(x) + (float64(sx)+0.5)/ss) / float64(size)
					v := (float64(y) + (float64(sy)+0.5)/ss) / float64(size)
					c := shade(u, v)
					r += float64(c.R)
					g += float64(c.G)
					b += float64(c.B)
					a += float64(c.A)
				}
			}
			n := float64(ss * ss)
			img.SetNRGBA(x, y, color.NRGBA{uint8(r / n), uint8(g / n), uint8(b / n), uint8(a / n)})
		}
	}
	return img
}

// shade returns the color at normalized point (u, v).
func shade(u, v float64) color.NRGBA {
	// Rounded-square background; transparent outside.
	const radius = 0.22
	if roundRectDist(u, v, radius) > 0 {
		return color.NRGBA{}
	}
	c := bg

	// Three horizontal tracks.
	for _, ty := range []float64{0.28, 0.5, 0.72} {
		if segDist(u, v, 0.20, ty, 0.80, ty) < 0.035 {
			c = track
		}
	}
	// The switched route: top-left track diving to the bottom-right track.
	if segDist(u, v, 0.20, 0.28, 0.44, 0.28) < 0.055 ||
		segDist(u, v, 0.44, 0.28, 0.62, 0.72) < 0.055 ||
		segDist(u, v, 0.62, 0.72, 0.80, 0.72) < 0.055 {
		c = route
	}
	// Destination node.
	if math.Hypot(u-0.80, v-0.72) < 0.085 {
		c = target
	}
	return c
}

// roundRectDist is positive outside a unit rounded square with the given
// corner radius, negative inside.
func roundRectDist(u, v, r float64) float64 {
	half := 0.5 - r
	dx := math.Max(math.Abs(u-0.5)-half, 0)
	dy := math.Max(math.Abs(v-0.5)-half, 0)
	return math.Hypot(dx, dy) - r
}

// segDist is the distance from (px, py) to segment (ax, ay)-(bx, by).
func segDist(px, py, ax, ay, bx, by float64) float64 {
	vx, vy := bx-ax, by-ay
	wx, wy := px-ax, py-ay
	t := (wx*vx + wy*vy) / (vx*vx + vy*vy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(px-(ax+t*vx), py-(ay+t*vy))
}
