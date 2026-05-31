package tui

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	"github.com/venkatkrishna07/mkdev/internal/client"
	"github.com/venkatkrishna07/mkdev/internal/config"
	mdnspkg "github.com/venkatkrishna07/mkdev/internal/mdns"
	"github.com/venkatkrishna07/mkdev/internal/proxy/prober"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

var jsonUnmarshal = json.Unmarshal

// Runtime is the shared state of the TUI. After the daemon migration the
// runtime no longer owns the proxy, store, mdns, or cert issuer — those live
// in the daemon process. The TUI uses Client for reads/writes; live stats
// and route changes arrive over the SSE event stream.
type Runtime struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	Home   string
	Cfg    config.Config
	Client *client.Client
	CA     *x509.Certificate // loaded from disk for display; may be nil

	mu        sync.Mutex
	routes    []store.Route
	tld       string
	liveStats api.Stats
	send      func(tea.Msg)

	daemonUp atomic.Bool
}

// LANState is a snapshot of LAN-share visibility for dashboard rendering.
// Advertising reflects daemon liveness (the daemon owns mDNS).
type LANState struct {
	IP          string
	Advertising bool
	SharedCount int
}

// NewRuntime resolves config, opens a client to the daemon, and constructs
// nil-safe placeholders for Stats/Prober used by the dashboard. The CA is
// loaded read-only for display; absence is non-fatal in the TUI (daemon
// reports its own cert state via /v1/status).
func NewRuntime(ctx context.Context, home string) (*Runtime, error) {
	ctx, cancel := context.WithCancel(ctx)
	cfg, err := config.Load(filepath.Join(home, "config.toml"))
	if err != nil {
		cancel()
		return nil, err
	}
	c, err := client.New(client.Options{})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("client: %w", err)
	}
	rt := &Runtime{
		Ctx:    ctx,
		Cancel: cancel,
		Home:   home,
		Cfg:    cfg,
		Client: c,
		tld:    cfg.TLD,
	}
	if ca, err := cert.LoadCA(filepath.Join(home, "ca")); err == nil {
		rt.CA = ca.Cert
	}
	return rt, nil
}

// Close releases client resources. Safe to call multiple times.
func (rt *Runtime) Close() error {
	if rt.Client != nil {
		err := rt.Client.Close()
		rt.Client = nil
		return err
	}
	return nil
}

// LoadRoutes fetches the current route set from the daemon and caches it for
// LANState/proberRoutes lookups. Returns store.Route shape for tab compat.
func (rt *Runtime) LoadRoutes() ([]store.Route, error) {
	ctx, cancel := context.WithTimeout(rt.Ctx, 3*time.Second)
	defer cancel()
	routes, err := rt.Client.Routes(ctx)
	if err != nil {
		return nil, err
	}
	out := routesFromAPI(routes, rt.tld)
	rt.mu.Lock()
	rt.routes = out
	rt.mu.Unlock()
	return out, nil
}

// StartProxy probes the daemon's liveness and emits ProxyState. It does NOT
// start a TLS listener — the daemon owns the proxy. The channel emits one
// initial state, then closes on Ctx.Done.
func (rt *Runtime) StartProxy() <-chan ProxyState {
	ch := make(chan ProxyState, 4)
	go func() {
		defer close(ch)
		ctx, cancel := context.WithTimeout(rt.Ctx, 2*time.Second)
		defer cancel()
		st, err := rt.Client.Status(ctx)
		if err != nil {
			rt.daemonUp.Store(false)
			ch <- ProxyState{Up: false, Err: fmt.Errorf("daemon not reachable: %w", err)}
		} else {
			rt.daemonUp.Store(true)
			ch <- ProxyState{Up: true, Addr: fmt.Sprintf(":%d", rt.Cfg.ProxyPort)}
			if rt.tld == "" {
				rt.mu.Lock()
				rt.tld = st.TLD
				rt.mu.Unlock()
			}
		}
		<-rt.Ctx.Done()
	}()
	return ch
}

func (rt *Runtime) SetSender(send func(tea.Msg)) {
	rt.mu.Lock()
	rt.send = send
	rt.mu.Unlock()
}

