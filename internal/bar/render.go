package bar

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/getlantern/systray"
	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/browser"
	"github.com/venkatkrishna07/mkdev/internal/client"
)

const clickTimeout = 5 * time.Second

type routeMenu struct {
	root  *systray.MenuItem
	open  *systray.MenuItem
	share *systray.MenuItem
	stop  chan struct{}
	name  string
}

type Renderer struct {
	c     *client.Client
	items map[string]*routeMenu

	brandItm   *systray.MenuItem
	statusItm  *systray.MenuItem
	uptimeItm  *systray.MenuItem
	trafficItm *systray.MenuItem
	routesLbl  *systray.MenuItem
	emptyItm   *systray.MenuItem

	toggleItm *systray.MenuItem
	quitItm   *systray.MenuItem

	footerReady bool
	pending     bool
	daemonUp    bool
}

func NewRenderer(c *client.Client) *Renderer {
	return &Renderer{c: c, items: map[string]*routeMenu{}}
}

func (r *Renderer) Init() {
	if len(iconRegularBytes) > 0 {
		systray.SetTemplateIcon(iconTemplateBytes, iconRegularBytes)
	}
	systray.SetTooltip("mkdev — local HTTPS dev proxy")

	r.brandItm = systray.AddMenuItem("mkdev", "")
	r.brandItm.Disable()
	systray.AddSeparator()

	r.statusItm = systray.AddMenuItem("Status:  …", "daemon liveness")
	r.statusItm.Disable()
	r.uptimeItm = systray.AddMenuItem("Uptime:  —", "")
	r.uptimeItm.Disable()
	r.trafficItm = systray.AddMenuItem("Traffic: —", "")
	r.trafficItm.Disable()
	systray.AddSeparator()

	r.routesLbl = systray.AddMenuItem("Routes", "")
	r.routesLbl.Disable()
	r.emptyItm = systray.AddMenuItem("  no routes yet", "")
	r.emptyItm.Disable()
}

func (r *Renderer) installFooter() {
	if r.footerReady {
		return
	}
	systray.AddSeparator()
	r.toggleItm = systray.AddMenuItem("Stop daemon", "")
	r.quitItm = systray.AddMenuItem("Quit", "Close the menu bar; daemon keeps running")
	r.footerReady = true
	go r.handleFooter()
}

func (r *Renderer) updateToggle(daemonUp bool) {
	if r.toggleItm == nil {
		return
	}
	if r.pending {
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

func (r *Renderer) Reconcile(snap Snapshot) {
	r.daemonUp = snap.DaemonUp
	r.brandItm.SetTitle(renderBrand(snap))
	r.statusItm.SetTitle(renderStatus(snap))
	r.uptimeItm.SetTitle(renderUptime(snap))
	r.trafficItm.SetTitle(renderTraffic(snap))

	if len(snap.Routes) == 0 {
		r.emptyItm.Show()
	} else {
		r.emptyItm.Hide()
	}

	r.routesLbl.SetTitle(fmt.Sprintf("Routes (%d)", len(snap.Routes)))

	seen := map[string]struct{}{}
	for _, route := range snap.Routes {
		seen[route.Name] = struct{}{}
		domain := route.Name + snap.TLD
		health := snap.Health[domain]
		title := fmt.Sprintf("  %s  %s  →  %s", healthDot(health), domain, route.Target)
		if item, ok := r.items[route.Name]; ok {
			item.root.SetTitle(title)
			item.root.Show()
			updateShareItem(item.share, route.Share)
			continue
		}
		r.items[route.Name] = r.addRoute(route, domain, title)
	}

	for name, item := range r.items {
		if _, ok := seen[name]; ok {
			continue
		}
		item.root.Hide()
		close(item.stop)
		delete(r.items, name)
	}

	r.installFooter()
	r.updateToggle(snap.DaemonUp)
}

func (r *Renderer) addRoute(route api.Route, domain, title string) *routeMenu {
	root := systray.AddMenuItem(title, "")
	openSub := root.AddSubMenuItem("Open in browser", "https://"+domain)
	shareSub := root.AddSubMenuItemCheckbox("Share on LAN", "Advertise via mDNS to LAN devices", route.Share == api.ShareLAN)
	rm := &routeMenu{
		root:  root,
		open:  openSub,
		share: shareSub,
		stop:  make(chan struct{}),
		name:  route.Name,
	}
	go r.handleRouteClicks(rm, domain)
	return rm
}

func (r *Renderer) handleRouteClicks(rm *routeMenu, domain string) {
	for {
		select {
		case <-rm.stop:
			return
		case <-rm.root.ClickedCh:
			openURL("https://" + domain)
		case <-rm.open.ClickedCh:
			openURL("https://" + domain)
		case <-rm.share.ClickedCh:
			enabled := !rm.share.Checked()
			ctx, cancel := context.WithTimeout(context.Background(), clickTimeout)
			_, err := r.c.ToggleShare(ctx, rm.name, enabled)
			cancel()
			if err != nil {
				slog.Warn("bar: ToggleShare failed", "name", rm.name, "err", err)
				continue
			}
			if enabled {
				rm.share.Check()
			} else {
				rm.share.Uncheck()
			}
		}
	}
}

func (r *Renderer) handleFooter() {
	for {
		select {
		case <-r.quitItm.ClickedCh:
			systray.Quit()
			return
		case <-r.toggleItm.ClickedCh:
			r.onToggleClick()
		}
	}
}

func (r *Renderer) onToggleClick() {
	if r.pending {
		return
	}
	wasUp := r.daemonUp
	r.pending = true
	r.updateToggle(wasUp)
	go func() {
		defer func() {
			r.pending = false
		}()
		if wasUp {
			ctx, cancel := context.WithTimeout(context.Background(), clickTimeout)
			defer cancel()
			if err := r.c.Shutdown(ctx); err != nil {
				slog.Warn("bar: shutdown daemon failed", "err", err)
				return
			}
			slog.Info("bar: daemon shutdown requested")
			return
		}
		if err := startDaemonDetached(); err != nil {
			slog.Warn("bar: start daemon failed", "err", err)
		} else {
			slog.Info("bar: daemon start requested")
		}
	}()
}

func startDaemonDetached() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve exe: %w", err)
	}
	cmd := exec.Command(exe, "daemon", "serve")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	detachProcess(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}

func updateShareItem(item *systray.MenuItem, share api.Share) {
	if share == api.ShareLAN {
		if !item.Checked() {
			item.Check()
		}
	} else if item.Checked() {
		item.Uncheck()
	}
}

func openURL(url string) {
	if err := browser.Open(url); err != nil {
		slog.Warn("bar: browser open failed", "url", url, "err", err)
	}
}
