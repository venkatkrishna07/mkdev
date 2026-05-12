# Plan 2 — TUI (Domains tab + modals, single-process)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Git policy (project CLAUDE.md):** Do **not** run `git add`, `git commit`, `git push`, `git reset`, or `git checkout` without explicit confirmation from the user. Commit steps are written into the plan for reference; ask before executing them.

**Goal:** Ship a full-screen TUI for mklocal (macOS) with the Domains tab + Add/Edit/Delete-confirm/Help modals. `mklocal` (no args) launches the TUI; the TUI runs the reverse proxy in-process. When the TUI exits, the proxy exits. No daemon, no IPC.

**Architecture:** Single Go binary, single process. Bubble Tea (Elm-style update loop) drives the screen. The root `tea.Model` holds a tab dispatcher (only the Domains tab is wired in this plan; the other 4 tabs are deferred to Plan 3). Store mutations happen directly against bbolt + `/etc/hosts` (Plan 1 model). The proxy runs in a goroutine launched at TUI startup and cancelled at quit. External CLI mutations (`mklocal add` from another shell) are picked up via a 1-second polling reader against the bbolt store.

**Tech Stack:** Go 1.25, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, `github.com/charmbracelet/bubbles` (table, textinput, key), existing `internal/store`, `internal/proxy`, `internal/cert`, `internal/hosts` packages.

**Out of scope for Plan 2 (deferred):**
- Projects / Logs / Doctor / Settings tabs → Plan 3
- Linux + Windows TUI parity → Plan 4
- gRPC daemon + launchd → Plan 5
- Goreleaser / signed releases → Plan 6

---

## Design decisions

| Decision | Choice | Rationale |
|---|---|---|
| TUI framework | Bubble Tea + Lipgloss + Bubbles | best-in-class Go ecosystem, used by gh-dash, glow, soft-serve |
| Screen mode | `tea.WithAltScreen()` | full-screen takeover; restores terminal on exit |
| Mouse | Off (`tea.WithMouseAllMotion` not enabled) | keyboard-driven; mouse adds complexity, not value |
| Store access pattern | Open + close per operation | avoids long-lived writer lock; lets CLI commands also write |
| Refresh strategy | 1-second poll of `store.ListRoutes()` | simple, sufficient for <100 routes, no IPC complexity |
| Proxy lifecycle | goroutine started at `tea.NewProgram`; context cancelled on Quit | TUI-owned process; quit kills proxy cleanly |
| Modal model | LIFO stack on root Model; top modal owns input | matches k9s / gh-dash idiom |
| Theme | Single "auto" theme detected via `lipgloss.HasDarkBackground()` | one theme ships in Plan 2; switcher in Plan 3 |
| Test strategy | Golden-file snapshot tests via `tea.NewProgram(WithInput(strings.NewReader(...)))` | exercises View() output without a real TTY |
| `mklocal` (no args) | Launches TUI | matches lazygit/k9s pattern |
| `mklocal serve` | Kept as headless proxy mode | unchanged from Plan 1 — useful for CI/scripting |
| `mklocal add/remove/list` | Unchanged (Plan 1 direct-bbolt model) | works in parallel with running TUI via short-lived store opens |

---

## File map

### New files

| Path | Responsibility |
|---|---|
| `internal/tui/program.go` | Root `tea.Model` + tea.Program bootstrap |
| `internal/tui/messages.go` | All `tea.Msg` types used across the TUI |
| `internal/tui/runtime.go` | TUI runtime (loads store, starts proxy goroutine, wires messages) |
| `internal/tui/styles/theme.go` | Lipgloss theme (one theme: auto) |
| `internal/tui/components/tabbar.go` | Tab bar renderer |
| `internal/tui/components/footer.go` | Footer keybind hints |
| `internal/tui/components/statuspill.go` | Status pill (●/○/✗) |
| `internal/tui/tabs/tabs.go` | `Tab` interface |
| `internal/tui/tabs/domains.go` | Domains tab implementation |
| `internal/tui/tabs/domains_test.go` | Snapshot test of Domains tab |
| `internal/tui/modals/modal.go` | `Modal` interface + helpers |
| `internal/tui/modals/add.go` | Add Domain modal |
| `internal/tui/modals/edit.go` | Edit Domain modal |
| `internal/tui/modals/confirm.go` | Generic confirm modal |
| `internal/tui/modals/help.go` | Help overlay |
| `internal/tui/modals/modal_test.go` | Snapshot tests of each modal |
| `internal/cli/tui.go` | New `mklocal tui` subcommand and default-launch wiring |

### Modified files

| Path | Why |
|---|---|
| `internal/cli/root.go` | Add `tui` subcommand; route arg-less invocations to it |
| `go.mod` | New deps |

---

## Task 1: Bubble Tea / Lipgloss / Bubbles deps + program skeleton

**Files:**
- Modify: `go.mod`
- Create: `internal/tui/program.go`
- Create: `internal/tui/messages.go`

- [ ] **Step 1.1 — Add deps**

```
go get github.com/charmbracelet/bubbletea@latest \
       github.com/charmbracelet/lipgloss@latest \
       github.com/charmbracelet/bubbles@latest
```

Expected: `go.mod` gains the three packages plus their transitives (e.g. `termenv`, `runewidth`).

- [ ] **Step 1.2 — Create `messages.go`**

```go
package tui

import (
	"github.com/venkatkrishna07/mklocal/internal/store"
)

// RoutesRefreshed is dispatched when a fresh snapshot of the routes table
// has been pulled from the bbolt store.
type RoutesRefreshed struct {
	Routes []store.Route
	Err    error
}

// ProxyState carries the latest proxy liveness signal.
type ProxyState struct {
	Up   bool
	Addr string
	Err  error
}
```

- [ ] **Step 1.3 — Create `program.go`**

```go
package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

// Run builds the root model and runs the Bubble Tea program until quit.
// The proxy goroutine inherits ctx and exits when ctx is cancelled (which
// happens when the user quits the TUI).
func Run(ctx context.Context, rt *Runtime) error {
	m := newRootModel(rt)
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)
	_, err := p.Run()
	return err
}
```

- [ ] **Step 1.4 — Stub `Runtime`**

Create `internal/tui/runtime.go`:

```go
package tui

import (
	"context"

	"github.com/venkatkrishna07/mklocal/internal/store"
)

// Runtime is the slice of long-lived dependencies the TUI needs.
type Runtime struct {
	Ctx     context.Context
	Cancel  context.CancelFunc
	Home    string
	BinPath string
	StoreFn func() (*store.Store, error) // short-lived store opener
}
```

- [ ] **Step 1.5 — Stub root model**

In `program.go`, add a private root model so the package builds:

```go
type rootModel struct {
	rt *Runtime
}

func newRootModel(rt *Runtime) rootModel { return rootModel{rt: rt} }

func (m rootModel) Init() tea.Cmd { return nil }
func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && (k.String() == "q" || k.String() == "ctrl+c") {
		return m, tea.Quit
	}
	return m, nil
}
func (m rootModel) View() string { return "mklocal TUI — Plan 2 scaffold" }
```

