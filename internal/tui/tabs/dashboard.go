package tabs

import (
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/proxy/prober"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui/components"
	"github.com/venkatkrishna07/mkdev/internal/tui/msg"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// LANState is a snapshot of mDNS advertise state for the dashboard.
// Mirrors tui.LANState shape to avoid an import cycle.
type LANState struct {
	IP          string
	Advertising bool
	SharedCount int
}

// DashSource lets the dashboard query live metrics without coupling to proxy.
type DashSource struct {
	Total    func() uint64
	RPS      func() []float64
	CA       *x509.Certificate
	Start    time.Time
	Routes   func() []store.Route
	Health   func(host string) prober.HealthState
	LastSeen func(host string) time.Time
	LAN      func() LANState
}

// Dashboard is the live overview tab: route counts, request totals, uptime,
// CA expiry, a sparkline of recent RPS, a per-route health table, and the
// LAN-advertise strip.
type Dashboard struct {
	th     styles.Theme
	src    DashSource
	routes []store.Route
	width  int
	height int
	now    time.Time
}

// NewDashboard constructs a Dashboard bound to src.
func NewDashboard(th styles.Theme, src DashSource) Dashboard {
	return Dashboard{th: th, src: src, now: time.Now()}
}

// Title implements tabs.Tab.
func (d Dashboard) Title() string { return "Dashboard" }

// Init schedules the first dashboard tick.
func (d Dashboard) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return DashboardTickMsg(t) })
}

// DashboardTickMsg drives the per-second refresh of the dashboard view.
type DashboardTickMsg time.Time

// Update handles ticks, window-size changes, and route refresh messages.
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

// View renders the dashboard as a vertical stack of KPI cards, sparkline,
// routes table, LAN strip, and a footer hint.
func (d Dashboard) View() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		d.renderKPIs(),
		"",
		d.renderSparkline(),
		"",
		d.renderRoutesTable(),
		"",
		d.renderLANStrip(),
		"",
		d.hint(),
	)
}

func (d Dashboard) renderKPIs() string {
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
	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

func (d Dashboard) renderSparkline() string {
	w := d.width
	if w <= 0 {
		w = 100
	}
	rps := []float64{}
	if d.src.RPS != nil {
		rps = d.src.RPS()
	}
	sparkW := max(w-4, 20)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.th.Muted).
		Padding(0, 1).
		Width(w - 4).
		Render(
			d.th.Title.Render("Requests / sec — last 60s") + "\n" +
				components.Sparkline(d.th, rps, sparkW),
		)
}

func (d Dashboard) renderRoutesTable() string {
	w := d.width
	if w <= 0 {
		w = 100
	}
	title := d.th.Title.Render("Routes")
	if d.src.Routes == nil || len(d.routes) == 0 {
		return boxed(d.th, title+"\n"+d.th.Dim.Render("no routes configured"), w)
	}
	rows := make([]string, 0, len(d.routes)+1)
	rows = append(rows, title)
	for _, r := range d.routes {
		rows = append(rows, d.routeRow(r))
	}
	return boxed(d.th, strings.Join(rows, "\n"), w)
}

func (d Dashboard) routeRow(r store.Route) string {
	var pillKind components.PillKind
	var right string

	switch {
	case !r.Enabled:
		pillKind = components.PillOff
		right = d.th.Dim.Render("—")
	default:
		h := prober.HealthState{}
		if d.src.Health != nil {
			h = d.src.Health(strings.ToLower(r.Domain))
		}
		switch h.Status {
		case prober.StatusUp:
			pillKind = components.PillUp
			label := "never"
			if d.src.LastSeen != nil {
				label = formatLastSeen(d.src.LastSeen(r.Domain))
			}
			right = d.th.Dim.Render(label)
		case prober.StatusDown:
			pillKind = components.PillDown
			right = d.th.Dim.Render(h.LastErr)
		default:
			pillKind = components.PillOff
			right = d.th.Dim.Render("—")
		}
	}

	pill := components.StatusPill(d.th, pillKind, "")
	host := truncate(r.Domain, 24)
	target := d.th.Dim.Render("→ " + truncate(r.Target, 30))
	badge := components.ShareBadge(d.th, r.Shared)

	cell := func(w int, s string) string {
		return lipgloss.NewStyle().Width(w).Render(s)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		cell(8, pill),
		cell(26, host),
		cell(32, target),
		cell(7, badge),
		right,
	)
}

func (d Dashboard) renderLANStrip() string {
	w := d.width
	if w <= 0 {
		w = 100
	}
	title := d.th.Title.Render("LAN advertise")
	if d.src.LAN == nil {
		return boxed(d.th, title+"\n"+d.th.Dim.Render("⊘ unavailable"), w)
	}
	st := d.src.LAN()
	var line string
	switch {
	case !st.Advertising:
		line = d.th.Dim.Render("⊘ LAN unavailable")
	case st.SharedCount == 0:
		line = d.th.Dim.Render(fmt.Sprintf("⊘ no shared routes (host %s)", st.IP))
	default:
		pill := components.StatusPill(d.th, components.PillUp, "broadcasting")
		line = fmt.Sprintf("%s  %s   %d shared route(s)", pill, st.IP, st.SharedCount)
	}
	return boxed(d.th, title+"\n"+line, w)
}

// boxed wraps body in a rounded bordered block tinted with the theme's muted color.
func boxed(th styles.Theme, body string, w int) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Muted).
		Padding(0, 1).
		Width(w - 4).
		Render(body)
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

// formatLastSeen turns a timestamp into a short relative label used in
// the dashboard routes table.
func formatLastSeen(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	if d < time.Second {
		return "just now"
	}
	return humanDuration(d) + " ago"
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

// truncate shortens s to at most n runes, appending an ellipsis when cut.
// For n <= 1 it returns the raw byte slice s[:n] without an ellipsis.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
