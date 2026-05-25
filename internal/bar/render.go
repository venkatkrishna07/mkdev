//go:build darwin

package bar

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/getlantern/systray"
	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/browser"
	"github.com/venkatkrishna07/mkdev/internal/client"
)

const clickTimeout = 5 * time.Second

type routeMenu struct {
	root  *systray.MenuItem
	share *systray.MenuItem
	open  *systray.MenuItem
	stop  chan struct{}
	name  string
}

type Renderer struct {
	c         *client.Client
	items     map[string]*routeMenu
	headerItm *systray.MenuItem
	quitItm   *systray.MenuItem
	disableD  *systray.MenuItem
	separator bool
}

func NewRenderer(c *client.Client) *Renderer {
	return &Renderer{c: c, items: map[string]*routeMenu{}}
}

func (r *Renderer) Init() {
	systray.SetTitle("mkdev")
	systray.SetTooltip("mkdev — local HTTPS dev proxy")

	r.headerItm = systray.AddMenuItem("daemon: …", "daemon connection status")
	r.headerItm.Disable()
	systray.AddSeparator()
}

func (r *Renderer) installFooter() {
	if r.separator {
		return
	}
	systray.AddSeparator()
	r.separator = true
	r.disableD = systray.AddMenuItem("Disable daemon", "POST /v1/shutdown to the daemon")
	r.quitItm = systray.AddMenuItem("Quit menu bar", "Exit only the menu bar; daemon keeps running")
	go r.handleFooter()
}

func (r *Renderer) Reconcile(snap Snapshot) {
	r.headerItm.SetTitle(renderHeader(snap))

	r.installFooter()

	seen := map[string]struct{}{}
	for _, route := range snap.Routes {
		seen[route.Name] = struct{}{}
		domain := route.Name + snap.TLD
		health := snap.Health[domain]
		title := fmt.Sprintf("%s  %s → %s", healthDot(health), domain, route.Target)
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
}

func (r *Renderer) addRoute(route api.Route, domain, title string) *routeMenu {
	root := systray.AddMenuItem(title, fmt.Sprintf("Open https://%s in your browser", domain))
	openSub := root.AddSubMenuItem(fmt.Sprintf("Open https://%s", domain), "Open in default browser")
	shareSub := root.AddSubMenuItemCheckbox("Share on LAN", "Advertise via mDNS to LAN devices", route.Share == api.ShareLAN)
	rm := &routeMenu{root: root, open: openSub, share: shareSub, stop: make(chan struct{}), name: route.Name}
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
		case <-r.disableD.ClickedCh:
			ctx, cancel := context.WithTimeout(context.Background(), clickTimeout)
			err := r.c.Shutdown(ctx)
			cancel()
			if err != nil {
				slog.Warn("bar: shutdown daemon failed", "err", err)
			} else {
				slog.Info("bar: daemon shutdown requested")
			}
		}
	}
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
