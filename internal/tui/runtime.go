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
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	"github.com/venkatkrishna07/mkdev/internal/config"
	mdnspkg "github.com/venkatkrishna07/mkdev/internal/mdns"
	"github.com/venkatkrishna07/mkdev/internal/proxy"
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
	mdnsPub *mdnspkg.Publisher
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
	r := proxy.NewRouter()
	is := cert.NewIssuer(ca, r.Has)
	st := proxy.NewStats()
	return &Runtime{Ctx: ctx, Cancel: cancel, Home: home, Cfg: cfg, Router: r, Issuer: is, Stats: st}, nil
}

// OpenStore returns a transient store handle. Caller MUST close.
func (rt *Runtime) OpenStore() (*store.Store, error) {
	return store.Open(filepath.Join(rt.Home, "state.db"))
}

// LoadRoutes opens the store, lists, closes, and returns.
func (rt *Runtime) LoadRoutes() ([]store.Route, error) {
	s, err := rt.OpenStore()
	if err != nil {
		return nil, err
	}
	defer s.Close()
	return s.ListRoutes()
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
		ch <- ProxyState{Up: true, Addr: ln.Addr().String()}
		routes, _ := rt.LoadRoutes()
		ip, ipErr := mdnspkg.PrimaryLANIPv4()
		if ipErr != nil {
			rt.mdnsPub = nil
			ch <- ProxyState{Up: true, Addr: ln.Addr().String(), Err: fmt.Errorf("mdns: %w", ipErr)}
		} else {
			rt.mdnsPub = mdnspkg.New(ip)
			if err := rt.mdnsPub.Set(routes); err != nil {
				slog.Warn("mdns set failed", "err", err)
				ch <- ProxyState{Up: true, Addr: ln.Addr().String(), Err: fmt.Errorf("mdns: %w", err)}
			}
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("mdns close goroutine panic", "panic", r)
					}
				}()
				<-rt.Ctx.Done()
				if err := rt.mdnsPub.Close(); err != nil {
					slog.Warn("mdns close failed", "err", err)
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
		rt.Router.Set(rs)
		rt.Issuer.Prune(rt.Router.Has)
		if rt.mdnsPub != nil {
			if mErr := rt.mdnsPub.Set(rs); mErr != nil {
				slog.Warn("mdns refresh failed", "err", mErr)
			}
		}
		return RoutesRefreshed{Routes: rs, Err: err}
	})
}
