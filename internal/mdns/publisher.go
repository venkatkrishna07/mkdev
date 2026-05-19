package mdns

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/hashicorp/mdns"
	"github.com/venkatkrishna07/mkdev/internal/store"
)

// quietLogger silences hashicorp/mdns's internal `log` output. Its default
// emits noisy "Failed to handle query: support for DNS requests with high
// truncate" lines whenever a peer on the LAN sends a non-standard query —
// not actionable for the user. The slog default we set globally would
// otherwise wrap those at INFO level and clog the TUI log.
var quietLogger = log.New(io.Discard, "", 0)

// Publisher manages a set of mDNS service registrations — one per enabled
// route whose TLD is ".local". Other TLDs are silently skipped.
type Publisher struct {
	mu      sync.Mutex
	ip      net.IP
	servers map[string]*mdns.Server // keyed by route domain
}

// New constructs a Publisher bound to the given LAN IPv4.
func New(ip net.IP) *Publisher {
	return &Publisher{ip: ip, servers: map[string]*mdns.Server{}}
}

// Set diffs the desired route set against the currently published set and
// adjusts: registers new .local enabled routes, deregisters removed ones.
//
// Transactional: all new registrations are attempted into a staging map
// before any mutation to p.servers. If any registration fails, the staging
// map is torn down and p.servers is left untouched, so the publisher never
// ends up in a half-applied state where future ticks skip stuck domains.
func (p *Publisher) Set(routes []store.Route) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	desired := map[string]store.Route{}
	for _, r := range routes {
		if !r.Enabled || !r.Shared || !strings.HasSuffix(r.Domain, ".local") {
			continue
		}
		desired[r.Domain] = r
	}

	// Phase 1: build everything new before touching p.servers.
	newSrvs := map[string]*mdns.Server{}
	var errs []error
	for dom := range desired {
		if _, exists := p.servers[dom]; exists {
			continue
		}
		srv, err := registerOne(dom, p.ip)
		if err != nil {
			errs = append(errs, fmt.Errorf("mdns register %s: %w", dom, err))
			continue
		}
		newSrvs[dom] = srv
	}
	if len(errs) > 0 {
		for _, srv := range newSrvs {
			_ = srv.Shutdown()
		}
		return errors.Join(errs...)
	}

	// Phase 2: commit. Shut down removed, install new.
	for dom, srv := range p.servers {
		if _, keep := desired[dom]; !keep {
			_ = srv.Shutdown()
			delete(p.servers, dom)
		}
	}
	for dom, srv := range newSrvs {
		p.servers[dom] = srv
	}
	return nil
}

// Close deregisters everything.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var firstErr error
	for dom, srv := range p.servers {
		if err := srv.Shutdown(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(p.servers, dom)
	}
	return firstErr
}

func registerOne(domain string, ip net.IP) (*mdns.Server, error) {
	if ip == nil {
		return nil, errors.New("mdns: nil LAN ip")
	}
	host := strings.TrimSuffix(domain, ".local") + ".local."
	service, err := mdns.NewMDNSService(
		strings.TrimSuffix(domain, ".local"),
		"_https._tcp",
		"",
		host,
		443,
		[]net.IP{ip},
		[]string{"managed=mkdev"},
	)
	if err != nil {
		return nil, err
	}
	srv, err := mdns.NewServer(&mdns.Config{Zone: service, Logger: quietLogger})
	if err != nil {
		return nil, err
	}
	return srv, nil
}
