package proxy

import (
	"crypto/tls"
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
	shared bool
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
		upstream, err := normalizeUpstream(rt.Target)
		if err != nil {
			slog.Warn("proxy: skipping invalid target", "domain", domain, "target", rt.Target, "err", err)
			continue
		}
		raw := rt.Target
		rp := httputil.NewSingleHostReverseProxy(upstream)
		if rt.Insecure {
			tr := http.DefaultTransport.(*http.Transport).Clone()
			tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // explicit per-route opt-in for private-CA upstreams
			rp.Transport = tr
		}
		rp.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
			slog.Warn("proxy: upstream error", "host", domain, "target", raw, "err", err)
			http.Error(rw, fmt.Sprintf("mkdev: upstream %s unreachable: %v", raw, err), http.StatusBadGateway)
		}
		upstreamCaptured := upstream
		proxyDomain := domain
		rp.ModifyResponse = func(resp *http.Response) error {
			rewriteLocationHeader(resp, upstreamCaptured, proxyDomain)
			rewriteCookieDomain(resp, upstreamCaptured.Hostname(), proxyDomain)
			return nil
		}
		next[domain] = entry{target: upstream.Host, shared: rt.Shared, proxy: rp}
	}
	r.table.Store(&next)
}

// normalizeUpstream parses target into a URL suitable for
// httputil.NewSingleHostReverseProxy. A bare host[:port] gets a default http
// scheme; explicit http:// or https:// is preserved along with any base path.
// Returns an error if the result has no host.
func normalizeUpstream(target string) (*url.URL, error) {
	s := strings.TrimSpace(target)
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("invalid url %q: %w", target, err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("no host in target %q", target)
	}
	return u, nil
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

// rewriteLocationHeader keeps upstream redirects on the proxy domain so
// clients don't follow them direct and lose credentials.
func rewriteLocationHeader(resp *http.Response, upstream *url.URL, proxyDomain string) {
	loc := resp.Header.Get("Location")
	if loc == "" {
		return
	}
	u, err := url.Parse(loc)
	if err != nil || u.Host == "" {
		return
	}
	if !strings.EqualFold(u.Host, upstream.Host) && !strings.EqualFold(u.Hostname(), upstream.Hostname()) {
		return
	}
	u.Scheme = "https"
	u.Host = proxyDomain
	resp.Header.Set("Location", u.String())
}

func rewriteCookieDomain(resp *http.Response, upstreamHost, proxyDomain string) {
	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		return
	}
	rewritten := make([]string, len(cookies))
	for i, c := range cookies {
		rewritten[i] = rewriteSingleCookieDomain(c, upstreamHost, proxyDomain)
	}
	resp.Header.Del("Set-Cookie")
	for _, c := range rewritten {
		resp.Header.Add("Set-Cookie", c)
	}
}

func rewriteSingleCookieDomain(cookie, upstreamHost, proxyDomain string) string {
	parts := strings.Split(cookie, ";")
	for i, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) != 2 || !strings.EqualFold(strings.TrimSpace(kv[0]), "Domain") {
			continue
		}
		val := strings.TrimSpace(kv[1])
		val = strings.TrimPrefix(val, ".")
		if strings.EqualFold(val, upstreamHost) {
			parts[i] = " Domain=" + proxyDomain
		}
	}
	return strings.Join(parts, ";")
}

// Shared returns true if domain is registered AND marked shared. Returns
// false for unknown domains and for known-but-not-shared domains.
func (r *Router) Shared(domain string) bool {
	t := *r.table.Load()
	v, ok := t[strings.ToLower(domain)]
	if !ok {
		return false
	}
	return v.shared
}