func (rt *Runtime) push(msg tea.Msg) {
	rt.mu.Lock()
	send := rt.send
	rt.mu.Unlock()
	if send != nil {
		send(msg)
	}
}

func (rt *Runtime) HealthOf(domain string) prober.HealthState {
	rt.mu.Lock()
	rs, ok := rt.liveStats.Routes[domain]
	tick := rt.liveStats.Tick
	rt.mu.Unlock()
	if !ok {
		return prober.HealthState{}
	}
	st := prober.HealthState{LastProbe: tick}
	switch rs.Health {
	case api.HealthUp:
		st.Status = prober.StatusUp
	case api.HealthDown:
		st.Status = prober.StatusDown
	default:
		st.Status = prober.StatusOff
	}
	return st
}

// LANState reports LAN-share visibility. Advertising tracks daemon liveness
// (the daemon owns mDNS). IP is reported on a best-effort basis.
func (rt *Runtime) LANState() LANState {
	rt.mu.Lock()
	routes := make([]store.Route, len(rt.routes))
	copy(routes, rt.routes)
	rt.mu.Unlock()
	st := LANState{Advertising: rt.daemonUp.Load()}
	if ip, err := mdnspkg.PrimaryLANIPv4(); err == nil {
		st.IP = ip.String()
	}
	for _, r := range routes {
		if r.Enabled && r.Shared {
			st.SharedCount++
		}
	}
	return st
}

// TotalReqs returns the cumulative request count reported by the daemon.
// Returns 0 if no stats.tick has been received yet.
func (rt *Runtime) TotalReqs() uint64 {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.liveStats.Total
}

// RPSWindow returns the latest rolling per-second request counts from the
// daemon. Returns nil if no stats.tick has been received yet.
func (rt *Runtime) RPSWindow() []float64 {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if len(rt.liveStats.RPS) == 0 {
		return nil
	}
	out := make([]float64, len(rt.liveStats.RPS))
	copy(out, rt.liveStats.RPS)
	return out
}

// LastSeenHost returns the time host last served a request, per the daemon.
func (rt *Runtime) LastSeenHost(host string) time.Time {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rs, ok := rt.liveStats.Routes[host]; ok {
		return rs.LastSeen
	}
	return time.Time{}
}

func (rt *Runtime) SubscribeEvents() {
	go func() {
		defer func() { _ = recover() }()
		ch := rt.Client.Subscribe(rt.Ctx)
		for ev := range ch {
			switch ev.Type {
			case api.EventStatsTick:
				var s api.Stats
				if err := decodeEventData(ev, &s); err != nil {
					continue
				}
				rt.mu.Lock()
				rt.liveStats = s
				rt.mu.Unlock()
			case api.EventRouteAdded, api.EventRouteChanged, api.EventRouteRemoved:
				rs, err := rt.LoadRoutes()
				rt.push(RoutesRefreshed{Routes: rs, Err: err})
			case client.EventClientReconnected:
				rt.daemonUp.Store(true)
				rs, err := rt.LoadRoutes()
				rt.push(RoutesRefreshed{Routes: rs, Err: err})
			case client.EventClientDisconnected:
				rt.daemonUp.Store(false)
			}
		}
	}()
}

func (rt *Runtime) RefreshNow() tea.Cmd {
	return func() tea.Msg {
		rs, err := rt.LoadRoutes()
		return RoutesRefreshed{Routes: rs, Err: err}
	}
}

func decodeEventData(ev api.Event, out any) error {
	if len(ev.Data) == 0 {
		return errors.New("empty event data")
	}
	return jsonUnmarshal(ev.Data, out)
}

func routesFromAPI(in []api.Route, tld string) []store.Route {
	out := make([]store.Route, 0, len(in))
	for _, r := range in {
		out = append(out, store.Route{
			Domain:   r.Name + tld,
			Target:   r.Target,
			TLD:      tld,
			Enabled:  true,
			Shared:   r.Share == api.ShareLAN,
			Insecure: r.Insecure,
			Source:   store.SourceAdHoc,
		})
	}
	return out
}