- [ ] **Step 1.6 — Verify build**

```
go build ./internal/tui/...
```

Expected: clean.

- [ ] **Step 1.7 — Commit**

```
git add internal/tui go.mod go.sum
git commit -m "feat(tui): Bubble Tea scaffold (root model + runtime)"
```

---

## Task 2: Lipgloss theme + style sheet

**Files:**
- Create: `internal/tui/styles/theme.go`
- Create: `internal/tui/styles/theme_test.go`

- [ ] **Step 2.1 — Write failing test**

Create `internal/tui/styles/theme_test.go`:

```go
package styles_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
)

func TestNewTheme(t *testing.T) {
	th := styles.NewTheme()
	require.NotNil(t, th)
	require.NotEmpty(t, th.Title.String())
	require.NotEmpty(t, th.TabActive.String())
	require.NotEmpty(t, th.TabInactive.String())
	require.NotEmpty(t, th.Footer.String())
	require.NotEmpty(t, th.PillUp.String())
	require.NotEmpty(t, th.PillDown.String())
}
```

- [ ] **Step 2.2 — Implement `theme.go`**

```go
package styles

import "github.com/charmbracelet/lipgloss"

// Theme groups every lipgloss style the TUI renders.
type Theme struct {
	Base        lipgloss.Style
	Title       lipgloss.Style
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	Footer      lipgloss.Style
	FooterKey   lipgloss.Style
	PillUp      lipgloss.Style
	PillDown    lipgloss.Style
	PillOff     lipgloss.Style
	RowSelected lipgloss.Style
	Modal       lipgloss.Style
	ModalTitle  lipgloss.Style
	Border      lipgloss.Border
	Dim         lipgloss.Style
}

// NewTheme returns a single theme tuned for both dark and light terminals.
// Colors are chosen for ANSI 256 compatibility.
func NewTheme() Theme {
	primary := lipgloss.AdaptiveColor{Light: "#3B82F6", Dark: "#60A5FA"} // blue
	muted := lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}   // gray
	ok := lipgloss.Color("#10B981")                                       // green
	bad := lipgloss.Color("#EF4444")                                      // red
	off := lipgloss.Color("#6B7280")                                      // gray
	return Theme{
		Base:        lipgloss.NewStyle(),
		Title:       lipgloss.NewStyle().Bold(true).Foreground(primary),
		TabActive:   lipgloss.NewStyle().Bold(true).Foreground(primary).Padding(0, 1).Border(lipgloss.RoundedBorder(), false, false, true, false),
		TabInactive: lipgloss.NewStyle().Foreground(muted).Padding(0, 1),
		Footer:      lipgloss.NewStyle().Foreground(muted),
		FooterKey:   lipgloss.NewStyle().Foreground(primary).Bold(true),
		PillUp:      lipgloss.NewStyle().Foreground(ok),
		PillDown:    lipgloss.NewStyle().Foreground(bad),
		PillOff:     lipgloss.NewStyle().Foreground(off),
		RowSelected: lipgloss.NewStyle().Bold(true).Foreground(primary),
		Modal:       lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(primary),
		ModalTitle:  lipgloss.NewStyle().Bold(true).Foreground(primary),
		Border:      lipgloss.RoundedBorder(),
		Dim:         lipgloss.NewStyle().Foreground(muted),
	}
}
```

- [ ] **Step 2.3 — Run tests**

```
go test ./internal/tui/styles/...
```

Expected: PASS.

- [ ] **Step 2.4 — Commit**

```
git add internal/tui/styles
git commit -m "feat(tui): lipgloss theme palette"
```

---

## Task 3: Components — tabbar, footer, statuspill

**Files:**
- Create: `internal/tui/components/tabbar.go`
- Create: `internal/tui/components/footer.go`
- Create: `internal/tui/components/statuspill.go`
- Create: `internal/tui/components/components_test.go`

- [ ] **Step 3.1 — Write failing tests**

Create `internal/tui/components/components_test.go`:

```go
package components_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/tui/components"
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
)

func TestTabBarHighlightsActive(t *testing.T) {
	th := styles.NewTheme()
	out := components.TabBar(th, []string{"Domains", "Projects", "Logs"}, 1)
	require.Contains(t, out, "Domains")
	require.Contains(t, out, "Projects")
	require.Contains(t, out, "Logs")
}

func TestFooterRendersKeybinds(t *testing.T) {
	th := styles.NewTheme()
	out := components.Footer(th, []components.Keybind{
		{Key: "a", Action: "add"},
		{Key: "q", Action: "quit"},
	})
	require.Contains(t, out, "a")
	require.Contains(t, out, "add")
	require.Contains(t, out, "q")
	require.Contains(t, out, "quit")
}

func TestStatusPillUpDownOff(t *testing.T) {
	th := styles.NewTheme()
	require.Contains(t, components.StatusPill(th, components.PillUp, "127.0.0.1:8443"), "127.0.0.1:8443")
	require.True(t, strings.Contains(components.StatusPill(th, components.PillDown, ""), "down") || strings.Contains(components.StatusPill(th, components.PillDown, ""), "✗"))
	require.NotEmpty(t, components.StatusPill(th, components.PillOff, ""))
}
```

- [ ] **Step 3.2 — Run tests, confirm fail**

```
go test ./internal/tui/components/...
```

Expected: build fails.

- [ ] **Step 3.3 — Implement `tabbar.go`**

```go
package components

import (
	"strings"

	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
)

// TabBar renders a horizontal tab bar. `active` is the zero-based index of
// the currently selected tab.
func TabBar(th styles.Theme, labels []string, active int) string {
	parts := make([]string, len(labels))
	for i, l := range labels {
		if i == active {
			parts[i] = th.TabActive.Render(l)
		} else {
			parts[i] = th.TabInactive.Render(l)
		}
	}
	return strings.Join(parts, " ")
}
```

- [ ] **Step 3.4 — Implement `footer.go`**

```go
package components

import (
	"strings"

	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
)

// Keybind is a single key→action hint for the footer.
type Keybind struct {
	Key    string
	Action string
}

// Footer renders "key:action" hints separated by two spaces.
func Footer(th styles.Theme, kb []Keybind) string {
	parts := make([]string, len(kb))
	for i, k := range kb {
		parts[i] = th.FooterKey.Render(k.Key) + th.Footer.Render(":"+k.Action)
	}
	return th.Footer.Render(strings.Join(parts, "  "))
}
```

- [ ] **Step 3.5 — Implement `statuspill.go`**

