package tui

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/browser"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui/components"
	"github.com/venkatkrishna07/mkdev/internal/tui/modals"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
	"github.com/venkatkrishna07/mkdev/internal/tui/tabs"
	"github.com/venkatkrishna07/mkdev/internal/version"
)

// NewRootForTest returns a fresh root model for snapshot/render tests.
func NewRootForTest(rt *Runtime) tea.Model { return newRootModel(rt) }

type tabIndex int

const (
	tabDashboard tabIndex = iota
	tabDomains
	tabProjects
	tabLogs
	tabDoctor
	tabSettings
)

var tabLabels = []string{"Dashboard", "Domains", "Projects", "Logs", "Doctor", "Settings"}

var tabSpecs = []components.Tab{
	{Label: "Dashboard", Icon: "󰕮"},
	{Label: "Domains", Icon: "󰖟"},
	{Label: "Projects", Icon: "󰉋"},
	{Label: "Logs", Icon: "󰦪"},
	{Label: "Doctor", Icon: "󰋽"},
	{Label: "Settings", Icon: "󰒓"},
}

// Run launches the TUI bound to rt. It blocks until the user quits, then
// cancels the runtime context so the proxy goroutine exits cleanly.
//
// LIFO defer order: Cancel signals goroutines to wind down first, then
// Close releases the bbolt file lock. Both fire even if p.Run panics.
func Run(rt *Runtime) error {
	defer func() { _ = rt.Close() }()
	defer rt.Cancel()
	m := newRootModel(rt)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	rt.SetSender(p.Send)
	rt.SubscribeEvents()
	_, err := p.Run()
	return err
}

type rootModel struct {
	rt          *Runtime
	th          styles.Theme
	width       int
	height      int
	dashboard   tabs.Dashboard
	domains     tabs.Domains
	logs        tabs.Logs
	doctor      tabs.Doctor
	settings    tabs.Settings
	modals      []any // LIFO stack of modals.Add / modals.Edit / modals.Confirm / modals.Help
	proxy       ProxyState
	proxyCh     <-chan ProxyState
	binPath     string
	keys        KeyMap
	help        help.Model
	spinner     spinner.Model
	busy        bool
	active      tabIndex
	pendingQuit bool
	lastErr     error
	lastErrAt   time.Time
	splash      bool
}

func newRootModel(rt *Runtime) rootModel {
	th := styles.NewTheme(rt.Cfg.Theme)
	bp, err := os.Executable()
	if err != nil || bp == "" {
		panic("tui: cannot resolve mkdev binary path: " + fmt.Sprint(err))
	}

	h := help.New()
	h.Styles.ShortKey = th.FooterKey
	h.Styles.ShortDesc = th.Footer
	h.Styles.ShortSeparator = th.Dim
	h.Styles.FullKey = th.FooterKey
	h.Styles.FullDesc = th.Footer
	h.Styles.FullSeparator = th.Dim

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = th.Title

	logPath := filepath.Join(rt.Home, "logs", "tui.log")
	dashSrc := tabs.DashSource{
		Total: rt.TotalReqs,
		RPS:   rt.RPSWindow,
		CA:    rt.CA,
		Start: time.Now(),
		Routes: func() []store.Route {
			rs, _ := rt.LoadRoutes()
			return rs
		},
		Health:   rt.HealthOf,
		LastSeen: rt.LastSeenHost,
		LAN: func() tabs.LANState {
			s := rt.LANState()
			return tabs.LANState{IP: s.IP, Advertising: s.Advertising, SharedCount: s.SharedCount}
		},
	}
	return rootModel{
		rt:        rt,
		th:        th,
		dashboard: tabs.NewDashboard(th, dashSrc),
		domains: tabs.NewDomainsWithSources(th, 100, 24,
			func(string) []time.Duration { return nil },
			rt.HealthOf,
			func() tabs.LANState {
				s := rt.LANState()
				return tabs.LANState{IP: s.IP, Advertising: s.Advertising, SharedCount: s.SharedCount}
			},
		),
		logs:     tabs.NewLogs(th, logPath),
		doctor:   tabs.NewDoctor(th, rt.Home, rt.LoadRoutes),
		settings: tabs.NewSettings(th, rt.Home),
		binPath:  bp,
		keys:     DefaultKeyMap,
		help:     h,
		spinner:  sp,
		splash:   true,
	}
}

type splashDoneMsg struct{}

