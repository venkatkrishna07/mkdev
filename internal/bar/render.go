package bar

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"
	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/browser"
	"github.com/venkatkrishna07/mkdev/internal/client"
	"github.com/venkatkrishna07/mkdev/internal/clipboard"
	"github.com/venkatkrishna07/mkdev/internal/daemon"
)

const clickTimeout = 5 * time.Second

// systray only appends menu items, so route rows are preallocated
// at Init and bound/unbound as the route list changes.
const routeSlotPool = 30

const (
	repoURL    = "https://github.com/venkatkrishna07/mkdev"
	issuesURL  = "https://github.com/venkatkrishna07/mkdev/issues"
	licenseURL = "https://github.com/venkatkrishna07/mkdev/blob/main/LICENSE"
)

// mu guards name/url/bound against concurrent click-loop reads vs Reconcile writes.
type routeSlot struct {
	root    *systray.MenuItem
	open    *systray.MenuItem
	copy    *systray.MenuItem
	share   *systray.MenuItem
	enabled *systray.MenuItem

	mu    sync.Mutex
	name  string
	url   string
	bound bool
}

func (s *routeSlot) snapshot() (name, url string, bound bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name, s.url, s.bound
}

type Renderer struct {
	c     *client.Client
	slots []*routeSlot

	brandItm  *systray.MenuItem
	statusItm *systray.MenuItem
	routesLbl *systray.MenuItem
	emptyItm  *systray.MenuItem

	dashItm *systray.MenuItem
	logsItm *systray.MenuItem

	aboutItm    *systray.MenuItem
	aboutVerItm *systray.MenuItem
	aboutGHItm  *systray.MenuItem
	aboutBugItm *systray.MenuItem
	aboutLicItm *systray.MenuItem

	autostartItm *systray.MenuItem

	toggleItm *systray.MenuItem
	quitItm   *systray.MenuItem

	pending  atomic.Bool
	daemonUp atomic.Bool
}

func NewRenderer(c *client.Client) *Renderer {
	return &Renderer{c: c}
}

func (r *Renderer) Init() {
	if len(iconRegularBytes) > 0 {
		systray.SetTemplateIcon(iconTemplateBytes, iconRegularBytes)
	}
	setAppTooltip("mkdev — local HTTPS dev proxy")

	r.brandItm = systray.AddMenuItem("mkdev", "")
	r.brandItm.Disable()

	r.statusItm = systray.AddMenuItem("…", "daemon liveness")
	r.statusItm.Disable()
	systray.AddSeparator()

	r.routesLbl = systray.AddMenuItem("ROUTES", "")
	r.routesLbl.Disable()
	r.emptyItm = systray.AddMenuItem("no routes yet", "")
	r.emptyItm.Disable()

	r.slots = make([]*routeSlot, routeSlotPool)
	for i := range r.slots {
		root := systray.AddMenuItem("", "")
		root.Hide()
		open := root.AddSubMenuItem("Open in browser", "")
		cpy := root.AddSubMenuItem("Copy URL", "")
		enabled := root.AddSubMenuItemCheckbox("Enabled", "Disable to stop proxying this route", true)
		share := root.AddSubMenuItemCheckbox("Share on LAN", "Advertise via mDNS to LAN devices", false)
		slot := &routeSlot{root: root, open: open, copy: cpy, share: share, enabled: enabled}
		r.slots[i] = slot
		go r.handleSlotClicks(slot)
	}

	systray.AddSeparator()
	r.dashItm = systray.AddMenuItem("Open Dashboard", "Launch the mkdev TUI")
	r.logsItm = systray.AddMenuItem("Open Logs Folder", "Open ~/.mkdev/logs")
	r.autostartItm = systray.AddMenuItemCheckbox("Start on Login", "Launch mkdev menu bar at login", AutostartEnabled())

	systray.AddSeparator()
	r.aboutItm = systray.AddMenuItem("About mkdev", "")
	r.aboutVerItm = r.aboutItm.AddSubMenuItem("Version —", "")
	r.aboutVerItm.Disable()
	r.aboutGHItm = r.aboutItm.AddSubMenuItem("View on GitHub", repoURL)
	r.aboutBugItm = r.aboutItm.AddSubMenuItem("Report an Issue", issuesURL)
	r.aboutLicItm = r.aboutItm.AddSubMenuItem("License", licenseURL)

	systray.AddSeparator()
	r.toggleItm = systray.AddMenuItem("Stop daemon", "")
	r.quitItm = systray.AddMenuItem("Quit", "Close the menu bar; daemon keeps running")
	go r.handleFooter()
}