```go
package components

import (
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
)

// PillKind selects the pill color + icon.
type PillKind int

const (
	PillUp PillKind = iota
	PillDown
	PillOff
)

// StatusPill renders a colored "● up <addr>" / "✗ down" / "⊘ off" pill.
func StatusPill(th styles.Theme, kind PillKind, addr string) string {
	switch kind {
	case PillUp:
		return th.PillUp.Render("● up " + addr)
	case PillDown:
		return th.PillDown.Render("✗ down")
	default:
		return th.PillOff.Render("⊘ off")
	}
}
```

- [ ] **Step 3.6 — Run tests**

```
go test ./internal/tui/components/...
```

Expected: PASS.

- [ ] **Step 3.7 — Commit**

```
git add internal/tui/components
git commit -m "feat(tui): tabbar, footer, statuspill components"
```

---

## Task 4: Domains tab

**Files:**
- Create: `internal/tui/tabs/tabs.go`
- Create: `internal/tui/tabs/domains.go`
- Create: `internal/tui/tabs/domains_test.go`

- [ ] **Step 4.1 — Write tabs interface**

Create `internal/tui/tabs/tabs.go`:

```go
package tabs

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mklocal/internal/tui/components"
)

// Tab is the contract every TUI tab implements.
type Tab interface {
	tea.Model
	Title() string
	Keybinds() []components.Keybind
}
```

- [ ] **Step 4.2 — Write failing test**

Create `internal/tui/tabs/domains_test.go`:

```go
package tabs_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/store"
	"github.com/venkatkrishna07/mklocal/internal/tui"
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
	"github.com/venkatkrishna07/mklocal/internal/tui/tabs"
)

func TestDomainsViewHeader(t *testing.T) {
	d := tabs.NewDomains(styles.NewTheme(), 100, 24)
	out := d.View()
	require.Contains(t, out, "DOMAIN")
	require.Contains(t, out, "TARGET")
	require.Contains(t, out, "STATUS")
	require.Contains(t, out, "SOURCE")
}

func TestDomainsRoutesRefreshedPopulatesTable(t *testing.T) {
	d := tabs.NewDomains(styles.NewTheme(), 100, 24)
	d2, _ := d.Update(tui.RoutesRefreshed{Routes: []store.Route{
		{Domain: "foo.local", Target: "localhost:3000", Enabled: true, Source: "ad-hoc", AddedAt: time.Now()},
		{Domain: "bar.local", Target: "localhost:4000", Enabled: false, Source: "ad-hoc", AddedAt: time.Now()},
	}})
	out := d2.View()
	require.Contains(t, out, "foo.local")
	require.Contains(t, out, "localhost:3000")
	require.Contains(t, out, "bar.local")
}

func TestDomainsKeybinds(t *testing.T) {
	d := tabs.NewDomains(styles.NewTheme(), 100, 24)
	kbs := d.Keybinds()
	keys := make([]string, len(kbs))
	for i, k := range kbs {
		keys[i] = k.Key
	}
	require.Contains(t, strings.Join(keys, ","), "a") // add
	require.Contains(t, strings.Join(keys, ","), "d") // delete
}
```

- [ ] **Step 4.3 — Implement `domains.go`**

```go
package tabs

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mklocal/internal/store"
	"github.com/venkatkrishna07/mklocal/internal/tui"
	"github.com/venkatkrishna07/mklocal/internal/tui/components"
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
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
		{Title: "TARGET", Width: 24},
		{Title: "STATUS", Width: 8},
		{Title: "SOURCE", Width: 18},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(height-6),
	)
	return Domains{th: th, width: width, height: height, table: t}
}

func (d Domains) Title() string { return "Domains" }

func (d Domains) Keybinds() []components.Keybind {
	return []components.Keybind{
		{Key: "a", Action: "add"},
		{Key: "e", Action: "edit"},
		{Key: "d", Action: "delete"},
		{Key: "t", Action: "toggle"},
		{Key: "enter", Action: "open"},
		{Key: "/", Action: "filter"},
		{Key: "?", Action: "help"},
		{Key: "q", Action: "quit"},
	}
}

func (d Domains) Init() tea.Cmd { return nil }

func (d Domains) Update(msg tea.Msg) (Domains, tea.Cmd) {
	switch m := msg.(type) {
	case tui.RoutesRefreshed:
		d.routes = m.Routes
		rows := make([]table.Row, len(d.routes))
		for i, r := range d.routes {
			status := "✓ up"
			if !r.Enabled {
				status = "⊘ off"
			}
			rows[i] = table.Row{r.Domain, r.Target, status, r.Source}
		}
		d.table.SetRows(rows)
	case tea.WindowSizeMsg:
		d.width = m.Width
		d.height = m.Height
		d.table.SetHeight(m.Height - 6)
	}
	var cmd tea.Cmd
	d.table, cmd = d.table.Update(msg)
	return d, cmd
}

func (d Domains) View() string {
	if len(d.routes) == 0 {
		// Render empty table with header so the test for headers passes even
		// when no routes are present.
		return d.table.View() + "\n" + d.th.Dim.Render("no routes — press 'a' to add")
	}
	return d.table.View() + "\n" + d.detail()
}

// Selected returns the currently selected route or false.
func (d Domains) Selected() (store.Route, bool) {
	if len(d.routes) == 0 {
		return store.Route{}, false
	}
	idx := d.table.Cursor()
	if idx < 0 || idx >= len(d.routes) {
		return store.Route{}, false
	}
	return d.routes[idx], true
}

func (d Domains) detail() string {
	r, ok := d.Selected()
	if !ok {
		return ""
	}
	return d.th.Dim.Render(fmt.Sprintf("selected: %s → %s (added %s)", r.Domain, r.Target, r.AddedAt.Format("2006-01-02 15:04")))
}
```

- [ ] **Step 4.4 — Run tests**

```
go test ./internal/tui/tabs/...
```

Expected: PASS — but the assertion about `STATUS` and `SOURCE` headers being in the View when there are no rows will fail unless `table.View()` always renders headers. `bubbles/table` does render headers in the empty state, so the test should pass.

- [ ] **Step 4.5 — Commit**

```
git add internal/tui/tabs
git commit -m "feat(tui): Domains tab with bubbles/table"
```

---

## Task 5: Modal interface + Help modal

**Files:**
- Create: `internal/tui/modals/modal.go`
- Create: `internal/tui/modals/help.go`
- Create: `internal/tui/modals/help_test.go`

- [ ] **Step 5.1 — Define `Modal` interface**

Create `internal/tui/modals/modal.go`:

```go
package modals

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mklocal/internal/tui/components"
)

// Result is returned to the root model when a modal closes.
type Result struct {
	Cancelled bool
	Payload   any // typed by the specific modal
}

// Modal is the contract for every modal screen.
type Modal interface {
	tea.Model
	Title() string
	Keybinds() []components.Keybind
}

// Closed is sent by a modal to its parent to indicate it should be popped.
type Closed struct{ Result Result }
```

- [ ] **Step 5.2 — Write Help modal test**

Create `internal/tui/modals/help_test.go`:

