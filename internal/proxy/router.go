package proxy

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/venkatkrishna07/mkdev/internal/store"
)

type entry struct {
	target string
	proxy  *httputil.ReverseProxy
}

// Router is a hot-reloadable domain → target map.
// Concurrent reads are lock-free via atomic pointer swap.
type Router struct {
	table atomic.Pointer[map[string]entry]
}

// NewRouter returns an empty router.
func NewRouter() *Router {
	r := &Router{}
	empty := map[string]entry{}
	r.table.Store(&empty)
	return r
}

// Set replaces the routing table atomically. Disabled routes are dropped.
// Domains are stored lowercase; lookups must match case-insensitively.
func (r *Router) Set(routes []store.Route) {
	next := make(map[string]entry, len(routes))
	for _, rt := range routes {
		if !rt.Enabled {
			continue
		}
		domain := strings.ToLower(rt.Domain)
		target := rt.Target
		upstream := &url.URL{Scheme: "http", Host: target}
		rp := httputil.NewSingleHostReverseProxy(upstream)
		rp.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
			slog.Warn("proxy: upstream error", "host", domain, "target", target, "err", err)
			http.Error(rw, fmt.Sprintf("mkdev: upstream %s unreachable: %v", target, err), http.StatusBadGateway)
		}
		next[domain] = entry{target: target, proxy: rp}
	}
	r.table.Store(&next)
}

// Lookup returns the upstream target for domain or "" / false if unknown/disabled.
func (r *Router) Lookup(domain string) (string, bool) {
	t := *r.table.Load()
	v, ok := t[strings.ToLower(domain)]
	if !ok {
		return "", false
	}
	return v.target, true
}

// LookupProxy returns the pre-built reverse proxy for domain.
func (r *Router) LookupProxy(domain string) (*httputil.ReverseProxy, bool) {
	t := *r.table.Load()
	v, ok := t[strings.ToLower(domain)]
	if !ok {
		return nil, false
	}
	return v.proxy, true
}

// Has returns true if the router has an enabled route for domain.
func (r *Router) Has(domain string) bool {
	_, ok := r.Lookup(domain)
	return ok
}
