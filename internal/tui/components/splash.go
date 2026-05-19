package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

var logoLines = []string{
	"█▄ ▄█ █  █ █▀▄ █▀▀ █ █",
	"█ ▀ █ █▄▀  █ █ █▀▀ ▀▄▀",
	"▀   ▀ ▀ ▀  ▀▀  ▀▀▀  ▀ ",
}

// Splash renders a centred ASCII logo with a primary→accent gradient,
// version pill, and tagline. Used on TUI launch.
func Splash(th styles.Theme, version, tagline string, width, height int) string {
	if width < 40 {
		width = 40
	}
	if height < 8 {
		height = 8
	}

	colored := make([]string, len(logoLines))
	for i, ln := range logoLines {
		colored[i] = gradientLine(th, ln, float64(i)/float64(len(logoLines)-1))
	}
	logo := lipgloss.JoinVertical(lipgloss.Center, colored...)

	ver := th.Dim.Render("v" + version)
	tag := th.Title.Render(tagline)

	block := lipgloss.JoinVertical(lipgloss.Center,
		logo,
		"",
		tag,
		ver,
	)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, block)
}

// gradientLine blends primary→accent based on row position t∈[0,1].
// Falls back to flat primary on terminals without truecolor.
func gradientLine(th styles.Theme, s string, t float64) string {
	cells := []rune(s)
	out := make([]string, 0, len(cells))
	for i, r := range cells {
		if r == ' ' {
			out = append(out, " ")
			continue
		}
		x := (float64(i)/float64(len(cells)-1) + t) / 2.0
		col := lerpAdaptive(th.Primary, th.Accent, x)
		out = append(out, lipgloss.NewStyle().Foreground(col).Bold(true).Render(string(r)))
	}
	return strings.Join(out, "")
}

func lerpAdaptive(a, b lipgloss.AdaptiveColor, t float64) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{
		Light: lerpHex(a.Light, b.Light, t),
		Dark:  lerpHex(a.Dark, b.Dark, t),
	}
}

func lerpHex(a, b string, t float64) string {
	ar, ag, ab := parseHex(a)
	br, bg, bb := parseHex(b)
	r := int(float64(ar) + (float64(br)-float64(ar))*t)
	g := int(float64(ag) + (float64(bg)-float64(ag))*t)
	bl := int(float64(ab) + (float64(bb)-float64(ab))*t)
	return toHex(r, g, bl)
}

func parseHex(s string) (int, int, int) {
	if len(s) != 7 || s[0] != '#' {
		return 255, 255, 255
	}
	r := hexByte(s[1])*16 + hexByte(s[2])
	g := hexByte(s[3])*16 + hexByte(s[4])
	b := hexByte(s[5])*16 + hexByte(s[6])
	return r, g, b
}

func hexByte(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return 0
}

func toHex(r, g, b int) string {
	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return v
	}
	return "#" + byteToHex(clamp(r)) + byteToHex(clamp(g)) + byteToHex(clamp(b))
}

func byteToHex(v int) string {
	const digits = "0123456789ABCDEF"
	return string([]byte{digits[v>>4], digits[v&0xF]})
}
