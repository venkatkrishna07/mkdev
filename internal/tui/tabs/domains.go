package tabs

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui/msg"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// Domains is the Domains tab.
type Domains struct {
	th     styles.Theme
	width  int
	height int
	table  table.Model
	routes []store.Route
}

// NewDomains constructs a Domains tab sized for the given viewport.
func NewDomains(th styles.Theme, width, height int) Domains {
	cols := []table.Column{
		{Title: "DOMAIN", Width: 28},
		{Title: "TARGET", Width: 26},
		{Title: "STATUS", Width: 10},
		{Title: "SOURCE", Width: 20},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(8),
	)
	t.SetStyles(tableStyles(th))
	return Domains{th: th, width: width, height: height, table: t}
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

// Title implements tabs.Tab.
func (d Domains) Title() string { return "Domains" }

// Init implements tea.Model.
func (d Domains) Init() tea.Cmd { return nil }

func (d Domains) Update(in tea.Msg) (Domains, tea.Cmd) {
	switch m := in.(type) {
	case msg.RoutesRefreshed:
		d.routes = m.Routes
		d.refreshRows()
	case tea.WindowSizeMsg:
		d.width = m.Width
		d.height = m.Height
		d.fitHeight()
	}
	var cmd tea.Cmd
	d.table, cmd = d.table.Update(in)
	return d, cmd
}

// fitHeight caps the table height to (routes + 1 header), clamped to the
// remaining viewport budget so the table never wastes vertical space.
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
		status := "✓ up"
		if !r.Enabled {
			status = "⊘ off"
		}
		rows[i] = table.Row{r.Domain, r.Target, status, r.Source}
	}
	d.table.SetRows(rows)
	d.fitHeight()
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
	rule := d.th.Rule.Render(strings.Repeat("─", w))
	return lipgloss.JoinVertical(lipgloss.Left, d.table.View(), rule, d.detail())
}

// Selected returns the currently selected route or false. Cursor index is
// clamped to [0, len) so the freshly-loaded state still resolves a selection.
func (d Domains) Selected() (store.Route, bool) {
	if len(d.routes) == 0 {
		return store.Route{}, false
	}
	idx := min(max(d.table.Cursor(), 0), len(d.routes)-1)
	return d.routes[idx], true
}

// detail renders a single-line inline summary for the selected route.
func (d Domains) detail() string {
	r, ok := d.Selected()
	if !ok {
		return ""
	}
	added := r.AddedAt.Format("2006-01-02")
	status := "enabled"
	if !r.Enabled {
		status = "disabled"
	}
	parts := []string{
		d.th.Title.Render(r.Domain) + d.th.Dim.Render(" → ") + r.Target,
		d.th.Dim.Render("status ") + status,
		d.th.Dim.Render("added ") + added,
		d.th.Dim.Render("source ") + r.Source,
		d.th.Dim.Render("issuer ") + "mkdev CA",
	}
	return strings.Join(parts, d.th.Dim.Render(" · "))
}
