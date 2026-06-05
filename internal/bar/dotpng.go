package bar

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

var (
	colorUp      = color.RGBA{R: 0x30, G: 0xD1, B: 0x58, A: 0xFF}
	colorDown    = color.RGBA{R: 0xFF, G: 0x45, B: 0x3A, A: 0xFF}
	colorProbing = color.RGBA{R: 0xFF, G: 0xCC, B: 0x00, A: 0xFF}
	colorOff     = color.RGBA{R: 0xAE, G: 0xAE, B: 0xB2, A: 0xFF}
)

// systray hardcodes NSImage size to 16x16 pt, so the source PNG is
// downscaled regardless of pixel dimensions. Trick: render a small dot inside
// a larger transparent canvas — the dot then occupies only a fraction of the
// 16pt menu cell. With canvasPx=32 and dotDiameterPx=14, displayed dot ≈ 7pt.
const (
	canvasPx      = 32
	dotDiameterPx = 14
	aaSamples     = 4
)

var (
	dotOnce            sync.Once
	dotUp, dotDown     []byte
	dotProbing, dotOff []byte
)

func ensureDots() {
	dotOnce.Do(func() {
		dotUp = drawDot(colorUp)
		dotDown = drawDot(colorDown)
		dotProbing = drawDot(colorProbing)
		dotOff = drawDot(colorOff)
	})
}

func drawDot(c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, canvasPx, canvasPx))
	cx := float64(canvasPx) / 2
	cy := float64(canvasPx) / 2
	radius := float64(dotDiameterPx) / 2
	r2 := radius * radius
	r4, g4, b4, _ := c.RGBA()
	cr, cg, cb := uint8(r4>>8), uint8(g4>>8), uint8(b4>>8)
	inv := 1.0 / float64(aaSamples)

	for y := range canvasPx {
		for x := range canvasPx {
			hits := 0
			for sy := range aaSamples {
				fy := float64(y) + (float64(sy)+0.5)*inv
				dy := fy - cy
				dy2 := dy * dy
				for sx := range aaSamples {
					fx := float64(x) + (float64(sx)+0.5)*inv
					dx := fx - cx
					if dx*dx+dy2 <= r2 {
						hits++
					}
				}
			}
			if hits == 0 {
				continue
			}
			alpha := uint8(hits * 255 / (aaSamples * aaSamples))
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(uint16(cr) * uint16(alpha) / 255),
				G: uint8(uint16(cg) * uint16(alpha) / 255),
				B: uint8(uint16(cb) * uint16(alpha) / 255),
				A: alpha,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(fmt.Sprintf("bar: encode dot icon: %v", err))
	}
	return buf.Bytes()
}

func iconForHealth(h api.Health) []byte {
	ensureDots()
	switch h {
	case api.HealthUp:
		return dotUp
	case api.HealthDown:
		return dotDown
	case api.HealthProbing:
		return dotProbing
	default:
		return dotOff
	}
}
