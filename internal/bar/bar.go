//go:build darwin

// Package bar implements the macOS menu-bar client for mkdev. It connects to
// the daemon over the local unix socket, subscribes to the SSE event stream,
// and renders a small set of menu items (status, per-route entries with a
// Share-on-LAN toggle, quit). All systray calls happen on the main goroutine.
package bar

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/getlantern/systray"
	"github.com/venkatkrishna07/mkdev/internal/client"
)

// reconcileDelay coalesces bursts of events into a single render pass. See
// the design spec, section 7.1 ("event → state → 250ms-coalesced reconcile").
const reconcileDelay = 250 * time.Millisecond

// Run launches the menu-bar app. It blocks the caller (systray must run on
// the main goroutine on macOS) until the user quits the bar.
//
// Run handles its own context: a SIGINT or a Quit menu click terminates the
// systray loop, which cancels the listener and renderer goroutines.
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
		// Initial fetch — populate state before any event arrives so the menu
		// is non-empty on first render.
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

// listenLoop drains the daemon SSE stream forever, applying events to state
// and signalling the renderer when anything changed.
func listenLoop(ctx context.Context, c *client.Client, st *State, dirty func()) {
	ch := c.Subscribe(ctx)
	for ev := range ch {
		if st.Apply(ev) {
			dirty()
		}
	}
}

// renderLoop waits for dirty signals, coalesces them across reconcileDelay,
// and calls Renderer.Reconcile with the latest snapshot. Runs on a goroutine
// other than the systray main goroutine; systray.MenuItem mutators are
// goroutine-safe.
func renderLoop(ctx context.Context, st *State, r *Renderer, dirty <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-dirty:
		}
		// Coalesce: wait for the burst to settle.
		timer := time.NewTimer(reconcileDelay)
	drain:
		for {
			select {
			case <-timer.C:
				break drain
			case <-dirty:
				// keep draining
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
		r.Reconcile(st.Snapshot())
	}
}
