//go:build darwin

package bar

import (
	"encoding/json"
	"sync"

	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/client"
)

// State is the in-memory snapshot the renderer consumes. All access is
// mutex-guarded; mutations come from the listener goroutine.
type State struct {
	mu       sync.Mutex
	daemonUp bool
	tld      string
	routes   map[string]api.Route // keyed by route name
}

// NewState returns an empty State.
func NewState() *State {
	return &State{routes: map[string]api.Route{}}
}

// Snapshot returns a copy of the current state, safe for the renderer.
type Snapshot struct {
	DaemonUp bool
	TLD      string
	Routes   []api.Route
}

// Snapshot copies the current state. Routes are sorted by name for stable
// menu order across renders.
func (s *State) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Snapshot{DaemonUp: s.daemonUp, TLD: s.tld, Routes: make([]api.Route, 0, len(s.routes))}
	for _, r := range s.routes {
		out.Routes = append(out.Routes, r)
	}
	sortRoutes(out.Routes)
	return out
}

// SetDaemonUp toggles the daemon-up flag, returning true if it changed.
func (s *State) SetDaemonUp(up bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.daemonUp == up {
		return false
	}
	s.daemonUp = up
	return true
}

// SetTLD records the daemon-reported TLD so we can render full domains.
func (s *State) SetTLD(tld string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tld = tld
}

// ReplaceRoutes overwrites the cached route set (used after initial fetch
// and on client.reconnected).
func (s *State) ReplaceRoutes(rs []api.Route) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes = make(map[string]api.Route, len(rs))
	for _, r := range rs {
		s.routes[r.Name] = r
	}
}

// Apply mutates state from a daemon SSE event. Returns true if the snapshot
// changed meaningfully (so the renderer should reconcile).
func (s *State) Apply(ev api.Event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch ev.Type {
	case api.EventRouteAdded, api.EventRouteChanged:
		var r api.Route
		if err := json.Unmarshal(ev.Data, &r); err != nil || r.Name == "" {
			return false
		}
		s.routes[r.Name] = r
		return true
	case api.EventRouteRemoved:
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(ev.Data, &p); err != nil || p.Name == "" {
			return false
		}
		if _, ok := s.routes[p.Name]; !ok {
			return false
		}
		delete(s.routes, p.Name)
		return true
	case client.EventClientDisconnected:
		if !s.daemonUp {
			return false
		}
		s.daemonUp = false
		return true
	case client.EventClientReconnected:
		if s.daemonUp {
			return false
		}
		s.daemonUp = true
		return true
	}
	return false
}