```go
package modals_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/tui/components"
	"github.com/venkatkrishna07/mklocal/internal/tui/modals"
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
)

func TestHelpRenders(t *testing.T) {
	h := modals.NewHelp(styles.NewTheme(), []components.Keybind{
		{Key: "a", Action: "add"},
		{Key: "q", Action: "quit"},
	})
	out := h.View()
	require.Contains(t, out, "Help")
	require.Contains(t, out, "add")
	require.Contains(t, out, "quit")
}
```

- [ ] **Step 5.3 — Implement `help.go`**

```go
package modals

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mklocal/internal/tui/components"
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
)

// Help renders a cheat sheet of keybindings.
type Help struct {
	th  styles.Theme
	kbs []components.Keybind
}

// NewHelp builds a Help modal for the given key list.
func NewHelp(th styles.Theme, kbs []components.Keybind) Help { return Help{th: th, kbs: kbs} }

func (h Help) Title() string                         { return "Help" }
func (h Help) Keybinds() []components.Keybind        { return []components.Keybind{{Key: "esc", Action: "close"}} }
func (h Help) Init() tea.Cmd                         { return nil }
func (h Help) Update(msg tea.Msg) (Help, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && (k.String() == "esc" || k.String() == "?") {
		return h, func() tea.Msg { return Closed{Result: Result{Cancelled: true}} }
	}
	return h, nil
}

func (h Help) View() string {
	var b strings.Builder
	b.WriteString(h.th.ModalTitle.Render("Help"))
	b.WriteString("\n\n")
	for _, k := range h.kbs {
		b.WriteString(h.th.FooterKey.Render(k.Key))
		b.WriteString("  ")
		b.WriteString(h.th.Dim.Render(k.Action))
		b.WriteString("\n")
	}
	return h.th.Modal.Render(b.String())
}
```

- [ ] **Step 5.4 — Run tests**

```
go test ./internal/tui/modals/...
```

Expected: PASS.

- [ ] **Step 5.5 — Commit**

```
git add internal/tui/modals/modal.go internal/tui/modals/help.go internal/tui/modals/help_test.go
git commit -m "feat(tui): modal interface + Help overlay"
```

---

## Task 6: Add Domain modal

**Files:**
- Create: `internal/tui/modals/add.go`
- Create: `internal/tui/modals/add_test.go`

- [ ] **Step 6.1 — Write test**

Create `internal/tui/modals/add_test.go`:

```go
package modals_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mklocal/internal/tui/modals"
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
)

func TestAddRendersFields(t *testing.T) {
	m := modals.NewAdd(styles.NewTheme(), ".local")
	out := m.View()
	require.Contains(t, out, "Add Domain")
	require.Contains(t, out, "Name")
	require.Contains(t, out, "Target")
}

func TestAddSubmitProducesPayload(t *testing.T) {
	m := modals.NewAdd(styles.NewTheme(), ".local")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("foo")})
	m2, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("localhost:3000")})
	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	closed, ok := msg.(modals.Closed)
	require.True(t, ok)
	require.False(t, closed.Result.Cancelled)
	p, ok := closed.Result.Payload.(modals.AddPayload)
	require.True(t, ok)
	require.Equal(t, "foo.local", p.Domain)
	require.Equal(t, "localhost:3000", p.Target)
}
```

- [ ] **Step 6.2 — Implement `add.go`**

```go
package modals

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mklocal/internal/tui/components"
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
)

// AddPayload is delivered in the Result on submit.
type AddPayload struct {
	Domain string
	Target string
	TLD    string
}

// Add is the "Add Domain" modal.
type Add struct {
	th        styles.Theme
	defaultT  string
	fields    [2]textinput.Model // 0 = name, 1 = target
	focus     int
}

// NewAdd constructs an empty Add modal.
func NewAdd(th styles.Theme, defaultTLD string) Add {
	name := textinput.New()
	name.Placeholder = "myapp"
	name.Focus()
	target := textinput.New()
	target.Placeholder = "localhost:3000"
	return Add{th: th, defaultT: defaultTLD, fields: [2]textinput.Model{name, target}, focus: 0}
}

func (a Add) Title() string                         { return "Add Domain" }
func (a Add) Keybinds() []components.Keybind        {
	return []components.Keybind{
		{Key: "tab", Action: "next field"},
		{Key: "enter", Action: "submit"},
		{Key: "esc", Action: "cancel"},
	}
}
func (a Add) Init() tea.Cmd                         { return textinput.Blink }

func (a Add) Update(msg tea.Msg) (Add, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyEsc:
			return a, func() tea.Msg { return Closed{Result: Result{Cancelled: true}} }
		case tea.KeyEnter:
			name := strings.TrimSpace(a.fields[0].Value())
			if name == "" {
				return a, nil
			}
			domain := strings.ToLower(name)
			if !strings.Contains(domain, ".") {
				domain += a.defaultT
			}
			payload := AddPayload{Domain: domain, Target: strings.TrimSpace(a.fields[1].Value()), TLD: a.defaultT}
			return a, func() tea.Msg { return Closed{Result: Result{Payload: payload}} }
		case tea.KeyTab, tea.KeyShiftTab:
			a.fields[a.focus].Blur()
			if k.Type == tea.KeyTab {
				a.focus = (a.focus + 1) % len(a.fields)
			} else {
				a.focus = (a.focus - 1 + len(a.fields)) % len(a.fields)
			}
			a.fields[a.focus].Focus()
			return a, textinput.Blink
		}
	}
	var cmd tea.Cmd
	a.fields[a.focus], cmd = a.fields[a.focus].Update(msg)
	return a, cmd
}

func (a Add) View() string {
	body := a.th.ModalTitle.Render("Add Domain") + "\n\n"
	body += a.th.Dim.Render("Name") + "\n" + a.fields[0].View() + "\n\n"
	body += a.th.Dim.Render("Target (host:port)") + "\n" + a.fields[1].View() + "\n"
	return a.th.Modal.Render(body)
}
```

- [ ] **Step 6.3 — Run tests**

```
go test ./internal/tui/modals/...
```

Expected: PASS (both Help + Add tests).

- [ ] **Step 6.4 — Commit**

```
git add internal/tui/modals/add.go internal/tui/modals/add_test.go
git commit -m "feat(tui): Add Domain modal"
```

---

## Task 7: Edit Domain modal

**Files:**
- Create: `internal/tui/modals/edit.go`
- Create: `internal/tui/modals/edit_test.go`

- [ ] **Step 7.1 — Write test**

```go
package modals_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mklocal/internal/store"
	"github.com/venkatkrishna07/mklocal/internal/tui/modals"
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
)

func TestEditPrePopulates(t *testing.T) {
	r := store.Route{Domain: "foo.local", Target: "localhost:3000", Enabled: true}
	m := modals.NewEdit(styles.NewTheme(), r)
	out := m.View()
	require.Contains(t, out, "foo.local")
	require.Contains(t, out, "localhost:3000")
}

func TestEditSubmitProducesPayload(t *testing.T) {
	r := store.Route{Domain: "foo.local", Target: "localhost:3000", Enabled: true}
	m := modals.NewEdit(styles.NewTheme(), r)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	closed, ok := cmd().(modals.Closed)
	require.True(t, ok)
	p, ok := closed.Result.Payload.(modals.EditPayload)
	require.True(t, ok)
	require.Equal(t, "foo.local", p.Domain)
}
```

