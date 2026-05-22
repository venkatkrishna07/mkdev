package tabs

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/proxy/prober"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui/msg"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// RTTSource resolves the rolling RTT window for a domain. nil = no live RTT.
type RTTSource func(domain string) []time.Duration

// HealthSource resolves the latest probe state for a route domain.
// nil = no live health (falls back to Enabled-only signal).
type HealthSource func(domain string) prober.HealthState

// LANSource resolves the current mDNS advertise snapshot. nil = no advertise info.
type LANSource func() LANState

// Domains is the routes table tab.
type Domains struct {
	th     styles.Theme
	width  int
	height int
	table  table.Model
	routes []store.Route
	rtt    RTTSource
	health HealthSource
	lan    LANSource
}

// Column widths are the content area. bubbles/table renders each cell with
// Cell.Padding(0,1), so each column occupies Width+2 visible columns.
const (
	domainsStatusW = 20
	domainsShareW  = 5
	domainsAddedW  = 10
	domainsNCols   = 5
	domainsCellPad = 2 // Cell.Padding(0,1) → 1 space each side
	domainsMinW    = 84
)

// NewDomains constructs a Domains tab without live RTT or health sources.
func NewDomains(th styles.Theme, width, height int) Domains {
	return NewDomainsWithSources(th, width, height, nil, nil, nil)
}

// NewDomainsWithRTT constructs a Domains tab that surfaces live RTT samples.
func NewDomainsWithRTT(th styles.Theme, width, height int, rtt RTTSource) Domains {
	return NewDomainsWithSources(th, width, height, rtt, nil, nil)
}

// NewDomainsWithSources constructs a Domains tab with RTT, Health, and LAN sources.
func NewDomainsWithSources(th styles.Theme, width, height int, rtt RTTSource, health HealthSource, lan LANSource) Domains {
	d := Domains{th: th, width: width, height: height, rtt: rtt, health: health, lan: lan}
	cols := d.layoutCols()
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(8),
		table.WithWidth(tableTotalWidth(cols)),
	)
	t.SetStyles(tableStyles(th))
	d.table = t
	return d
}

// tableTotalWidth returns the visible width occupied by the table — the sum of
// column content widths plus the Cell.Padding(0,1) added per column by
// bubbles/table.
func tableTotalWidth(cols []table.Column) int {
	w := 0
	for _, c := range cols {
		w += c.Width + domainsCellPad
	}
	return w
}

func tableStyles(th styles.Theme) table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		Bold(true).
		Foreground(th.Primary).
		BorderStyle(lipgloss.HiddenBorder()).
		BorderBottom(false).
		Padding(0, 1)
	s.Selected = s.Selected.
		Bold(true).
		Foreground(th.OnPill).
		Background(th.Accent)
	s.Cell = s.Cell.Padding(0, 1)
	return s
}

func (d Domains) layoutCols() []table.Column {
	w := d.width
	if w < domainsMinW {
		w = domainsMinW
	}
	fixed := domainsStatusW + domainsShareW + domainsAddedW
	padTotal := domainsCellPad * domainsNCols
	rem := w - fixed - padTotal
	if rem < 24 {
		rem = 24
	}
	domW := rem / 2
	tgtW := rem - domW
	return []table.Column{
		{Title: "DOMAIN", Width: domW},
		{Title: "TARGET", Width: tgtW},
		{Title: "STATUS", Width: domainsStatusW},
		{Title: "SHARE", Width: domainsShareW},
		{Title: "ADDED", Width: domainsAddedW},
	}
}

// Title implements tabs.Tab.
func (d Domains) Title() string { return "Domains" }

// Init implements tea.Model; Domains has no async startup work.
func (d Domains) Init() tea.Cmd { return nil }

// Update handles route refresh and window-size events.
func (d Domains) Update(in tea.Msg) (Domains, tea.Cmd) {
	var cmd tea.Cmd
	d.table, cmd = d.table.Update(in)
	switch m := in.(type) {
	case msg.RoutesRefreshed:
		d.routes = m.Routes
		d.refreshRows()
	case tea.WindowSizeMsg:
		d.width = m.Width
		d.height = m.Height
		cols := d.layoutCols()
		d.table.SetColumns(cols)
		d.table.SetWidth(tableTotalWidth(cols))
		d.fitHeight()
	}
	return d, cmd
}

func (d *Domains) fitHeight() {
	rows := max(len(d.routes), 1)
	budget := max(d.height-6, 3)
	h := min(rows+1, budget)
	d.table.SetHeight(h)
}

