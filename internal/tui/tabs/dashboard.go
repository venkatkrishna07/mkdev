package tabs

import (
	"crypto/x509"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui/components"
	"github.com/venkatkrishna07/mkdev/internal/tui/msg"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// DashSource lets the dashboard query live metrics without coupling to proxy.
type DashSource struct {
	Total func() uint64
	RPS   func() []float64
	CA    *x509.Certificate
	Start time.Time
}

type Dashboard struct {
	th     styles.Theme
	src    DashSource
	routes []store.Route
	width  int
	height int
	now    time.Time
}

func NewDashboard(th styles.Theme, src DashSource) Dashboard {
	return Dashboard{th: th, src: src, now: time.Now()}
}

func (d Dashboard) Title() string { return "Dashboard" }

func (d Dashboard) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return DashboardTickMsg(t) })
}

type DashboardTickMsg time.Time

func (d Dashboard) Update(in tea.Msg) (Dashboard, tea.Cmd) {
	switch m := in.(type) {
	case tea.WindowSizeMsg:
		d.width = m.Width
		d.height = m.Height
	case msg.RoutesRefreshed:
		d.routes = m.Routes
	case DashboardTickMsg:
		d.now = time.Time(m)
		return d, tea.Tick(time.Second, func(t time.Time) tea.Msg { return DashboardTickMsg(t) })
	}
	return d, nil
}

func (d Dashboard) View() string {
	w := d.width
	if w <= 0 {
		w = 100
	}

	total := uint64(0)
	if d.src.Total != nil {
		total = d.src.Total()
	}
	active := 0
	for _, r := range d.routes {
		if r.Enabled {
			active++
		}
	}

	cards := []string{
		d.card("ROUTES", fmt.Sprintf("%d / %d", active, len(d.routes)), "active / total"),
		d.card("REQUESTS", fmt.Sprintf("%d", total), "since start"),
		d.card("UPTIME", humanDuration(time.Since(d.src.Start)), "process"),
		d.card("CA EXPIRY", d.expiryLabel(), d.expiryDetail()),
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, cards...)

	rps := []float64{}
	if d.src.RPS != nil {
		rps = d.src.RPS()
	}
	sparkW := max(w-4, 20)
	rpsBlock := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.th.Muted).
		Padding(0, 1).
		Width(w - 4).
		Render(
			d.th.Title.Render("Requests / sec — last 60s") + "\n" +
				components.Sparkline(d.th, rps, sparkW),
		)

	return lipgloss.JoinVertical(lipgloss.Left, row, "", rpsBlock, "", d.hint())
}

func (d Dashboard) card(label, big, sub string) string {
	body := d.th.Dim.Render(label) + "\n" +
		d.th.Title.Render(big) + "\n" +
		d.th.Dim.Render(sub)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.th.Primary).
		Padding(0, 2).
		MarginRight(1).
		Width(22).
		Render(body)
}

func (d Dashboard) expiryLabel() string {
	if d.src.CA == nil {
		return "—"
	}
	left := time.Until(d.src.CA.NotAfter)
	if left <= 0 {
		return "EXPIRED"
	}
	days := int(left.Hours() / 24)
	return fmt.Sprintf("%dd", days)
}

func (d Dashboard) expiryDetail() string {
	if d.src.CA == nil {
		return ""
	}
	return d.src.CA.NotAfter.Format("2006-01-02")
}

func (d Dashboard) hint() string {
	return d.th.Dim.Render("Domains tab for routing · Logs for live tail · Doctor for health")
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd", days)
}