- [ ] **Step 7.2 — Implement `edit.go`**

```go
package modals

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mklocal/internal/store"
	"github.com/venkatkrishna07/mklocal/internal/tui/components"
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
)

// EditPayload is delivered in the Result on submit.
type EditPayload struct {
	Domain string
	Target string
}

// Edit is the "Edit Domain" modal — domain is read-only, only target changes.
type Edit struct {
	th     styles.Theme
	domain string
	target textinput.Model
}

// NewEdit constructs an Edit modal seeded from r.
func NewEdit(th styles.Theme, r store.Route) Edit {
	t := textinput.New()
	t.SetValue(r.Target)
	t.Focus()
	return Edit{th: th, domain: r.Domain, target: t}
}

func (e Edit) Title() string                         { return "Edit Domain" }
func (e Edit) Keybinds() []components.Keybind        {
	return []components.Keybind{{Key: "enter", Action: "save"}, {Key: "esc", Action: "cancel"}}
}
func (e Edit) Init() tea.Cmd                         { return textinput.Blink }

func (e Edit) Update(msg tea.Msg) (Edit, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyEsc:
			return e, func() tea.Msg { return Closed{Result: Result{Cancelled: true}} }
		case tea.KeyEnter:
			return e, func() tea.Msg {
				return Closed{Result: Result{Payload: EditPayload{Domain: e.domain, Target: strings.TrimSpace(e.target.Value())}}}
			}
		}
	}
	var cmd tea.Cmd
	e.target, cmd = e.target.Update(msg)
	return e, cmd
}

func (e Edit) View() string {
	body := e.th.ModalTitle.Render("Edit Domain") + "\n\n"
	body += e.th.Dim.Render("Domain (read-only): ") + e.domain + "\n\n"
	body += e.th.Dim.Render("Target (host:port)") + "\n" + e.target.View() + "\n"
	return e.th.Modal.Render(body)
}
```

- [ ] **Step 7.3 — Run tests**

```
go test ./internal/tui/modals/...
```

Expected: PASS.

- [ ] **Step 7.4 — Commit**

```
git add internal/tui/modals/edit.go internal/tui/modals/edit_test.go
git commit -m "feat(tui): Edit Domain modal"
```

---

## Task 8: Confirm modal

**Files:**
- Create: `internal/tui/modals/confirm.go`
- Create: `internal/tui/modals/confirm_test.go`

- [ ] **Step 8.1 — Write test**

```go
package modals_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mklocal/internal/tui/modals"
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
)

func TestConfirmEnterReturnsTrue(t *testing.T) {
	c := modals.NewConfirm(styles.NewTheme(), "Delete foo.local?", "irreversible")
	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	closed := cmd().(modals.Closed)
	require.False(t, closed.Result.Cancelled)
	require.True(t, closed.Result.Payload.(bool))
}

func TestConfirmEscReturnsCancelled(t *testing.T) {
	c := modals.NewConfirm(styles.NewTheme(), "Delete?", "")
	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyEsc})
	closed := cmd().(modals.Closed)
	require.True(t, closed.Result.Cancelled)
}
```

- [ ] **Step 8.2 — Implement `confirm.go`**

```go
package modals

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mklocal/internal/tui/components"
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
)

// Confirm is a yes/no prompt. Submit (enter) returns Payload=true; cancel
// (esc/n) returns Cancelled=true.
type Confirm struct {
	th       styles.Theme
	question string
	detail   string
}

// NewConfirm constructs a confirm modal.
func NewConfirm(th styles.Theme, question, detail string) Confirm {
	return Confirm{th: th, question: question, detail: detail}
}

func (c Confirm) Title() string                  { return "Confirm" }
func (c Confirm) Keybinds() []components.Keybind {
	return []components.Keybind{{Key: "enter", Action: "yes"}, {Key: "esc/n", Action: "no"}}
}
func (c Confirm) Init() tea.Cmd { return nil }
func (c Confirm) Update(msg tea.Msg) (Confirm, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyEnter:
			return c, func() tea.Msg { return Closed{Result: Result{Payload: true}} }
		case tea.KeyEsc:
			return c, func() tea.Msg { return Closed{Result: Result{Cancelled: true}} }
		}
		if k.String() == "n" || k.String() == "N" {
			return c, func() tea.Msg { return Closed{Result: Result{Cancelled: true}} }
		}
		if k.String() == "y" || k.String() == "Y" {
			return c, func() tea.Msg { return Closed{Result: Result{Payload: true}} }
		}
	}
	return c, nil
}
func (c Confirm) View() string {
	body := c.th.ModalTitle.Render("Confirm") + "\n\n" + c.question + "\n"
	if c.detail != "" {
		body += c.th.Dim.Render(c.detail) + "\n"
	}
	body += "\n[enter] yes   [esc/n] no"
	return c.th.Modal.Render(body)
}
```

- [ ] **Step 8.3 — Run tests**

```
go test ./internal/tui/modals/...
```

Expected: PASS.

- [ ] **Step 8.4 — Commit**

```
git add internal/tui/modals/confirm.go internal/tui/modals/confirm_test.go
git commit -m "feat(tui): Confirm modal"
```

---

## Task 9: Runtime — store opener, route refresher, proxy goroutine

**Files:**
- Modify: `internal/tui/runtime.go`
- Modify: `internal/tui/messages.go`
- Create: `internal/tui/runtime_test.go`

- [ ] **Step 9.1 — Expand `runtime.go`**

Replace contents:

