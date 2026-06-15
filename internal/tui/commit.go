package tui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/client"
	"github.com/venkatkrishna07/mkdev/internal/hosts"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui/modals"
)

func (m rootModel) commitAdd(p modals.AddPayload) tea.Cmd {
	return func() tea.Msg {
		if !hosts.ValidHostname(p.Domain) {
			return errMsg(fmt.Errorf("invalid domain %q", p.Domain))
		}
		ctx, cancel := context.WithTimeout(m.rt.Ctx, 5*time.Second)
		defer cancel()
		editor := hosts.NewGUIEditor(m.binPath)
		if err := editor.Add(p.Domain); err != nil {
			return errMsg(fmt.Errorf("hosts: %w", err))
		}
		share := api.ShareNone
		_, err := m.rt.Client.AddRoute(ctx, api.Route{
			Name:     trimTLD(p.Domain, p.TLD),
			Target:   p.Target,
			Share:    share,
			Insecure: p.Insecure,
		})
		if err != nil {
			if remErr := editor.Remove(p.Domain); remErr != nil {
				slog.Error("inconsistent state", "domain", p.Domain, "primary", err, "rollback", remErr)
				return errMsg(errors.Join(err, fmt.Errorf("rollback: %w", remErr)))
			}
			return errMsg(daemonHint(err))
		}
		return refreshAfterMutate(m.rt)
	}
}

func (m rootModel) commitEdit(p modals.EditPayload) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.rt.Ctx, 5*time.Second)
		defer cancel()
		name := trimTLD(p.Domain, m.rt.Cfg.TLD)
		target := p.Target
		_, err := m.rt.Client.EditRoute(ctx, name, client.RouteEdit{Target: &target})
		if err != nil {
			return errMsg(daemonHint(err))
		}
		return refreshAfterMutate(m.rt)
	}
}

func (m rootModel) commitDelete(r store.Route) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.rt.Ctx, 5*time.Second)
		defer cancel()
		editor := hosts.NewGUIEditor(m.binPath)
		if err := editor.Remove(r.Domain); err != nil {
			return errMsg(fmt.Errorf("hosts: %w", err))
		}
		name := trimTLD(r.Domain, r.TLD)
		if err := m.rt.Client.RemoveRoute(ctx, name); err != nil {
			if addErr := editor.Add(r.Domain); addErr != nil {
				slog.Error("inconsistent state", "domain", r.Domain, "primary", err, "rollback", addErr)
				return errMsg(errors.Join(err, fmt.Errorf("rollback: %w", addErr)))
			}
			return errMsg(daemonHint(err))
		}
		return refreshAfterMutate(m.rt)
	}
}

func (m rootModel) toggleShare(r store.Route) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.rt.Ctx, 5*time.Second)
		defer cancel()
		name := trimTLD(r.Domain, r.TLD)
		_, err := m.rt.Client.ToggleShare(ctx, name, !r.Shared)
		if err != nil {
			return errMsg(daemonHint(err))
		}
		return refreshAfterMutate(m.rt)
	}
}

func (m rootModel) toggleEnabled(r store.Route) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.rt.Ctx, 5*time.Second)
		defer cancel()
		name := trimTLD(r.Domain, r.TLD)
		next := !r.Enabled
		_, err := m.rt.Client.EditRoute(ctx, name, client.RouteEdit{Enabled: &next})
		if err != nil {
			return errMsg(daemonHint(err))
		}
		return refreshAfterMutate(m.rt)
	}
}

func refreshAfterMutate(rt *Runtime) tea.Msg {
	rs, err := rt.LoadRoutes()
	if err != nil {
		return errMsg(daemonHint(err))
	}
	return RoutesRefreshed{Routes: rs}
}

func trimTLD(domain, tld string) string {
	if tld == "" {
		return domain
	}
	return strings.TrimSuffix(domain, tld)
}

func daemonHint(err error) error {
	if errors.Is(err, client.ErrDaemonDown) {
		return fmt.Errorf("%w — run `mkdev daemon serve`", err)
	}
	return err
}
