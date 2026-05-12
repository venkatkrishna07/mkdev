package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Server is a HTTPS reverse proxy backed by a Router.
type Server struct {
	router *Router
	ln     net.Listener
	srv    *http.Server
}

// NewServer wires a server using the given listener (which must already be TLS).
func NewServer(r *Router, ln net.Listener) *Server {
	s := &Server{router: r, ln: ln}
	s.srv = &http.Server{
		Handler:           http.HandlerFunc(s.handle),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	return s
}

// Serve blocks until the listener closes.
func (s *Server) Serve() error { return s.srv.Serve(s.ln) }

// Close stops the server immediately.
func (s *Server) Close() error { return s.srv.Close() }

// Shutdown gracefully drains in-flight requests within ctx's deadline.
func (s *Server) Shutdown(ctx context.Context) error { return s.srv.Shutdown(ctx) }

// Addr returns the listener address.
func (s *Server) Addr() net.Addr { return s.ln.Addr() }

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	rp, ok := s.router.LookupProxy(host)
	if !ok {
		slog.Info("proxy: no route", "host", host)
		http.Error(w, fmt.Sprintf("mkdev: no route for %s", host), http.StatusNotFound)
		return
	}
	target, _ := s.router.Lookup(host)
	r.Host = target
	rp.ServeHTTP(w, r)
}