// proxyStartedMsg carries the proxy state channel from Init into Update so it
// persists across model copies (Init's value-receiver mutations are discarded).
type proxyStartedMsg struct{ ch <-chan ProxyState }

// errMsg lets us deliver an error through the tea.Cmd pipeline. The root
// Update captures it as a transient toast in the footer area.
type errMsg error

// errExpiredMsg is delivered ~5s after an errMsg to clear the toast.
type errExpiredMsg struct{}

func (m rootModel) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return proxyStartedMsg{ch: m.rt.StartProxy()} },
		m.rt.RefreshNow(),
		m.spinner.Tick,
		m.logs.Init(),
		m.dashboard.Init(),
		tea.Tick(1500*time.Millisecond, func(time.Time) tea.Msg { return splashDoneMsg{} }),
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
		m.help.Width = msg.Width
		var dCmd, lCmd, docCmd, sCmd, dashCmd tea.Cmd
		m.dashboard, dashCmd = m.dashboard.Update(msg)
		m.domains, dCmd = m.domains.Update(msg)
		m.logs, lCmd = m.logs.Update(msg)
		m.doctor, docCmd = m.doctor.Update(msg)
		m.settings, sCmd = m.settings.Update(msg)
		return m, tea.Batch(dashCmd, dCmd, lCmd, docCmd, sCmd)

	case tabs.LogsTickMsg:
		var cmd tea.Cmd
		m.logs, cmd = m.logs.Update(msg)
		return m, cmd

	case tabs.SettingsSavedMsg:
		m.rt.Cfg = msg.Cfg
		return m, nil

	case proxyStartedMsg:
		m.proxyCh = msg.ch
		return m, m.waitProxy()

	case ProxyState:
		m.proxy = msg
		return m, m.waitProxy()

	case RoutesRefreshed:
		m.busy = false
		var cmd, dashCmd tea.Cmd
		m.domains, cmd = m.domains.Update(msg)
		m.dashboard, dashCmd = m.dashboard.Update(msg)
		return m, tea.Batch(cmd, dashCmd)

	case tabs.DashboardTickMsg:
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.Update(msg)
		return m, cmd

	case errMsg:
		m.busy = false
		m.lastErr = error(msg)
		m.lastErrAt = time.Now()
		slog.Error("tui: mutation failed", "err", error(msg))
		return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return errExpiredMsg{} })

	case errExpiredMsg:
		if time.Since(m.lastErrAt) >= 5*time.Second {
			m.lastErr = nil
		}
		return m, nil

	case splashDoneMsg:
		m.splash = false
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case modals.Closed:
		if len(m.modals) == 0 {
			return m, nil
		}
		top := m.modals[len(m.modals)-1]
		m.modals = m.modals[:len(m.modals)-1]
		if m.pendingQuit {
			m.pendingQuit = false
			if !msg.Result.Cancelled {
				if confirmed, ok := msg.Result.Payload.(bool); ok && confirmed {
					return m, tea.Quit
				}
			}
			return m, nil
		}
		cmd := m.handleModalResult(top, msg.Result)
		if cmd != nil {
			m.busy = true
			return m, tea.Batch(cmd, m.spinner.Tick)
		}
		return m, nil

	case tea.KeyMsg:
		if m.splash {
			m.splash = false
			return m, nil
		}
		if len(m.modals) > 0 {
			return m.updateTopModal(msg)
		}
		return m.handleGlobalKey(msg)
	}
	return m, nil
}

