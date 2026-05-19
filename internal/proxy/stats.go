package proxy

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	statsWindow  = 20
	rpsWindowSec = 60
)

type Stats struct {
	mu  sync.RWMutex
	buf map[string]*ring

	totalReqs atomic.Uint64

	rpsMu  sync.Mutex
	rpsBuf [rpsWindowSec]uint32
	rpsTs  int64 // last bucket epoch second written
}

type ring struct {
	xs   [statsWindow]time.Duration
	n    int
	full bool
	last time.Time
}

func NewStats() *Stats {
	return &Stats{buf: make(map[string]*ring)}
}

func (s *Stats) Record(domain string, d time.Duration) {
	domain = strings.ToLower(domain)
	s.totalReqs.Add(1)

	s.mu.Lock()
	r, ok := s.buf[domain]
	if !ok {
		r = &ring{}
		s.buf[domain] = r
	}
	r.xs[r.n] = d
	r.n = (r.n + 1) % statsWindow
	if r.n == 0 {
		r.full = true
	}
	r.last = time.Now()
	s.mu.Unlock()

	s.bumpRPS()
}

func (s *Stats) bumpRPS() {
	now := time.Now().Unix()
	s.rpsMu.Lock()
	defer s.rpsMu.Unlock()
	if s.rpsTs == 0 {
		s.rpsTs = now
	}
	// Advance + zero any intermediate buckets we skipped.
	delta := now - s.rpsTs
	if delta < 0 {
		delta = 0
	}
	if delta >= rpsWindowSec {
		s.rpsBuf = [rpsWindowSec]uint32{}
		s.rpsTs = now
	} else {
		for i := int64(0); i < delta; i++ {
			idx := (s.rpsTs + 1 + i) % rpsWindowSec
			s.rpsBuf[idx] = 0
		}
		s.rpsTs = now
	}
	s.rpsBuf[now%rpsWindowSec]++
}

func (s *Stats) Snapshot(domain string) []time.Duration {
	domain = strings.ToLower(domain)
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.buf[domain]
	if !ok {
		return nil
	}
	count := r.n
	if r.full {
		count = statsWindow
	}
	out := make([]time.Duration, 0, count)
	if r.full {
		for i := r.n; i < statsWindow; i++ {
			out = append(out, r.xs[i])
		}
	}
	for i := 0; i < r.n; i++ {
		out = append(out, r.xs[i])
	}
	return out
}

func (s *Stats) Last(domain string) time.Time {
	domain = strings.ToLower(domain)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if r, ok := s.buf[domain]; ok {
		return r.last
	}
	return time.Time{}
}

// Total returns the cumulative request count since process start.
func (s *Stats) Total() uint64 { return s.totalReqs.Load() }

// RPS returns the per-second request counts for the last rpsWindowSec
// seconds, oldest first. Buckets that pre-date the first request are zero.
func (s *Stats) RPS() []float64 {
	now := time.Now().Unix()
	out := make([]float64, rpsWindowSec)
	s.rpsMu.Lock()
	defer s.rpsMu.Unlock()
	for i := range rpsWindowSec {
		bucketTs := now - int64(rpsWindowSec-1-i)
		if bucketTs < s.rpsTs-int64(rpsWindowSec-1) {
			out[i] = 0
			continue
		}
		out[i] = float64(s.rpsBuf[bucketTs%rpsWindowSec])
	}
	return out
}

// Domains returns the set of domains that have recorded at least one request.
func (s *Stats) Domains() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.buf))
	for d := range s.buf {
		out = append(out, d)
	}
	return out
}
