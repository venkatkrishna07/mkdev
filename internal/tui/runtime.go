package tui

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
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	"github.com/venkatkrishna07/mkdev/internal/config"
	mdnspkg "github.com/venkatkrishna07/mkdev/internal/mdns"
	"github.com/venkatkrishna07/mkdev/internal/proxy"
	"github.com/venkatkrishna07/mkdev/internal/proxy/prober"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

// Runtime is the shared state of the TUI.
type Runtime struct {
	Ctx     context.Context
	Cancel  context.CancelFunc
	Home    string
	Cfg     config.Config
	Router  *proxy.Router
	Issuer  *cert.Issuer
	Stats   *proxy.Stats
	Store   *store.Store
	Prober  *prober.Prober
	mdnsPub atomic.Pointer[mdnspkg.Publisher]
}

// LANState is a snapshot of LAN-share visibility for dashboard rendering.
type LANState struct {
	IP          string
	Advertising bool
	SharedCount int
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
		return nil, fmt.Errorf("CA not found — run `mkdev install` first: %w", err)
	}
	st, err := store.Open(filepath.Join(home, "state.db"))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open store: %w", err)
	}
	r := proxy.NewRouter()
	is := cert.NewIssuer(ca, r.Has)
	stats := proxy.NewStats()
	pr := prober.New(st.ListRoutes, 2*time.Second, 500*time.Millisecond)
	return &Runtime{Ctx: ctx, Cancel: cancel, Home: home, Cfg: cfg, Router: r, Issuer: is, Stats: stats, Store: st, Prober: pr}, nil
}

// Close releases long-lived resources held by the runtime (currently the
// shared bbolt store handle). Safe to call multiple times. Cancel should be
// called first so background goroutines stop touching the store.
func (rt *Runtime) Close() error {
	if rt.Store != nil {
		return rt.Store.Close()
	}
	return nil
}

// LoadRoutes returns the current route set from the shared store handle.
func (rt *Runtime) LoadRoutes() ([]store.Route, error) {
	return rt.Store.ListRoutes()
}

// StartProxy binds the TLS listener and serves until Ctx is cancelled.
// Sends ProxyState updates via the returned channel.
func (rt *Runtime) StartProxy() <-chan ProxyState {
	ch := make(chan ProxyState, 4)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("proxy goroutine panic", "panic", r)
				ch <- ProxyState{Up: false, Err: fmt.Errorf("panic: %v", r)}
			}
			close(ch)
		}()
		addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(rt.Cfg.ProxyPort))
		ln, err := tls.Listen("tcp", addr, &tls.Config{
			GetCertificate: rt.Issuer.GetCertificate,
			MinVersion:     tls.VersionTLS13,
		})
		if err != nil {
			ch <- ProxyState{Up: false, Err: err}
			return
		}
		ch <- ProxyState{Up: true, Addr: fmt.Sprintf(":%d", rt.Cfg.ProxyPort)}
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("prober goroutine panic", "panic", r)
				}
			}()
			rt.Prober.Run(rt.Ctx)
		}()
		routes, _ := rt.LoadRoutes()
		ip, ipErr := mdnspkg.PrimaryLANIPv4()
		if ipErr != nil {
			rt.mdnsPub.Store(nil)
			ch <- ProxyState{Up: true, Addr: fmt.Sprintf(":%d", rt.Cfg.ProxyPort), Err: fmt.Errorf("mdns: %w", ipErr)}
		} else {
			pub := mdnspkg.New(ip)
			rt.mdnsPub.Store(pub)
			if err := pub.Set(routes); err != nil {
				slog.Warn("mdns set failed", "err", err)
				ch <- ProxyState{Up: true, Addr: fmt.Sprintf(":%d", rt.Cfg.ProxyPort), Err: fmt.Errorf("mdns: %w", err)}
			}
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("mdns close goroutine panic", "panic", r)
					}
				}()
				<-rt.Ctx.Done()
				if p := rt.mdnsPub.Load(); p != nil {
					if err := p.Close(); err != nil {
						slog.Warn("mdns close failed", "err", err)
					}
				}
			}()
		}
		srv := proxy.NewServer(rt.Router, ln, rt.Stats)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("proxy shutdown goroutine panic", "panic", r)
				}
			}()
			<-rt.Ctx.Done()
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(shutCtx); err != nil {
				slog.Warn("proxy shutdown failed", "err", err)
			}
		}()
		if err := srv.Serve(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			ch <- ProxyState{Up: false, Err: err}
		}
	}()
	return ch
}

// RefreshTick is a tea.Cmd that returns a RoutesRefreshed after delay.
func (rt *Runtime) RefreshTick(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		rs, err := rt.LoadRoutes()
		if err == nil {
			rt.Router.Set(rs)
			rt.Issuer.Prune(rt.Router.Has)
			if p := rt.mdnsPub.Load(); p != nil {
				if mErr := p.Set(rs); mErr != nil {
					slog.Warn("mdns refresh failed", "err", mErr)
				}
			}
		}
		return RoutesRefreshed{Routes: rs, Err: err}
	})
}

// LANState returns a snapshot of mDNS advertising state and the number of
// routes currently shared on the LAN.
func (rt *Runtime) LANState() LANState {
	var st LANState
	if p := rt.mdnsPub.Load(); p != nil {
		st.Advertising = true
		if ip, err := mdnspkg.PrimaryLANIPv4(); err == nil {
			st.IP = ip.String()
		}
	}
	routes, _ := rt.Store.ListRoutes()
	for _, r := range routes {
		if r.Enabled && r.Shared {
			st.SharedCount++
		}
	}
	return st
}
