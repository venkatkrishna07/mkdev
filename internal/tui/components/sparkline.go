package components

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

var sparkBars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders xs as an 8-level Braille-style bar strip. Empty input
// returns a row of dim dashes of width n.
func Sparkline(th styles.Theme, xs []float64, n int) string {
	if n <= 0 {
		n = 10
	}
	if len(xs) == 0 {
		return th.Dim.Render(strings.Repeat("·", n))
	}
	// downsample / pad to n
	src := xs
	if len(src) > n {
		src = src[len(src)-n:]
	}
	var maxV float64
	for _, v := range src {
		if v > maxV {
			maxV = v
		}
	}
	if maxV == 0 {
		return th.Dim.Render(strings.Repeat("▁", len(src)))
	}
	var b strings.Builder
	for _, v := range src {
		idx := int(v / maxV * float64(len(sparkBars)-1))
		idx = max(idx, 0)
		idx = min(idx, len(sparkBars)-1)
		col := th.OK
		if v/maxV > 0.66 {
			col = th.Bad
		} else if v/maxV > 0.33 {
			col = th.Warn
		}
		b.WriteString(lipgloss.NewStyle().Foreground(col).Render(string(sparkBars[idx])))
	}
	if pad := n - len(src); pad > 0 {
		b.WriteString(th.Dim.Render(strings.Repeat("·", pad)))
	}
	return b.String()
}

// SparklineDur is a convenience for []time.Duration; converts to milliseconds.
func SparklineDur(th styles.Theme, ds []time.Duration, n int) string {
	xs := make([]float64, len(ds))
	for i, d := range ds {
		xs[i] = float64(d.Milliseconds())
	}
	return Sparkline(th, xs, n)
}
