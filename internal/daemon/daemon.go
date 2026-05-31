package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/version"
)

type Options struct {
	HomeDir   string
	TLD       string
	ProxyPort int
}

type Daemon struct {
	opts      Options
	store     *store.Store
	startedAt time.Time
	hub       *Hub
	engine    *engine

	mu         sync.Mutex
	shutdownFn func()
}

type RouteEdit struct {
	Target  *string
	Share   *api.Share
	Enabled *bool
}

func New(opts Options) (*Daemon, error) {
	if opts.HomeDir == "" {
		return nil, fmt.Errorf("daemon: HomeDir required")
	}
	if opts.TLD == "" {
		opts.TLD = ".local"
	}
	if err := os.MkdirAll(opts.HomeDir, 0o700); err != nil {
		return nil, fmt.Errorf("daemon: mkdir homedir: %w", err)
	}
	st, err := store.Open(filepath.Join(opts.HomeDir, "state.db"))
	if err != nil {
		return nil, fmt.Errorf("daemon: open store: %w", err)
	}
	d := &Daemon{opts: opts, store: st, startedAt: time.Now(), hub: NewHub()}
	if opts.ProxyPort > 0 {
		eng, engErr := newEngine(engineOptions{HomeDir: opts.HomeDir, ProxyPort: opts.ProxyPort}, st.ListRoutes)
		if engErr != nil {
			slog.Warn("daemon: proxy disabled", "err", engErr)
		} else {
			d.engine = eng
		}
	}
	slog.Info(
		"daemon: ready",
		"home", opts.HomeDir,
		"tld", opts.TLD,
		"proxy_port", opts.ProxyPort,
		"version", version.String(),
		"engine", d.engine != nil,
	)
	return d, nil
}

func (d *Daemon) Close() error {
	if d.hub != nil {
		d.hub.Close()
		d.hub = nil
	}

	if d.store == nil {
		return nil
	}
	err := d.store.Close()
	d.store = nil
	return err
}

func (d *Daemon) RunEngine(ctx context.Context) error {
	if d.engine == nil {
		return nil
	}
	routes, err := d.store.ListRoutes()
	if err != nil {
		return fmt.Errorf("daemon: list routes: %w", err)
	}
	go d.runStatsTicker(ctx)
	return d.engine.Start(ctx, routes)
}

func (d *Daemon) RunStatusTicker(ctx context.Context) {
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if d.hub == nil {
				continue
			}
			d.hub.Publish(api.NewEvent(api.EventStatusTick, d.Status()))
		}
	}
}

func (d *Daemon) runStatsTicker(ctx context.Context) {
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if d.engine == nil || d.hub == nil {
				continue
			}
			routes, err := d.store.ListRoutes()
			if err != nil {
				continue
			}
			d.hub.Publish(api.NewEvent(api.EventStatsTick, d.engine.StatsSnapshot(routes)))
		}
	}
}

func (d *Daemon) reloadEngine() {
	if d.engine == nil {
		return
	}
	routes, err := d.store.ListRoutes()
	if err != nil {
		slog.Warn("daemon: reloadEngine list routes", "err", err)
		return
	}
	d.engine.Reload(routes)
}

func (d *Daemon) Routes() ([]api.Route, error) {
	srs, err := d.store.ListRoutes()
	if err != nil {
		return nil, err
	}
	out := make([]api.Route, 0, len(srs))
	for _, r := range srs {
		out = append(out, APIFromStore(r))
	}
	return out, nil
}

func (d *Daemon) AddRoute(r api.Route) (api.Route, error) {
	if err := ValidateName(r.Name); err != nil {
		return api.Route{}, err
	}
	if err := ValidateTarget(r.Target); err != nil {
		return api.Route{}, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	domain := r.Name + d.opts.TLD
	if _, err := d.store.GetRoute(domain); err == nil {
		return api.Route{}, api.Error{
			Code:    api.CodeRouteDuplicate,
			Message: fmt.Sprintf("route %q already exists", r.Name),
		}
	}
	sr := StoreFromAPI(r, d.opts.TLD)
	if err := d.store.PutRoute(sr); err != nil {
		return api.Route{}, api.Error{
			Code:    api.CodeStoreWriteFailed,
			Message: err.Error(),
		}
	}
	out := APIFromStore(sr)
	d.hub.Publish(api.NewEvent(api.EventRouteAdded, out))
	d.reloadEngine()
	slog.Info("daemon: route added", "name", r.Name, "domain", domain, "target", r.Target, "share", out.Share)
	return out, nil
}

func (d *Daemon) RemoveRoute(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	domain := name + d.opts.TLD
	if _, err := d.store.GetRoute(domain); err != nil {
		return api.Error{Code: api.CodeRouteNotFound, Message: fmt.Sprintf("no route %q", name)}
	}
	if err := d.store.DeleteRoute(domain); err != nil {
		return api.Error{Code: api.CodeStoreWriteFailed, Message: err.Error()}
	}
	d.hub.Publish(api.NewEvent(api.EventRouteRemoved, map[string]string{"name": name}))
	d.reloadEngine()
	slog.Info("daemon: route removed", "name", name, "domain", domain)
	return nil
}

func (d *Daemon) EditRoute(name string, e RouteEdit) (api.Route, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	domain := name + d.opts.TLD
	cur, err := d.store.GetRoute(domain)
	if err != nil {
		return api.Route{}, api.Error{Code: api.CodeRouteNotFound, Message: fmt.Sprintf("no route %q", name)}
	}
	if e.Target != nil {
		if err := ValidateTarget(*e.Target); err != nil {
			return api.Route{}, err
		}
		cur.Target = *e.Target
	}
	if e.Share != nil {
		cur.Shared = *e.Share == api.ShareLAN
	}
	if e.Enabled != nil {
		cur.Enabled = *e.Enabled
	}
	if err := d.store.PutRoute(cur); err != nil {
		return api.Route{}, api.Error{Code: api.CodeStoreWriteFailed, Message: err.Error()}
	}
	out := APIFromStore(cur)
	d.hub.Publish(api.NewEvent(api.EventRouteChanged, out))
	d.reloadEngine()
	slog.Info("daemon: route changed", "name", name, "domain", domain, "target", cur.Target, "share", out.Share)
	return out, nil
}

func (d *Daemon) ToggleShare(name string, enabled bool) (api.Route, error) {
	share := api.ShareNone
	if enabled {
		share = api.ShareLAN
	}
	return d.EditRoute(name, RouteEdit{Share: &share})
}

func (d *Daemon) Status() api.Status {
	return api.Status{
		Version:    version.String(),
		APIVersion: api.APIVersion,
		PID:        os.Getpid(),
		Uptime:     time.Since(d.startedAt).Truncate(time.Second).String(),
		CertReady:  d.engine != nil,
		StartedAt:  d.startedAt,
		TLD:        d.opts.TLD,
		ProxyPort:  d.opts.ProxyPort,
	}
}

func (d *Daemon) SetShutdownHook(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.shutdownFn = fn
}

func (d *Daemon) invokeShutdownHook() {
	d.mu.Lock()
	fn := d.shutdownFn
	d.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (d *Daemon) Hub() *Hub { return d.hub }
