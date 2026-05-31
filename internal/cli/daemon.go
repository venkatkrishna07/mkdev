package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/config"
	"github.com/venkatkrishna07/mkdev/internal/daemon"
)

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the mkdev background daemon",
		Long:  "Manage the mkdev background daemon that owns the proxy and serves a local HTTP API over a unix socket.",
	}
	cmd.AddCommand(
		newDaemonServeCmd(),
		newDaemonInstallCmd(),
		newDaemonUninstallCmd(),
		newDaemonEnableCmd(),
		newDaemonDisableCmd(),
		newDaemonStopCmd(),
		newDaemonStatusCmd(),
	)
	return cmd
}

func newDaemonServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the mkdev daemon in the foreground",
		Long: `Run the mkdev daemon. The daemon binds a unix socket at
~/.mkdev/daemon.sock and exposes the local HTTP API. Send SIGTERM or SIGINT to
stop gracefully.`,
		RunE: runDaemonServe,
	}
}

func runDaemonServe(cmd *cobra.Command, _ []string) error {
	configureLogLevel()
	home, err := HomeDir()
	if err != nil {
		return err
	}
	release, err := daemon.AcquirePidfile(home)
	if err != nil {
		return err
	}
	defer release()

	cfg, err := config.Load(filepath.Join(home, "config.toml"))
	if err != nil {
		return fmt.Errorf("daemon: load config: %w", err)
	}
	tld := cfg.TLD
	if tld == "" {
		tld = ".local"
	}
	d, err := daemon.New(daemon.Options{HomeDir: home, TLD: tld, ProxyPort: cfg.ProxyPort})
	if err != nil {
		return fmt.Errorf("daemon: %w", err)
	}
	defer func() { _ = d.Close() }()

	ln, err := daemon.OpenSocket(home)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Handler:           daemon.NewHandler(d),
		ReadHeaderTimeout: 10 * time.Second,
	}

	socketPath := daemon.SocketPath(home)
	_, _ = fmt.Fprintf(cmd.OutOrStderr(), "mkdev daemon: API listening on %s\n", socketPath)

	stopCh := make(chan struct{}, 1)
	d.SetShutdownHook(func() {
		select {
		case stopCh <- struct{}{}:
		default:
		}
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	engineCtx, engineCancel := context.WithCancel(cmd.Context())
	defer engineCancel()
	go d.RunStatusTicker(engineCtx)
	var engineErr error
	engineExited := make(chan struct{})
	go func() {
		engineErr = d.RunEngine(engineCtx)
		if engineErr != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStderr(), "mkdev daemon: proxy failed: %v\n", engineErr)
		}
		close(engineExited)
	}()

	go func() {
		select {
		case <-sigCh:
		case <-stopCh:
		case <-engineExited:

		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	serveErr := srv.Serve(ln)
	engineCancel()
	<-engineExited
	if engineErr != nil && !errors.Is(engineErr, context.Canceled) {
		return fmt.Errorf("daemon: engine: %w", engineErr)
	}
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return fmt.Errorf("daemon: serve: %w", serveErr)
	}
	return nil
}