func (r *Renderer) updateToggle(daemonUp bool) {
	if r.toggleItm == nil {
		return
	}
	if r.pending.Load() {
		r.toggleItm.SetTitle("Working…")
		r.toggleItm.Disable()
		return
	}
	r.toggleItm.Enable()
	if daemonUp {
		r.toggleItm.SetTitle("Stop daemon")
		r.toggleItm.SetTooltip("POST /v1/shutdown")
	} else {
		r.toggleItm.SetTitle("Start daemon")
		r.toggleItm.SetTooltip("Spawn `mkdev daemon serve`")
	}
}

func (r *Renderer) updateAbout(snap Snapshot) {
	if r.aboutVerItm == nil {
		return
	}
	title := "Version —"
	if snap.Version != "" {
		title = "Version " + versionLabel(snap.Version)
	}
	r.aboutVerItm.SetTitle(title)
}

func (r *Renderer) Reconcile(snap Snapshot) {
	r.daemonUp.Store(snap.DaemonUp)
	r.brandItm.SetTitle(renderBrand(snap))
	r.statusItm.SetTitle(renderStatusLine(snap))
	statusHealth := api.HealthDown
	if snap.DaemonUp {
		statusHealth = api.HealthUp
	}
	r.statusItm.SetIcon(iconForHealth(statusHealth))

	if len(snap.Routes) == 0 {
		r.emptyItm.Show()
	} else {
		r.emptyItm.Hide()
	}
	r.routesLbl.SetTitle(fmt.Sprintf("ROUTES (%d)", len(snap.Routes)))

	r.bindRoutes(snap)
	r.updateToggle(snap.DaemonUp)
	r.updateAbout(snap)
}

func (r *Renderer) bindRoutes(snap Snapshot) {
	byName := map[string]*routeSlot{}
	for _, s := range r.slots {
		name, _, bound := s.snapshot()
		if bound {
			byName[name] = s
		}
	}

	used := map[*routeSlot]struct{}{}
	for _, route := range snap.Routes {
		domain := route.Name + snap.TLD
		health := snap.Health[domain]
		title := fmt.Sprintf("%s  →  %s%s", domain, route.Target, routeBadges(route))
		url := buildRouteURL(domain, snap.ProxyPort)

		s := byName[route.Name]
		if s == nil {
			s = r.freeSlot(used)
			if s == nil {
				slog.Warn("bar: route slot pool exhausted", "limit", routeSlotPool)
				break
			}
		}
		used[s] = struct{}{}

		s.mu.Lock()
		s.name = route.Name
		s.url = url
		s.bound = true
		s.mu.Unlock()

		s.root.SetTitle(title)
		s.root.SetIcon(iconForHealth(health))
		s.root.SetTooltip(url)
		s.open.SetTooltip(url)
		s.copy.SetTooltip(url)
		updateBoolItem(s.share, route.Share == api.ShareLAN)
		updateBoolItem(s.enabled, route.Enabled)
		s.root.Show()
	}

	for _, s := range r.slots {
		if _, ok := used[s]; ok {
			continue
		}
		s.mu.Lock()
		s.bound = false
		s.name = ""
		s.url = ""
		s.mu.Unlock()
		s.root.Hide()
	}
}

func (r *Renderer) freeSlot(used map[*routeSlot]struct{}) *routeSlot {
	for _, s := range r.slots {
		if _, ok := used[s]; ok {
			continue
		}
		_, _, bound := s.snapshot()
		if bound {
			continue
		}
		return s
	}
	return nil
}

func buildRouteURL(domain string, proxyPort int) string {
	if proxyPort == 0 || proxyPort == 443 {
		return "https://" + domain
	}
	return fmt.Sprintf("https://%s:%d", domain, proxyPort)
}

func (r *Renderer) handleSlotClicks(s *routeSlot) {
	for {
		select {
		case <-s.root.ClickedCh:
			r.slotOpenURL(s)
		case <-s.open.ClickedCh:
			r.slotOpenURL(s)
		case <-s.copy.ClickedCh:
			r.slotCopyURL(s)
		case <-s.share.ClickedCh:
			r.slotToggleShare(s)
		case <-s.enabled.ClickedCh:
			r.slotToggleEnabled(s)
		}
	}
}

func (r *Renderer) slotOpenURL(s *routeSlot) {
	if _, url, bound := s.snapshot(); bound {
		openURL(url)
	}
}

func (r *Renderer) slotCopyURL(s *routeSlot) {
	_, url, bound := s.snapshot()
	if !bound {
		return
	}
	if err := clipboard.Copy(url); err != nil {
		slog.Warn("bar: copy URL failed", "url", url, "err", err)
	}
}