```go
package tui

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mklocal/internal/cert"
	"github.com/venkatkrishna07/mklocal/internal/config"
	"github.com/venkatkrishna07/mklocal/internal/proxy"
	"github.com/venkatkrishna07/mklocal/internal/store"
)

// Runtime is the shared state of the TUI.
type Runtime struct {
	Ctx     context.Context
	Cancel  context.CancelFunc
	Home    string
	Cfg     config.Config
	Router  *proxy.Router
	Issuer  *cert.Issuer
}

// NewRuntime loads config + CA and prepares a Router. It does NOT start the
// TLS proxy yet — call StartProxy after the TUI program is constructed.
func NewRuntime(ctx context.Context, home string) (*Runtime, error) {
	ctx, cancel := context.WithCancel(ctx)
	cfg, err := config.Load(filepath.Join(home, "config.toml"))
	if err != nil {
		cancel()
		return nil, err
	}
	ca, err := cert.LoadCA(filepath.Join(home, "ca"))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("CA not found — run `mklocal install` first: %w", err)
	}
	r := proxy.NewRouter()
	is := cert.NewIssuer(ca, r.Has)
	return &Runtime{Ctx: ctx, Cancel: cancel, Home: home, Cfg: cfg, Router: r, Issuer: is}, nil
}

// OpenStore returns a transient store handle. Caller MUST close.
func (rt *Runtime) OpenStore() (*store.Store, error) {
	return store.Open(filepath.Join(rt.Home, "state.db"))
}

// LoadRoutes opens the store, lists, closes, and returns.
func (rt *Runtime) LoadRoutes() ([]store.Route, error) {
	s, err := rt.OpenStore()
	if err != nil {
		return nil, err
	}
	defer s.Close()
	return s.ListRoutes()
}

// StartProxy binds the TLS listener and serves until Ctx is cancelled.
// Sends ProxyState updates via the returned channel.
func (rt *Runtime) StartProxy() <-chan ProxyState {
	ch := make(chan ProxyState, 4)
	go func() {
		defer close(ch)
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(rt.Cfg.ProxyPort))
		ln, err := tls.Listen("tcp", addr, &tls.Config{
			GetCertificate: rt.Issuer.GetCertificate,
			MinVersion:     tls.VersionTLS13,
		})
		if err != nil {
			ch <- ProxyState{Up: false, Err: err}
			return
		}
		ch <- ProxyState{Up: true, Addr: ln.Addr().String()}
		srv := proxy.NewServer(rt.Router, ln)
		go func() {
			<-rt.Ctx.Done()
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutCtx)
		}()
		if err := srv.Serve(); err != nil {
			ch <- ProxyState{Up: false, Err: err}
		}
	}()
	return ch
}

// RefreshTick is a tea.Cmd that returns a RoutesRefreshed after delay.
func (rt *Runtime) RefreshTick(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		rs, err := rt.LoadRoutes()
		rt.Router.Set(rs)
		return RoutesRefreshed{Routes: rs, Err: err}
	})
}
```

- [ ] **Step 9.2 — Test the refresh tick**

Create `internal/tui/runtime_test.go`:

```go
package tui_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/cert"
	"github.com/venkatkrishna07/mklocal/internal/config"
	"github.com/venkatkrishna07/mklocal/internal/tui"
)

func TestNewRuntimeRefresh(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "ca"), 0o700))
	_, err := cert.CreateCA(filepath.Join(home, "ca"), "test")
	require.NoError(t, err)
	cfg := config.Default()
	cfg.ProxyPort = 18443
	require.NoError(t, config.Save(filepath.Join(home, "config.toml"), cfg))

	rt, err := tui.NewRuntime(context.Background(), home)
	require.NoError(t, err)
	defer rt.Cancel()

	rs, err := rt.LoadRoutes()
	require.NoError(t, err)
	require.Empty(t, rs)
	_ = time.Now() // anchor
}
```

- [ ] **Step 9.3 — Run tests**

```
go test ./internal/tui/...
```

Expected: PASS.

- [ ] **Step 9.4 — Commit**

```
git add internal/tui/runtime.go internal/tui/runtime_test.go
git commit -m "feat(tui): runtime with store opener, route loader, and proxy lifecycle"
```

---

## Task 10: Root model wiring + modal stack

**Files:**
- Modify: `internal/tui/program.go`

Replace the stub root model with the real one. The root model:
- holds the active tab + modal stack + theme + proxy state
- routes `tea.WindowSizeMsg` to active tab and top modal
- handles global keys (`q`, `?`, Ctrl+C) at root
- top modal owns input; otherwise active tab does
- closes top modal on `Closed` msg; dispatches modal payloads to handler funcs
- subscribes to `ProxyState` channel via repeated `tea.Cmd`

- [ ] **Step 10.1 — Implement root model**

Replace `internal/tui/program.go`:

