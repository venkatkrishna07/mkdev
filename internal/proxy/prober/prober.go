// Package prober TCP-probes route upstreams on a fixed interval and exposes
// per-host health snapshots for the dashboard and runtime status reporting.
package prober

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/venkatkrishna07/mkdev/internal/store"
)

// Status is the health verdict for a single route's upstream.
type Status int

const (
	// StatusOff is the zero value and also the recorded status for disabled routes.
	StatusOff Status = iota
	StatusUp
	StatusDown
)

func (s Status) String() string {
	switch s {
	case StatusUp:
		return "up"
	case StatusDown:
		return "down"
	default:
		return "off"
	}
}

// HealthState is the most recent probe outcome for one host.
type HealthState struct {
	Status    Status
	LastErr   string
	LastProbe time.Time
}

const (
	probePoolSize = 8
	errMaxLen     = 80
)

// dialer is overridable so future code can swap the network call without
// changing the Prober's structure. Default does a bounded TCP DialContext.
var dialer = func(ctx context.Context, target string, timeout time.Duration) error {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", target)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// Prober periodically TCP-dials every enabled route's upstream.
type Prober struct {
	interval time.Duration
	timeout  time.Duration
	routes   func() ([]store.Route, error)
	states   sync.Map // host (lowercased) -> HealthState
}

// New returns a Prober that pulls routes from the given function and probes
// each enabled upstream every interval with per-dial timeout.
func New(routes func() ([]store.Route, error), interval, timeout time.Duration) *Prober {
	return &Prober{
		interval: interval,
		timeout:  timeout,
		routes:   routes,
	}
}

// Health returns the last known state for host, or the zero value (StatusOff)
// if the host has never been probed.
func (p *Prober) Health(host string) HealthState {
	v, ok := p.states.Load(strings.ToLower(host))
	if !ok {
		return HealthState{}
	}
	return v.(HealthState)
}

// Snapshot returns a copy of every host's current health state.
func (p *Prober) Snapshot() map[string]HealthState {
	out := map[string]HealthState{}
	p.states.Range(func(k, v any) bool {
		out[k.(string)] = v.(HealthState)
		return true
	})
	return out
}

// Run probes immediately, then on every interval tick, until ctx is cancelled.
func (p *Prober) Run(ctx context.Context) {
	p.tick(ctx)
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.tick(ctx)
		}
	}
}

func (p *Prober) tick(ctx context.Context) {
	routes, err := p.routes()
	if err != nil {
		return
	}

	live := make(map[string]struct{}, len(routes))
	jobs := make(chan store.Route)
	var wg sync.WaitGroup

	for range probePoolSize {
		wg.Go(func() {
			for r := range jobs {
				p.probe(ctx, r)
			}
		})
	}

	for _, r := range routes {
		host := strings.ToLower(r.Domain)
		live[host] = struct{}{}
		if !r.Enabled {
			p.states.Store(host, HealthState{Status: StatusOff, LastProbe: time.Now()})
			continue
		}
		select {
		case jobs <- r:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		}
	}
	close(jobs)
	wg.Wait()

	p.states.Range(func(k, _ any) bool {
		if _, ok := live[k.(string)]; !ok {
			p.states.Delete(k)
		}
		return true
	})
}

func (p *Prober) probe(ctx context.Context, r store.Route) {
	host := strings.ToLower(r.Domain)
	st := HealthState{Status: StatusUp, LastProbe: time.Now()}
	target := strings.TrimSpace(r.Target)
	switch target {
	case "":
		st.Status, st.LastErr = StatusDown, "bad upstream"
	default:
		addr, err := dialAddress(target)
		if err == nil {
			dctx, cancel := context.WithTimeout(ctx, p.timeout)
			err = dialer(dctx, addr, p.timeout)
			cancel()
		}
		if err != nil {
			st.Status, st.LastErr = StatusDown, truncErr(err.Error())
		}
	}
	p.states.Store(host, st)
}

// dialAddress normalises a route Target (bare host[:port] or full URL) into a
// host:port string suitable for net.Dialer.
func dialAddress(target string) (string, error) {
	s := target
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	if host == "" {
		return "", errors.New("no host in target")
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return net.JoinHostPort(host, port), nil
}

func truncErr(s string) string {
	if len(s) <= errMaxLen {
		return s
	}
	return s[:errMaxLen-3] + "..."
}
