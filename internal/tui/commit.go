package tui

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mkdev/internal/hosts"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui/modals"
)

func (m rootModel) commitAdd(p modals.AddPayload) tea.Cmd {
	return func() tea.Msg {
		if !hosts.ValidHostname(p.Domain) {
			return errMsg(fmt.Errorf("invalid domain %q", p.Domain))
		}
		s := m.rt.Store
		if _, err := s.GetRoute(p.Domain); err == nil {
			return errMsg(fmt.Errorf("route exists: %s", p.Domain))
		} else if !errors.Is(err, store.ErrNotFound) {
			return errMsg(err)
		}
		editor := hosts.NewGUIEditor(m.binPath)
		if err := editor.Add(p.Domain); err != nil {
			return errMsg(fmt.Errorf("hosts: %w", err))
		}
		r := store.Route{
			Domain:   p.Domain,
			Target:   p.Target,
			TLD:      p.TLD,
			Enabled:  true,
			Insecure: p.Insecure,
			Source:   store.SourceAdHoc,
			AddedAt:  time.Now().UTC(),
		}
		if err := s.PutRoute(r); err != nil {
			if remErr := editor.Remove(p.Domain); remErr != nil {
				slog.Error("inconsistent state", "domain", p.Domain, "primary", err, "rollback", remErr)
				return errMsg(errors.Join(err, fmt.Errorf("rollback: %w", remErr)))
			}
			return errMsg(err)
		}
		rs, err := s.ListRoutes()
		if err != nil {
			return errMsg(err)
		}
		m.rt.Router.Set(rs)
		return RoutesRefreshed{Routes: rs}
	}
}

func (m rootModel) commitEdit(p modals.EditPayload) tea.Cmd {
	return func() tea.Msg {
		s := m.rt.Store
		cur, err := s.GetRoute(p.Domain)
		if err != nil {
			return errMsg(err)
		}
		cur.Target = p.Target
		if err := s.PutRoute(cur); err != nil {
			return errMsg(err)
		}
		rs, err := s.ListRoutes()
		if err != nil {
			return errMsg(err)
		}
		m.rt.Router.Set(rs)
		return RoutesRefreshed{Routes: rs}
	}
}

func (m rootModel) commitDelete(r store.Route) tea.Cmd {
	return func() tea.Msg {
		s := m.rt.Store
		editor := hosts.NewGUIEditor(m.binPath)
		if err := editor.Remove(r.Domain); err != nil {
			return errMsg(fmt.Errorf("hosts: %w", err))
		}
		if err := s.DeleteRoute(r.Domain); err != nil {
			if addErr := editor.Add(r.Domain); addErr != nil {
				slog.Error("inconsistent state", "domain", r.Domain, "primary", err, "rollback", addErr)
				return errMsg(errors.Join(err, fmt.Errorf("rollback: %w", addErr)))
			}
			return errMsg(err)
		}
		rs, err := s.ListRoutes()
		if err != nil {
			return errMsg(err)
		}
		m.rt.Router.Set(rs)
		return RoutesRefreshed{Routes: rs}
	}
}

func (m rootModel) toggleRoute(r store.Route) tea.Cmd {
	return func() tea.Msg {
		s := m.rt.Store
		r.Enabled = !r.Enabled
		if err := s.PutRoute(r); err != nil {
			return errMsg(err)
		}
		rs, err := s.ListRoutes()
		if err != nil {
			return errMsg(err)
		}
		m.rt.Router.Set(rs)
		return RoutesRefreshed{Routes: rs}
	}
}

func (m rootModel) toggleShare(r store.Route) tea.Cmd {
	return func() tea.Msg {
		s := m.rt.Store
		r.Shared = !r.Shared
		if err := s.PutRoute(r); err != nil {
			return errMsg(err)
		}
		rs, err := s.ListRoutes()
		if err != nil {
			return errMsg(err)
		}
		m.rt.Router.Set(rs)
		return RoutesRefreshed{Routes: rs}
	}
}