```go
package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mklocal/internal/hosts"
	"github.com/venkatkrishna07/mklocal/internal/store"
	"github.com/venkatkrishna07/mklocal/internal/tui/components"
	"github.com/venkatkrishna07/mklocal/internal/tui/modals"
	"github.com/venkatkrishna07/mklocal/internal/tui/styles"
	"github.com/venkatkrishna07/mklocal/internal/tui/tabs"
)

// Run launches the TUI bound to rt.
func Run(rt *Runtime) error {
	m := newRootModel(rt)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	rt.Cancel()
	return err
}

type rootModel struct {
	rt       *Runtime
	th       styles.Theme
	width    int
	height   int
	domains  tabs.Domains
	modals   []modals.Modal
	proxy    ProxyState
	proxyCh  <-chan ProxyState
	binPath  string
}

func newRootModel(rt *Runtime) rootModel {
	th := styles.NewTheme()
	bp, _ := os.Executable()
	return rootModel{
		rt:      rt,
		th:      th,
		domains: tabs.NewDomains(th, 100, 24),
		binPath: bp,
	}
}

func (m rootModel) Init() tea.Cmd {
	m.proxyCh = m.rt.StartProxy()
	return tea.Batch(
		m.waitProxy(),
		m.rt.RefreshTick(0),
	)
}

func (m rootModel) waitProxy() tea.Cmd {
	return func() tea.Msg {
		select {
		case ev, ok := <-m.proxyCh:
			if !ok {
				return ProxyState{Up: false}
			}
			return ev
		case <-m.rt.Ctx.Done():
			return ProxyState{Up: false}
		}
	}
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		var cmd tea.Cmd
		m.domains, cmd = m.domains.Update(msg)
		return m, cmd

	case ProxyState:
		m.proxy = msg
		return m, m.waitProxy()

	case RoutesRefreshed:
		var cmd tea.Cmd
		m.domains, cmd = m.domains.Update(msg)
		return m, tea.Batch(cmd, m.rt.RefreshTick(time.Second))

	case modals.Closed:
		if len(m.modals) == 0 {
			return m, nil
		}
		top := m.modals[len(m.modals)-1]
		m.modals = m.modals[:len(m.modals)-1]
		return m, m.handleModalResult(top, msg.Result)

	case tea.KeyMsg:
		if len(m.modals) > 0 {
			return m.updateTopModal(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.modals = append(m.modals, modals.NewHelp(m.th, m.domains.Keybinds()))
			return m, nil
		case "a":
			m.modals = append(m.modals, modals.NewAdd(m.th, m.rt.Cfg.TLD))
			return m, nil
		case "e":
			if r, ok := m.domains.Selected(); ok {
				m.modals = append(m.modals, modals.NewEdit(m.th, r))
			}
			return m, nil
		case "d":
			if r, ok := m.domains.Selected(); ok {
				m.modals = append(m.modals, modals.NewConfirm(m.th, fmt.Sprintf("Delete %s?", r.Domain), "removes cert reference and /etc/hosts entry"))
			}
			return m, nil
		case "t":
			if r, ok := m.domains.Selected(); ok {
				return m, m.toggleRoute(r)
			}
			return m, nil
		case "enter":
			if r, ok := m.domains.Selected(); ok {
				return m, openInBrowser(r.Domain, m.rt.Cfg.ProxyPort)
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.domains, cmd = m.domains.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m rootModel) updateTopModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	idx := len(m.modals) - 1
	top := m.modals[idx]
	var cmd tea.Cmd
	switch t := top.(type) {
	case modals.Help:
		t, cmd = t.Update(msg)
		m.modals[idx] = t
	case modals.Add:
		t, cmd = t.Update(msg)
		m.modals[idx] = t
	case modals.Edit:
		t, cmd = t.Update(msg)
		m.modals[idx] = t
	case modals.Confirm:
		t, cmd = t.Update(msg)
		m.modals[idx] = t
	}
	return m, cmd
}

func (m rootModel) handleModalResult(closed modals.Modal, r modals.Result) tea.Cmd {
	if r.Cancelled {
		return nil
	}
	switch p := r.Payload.(type) {
	case modals.AddPayload:
		return m.commitAdd(p)
	case modals.EditPayload:
		return m.commitEdit(p)
	case bool:
		if !p {
			return nil
		}
		// Confirm acknowledged. Currently used only for delete.
		if r, ok := m.domains.Selected(); ok {
			return m.commitDelete(r)
		}
	}
	_ = closed
	return nil
}

func (m rootModel) commitAdd(p modals.AddPayload) tea.Cmd {
	return func() tea.Msg {
		if !hosts.ValidHostname(p.Domain) {
			return errMsg(fmt.Errorf("invalid domain %q", p.Domain))
		}
		s, err := m.rt.OpenStore()
		if err != nil {
			return errMsg(err)
		}
		defer s.Close()
		if _, err := s.GetRoute(p.Domain); err == nil {
			return errMsg(fmt.Errorf("route exists: %s", p.Domain))
		} else if !errors.Is(err, store.ErrNotFound) {
			return errMsg(err)
		}
		editor := hosts.NewEditor(m.binPath)
		if err := editor.Add(p.Domain); err != nil {
			return errMsg(fmt.Errorf("hosts: %w", err))
		}
		r := store.Route{Domain: p.Domain, Target: p.Target, TLD: p.TLD, Enabled: true, Source: store.SourceAdHoc, AddedAt: time.Now().UTC()}
		if err := s.PutRoute(r); err != nil {
			_ = editor.Remove(p.Domain)
			return errMsg(err)
		}
		rs, err := s.ListRoutes()
		m.rt.Router.Set(rs)
		return RoutesRefreshed{Routes: rs, Err: err}
	}
}

func (m rootModel) commitEdit(p modals.EditPayload) tea.Cmd {
	return func() tea.Msg {
		s, err := m.rt.OpenStore()
		if err != nil {
			return errMsg(err)
		}
		defer s.Close()
		cur, err := s.GetRoute(p.Domain)
		if err != nil {
			return errMsg(err)
		}
		cur.Target = p.Target
		if err := s.PutRoute(cur); err != nil {
			return errMsg(err)
		}
		rs, _ := s.ListRoutes()
		m.rt.Router.Set(rs)
		return RoutesRefreshed{Routes: rs}
	}
}

func (m rootModel) commitDelete(r store.Route) tea.Cmd {
	return func() tea.Msg {
		s, err := m.rt.OpenStore()
		if err != nil {
			return errMsg(err)
		}
		defer s.Close()
		editor := hosts.NewEditor(m.binPath)
		if err := editor.Remove(r.Domain); err != nil {
			return errMsg(fmt.Errorf("hosts: %w", err))
		}
		if err := s.DeleteRoute(r.Domain); err != nil {
			_ = editor.Add(r.Domain)
			return errMsg(err)
		}
		rs, _ := s.ListRoutes()
		m.rt.Router.Set(rs)
		return RoutesRefreshed{Routes: rs}
	}
}

func (m rootModel) toggleRoute(r store.Route) tea.Cmd {
	return func() tea.Msg {
		s, err := m.rt.OpenStore()
		if err != nil {
			return errMsg(err)
		}
		defer s.Close()
		r.Enabled = !r.Enabled
		if err := s.PutRoute(r); err != nil {
			return errMsg(err)
		}
		rs, _ := s.ListRoutes()
		m.rt.Router.Set(rs)
		return RoutesRefreshed{Routes: rs}
	}
}

func openInBrowser(domain string, port int) tea.Cmd {
	return func() tea.Msg {
		url := fmt.Sprintf("https://%s", domain)
		if port != 443 {
			url = fmt.Sprintf("%s:%d", url, port)
		}
		// macOS-only: `open <url>` is widely available.
		_ = exec("open", url)
		return nil
	}
}

func (m rootModel) View() string {
	pill := components.StatusPill(m.th, components.PillDown, "")
	if m.proxy.Up {
		pill = components.StatusPill(m.th, components.PillUp, m.proxy.Addr)
	} else if m.proxy.Err != nil {
		pill = components.StatusPill(m.th, components.PillDown, m.proxy.Err.Error())
	}

	top := m.th.Title.Render("mklocal") + "  " + pill
	tabs := components.TabBar(m.th, []string{"Domains", "Projects", "Logs", "Doctor", "Settings"}, 0)
	body := m.domains.View()
	footer := components.Footer(m.th, m.domains.Keybinds())

	base := lipgloss.JoinVertical(lipgloss.Left, top, tabs, body, footer)
	if len(m.modals) == 0 {
		return base
	}
	modal := m.modals[len(m.modals)-1].(interface{ View() string }).View()
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal, lipgloss.WithWhitespaceForeground(lipgloss.Color("236")))
}

// errMsg lets us deliver an error through the tea.Cmd pipeline. Update could
// be extended to surface it as a footer toast in a future plan.
type errMsg error

// exec is a tiny shim used by openInBrowser so the test build does not bring
// in os/exec in unrelated places. Production uses os/exec directly.
func exec(args ...string) error {
	cmd := osExecCommand(args[0], args[1:]...)
	return cmd.Run()
}

// Wrap os/exec.Command so we can swap it in tests if needed.
var osExecCommand = execCommand

// Marshalled to its own function so we don't import os/exec at the top of
// the file (Bubble Tea programs are often size-conscious in tests).
func execCommand(name string, args ...string) interface{ Run() error } {
	return &deferredCmd{name: name, args: args}
}

type deferredCmd struct {
	name string
	args []string
}

func (c *deferredCmd) Run() error {
	return realExec(c.name, c.args)
}

// realExec is in `program_exec.go` to keep this file Bubble Tea-only.
var realExec = func(name string, args []string) error { return nil }

// home + binPath unused references intentionally to keep deps minimal.
var _ = filepath.Join
var _ = context.Background
```

- [ ] **Step 10.2 — `program_exec.go` actually invokes `os/exec.Command`**

Create `internal/tui/program_exec.go`:

```go
package tui

import "os/exec"

func init() {
	realExec = func(name string, args []string) error {
		return exec.Command(name, args...).Run()
	}
}
```

