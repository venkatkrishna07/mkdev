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

// Options configures a Daemon.
type Options struct {
	HomeDir   string // resolved ~/.mkdev (or override)
	TLD       string // route suffix, e.g. ".local"
	ProxyPort int    // TLS listen port for the proxy; 0 disables proxy (API-only mode)
}

// Daemon is the orchestrator. It owns the store and (later) proxy/mdns.
// HTTP server lifecycle is managed separately (see lifecycle.go and cli wiring).
type Daemon struct {
	opts      Options
	store     *store.Store
	startedAt time.Time
	hub       *Hub
	engine    *engine // nil when CA missing or ProxyPort == 0

	mu         sync.Mutex // serializes mutations and shutdownFn writes
	shutdownFn func()
}

// RouteEdit is the patch payload for EditRoute. Nil fields are not modified.
type RouteEdit struct {
	Target *string
	Share  *api.Share
}

// New opens the store and returns a Daemon ready to serve requests.
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
	return d, nil
}

// Close releases the store. Idempotent.
func (d *Daemon) Close() error {
	if d.hub != nil {
		d.hub.Close()
		d.hub = nil
	}
	// engine lifetime is bound to RunEngine's ctx; do not call into it here.
	if d.store == nil {
		return nil
	}
	err := d.store.Close()
	d.store = nil
	return err
}

// RunEngine starts the TLS proxy engine if available and blocks until ctx is
// cancelled. Returns nil (no-op) when the engine was not constructed.
// Safe to call once per Daemon.
func (d *Daemon) RunEngine(ctx context.Context) error {
	if d.engine == nil {
		return nil
	}
	routes, err := d.store.ListRoutes()
	if err != nil {
		return fmt.Errorf("daemon: list routes: %w", err)
	}
	return d.engine.Start(ctx, routes)
}

// reloadEngine pushes the latest route snapshot to the engine's router + mDNS.
// No-op when engine is nil.
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

// Routes returns all routes as api.Route.
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

// AddRoute validates, persists, and returns the route as api.Route.
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
	return out, nil
}

// RemoveRoute deletes by name.
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
	return nil
}

// EditRoute applies non-nil fields and persists.
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
	if err := d.store.PutRoute(cur); err != nil {
		return api.Route{}, api.Error{Code: api.CodeStoreWriteFailed, Message: err.Error()}
	}
	out := APIFromStore(cur)
	d.hub.Publish(api.NewEvent(api.EventRouteChanged, out))
	d.reloadEngine()
	return out, nil
}

// ToggleShare flips the shared bit on a route.
// M1 does not publish via mDNS; that wiring lands in a later milestone.
func (d *Daemon) ToggleShare(name string, enabled bool) (api.Route, error) {
	share := api.ShareNone
	if enabled {
		share = api.ShareLAN
	}
	return d.EditRoute(name, RouteEdit{Share: &share})
}

// Status returns daemon metadata for GET /v1/status.
func (d *Daemon) Status() api.Status {
	return api.Status{
		Version:    version.String(),
		APIVersion: api.APIVersion,
		PID:        os.Getpid(),
		Uptime:     time.Since(d.startedAt).Truncate(time.Second).String(),
		CertReady:  false, // M1: not wired to cert package
		StartedAt:  d.startedAt,
		TLD:        d.opts.TLD,
	}
}

// SetShutdownHook installs a callback invoked when /v1/shutdown is POSTed.
func (d *Daemon) SetShutdownHook(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.shutdownFn = fn
}

// invokeShutdownHook calls the installed hook (if any). Used by handlers.
func (d *Daemon) invokeShutdownHook() {
	d.mu.Lock()
	fn := d.shutdownFn
	d.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Hub returns the SSE hub so HTTP handlers can subscribe consumers to events.
func (d *Daemon) Hub() *Hub { return d.hub }
