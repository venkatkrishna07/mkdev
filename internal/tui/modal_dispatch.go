package tui

import (
	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mkdev/internal/tui/modals"
)

// forwardToActiveTab routes msg to the currently focused tab's Update. The
// Domains tab is handled inline by handleGlobalKey, so it is not included here.
func (m rootModel) forwardToActiveTab(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.active {
	case tabLogs:
		m.logs, cmd = m.logs.Update(msg)
	case tabDoctor:
		m.doctor, cmd = m.doctor.Update(msg)
	case tabSettings:
		m.settings, cmd = m.settings.Update(msg)
	}
	return m, cmd
}

func (m rootModel) updateTopModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	idx := len(m.modals) - 1
	var cmd tea.Cmd
	switch t := m.modals[idx].(type) {
	case modals.Add:
		t, cmd = t.Update(msg)
		m.modals[idx] = t
	case modals.Edit:
		t, cmd = t.Update(msg)
		m.modals[idx] = t
	case modals.Confirm:
		t, cmd = t.Update(msg)
		m.modals[idx] = t
	case modals.Help:
		t, cmd = t.Update(msg)
		m.modals[idx] = t
	}
	return m, cmd
}

func (m rootModel) handleModalResult(closedModal any, r modals.Result) tea.Cmd {
	_ = closedModal
	if r.Cancelled {
		return nil
	}
	switch p := r.Payload.(type) {
	case modals.AddPayload:
		return m.commitAdd(p)
	case modals.EditPayload:
		return m.commitEdit(p)
	case bool:
		if !p {
			return nil
		}
		if sel, ok := m.domains.Selected(); ok {
			return m.commitDelete(sel)
		}
	}
	return nil
}

// activeKeyMap returns the help.KeyMap to advertise in the footer: the top
// modal's when the stack is non-empty, otherwise the root key map.
func (m rootModel) activeKeyMap() help.KeyMap {
	if len(m.modals) == 0 {
		return m.keys
	}
	switch t := m.modals[len(m.modals)-1].(type) {
	case modals.Add:
		return t.Keys()
	case modals.Edit:
		return t.Keys()
	case modals.Confirm:
		return t.Keys()
	case modals.Help:
		return t.Keys()
	}
	return m.keys
}