func (r *Renderer) slotToggleShare(s *routeSlot) {
	name, _, bound := s.snapshot()
	if !bound {
		return
	}
	enabled := !s.share.Checked()
	ctx, cancel := context.WithTimeout(context.Background(), clickTimeout)
	defer cancel()
	if _, err := r.c.ToggleShare(ctx, name, enabled); err != nil {
		slog.Warn("bar: ToggleShare failed", "name", name, "err", err)
		return
	}
	updateBoolItem(s.share, enabled)
}

func (r *Renderer) slotToggleEnabled(s *routeSlot) {
	name, _, bound := s.snapshot()
	if !bound {
		return
	}
	newEnabled := !s.enabled.Checked()
	ctx, cancel := context.WithTimeout(context.Background(), clickTimeout)
	defer cancel()
	if _, err := r.c.EditRoute(ctx, name, client.RouteEdit{Enabled: &newEnabled}); err != nil {
		slog.Warn("bar: toggle enabled failed", "name", name, "err", err)
		return
	}
	updateBoolItem(s.enabled, newEnabled)
}

func (r *Renderer) handleFooter() {
	for {
		select {
		case <-r.quitItm.ClickedCh:
			systray.Quit()
			return
		case <-r.toggleItm.ClickedCh:
			r.onToggleClick()
		case <-r.dashItm.ClickedCh:
			if err := launchInTerminal("tui"); err != nil {
				slog.Warn("bar: launch dashboard failed", "err", err)
			}
		case <-r.logsItm.ClickedCh:
			r.openLogs()
		case <-r.aboutGHItm.ClickedCh:
			openURL(repoURL)
		case <-r.aboutBugItm.ClickedCh:
			openURL(issuesURL)
		case <-r.aboutLicItm.ClickedCh:
			openURL(licenseURL)
		case <-r.autostartItm.ClickedCh:
			r.onAutostartToggle()
		}
	}
}

func (r *Renderer) onAutostartToggle() {
	if r.autostartItm.Checked() {
		if err := UninstallAutostart(); err != nil {
			slog.Warn("bar: uninstall autostart failed", "err", err)
			return
		}
		r.autostartItm.Uncheck()
		return
	}
	if err := InstallAutostart(); err != nil {
		slog.Warn("bar: install autostart failed", "err", err)
		return
	}
	r.autostartItm.Check()
}

func (r *Renderer) openLogs() {
	dir, err := logsDir()
	if err != nil {
		slog.Warn("bar: resolve logs dir failed", "err", err)
		return
	}
	if err := browser.Open(dir); err != nil {
		slog.Warn("bar: open logs dir failed", "dir", dir, "err", err)
	}
}

func (r *Renderer) onToggleClick() {
	if !r.pending.CompareAndSwap(false, true) {
		return
	}
	wasUp := r.daemonUp.Load()
	r.updateToggle(wasUp)
	go func() {
		defer r.pending.Store(false)
		if wasUp {
			r.stopDaemon()
			return
		}
		r.startDaemon()
	}()
}

// stopDaemon attempts to stop the daemon both via the OS supervisor (so
// KeepAlive=true in the LaunchAgent doesn't respawn it) and via the HTTP
// shutdown endpoint (covers the case where the daemon was started manually
// without an installed service).
func (r *Renderer) stopDaemon() {
	disableErr := daemon.DisableUnit()
	if disableErr != nil && !errors.Is(disableErr, daemon.ErrUnitUnsupported) {
		slog.Warn("bar: disable daemon unit failed", "err", disableErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), clickTimeout)
	defer cancel()
	if err := r.c.Shutdown(ctx); err != nil && !errors.Is(err, client.ErrDaemonDown) {
		slog.Warn("bar: shutdown daemon failed", "err", err)
		return
	}
	slog.Info("bar: daemon stop requested")
}

// startDaemon prefers the OS supervisor (`launchctl enable` / systemd unit
// start) so the daemon comes back up the same way the user originally set it
// up. Falls back to spawning the binary directly when no service is
// installed.
func (r *Renderer) startDaemon() {
	if err := daemon.EnableUnit(); err == nil {
		slog.Info("bar: daemon enabled via supervisor")
		return
	} else if !errors.Is(err, daemon.ErrUnitUnsupported) {
		slog.Warn("bar: enable daemon unit failed, falling back to spawn", "err", err)
	}
	if err := spawnDetached("daemon", "serve"); err != nil {
		slog.Warn("bar: start daemon failed", "err", err)
		return
	}
	slog.Info("bar: daemon start requested")
}

func updateBoolItem(item *systray.MenuItem, on bool) {
	if on && !item.Checked() {
		item.Check()
	} else if !on && item.Checked() {
		item.Uncheck()
	}
}

func openURL(url string) {
	if err := browser.Open(url); err != nil {
		slog.Warn("bar: browser open failed", "url", url, "err", err)
	}
}
