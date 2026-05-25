//go:build darwin

package bar

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/getlantern/systray"
	"github.com/venkatkrishna07/mkdev/internal/client"
)

const reconcileDelay = 250 * time.Millisecond

func Run() error {
	c, err := client.New(client.Options{})
	if err != nil {
		return fmt.Errorf("bar: client: %w", err)
	}
	defer func() { _ = c.Close() }()

	state := NewState()
	renderer := NewRenderer(c)
	ctx, cancel := context.WithCancel(context.Background())

	dirty := make(chan struct{}, 1)
	markDirty := func() {
		select {
		case dirty <- struct{}{}:
		default:
		}
	}

	onReady := func() {
		renderer.Init()

		go func() {
			initCtx, ic := context.WithTimeout(ctx, 5*time.Second)
			defer ic()
			if st, err := c.Status(initCtx); err == nil {
				state.SetTLD(st.TLD)
				state.SetMeta(st.Version, st.PID, st.Uptime)
				state.SetDaemonUp(true)
			} else {
				slog.Warn("bar: daemon status failed", "err", err)
			}
			if rs, err := c.Routes(initCtx); err == nil {
				state.ReplaceRoutes(rs)
			} else {
				slog.Warn("bar: daemon routes failed", "err", err)
			}
			markDirty()
		}()

		go listenLoop(ctx, c, state, markDirty)
		go renderLoop(ctx, state, renderer, dirty)
	}

	onExit := func() {
		cancel()
	}

	systray.Run(onReady, onExit)
	return nil
}

func listenLoop(ctx context.Context, c *client.Client, st *State, dirty func()) {
	ch := c.Subscribe(ctx)
	for ev := range ch {
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
