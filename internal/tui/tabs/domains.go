package tabs

import (
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui/msg"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// RTTSource resolves the rolling RTT window for a domain. nil = no live RTT.
type RTTSource func(domain string) []time.Duration

type Domains struct {
	th     styles.Theme
	width  int
	height int
	table  table.Model
	routes []store.Route
	rtt    RTTSource
}

// Column widths are the content area. bubbles/table renders each cell with
// Cell.Padding(0,1), so each column occupies Width+2 visible columns.
const (
	domainsStatusW = 8
	domainsShareW  = 5
	domainsSourceW = 10
	domainsAddedW  = 10
	domainsNCols   = 6
	domainsCellPad = 2 // Cell.Padding(0,1) → 1 space each side
	domainsMinW    = 80
)

func NewDomains(th styles.Theme, width, height int) Domains {
	return NewDomainsWithRTT(th, width, height, nil)
}

func NewDomainsWithRTT(th styles.Theme, width, height int, rtt RTTSource) Domains {
	d := Domains{th: th, width: width, height: height, rtt: rtt}
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
	fixed := domainsStatusW + domainsShareW + domainsSourceW + domainsAddedW
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
		{Title: "SOURCE", Width: domainsSourceW},
		{Title: "ADDED", Width: domainsAddedW},
	}
}

func (d Domains) Title() string { return "Domains" }

func (d Domains) Init() tea.Cmd { return nil }

func (d Domains) Update(in tea.Msg) (Domains, tea.Cmd) {
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
	var cmd tea.Cmd
	d.table, cmd = d.table.Update(in)
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
			r.Source,
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
	switch {
	case !r.Enabled:
		return "● off"
	case d.rtt != nil && len(d.rtt(r.Domain)) > 0:
		return "● live"
	default:
		return "● up"
	}
}

func shareCell(shared bool) string {
	if shared {
		return "LAN"
	}
	return "—"
}

func (d Domains) View() string {
	if len(d.routes) == 0 {
		hint := d.th.Dim.Render("no routes yet — press ") + d.th.FooterKey.Render("a") + d.th.Dim.Render(" to add")
		return lipgloss.JoinVertical(lipgloss.Left, hint, d.table.View())
	}
	return d.table.View()
}

func (d Domains) Selected() (store.Route, bool) {
	if len(d.routes) == 0 {
		return store.Route{}, false
	}
	idx := min(max(d.table.Cursor(), 0), len(d.routes)-1)
	return d.routes[idx], true
}
