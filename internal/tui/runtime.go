package tui

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	"github.com/venkatkrishna07/mkdev/internal/config"
	"github.com/venkatkrishna07/mkdev/internal/proxy"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

// Runtime is the shared state of the TUI.
type Runtime struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	Home   string
	Cfg    config.Config
	Router *proxy.Router
	Issuer *cert.Issuer
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
	return &Runtime{Ctx: ctx, Cancel: cancel, Home: home, Cfg: cfg, Router: r, Issuer: is}, nil
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
		defer close(ch)
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(rt.Cfg.ProxyPort))
		ln, err := tls.Listen("tcp", addr, &tls.Config{
			GetCertificate: rt.Issuer.GetCertificate,
			MinVersion:     tls.VersionTLS13,
		})
		if err != nil {
			ch <- ProxyState{Up: false, Err: err}
			return
		}
		ch <- ProxyState{Up: true, Addr: ln.Addr().String()}
		srv := proxy.NewServer(rt.Router, ln)
		go func() {
			<-rt.Ctx.Done()
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutCtx)
		}()
		if err := srv.Serve(); err != nil {
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
		return RoutesRefreshed{Routes: rs, Err: err}
	})
}
