package bar

import (
	"encoding/json"
	"sync"

	"github.com/venkatkrishna07/mkdev/internal/api"
	"github.com/venkatkrishna07/mkdev/internal/client"
)

type State struct {
	mu       sync.Mutex
	daemonUp bool
	tld      string
	version  string
	pid      int
	uptime   string
	routes   map[string]api.Route
	stats    api.Stats
	health   map[string]api.Health
}

func NewState() *State {
	return &State{routes: map[string]api.Route{}, health: map[string]api.Health{}}
}

type Snapshot struct {
	DaemonUp bool
	TLD      string
	Version  string
	PID      int
	Uptime   string
	Routes   []api.Route
	Stats    api.Stats
	Health   map[string]api.Health
}

func (s *State) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Snapshot{
		DaemonUp: s.daemonUp,
		TLD:      s.tld,
		Version:  s.version,
		PID:      s.pid,
		Uptime:   s.uptime,
		Routes:   make([]api.Route, 0, len(s.routes)),
		Stats:    s.stats,
		Health:   make(map[string]api.Health, len(s.health)),
	}
	for _, r := range s.routes {
		out.Routes = append(out.Routes, r)
	}
	for k, v := range s.health {
		out.Health[k] = v
	}
	sortRoutes(out.Routes)
	return out
}

func (s *State) SetMeta(version string, pid int, uptime string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version = version
	s.pid = pid
	s.uptime = uptime
}

func (s *State) SetDaemonUp(up bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.daemonUp == up {
		return false
	}
	s.daemonUp = up
	return true
}

func (s *State) SetTLD(tld string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tld = tld
}

func (s *State) ReplaceRoutes(rs []api.Route) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes = make(map[string]api.Route, len(rs))
	for _, r := range rs {
		s.routes[r.Name] = r
	}
}

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
	case api.EventStatusTick:
		var st api.Status
		if err := json.Unmarshal(ev.Data, &st); err != nil {
			return false
		}
		s.version = st.Version
		s.pid = st.PID
		s.uptime = st.Uptime
		if s.tld == "" {
			s.tld = st.TLD
		}
		return true
	case api.EventStatsTick:
		var st api.Stats
		if err := json.Unmarshal(ev.Data, &st); err != nil {
			return false
		}
		s.stats = st

		changed := false
		for domain, rs := range st.Routes {
			prev, ok := s.health[domain]
			if !ok || prev != rs.Health {
				s.health[domain] = rs.Health
				changed = true
			}
		}

		for domain := range s.health {
			if _, ok := st.Routes[domain]; !ok {
				delete(s.health, domain)
				changed = true
			}
		}
		return changed
	}
	return false
}
