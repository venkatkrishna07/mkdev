package tabs

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui/components"
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

func NewDomains(th styles.Theme, width, height int) Domains {
	return NewDomainsWithRTT(th, width, height, nil)
}

func NewDomainsWithRTT(th styles.Theme, width, height int, rtt RTTSource) Domains {
	d := Domains{th: th, width: width, height: height, rtt: rtt}
	t := table.New(
		table.WithColumns(d.layoutCols(d.tableWidth())),
		table.WithFocused(true),
		table.WithHeight(8),
	)
	t.SetStyles(tableStyles(th))
	d.table = t
	return d
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

const (
	domainsStatusW    = 12
	domainsSourceW    = 12
	domainsDetailW    = 32
	domainsDetailMinW = 100
)

func (d *Domains) tableWidth() int {
	w := d.width
	if w <= 0 {
		w = 100
	}
	if w >= domainsDetailMinW {
		return w - domainsDetailW - 2
	}
	return w
}

func (d Domains) layoutCols(tableW int) []table.Column {
	fixed := domainsStatusW + domainsSourceW
	rem := tableW - fixed - 8 // padding budget across cols
	if rem < 24 {
		rem = 24
	}
	domW := rem / 2
	tgtW := rem - domW
	return []table.Column{
		{Title: "DOMAIN", Width: domW},
		{Title: "TARGET", Width: tgtW},
		{Title: "STATUS", Width: domainsStatusW},
		{Title: "SOURCE", Width: domainsSourceW},
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
		d.table.SetColumns(d.layoutCols(d.tableWidth()))
		d.fitHeight()
	}
	var cmd tea.Cmd
	d.table, cmd = d.table.Update(in)
	return d, cmd
}

func (d *Domains) fitHeight() {
	rows := max(len(d.routes), 1)
	budget := max(d.height-8, 3)
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
			r.Source,
		}
	}
	d.table.SetRows(rows)
	d.fitHeight()
}

func (d Domains) statusCell(r store.Route) string {
	var pill string
	switch {
	case !r.Enabled:
		pill = d.th.PillOff.Render("● off")
	case d.rtt != nil && len(d.rtt(r.Domain)) > 0:
		pill = d.th.PillUp.Render("● live")
	default:
		pill = d.th.PillUp.Render("● up")
	}
	if r.Shared {
		return pill + " " + d.th.Dim.Render("LAN")
	}
	return pill
}

func (d Domains) View() string {
	w := d.width
	if w <= 0 {
		w = 100
	}
	if len(d.routes) == 0 {
		hint := d.th.Dim.Render("no routes yet — press ") + d.th.FooterKey.Render("a") + d.th.Dim.Render(" to add")
		return lipgloss.JoinVertical(lipgloss.Left, hint, d.table.View())
	}
	if w < domainsDetailMinW {
		return d.table.View()
	}
	leftW := w - domainsDetailW - 2
	left := lipgloss.NewStyle().Width(leftW).Render(d.table.View())
	right := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.th.Primary).
		Padding(0, 1).
		Width(domainsDetailW).
		Render(d.detailPane())
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (d Domains) detailPane() string {
	r, ok := d.Selected()
	if !ok {
		return d.th.Dim.Render("select a route")
	}
	status := "enabled"
	statusStyle := d.th.PillUp
	if !r.Enabled {
		status = "disabled"
		statusStyle = d.th.PillOff
	}
	share := boolWord(r.Shared, "LAN", "local-only")
	added := r.AddedAt.Format("2006-01-02")
	rttRow := d.th.Dim.Render("RTT  ") + d.th.Dim.Render("—")
	if d.rtt != nil {
		xs := d.rtt(r.Domain)
		if len(xs) > 0 {
			last := xs[len(xs)-1].Milliseconds()
			rttRow = d.th.Dim.Render("RTT  ") + components.SparklineDur(d.th, xs, 16) + " " + d.th.Title.Render(fmt.Sprintf("%dms", last))
		}
	}
	lines := []string{
		d.th.Title.Render(r.Domain),
		d.th.Dim.Render("→ ") + r.Target,
		"",
		d.th.Dim.Render("status ") + statusStyle.Render(status),
		d.th.Dim.Render("share  ") + share,
		d.th.Dim.Render("source ") + r.Source,
		d.th.Dim.Render("added  ") + added,
		"",
		rttRow,
	}
	return strings.Join(lines, "\n")
}

func (d Domains) Selected() (store.Route, bool) {
	if len(d.routes) == 0 {
		return store.Route{}, false
	}
	idx := min(max(d.table.Cursor(), 0), len(d.routes)-1)
	return d.routes[idx], true
}

func boolWord(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}
