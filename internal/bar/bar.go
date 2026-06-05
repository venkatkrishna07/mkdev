package bar

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fyne.io/systray"
	"github.com/venkatkrishna07/mkdev/internal/client"
)

//go:embed assets/icon-template.png
var iconTemplateBytes []byte

//go:embed assets/icon-regular.png
var iconRegularBytes []byte

const reconcileDelay = 250 * time.Millisecond

func Run() error {
	release, err := acquireLock()
	if err != nil {
		return err
	}
	defer release()

	c, err := client.New(client.Options{})
	if err != nil {
		return fmt.Errorf("bar: client: %w", err)
	}
	defer func() { _ = c.Close() }()

	state := NewState()
	renderer := NewRenderer(c)
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			slog.Info("bar: signal received, quitting")
			systray.Quit()
		case <-ctx.Done():
		}
	}()

	dirty := make(chan struct{}, 1)
	markDirty := func() {
		select {
		case dirty <- struct{}{}:
		default:
		}
	}

	refresh := func() {
		rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if st, err := c.Status(rctx); err == nil {
			state.SetTLD(st.TLD)
			state.SetMeta(st.Version, st.PID, st.Uptime, st.ProxyPort)
			state.SetDaemonUp(true)
		} else {
			slog.Warn("bar: daemon status failed", "err", err)
		}
		if rs, err := c.Routes(rctx); err == nil {
			state.ReplaceRoutes(rs)
		} else {
			slog.Warn("bar: daemon routes failed", "err", err)
		}
		markDirty()
	}

	onReady := func() {
		renderer.Init()
		go refresh()
		go listenLoop(ctx, c, state, refresh, markDirty)
		go renderLoop(ctx, state, renderer, dirty)
	}

	onExit := func() {
		cancel()
	}

	systray.Run(onReady, onExit)
	return nil
}

func listenLoop(ctx context.Context, c *client.Client, st *State, refresh, dirty func()) {
	ch := c.Subscribe(ctx)
	for ev := range ch {
		if ev.Type == client.EventClientReconnected {
			go refresh()
		}
		if st.Apply(ev) {
			dirty()
		}
	}
}

func renderLoop(ctx context.Context, st *State, r *Renderer, dirty <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-dirty:
		}

		timer := time.NewTimer(reconcileDelay)
	drain:
		for {
			select {
			case <-timer.C:
				break drain
			case <-dirty:

			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
		r.Reconcile(st.Snapshot())
	}
}