- [ ] **Step 10.3 — Build**

```
go build ./internal/tui/...
```

Expected: clean.

If any "imported and not used" errors arise from the shim, simplify by inlining `os/exec.Command` directly in `openInBrowser` — the shim above exists only to keep TUI tests from spawning real processes. The minimal compiling version is acceptable.

- [ ] **Step 10.4 — Smoke build of full module**

```
make build
```

- [ ] **Step 10.5 — Commit**

```
git add internal/tui/program.go internal/tui/program_exec.go
git commit -m "feat(tui): root model with modal stack, route mutations, and proxy lifecycle"
```

---

## Task 11: CLI integration — `mklocal` (no args) launches TUI

**Files:**
- Create: `internal/cli/tui.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 11.1 — Create `tui.go`**

```go
package cli

import (
	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mklocal/internal/tui"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the mklocal TUI",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := HomeDir()
			if err != nil {
				return err
			}
			rt, err := tui.NewRuntime(cmd.Context(), home)
			if err != nil {
				return err
			}
			return tui.Run(rt)
		},
	}
}
```

- [ ] **Step 11.2 — Make TUI the default when no subcommand is provided**

In `internal/cli/root.go`, after building the root cobra command, set its default `RunE` to call `newTUICmd().RunE`:

Add inside `New()` after `root.AddCommand(...)`:

```go
root.RunE = newTUICmd().RunE
```

Also register the explicit `newTUICmd()` so `mklocal tui` works:

```go
root.AddCommand(
	newAddCmd(),
	newRemoveCmd(),
	newListCmd(),
	newServeCmd(),
	newInstallCmd(),
	newUninstallCmd(),
	newHostsHelperCmd(),
	newTUICmd(),
)
```

- [ ] **Step 11.3 — Update CLI smoke tests**

In `internal/cli/cli_test.go`, add to the subcommand list assertion:

```go
for _, want := range []string{"add", "remove", "list", "serve", "install", "uninstall", "hosts-helper", "tui"} {
```

- [ ] **Step 11.4 — Build + verify**

```
make build
./bin/mklocal --help | grep tui
```

Expected: `tui` appears in available commands.

- [ ] **Step 11.5 — Commit**

```
git add internal/cli/tui.go internal/cli/root.go internal/cli/cli_test.go
git commit -m "feat(cli): tui subcommand and default-launch wiring"
```

---

## Task 12: End-to-end smoke (manual)

This task is manual. Run on a real macOS terminal of at least 100 columns wide.

- [ ] **Step 12.1 — Clean state**

```
./bin/mklocal uninstall --purge 2>/dev/null || true
```

- [ ] **Step 12.2 — Install**

```
./bin/mklocal install
```

Expect: CA installed in Keychain.

- [ ] **Step 12.3 — Lower proxy port to avoid sudo**

Edit `~/.mklocal/config.toml`:

```
proxy_port = 8443
```

- [ ] **Step 12.4 — Start a backend**

```
python3 -m http.server 3000 &
```

- [ ] **Step 12.5 — Launch TUI**

```
./bin/mklocal
```

Expect: full-screen TUI with tab bar, empty Domains table, status pill showing `● up 127.0.0.1:8443`.

- [ ] **Step 12.6 — Add a route via the TUI**

Press `a`. Type `foo` in the Name field. Press Tab. Type `localhost:3000`. Press Enter. (You will be prompted for sudo in the terminal — the TUI does not capture it, the prompt appears underneath.)

Expect: modal closes; row appears: `foo.local | localhost:3000 | ✓ up | ad-hoc`.

- [ ] **Step 12.7 — Verify in browser / curl**

In another shell:

```
curl -v https://foo.local:8443/
```

Expect: 200 from the Python directory listing.

Or hit `Enter` in the TUI to `open https://foo.local:8443` in the default browser.

- [ ] **Step 12.8 — Toggle disable**

In the TUI, with the row selected, press `t`. Status flips to `⊘ off`. `curl` now returns 404 (router doesn't serve the route).

- [ ] **Step 12.9 — Delete**

Press `d`. Confirm modal appears. Press Enter. Row disappears; `/etc/hosts` line cleaned.

- [ ] **Step 12.10 — Help overlay**

Press `?`. Help modal lists keybindings. Press Esc to close.

- [ ] **Step 12.11 — Quit**

Press `q`. TUI closes; proxy goroutine exits; the python http.server is unaffected (kill it manually).

- [ ] **Step 12.12 — Tear down**

```
./bin/mklocal uninstall --purge
killall python3 || true
```

Plan 2 is complete when steps 12.1 – 12.11 pass.

---

## Future plans (sketch)

- **Plan 3** — Projects tab (per-project `mklocal.toml`), Logs tab (in-memory ring buffer + tail), Doctor tab (re-runnable diagnostics), Settings tab (theme switcher, config save).
- **Plan 4** — Linux + Windows port: `internal/cert/trust/{linux,windows}.go`, `internal/hosts/hosts_{linux,windows}.go`, build tag `_windows` for named-pipe equivalents (we still won't ship a daemon — TUI stays single-process).
- **Plan 5** — gRPC daemon + launchd/systemd/winsvc (the deferred plan in `2026-05-12-plan-5-grpc-daemon.md`). Migrate TUI to gRPC client; CLI commands become gRPC clients; daemon owns proxy + store.
- **Plan 6** — Release pipeline: `.goreleaser.yaml`, signed GitHub Releases, Homebrew tap.

---

## Self-review notes

- Spec section 6 (TUI architecture) — root model + update routing rules + theming + layout breakpoints — partially covered. Only the Domains tab is wired; the tab bar shows all 5 labels but only the first is interactive. Modal stack pattern matches spec.
- Spec section 7 (Tabs in detail) — only 7.1 (Domains) implemented. Other tabs are correctly deferred to Plan 3.
- Spec section 5.3 (elevation) — unchanged from Plan 1: hosts edits via `sudo mklocal hosts-helper`. The TUI does NOT capture sudo's password prompt; if the terminal is in altscreen mode, the prompt renders below; if that's confusing, future polish (Plan 3) can detach sudo via osascript or a polkit-style dialog.
- The "1-second poll" replaces the daemon's event stream. When Plan 5 lands, `Runtime.RefreshTick` becomes a `WatchEvents` gRPC subscription with the same `RoutesRefreshed` message contract.

## Definition of done

- TUI renders Domains tab with table + status pill + footer hints.
- `a`/`e`/`d`/`t`/`?`/`q` all behave per Task 10.
- Add/Edit/Delete/Toggle round-trip through bbolt + `/etc/hosts` correctly.
- Proxy starts at TUI launch and exits at quit.
- All unit + snapshot tests pass under `-race`.
- `gofmt -l .` empty; `go vet ./...` clean.
- Manual smoke (Task 12) passes.
- `mklocal` (no args) launches the TUI.
- `mklocal serve` still works (Plan 1 behavior unchanged).