func (d *Domains) refreshRows() {
	if len(d.routes) == 0 {
		d.table.SetRows(nil)
		return
	}
	rows := make([]table.Row, len(d.routes))
	for i, r := range d.routes {
		rows[i] = table.Row{
			r.Domain,
			r.Target,
			d.statusCell(r),
			shareCell(r.Shared),
			r.AddedAt.Format("2006-01-02"),
		}
	}
	d.table.SetRows(rows)
	d.fitHeight()
}

// statusCell returns the visible status label. No ANSI styling — bubbles/table
// truncates each cell with runewidth before the style render, which counts ANSI
// escape bytes toward visible width and mangles the output. Color belongs to
// the row Selected style only.
func (d Domains) statusCell(r store.Route) string {
	if !r.Enabled {
		return "⊘ off"
	}
	if d.health != nil {
		h := d.health(strings.ToLower(r.Domain))
		switch h.Status {
		case prober.StatusUp:
			return "● up"
		case prober.StatusDown:
			if h.LastErr == "" {
				return "✗ down"
			}
			return truncate("✗ down: "+h.LastErr, domainsStatusW)
		}
	}
	if d.rtt != nil && len(d.rtt(r.Domain)) > 0 {
		return "● live"
	}
	return "● up"
}

func shareCell(shared bool) string {
	if shared {
		return "LAN"
	}
	return "local"
}

// View renders the table plus a selected-route detail panel.
// Empty state (no routes) preserves the existing "no routes yet" hint.
func (d Domains) View() string {
	if len(d.routes) == 0 {
		hint := d.th.Dim.Render("no routes yet — press ") + d.th.FooterKey.Render("a") + d.th.Dim.Render(" to add")
		return lipgloss.JoinVertical(lipgloss.Left, hint, d.table.View())
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		d.table.View(),
		"",
		d.detailPanel(),
	)
}

// Selected returns the route under the table cursor, if any.
func (d Domains) Selected() (store.Route, bool) {
	if len(d.routes) == 0 {
		return store.Route{}, false
	}
	idx := min(max(d.table.Cursor(), 0), len(d.routes)-1)
	return d.routes[idx], true
}

// detailPanel renders a bordered block beneath the table with full info for
// the currently selected route. Returns "" when no route is selected.
func (d Domains) detailPanel() string {
	r, ok := d.Selected()
	if !ok {
		return ""
	}

	w := d.width
	if w <= 0 {
		w = 100
	}

	title := d.th.Title.Render(r.Domain)
	target := d.th.Dim.Render("Target  ") + r.Target
	scope := d.th.Dim.Render("Scope   ") + d.scopeValue(r)
	health, errLine := d.healthLines(r)
	url := d.th.Dim.Render("URL     ") + "https://" + r.Domain + "/"

	rows := []string{title, target, scope, health}
	if errLine != "" {
		rows = append(rows, errLine)
	}
	rows = append(rows, url)

	return boxed(d.th, strings.Join(rows, "\n"), w)
}

func (d Domains) scopeValue(r store.Route) string {
	if !r.Shared {
		return "local — not advertised on LAN"
	}
	ip := ""
	if d.lan != nil {
		if st := d.lan(); st.Advertising {
			ip = st.IP
		}
	}
	if ip == "" {
		return "LAN"
	}
	return "LAN · " + ip
}

// healthLines returns the Health row and (optionally) a follow-up error row
// for the detail panel.
func (d Domains) healthLines(r store.Route) (string, string) {
	pill := d.healthPill(r)
	if d.health == nil {
		return d.th.Dim.Render("Health  ") + pill, ""
	}
	h := d.health(strings.ToLower(r.Domain))
	line := d.th.Dim.Render("Health  ") + pill
	if !h.LastProbe.IsZero() {
		line += " · last probe " + humanDuration(time.Since(h.LastProbe))
	}
	if h.Status == prober.StatusDown && h.LastErr != "" {
		return line, d.th.Dim.Render("error   ") + h.LastErr
	}
	return line, ""
}

// healthPill returns the plain-text pill label matching statusCell semantics
// for the panel (no error appended; the panel renders errors on a separate row).
func (d Domains) healthPill(r store.Route) string {
	if !r.Enabled {
		return "⊘ off"
	}
	if d.health == nil {
		return "● up"
	}
	switch d.health(strings.ToLower(r.Domain)).Status {
	case prober.StatusUp:
		return "● up"
	case prober.StatusDown:
		return "✗ down"
	default:
		return "⊘ off"
	}
}
