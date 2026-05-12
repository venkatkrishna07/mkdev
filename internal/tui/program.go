package tui

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/hosts"
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
	tabDomains tabIndex = iota
	tabProjects
	tabLogs
	tabDoctor
	tabSettings
)

var tabLabels = []string{"Domains", "Projects", "Logs", "Doctor", "Settings"}

// Run launches the TUI bound to rt. It blocks until the user quits, then
// cancels the runtime context so the proxy goroutine exits cleanly.
func Run(rt *Runtime) error {
	m := newRootModel(rt)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	rt.Cancel()
	return err
}

type rootModel struct {
	rt          *Runtime
	th          styles.Theme
	width       int
	height      int
	domains     tabs.Domains
	modals      []any // LIFO stack of modals.Add / modals.Edit / modals.Confirm
	proxy       ProxyState
	proxyCh     <-chan ProxyState
	binPath     string
	keys        KeyMap
	help        help.Model
	showHelp    bool
	spinner     spinner.Model
	busy        bool
	active      tabIndex
	pendingQuit bool
}

func newRootModel(rt *Runtime) rootModel {
	th := styles.NewTheme()
	bp, _ := os.Executable()

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

	return rootModel{
		rt:      rt,
		th:      th,
		domains: tabs.NewDomains(th, 100, 24),
		binPath: bp,
		keys:    DefaultKeyMap,
		help:    h,
		spinner: sp,
	}
}

// proxyStartedMsg carries the proxy state channel from Init into Update so it
// persists across model copies (Init's value-receiver mutations are discarded).
type proxyStartedMsg struct{ ch <-chan ProxyState }

func (m rootModel) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return proxyStartedMsg{ch: m.rt.StartProxy()} },
		m.rt.RefreshTick(0),
		m.spinner.Tick,
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
		var cmd tea.Cmd
		m.domains, cmd = m.domains.Update(msg)
		return m, cmd

	case proxyStartedMsg:
		m.proxyCh = msg.ch
		return m, m.waitProxy()

	case ProxyState:
		m.proxy = msg
		return m, m.waitProxy()

	case RoutesRefreshed:
		m.busy = false
		var cmd tea.Cmd
		m.domains, cmd = m.domains.Update(msg)
		return m, tea.Batch(cmd, m.rt.RefreshTick(time.Second))

	case errMsg:
		m.busy = false
		slog.Error("tui: mutation failed", "err", error(msg))
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
		m.showHelp = !m.showHelp
		return m, nil
	case key.Matches(k, m.keys.NextTab):
		m.active = (m.active + 1) % tabIndex(len(tabLabels))
		return m, nil
	case key.Matches(k, m.keys.PrevTab):
		m.active = (m.active - 1 + tabIndex(len(tabLabels))) % tabIndex(len(tabLabels))
		return m, nil
	case key.Matches(k, m.keys.Tab1):
		m.active = tabDomains
		return m, nil
	case key.Matches(k, m.keys.Tab2):
		m.active = tabProjects
		return m, nil
	case key.Matches(k, m.keys.Tab3):
		m.active = tabLogs
		return m, nil
	case key.Matches(k, m.keys.Tab4):
		m.active = tabDoctor
		return m, nil
	case key.Matches(k, m.keys.Tab5):
		m.active = tabSettings
		return m, nil
	}
	if m.active != tabDomains {
		return m, nil
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
	case key.Matches(k, m.keys.Toggle):
		if r, ok := m.domains.Selected(); ok {
			m.busy = true
			return m, tea.Batch(m.toggleRoute(r), m.spinner.Tick)
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

func (m rootModel) updateTopModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	idx := len(m.modals) - 1
	var cmd tea.Cmd
	switch t := m.modals[idx].(type) {
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

func (m rootModel) handleModalResult(closedModal any, r modals.Result) tea.Cmd {
	_ = closedModal
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
		if sel, ok := m.domains.Selected(); ok {
			return m.commitDelete(sel)
		}
	}
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
		editor := hosts.NewGUIEditor(m.binPath)
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
		editor := hosts.NewGUIEditor(m.binPath)
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
		_ = exec.Command("open", url).Run()
		return nil
	}
}

// errMsg lets us deliver an error through the tea.Cmd pipeline. Update could
// be extended to surface it as a footer toast in a future iteration.
type errMsg error

// activeKeyMap returns the help.KeyMap to advertise in the footer: the top
// modal's when the stack is non-empty, otherwise the root key map.
func (m rootModel) activeKeyMap() help.KeyMap {
	if len(m.modals) == 0 {
		return m.keys
	}
	switch t := m.modals[len(m.modals)-1].(type) {
	case modals.Add:
		return t.Keys()
	case modals.Edit:
		return t.Keys()
	case modals.Confirm:
		return t.Keys()
	}
	return m.keys
}

func (m rootModel) View() string {
	width := m.width
	if width <= 0 {
		width = 100
	}

	pill := components.StatusPill(m.th, components.PillDown, "")
	if m.proxy.Up {
		pill = components.StatusPill(m.th, components.PillUp, m.proxy.Addr)
	} else if m.proxy.Err != nil {
		pill = components.StatusPill(m.th, components.PillDown, m.proxy.Err.Error())
	}

	header := components.Banner(m.th, version.Version, pill, width)
	tabBar := components.TabBar(m.th, tabLabels, int(m.active))
	rule := m.th.Rule.Render(strings.Repeat("─", width))
	var body string
	switch m.active {
	case tabDomains:
		body = m.domains.View()
	default:
		body = m.th.Dim.Render("  this tab is not implemented yet — switch back to Domains with ") +
			m.th.FooterKey.Render("1") +
			m.th.Dim.Render(" or ") +
			m.th.FooterKey.Render("shift+tab")
	}
	if m.busy {
		body = m.spinner.View() + " " + m.th.Dim.Render("working…") + "\n" + body
	}

	m.help.ShowAll = m.showHelp
	footer := m.help.View(m.activeKeyMap())

	view := lipgloss.JoinVertical(lipgloss.Left, header, tabBar, rule, body, rule, footer)

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
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalView, lipgloss.WithWhitespaceForeground(lipgloss.Color("236")))
}
