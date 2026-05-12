package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	"github.com/venkatkrishna07/mkdev/internal/config"
	"github.com/venkatkrishna07/mkdev/internal/proxy"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

const reloadInterval = 2 * time.Second

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the foreground reverse proxy on :<proxy_port>",
		RunE:  runServe,
	}
}

func runServe(cmd *cobra.Command, _ []string) error {
	home, err := HomeDir()
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(home, "config.toml"))
	if err != nil {
		return err
	}
	ca, err := cert.LoadCA(filepath.Join(home, "ca"))
	if err != nil {
		return fmt.Errorf("CA not found — run `mkdev install` first: %w", err)
	}

	s, err := store.Open(filepath.Join(home, "state.db"))
	if err != nil {
		return err
	}
	defer s.Close()

	router := proxy.NewRouter()
	routes, err := s.ListRoutes()
	if err != nil {
		return err
	}
	router.Set(routes)

	is := cert.NewIssuer(ca, router.Has)

	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Go(func() {
		t := time.NewTicker(reloadInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if rs, err := s.ListRoutes(); err == nil {
					router.Set(rs)
				}
			}
		}
	})

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.ProxyPort))
	ln, err := tls.Listen("tcp", addr, &tls.Config{
		GetCertificate: is.GetCertificate,
		MinVersion:     tls.VersionTLS13,
	})
	if err != nil {
		return fmt.Errorf("listen %s: %w (binding :443 requires sudo or CAP_NET_BIND_SERVICE)", addr, err)
	}
	defer ln.Close()
	srv := proxy.NewServer(router, ln)
	slog.Info("proxy: listening", "addr", ln.Addr().String(), "routes", len(routes))

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve() }()

	select {
	case <-ctx.Done():
		slog.Info("proxy: shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutErr := srv.Shutdown(shutCtx)
		<-errCh
		wg.Wait()
		return shutErr
	case err := <-errCh:
		wg.Wait()
		return err
	}
}
