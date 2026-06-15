package upgrade

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/venkatkrishna07/mkdev/internal/bar"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	"github.com/venkatkrishna07/mkdev/internal/cert/trust"
	"github.com/venkatkrishna07/mkdev/internal/daemon"
	"github.com/venkatkrishna07/mkdev/internal/hosts"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/version"
)

type Mode int

const (
	ModeCLI Mode = iota
	ModeDaemon
)

type Result struct {
	From    string
	To      string
	Applied []string
	Pending []string
}

func (r Result) Skipped() bool { return r.From == r.To }

func Check(home string) (needed bool, from, to string) {
	to = version.String()
	from = ReadMarker(home)
	return from != to, from, to
}

func Run(_ context.Context, mode Mode, home, exe string, w io.Writer) (Result, error) {
	to := version.String()
	from := ReadMarker(home)
	if from == to {
		return Result{From: from, To: to}, nil
	}
	res := Result{From: from, To: to}

	if exe == "" {
		if self, err := os.Executable(); err == nil {
			exe = self
		}
	}

	if exe != "" {
		if _, err := daemon.InstallUnit(exe); err == nil {
			res.Applied = append(res.Applied, "daemon service plist re-pointed")
		} else if !errors.Is(err, daemon.ErrUnitUnsupported) {
			slog.Warn("upgrade: daemon unit reinstall", "err", err)
		}
	}
	if err := bar.InstallAutostart(); err == nil {
		res.Applied = append(res.Applied, "menu bar autostart re-pointed")
	} else {
		slog.Warn("upgrade: bar autostart", "err", err)
	}

	missing := missingHosts(home, exe)
	if len(missing) > 0 {
		if mode == ModeCLI && exe != "" {
			editor := hosts.NewEditor(exe)
			added := 0
			for _, d := range missing {
				if err := editor.Add(d); err != nil {
					slog.Warn("upgrade: hosts add", "domain", d, "err", err)
					continue
				}
				added++
			}
			if added > 0 {
				res.Applied = append(res.Applied, fmt.Sprintf("re-asserted %d /etc/hosts entries", added))
			}
			if added < len(missing) {
				res.Pending = append(res.Pending, "hosts-sync")
			}
		} else {
			res.Pending = append(res.Pending, "hosts-sync")
		}
	}

	if needs, err := caTrustStale(home); err == nil && needs {
		if mode == ModeCLI {
			caPath := filepath.Join(home, "ca", "rootCA.pem")
			if err := trust.Install(caPath); err == nil {
				res.Applied = append(res.Applied, "CA re-trusted")
			} else {
				slog.Warn("upgrade: trust refresh", "err", err)
				res.Pending = append(res.Pending, "ca-trust")
			}
		} else {
			res.Pending = append(res.Pending, "ca-trust")
		}
	}

	if err := bar.SpawnIfNeeded(); err == nil {
		res.Applied = append(res.Applied, "menu bar spawned")
	}

	if mode == ModeCLI && len(res.Pending) == 0 {
		if err := WriteMarker(home, to); err != nil {
			slog.Warn("upgrade: write marker", "err", err)
		}
		_ = ClearPending(home)
	} else if err := WritePending(home, res); err != nil {
		slog.Warn("upgrade: write pending", "err", err)
	}

	report(w, res)
	return res, nil
}

func report(w io.Writer, res Result) {
	if w == nil || len(res.Applied)+len(res.Pending) == 0 {
		return
	}
	label := res.From
	if label == "" {
		label = "(new)"
	}
	_, _ = fmt.Fprintf(w, "mkdev: upgrade %s → %s\n", label, res.To)
	for _, a := range res.Applied {
		_, _ = fmt.Fprintf(w, "  ✓ %s\n", a)
	}
	if len(res.Pending) > 0 {
		_, _ = fmt.Fprintln(w, "  → run `mkdev install` to finish (needs sudo)")
	}
}

func missingHosts(home, exe string) []string {
	if exe == "" {
		return nil
	}
	dbPath := filepath.Join(home, "state.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}
	s, err := store.Open(dbPath)
	if err != nil {
		return nil
	}
	defer func() { _ = s.Close() }()
	routes, err := s.ListRoutes()
	if err != nil || len(routes) == 0 {
		return nil
	}
	current, err := hosts.NewEditor(exe).Read()
	if err != nil {
		return nil
	}
	have := map[string]bool{}
	for _, e := range hosts.Parse(strings.NewReader(current)) {
		have[e.Host] = true
	}
	var out []string
	for _, rt := range routes {
		if !rt.Enabled {
			continue
		}
		if !have[rt.Domain] {
			out = append(out, rt.Domain)
		}
	}
	return out
}

func caTrustStale(home string) (bool, error) {
	caDir := filepath.Join(home, "ca")
	ca, err := cert.LoadCA(caDir)
	if err != nil {
		return false, err
	}
	ok, err := trust.IsTrusted(ca.Cert)
	if err != nil {
		return false, err
	}
	return !ok, nil
}
