package daemon

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	mdnspkg "github.com/venkatkrishna07/mkdev/internal/mdns"
	"github.com/venkatkrishna07/mkdev/internal/proxy"
	"github.com/venkatkrishna07/mkdev/internal/proxy/prober"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

// engineOptions tunes the proxy engine. ProxyPort comes from config.toml.
type engineOptions struct {
	HomeDir   string
	ProxyPort int
}

// engine owns the daemon's TLS proxy + cert issuer + prober + mDNS publisher.
// Constructed lazily because it requires the CA on disk; a daemon may run
// without it (API-only) when the user has not yet executed `mkdev install`.
type engine struct {
	opts   engineOptions
	router *proxy.Router
	issuer *cert.Issuer
	stats  *proxy.Stats
	prober *prober.Prober

	mu      sync.Mutex
	srv     *proxy.Server
	pub     atomic.Pointer[mdnspkg.Publisher]
	cancel  context.CancelFunc
	started bool
}

// newEngine constructs the engine. Returns an error only if the CA cannot be
// loaded; callers may treat that as "skip proxy, API only".
func newEngine(opts engineOptions, listRoutes func() ([]store.Route, error)) (*engine, error) {
	ca, err := cert.LoadCA(filepath.Join(opts.HomeDir, "ca"))
	if err != nil {
		return nil, fmt.Errorf("daemon engine: load CA: %w", err)
	}
	router := proxy.NewRouter()
	issuer := cert.NewIssuer(ca, router.Has)
	stats := proxy.NewStats()
	pr := prober.New(listRoutes, 2*time.Second, 500*time.Millisecond)
	return &engine{opts: opts, router: router, issuer: issuer, stats: stats, prober: pr}, nil
}

// Start binds the TLS listener and runs the prober + mDNS publisher.
// Blocks until ctx is cancelled or the server errors. Idempotent: calling
// twice returns an error on the second call.
func (e *engine) Start(ctx context.Context, initial []store.Route) error {
	e.mu.Lock()
	if e.started {
		e.mu.Unlock()
		return errors.New("daemon engine: already started")
	}
	e.started = true
	e.mu.Unlock()

	e.router.Set(initial)

	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(e.opts.ProxyPort))
	ln, err := tls.Listen("tcp", addr, &tls.Config{
		GetCertificate: e.issuer.GetCertificate,
		MinVersion:     tls.VersionTLS13,
	})
	if err != nil {
		return fmt.Errorf("daemon engine: listen %s: %w", addr, err)
	}

	probeCtx, cancel := context.WithCancel(ctx)
	e.mu.Lock()
	e.cancel = cancel
	e.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("daemon engine: prober panic", "panic", r)
			}
		}()
		e.prober.Run(probeCtx)
	}()

	if ip, ipErr := mdnspkg.PrimaryLANIPv4(); ipErr == nil {
		pub := mdnspkg.New(ip)
		if mErr := pub.Set(initial); mErr != nil {
			slog.Warn("daemon engine: mdns initial set", "err", mErr)
		}
		e.pub.Store(pub)
	} else {
		slog.Warn("daemon engine: no LAN IP, mDNS disabled", "err", ipErr)
	}

	srv := proxy.NewServer(e.router, ln, e.stats)
	e.mu.Lock()
	e.srv = srv
	e.mu.Unlock()

	go func() { //nolint:gosec // shutdown uses a fresh context; parent ctx is already Done
		<-ctx.Done()
		shutCtx, sc := context.WithTimeout(context.Background(), 5*time.Second)
		defer sc()
		if err := srv.Shutdown(shutCtx); err != nil {
			slog.Warn("daemon engine: proxy shutdown", "err", err)
		}
		if p := e.pub.Swap(nil); p != nil {
			if err := p.Close(); err != nil {
				slog.Warn("daemon engine: mdns close", "err", err)
			}
		}
		cancel()
	}()

	slog.Info("daemon engine: proxy listening", "addr", addr)
	if err := srv.Serve(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("daemon engine: serve: %w", err)
	}
	return nil
}

// Reload refreshes the router, prunes unused certs, and resyncs mDNS.
// Safe to call from any goroutine; called by Daemon after each mutation.
func (e *engine) Reload(routes []store.Route) {
	e.router.Set(routes)
	e.issuer.Prune(e.router.Has)
	if p := e.pub.Load(); p != nil {
		if err := p.Set(routes); err != nil {
			slog.Warn("daemon engine: mdns refresh", "err", err)
		}
	}
}

// StatsSnapshot returns a current api.Stats payload built from the proxy
// stats counters and the prober's health view. Routes argument supplies
// the active domain list (used as the key set for the routes map).
func (e *engine) StatsSnapshot(routes []store.Route) api.Stats {
	out := api.Stats{
		Tick:   time.Now(),
		Total:  e.stats.Total(),
		RPS:    e.stats.RPS(),
		Routes: make(map[string]api.RouteStats, len(routes)),
	}
	for _, r := range routes {
		health := api.HealthUnknown
		hs := e.prober.Health(r.Domain)
		switch hs.Status.String() {
		case "up":
			health = api.HealthUp
		case "down":
			health = api.HealthDown
		case "checking":
			health = api.HealthProbing
		}
		out.Routes[r.Domain] = api.RouteStats{
			LastSeen: e.stats.LastSeen(r.Domain),
			Health:   health,
		}
	}
	return out
}