func (m rootModel) handleGlobalKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(k, m.keys.Quit):
		m.modals = append(m.modals, modals.NewConfirm(m.th, "Quit mkdev?", "stops the proxy and closes the TUI"))
		m.pendingQuit = true
		return m, nil
	case key.Matches(k, m.keys.Help):
		var flat []key.Binding
		for _, row := range m.keys.FullHelp() {
			flat = append(flat, row...)
		}
		m.modals = append(m.modals, modals.NewHelp(m.th, flat))
		return m, nil
	case key.Matches(k, m.keys.NextTab):
		m.active = (m.active + 1) % tabIndex(len(tabLabels))
		return m, nil
	case key.Matches(k, m.keys.PrevTab):
		m.active = (m.active - 1 + tabIndex(len(tabLabels))) % tabIndex(len(tabLabels))
		return m, nil
	case key.Matches(k, m.keys.Tab1):
		m.active = tabDashboard
		return m, nil
	case key.Matches(k, m.keys.Tab2):
		m.active = tabDomains
		return m, nil
	case key.Matches(k, m.keys.Tab3):
		m.active = tabProjects
		return m, nil
	case key.Matches(k, m.keys.Tab4):
		m.active = tabLogs
		return m, nil
	case key.Matches(k, m.keys.Tab5):
		m.active = tabDoctor
		return m, nil
	case key.Matches(k, m.keys.Tab6):
		m.active = tabSettings
		return m, nil
	}
	if m.active != tabDomains {
		return m.forwardToActiveTab(k)
	}
	switch {
	case key.Matches(k, m.keys.Add):
		m.modals = append(m.modals, modals.NewAdd(m.th, m.rt.Cfg.TLD))
		return m, nil
	case key.Matches(k, m.keys.Edit):
		if r, ok := m.domains.Selected(); ok {
			m.modals = append(m.modals, modals.NewEdit(m.th, r))
		}
		return m, nil
	case key.Matches(k, m.keys.Delete):
		if r, ok := m.domains.Selected(); ok {
			m.modals = append(m.modals, modals.NewConfirm(m.th, fmt.Sprintf("Delete %s?", r.Domain), "removes /etc/hosts entry"))
		}
		return m, nil
	case key.Matches(k, m.keys.Share):
		if r, ok := m.domains.Selected(); ok {
			m.busy = true
			return m, tea.Batch(m.toggleShare(r), m.spinner.Tick)
		}
		return m, nil
	case key.Matches(k, m.keys.Open):
		if r, ok := m.domains.Selected(); ok {
			return m, openInBrowser(r.Domain, m.rt.Cfg.ProxyPort)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.domains, cmd = m.domains.Update(k)
	return m, cmd
}

func openInBrowser(domain string, port int) tea.Cmd {
	return func() tea.Msg {
		url := fmt.Sprintf("https://%s", domain)
		if port != 443 {
			url = fmt.Sprintf("%s:%d", url, port)
		}
		if err := browser.Open(url); err != nil {
			return errMsg(fmt.Errorf("open browser: %w", err))
		}
		return nil
	}
}

func (m rootModel) View() string {
	width := m.width
	if width <= 0 {
		width = 100
	}
	if m.splash {
		h := m.height
		if h <= 0 {
			h = 24
		}
		return components.Splash(m.th, version.Version, "local HTTPS for dev servers", width, h)
	}

	pill := components.StatusPill(m.th, components.PillDown, "")
	if m.proxy.Up {
		pill = components.StatusPill(m.th, components.PillUp, m.proxy.Addr)
	} else if m.proxy.Err != nil {
		pill = components.StatusPill(m.th, components.PillDown, m.proxy.Err.Error())
	}

	header := components.Banner(m.th, version.Version, pill, width)
	tabBar := components.TabBarRich(m.th, tabSpecs, int(m.active), width)
	rule := m.th.Rule.Render(strings.Repeat("─", width))
	var body string
	switch m.active {
	case tabDashboard:
		body = m.dashboard.View()
	case tabDomains:
		body = m.domains.View()
	case tabProjects:
		body = m.th.Dim.Render("Projects — coming in the next release")
	case tabLogs:
		body = m.logs.View()
	case tabDoctor:
		body = m.doctor.View()
	case tabSettings:
		body = m.settings.View()
	}
	if m.busy {
		body = m.spinner.View() + " " + m.th.Dim.Render("working…") + "\n" + body
	}

	footer := m.help.View(m.activeKeyMap())

	var toast string
	if m.lastErr != nil && time.Since(m.lastErrAt) < 5*time.Second {
		toast = m.th.PillDown.Render("✗ " + m.lastErr.Error())
	}

	sections := []string{header, tabBar, rule, body, rule}
	if toast != "" {
		sections = append(sections, toast)
	}
	sections = append(sections, footer)
	view := lipgloss.JoinVertical(lipgloss.Left, sections...)
	if m.height > 0 {
		view = lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, view)
	}

	if len(m.modals) == 0 {
		return view
	}

	var modalView string
	switch t := m.modals[len(m.modals)-1].(type) {
	case modals.Add:
		modalView = t.View()
	case modals.Edit:
		modalView = t.View()
	case modals.Confirm:
		modalView = t.View()
	case modals.Help:
		modalView = t.View()
	}
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		modalView,
		lipgloss.WithWhitespaceForeground(lipgloss.Color("236")),
	)
}
