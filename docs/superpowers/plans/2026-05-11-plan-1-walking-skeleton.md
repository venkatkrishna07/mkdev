# Plan 1 — Walking Skeleton (macOS)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Git policy (project CLAUDE.md):** Do **not** run `git add`, `git commit`, `git push`, `git reset`, or `git checkout` without explicit confirmation from the user. Commit steps are written into the plan for reference; ask before executing them.

**Goal:** Produce a working single-process `mklocal` binary on macOS where the following sequence succeeds end-to-end:

```
mklocal install                 # generates CA, trusts in Keychain, prepares dirs
mklocal add foo localhost:3000  # writes route, /etc/hosts entry, preheat cert
mklocal serve &                 # foreground daemon binding :443
curl https://foo.local          # 200 from upstream
```

**Architecture:** Single Go binary, single process (no gRPC yet, no TUI, no service install). Cobra-driven CLI. Bbolt KV for route state. Vendored mkcert utilities for CA/leaf cert + macOS Keychain trust. `/etc/hosts` edits via a `sudo`-elevated `mklocal hosts-helper` subcommand. Reverse proxy on `:443` using `crypto/tls` + `net/http/httputil`.

**Tech Stack:** Go 1.25+, `spf13/cobra`, `BurntSushi/toml`, `go.etcd.io/bbolt`, `log/slog` (stdlib), `crypto/tls` + `crypto/x509` + `net/http/httputil` (stdlib), `testify/require` (tests).

**Out of scope for Plan 1 (deferred to later plans):**
- gRPC IPC, daemon as separate process (Plan 2)
- TUI (Plans 3 & 4)
- Linux + Windows (Plan 5)
- Goreleaser, CI, signing (Plan 6)
- Projects tab / `mklocal.toml` per-project files (Plan 4)
- Live event streams, log tailing (Plan 2)

---

## File map

| File | Responsibility |
|------|----------------|
| `go.mod` | Module + deps |
| `LICENSE` | MIT license text |
| `README.md` | Short usage + status banner |
| `.gitignore` | Standard Go ignores + `bin/`, `.mklocal/`, `*.pem` |
| `Makefile` | `build`, `test`, `lint`, `run` |
| `cmd/mklocal/main.go` | Entrypoint; calls `cli.Execute()` |
| `internal/version/version.go` | `Version`, `Commit`, `Date` ldflags vars |
| `internal/config/config.go` | TOML load/save for `~/.mklocal/config.toml` |
| `internal/config/config_test.go` | Round-trip + default tests |
| `internal/store/store.go` | bbolt wrapper: open, close, schema bootstrap |
| `internal/store/routes.go` | `Route` type + CRUD on `routes` bucket |
| `internal/store/store_test.go` | Routes CRUD + concurrent writer |
| `internal/hosts/hosts.go` | `Editor` interface + parsing + helpers |
| `internal/hosts/hosts_darwin.go` | macOS impl: writes via `mklocal hosts-helper` |
| `internal/hosts/hosts_test.go` | Parse, dedup, idempotent add/remove |
| `internal/cert/ca.go` | Root CA gen, save, load (vendored mkcert) |
| `internal/cert/issuer.go` | In-memory leaf cert cache, mint on demand |
| `internal/cert/ca_test.go` | CA gen round-trip + expiry math |
| `internal/cert/issuer_test.go` | Issue + cache + SNI fallback |
| `internal/cert/trust/darwin.go` | Keychain install/uninstall via `security` CLI |
| `internal/cert/trust/trust_darwin_test.go` | Skipped unless `MKLOCAL_TEST_KEYCHAIN=1` |
| `internal/proxy/router.go` | Domain → upstream lookup, hot-reload from store |
| `internal/proxy/router_test.go` | Lookup + reload + miss |
| `internal/proxy/server.go` | TLS listener, SNI cert callback, reverse proxy |
| `internal/proxy/server_test.go` | End-to-end on random high port (not :443) |
| `internal/cli/root.go` | Cobra root cmd, persistent flags, logger init |
| `internal/cli/add.go` | `mklocal add <name> <target>` |
| `internal/cli/remove.go` | `mklocal remove <name>` |
| `internal/cli/list.go` | `mklocal list` (`--json` flag) |
| `internal/cli/serve.go` | `mklocal serve` (foreground daemon) |
| `internal/cli/install.go` | `mklocal install` (CA gen + trust + dirs) |
| `internal/cli/uninstall.go` | `mklocal uninstall` (revert install) |
| `internal/cli/hostshelper.go` | `mklocal hosts-helper add/remove <line>` (sudo-only) |
| `internal/cli/cli_test.go` | Cobra wiring smoke tests |

26 files. Roughly one task per file (some grouped).

---

## Task 1: Scaffold project

**Files:**
- Create: `go.mod`
- Create: `LICENSE`
- Create: `README.md`
- Create: `.gitignore`
- Create: `Makefile`
- Create: `cmd/mklocal/main.go`
- Create: `internal/version/version.go`

- [ ] **Step 1.1 — Initialize module**

Run:
```
cd /Users/venkatkrishnas/Downloads/Projects/Personal/mklocal
go mod init github.com/venkatkrishna07/mklocal
```

Expected: creates `go.mod` with `module github.com/venkatkrishna07/mklocal` and `go 1.25` line. If your installed Go is older, the file will pin to that version — that is fine; update later.

- [ ] **Step 1.2 — Add LICENSE (MIT)**

Create `LICENSE` with MIT license body. Copyright line: `Copyright (c) 2026 venkatkrishna07`.

- [ ] **Step 1.3 — Add .gitignore**

Create `.gitignore`:

```
# binaries
/bin/
mklocal
mklocal.exe

# go
*.test
*.out
coverage.txt

# state
.mklocal/
*.pem
*.key

# IDE
.idea/
.vscode/
.DS_Store
```

- [ ] **Step 1.4 — Add Makefile**

Create `Makefile`:

```make
.PHONY: build test lint run clean

BIN := bin/mklocal
PKG := ./...
GOFLAGS := -trimpath

VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X github.com/venkatkrishna07/mklocal/internal/version.Version=$(VERSION) \
           -X github.com/venkatkrishna07/mklocal/internal/version.Commit=$(COMMIT) \
           -X github.com/venkatkrishna07/mklocal/internal/version.Date=$(DATE)

build:
	@mkdir -p bin
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/mklocal

test:
	go test $(GOFLAGS) -race -count=1 -timeout=60s $(PKG)

lint:
	golangci-lint run

run: build
	$(BIN)

clean:
	rm -rf bin coverage.txt
```

- [ ] **Step 1.5 — Add README.md**

Create `README.md`:

````markdown
# mklocal

Local HTTPS for your dev servers. Map `https://myapp.local` → `http://localhost:3000` with auto-trusted TLS and a beautiful TUI.

> **Status:** under construction. Plan 1 (walking skeleton on macOS) in progress. See `docs/superpowers/specs/` for design, `docs/superpowers/plans/` for plans.

## Quick start (after Plan 1 ships)

```
mklocal install
mklocal add foo localhost:3000
mklocal serve &
open https://foo.local
```
````

- [ ] **Step 1.6 — Add `internal/version/version.go`**

Create:

```go
package version

// Set via -ldflags at build time. Defaults are placeholders for go run / go test.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String formats version, commit, and date as one line.
func String() string {
	return Version + " (" + Commit + " @ " + Date + ")"
}
```

- [ ] **Step 1.7 — Add `cmd/mklocal/main.go` (skeleton)**

Create:

```go
package main

import (
	"fmt"
	"os"

	"github.com/venkatkrishna07/mklocal/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version.String())
		return
	}
	fmt.Fprintln(os.Stderr, "mklocal: CLI not yet wired")
	os.Exit(2)
}
```

- [ ] **Step 1.8 — Verify build**

Run:
```
make build
./bin/mklocal version
```

Expected: prints `dev (none @ <date>)`.

- [ ] **Step 1.9 — Commit (ask user first)**

```
git add .gitignore LICENSE Makefile README.md cmd/mklocal/main.go go.mod internal/version/version.go
git commit -m "chore: scaffold mklocal module and version package"
```

---

## Task 2: Config package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 2.1 — Add toml dep**

```
go get github.com/BurntSushi/toml@latest
```

- [ ] **Step 2.2 — Write failing tests**

Create `internal/config/config_test.go`:

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/config"
)

func TestDefault(t *testing.T) {
	c := config.Default()
	require.Equal(t, ".local", c.TLD)
	require.Equal(t, 443, c.ProxyPort)
	require.Equal(t, "auto", c.Theme)
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	in := config.Default()
	in.TLD = ".test"
	in.ProxyPort = 8443
	require.NoError(t, config.Save(path, in))

	out, err := config.Load(path)
	require.NoError(t, err)
	require.Equal(t, in, out)
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	c, err := config.Load(filepath.Join(t.TempDir(), "missing.toml"))
	require.NoError(t, err)
	require.Equal(t, config.Default(), c)
}

func TestLoadMalformedReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	require.NoError(t, os.WriteFile(path, []byte("this is not = valid = toml"), 0o600))
	_, err := config.Load(path)
	require.Error(t, err)
}
```

Add testify dep:
```
go get github.com/stretchr/testify@latest
```

- [ ] **Step 2.3 — Run tests, confirm fail**

```
go test ./internal/config/...
```

Expected: build fails — `config` package missing.

- [ ] **Step 2.4 — Implement `config.go`**

Create `internal/config/config.go`:

```go
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the global mklocal configuration loaded from ~/.mklocal/config.toml.
type Config struct {
	TLD          string `toml:"tld"`
	ProxyPort    int    `toml:"proxy_port"`
	Theme        string `toml:"theme"`
	LogRetention string `toml:"log_retention"` // e.g. "7d"
	LogMaxSize   string `toml:"log_max_size"`  // e.g. "100MB"
}

// Default returns a Config populated with built-in defaults.
func Default() Config {
	return Config{
		TLD:          ".local",
		ProxyPort:    443,
		Theme:        "auto",
		LogRetention: "7d",
		LogMaxSize:   "100MB",
	}
}

// Load reads the config file at path. If the file does not exist, Default is
// returned. Malformed TOML returns an error.
func Load(path string) (Config, error) {
	c := Default()
	_, err := toml.DecodeFile(path, &c)
	switch {
	case err == nil:
		return c, nil
	case errors.Is(err, fs.ErrNotExist):
		return Default(), nil
	default:
		return Config{}, fmt.Errorf("config: decode %s: %w", path, err)
	}
}

// Save writes the config to path with 0600 perms.
func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("config: open: %w", err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(c); err != nil {
		return fmt.Errorf("config: encode: %w", err)
	}
	return nil
}
```

- [ ] **Step 2.5 — Run tests, confirm pass**

```
go test ./internal/config/...
```

Expected: PASS.

- [ ] **Step 2.6 — Commit**

```
git add internal/config go.mod go.sum
git commit -m "feat(config): TOML load/save with defaults"
```

---

## Task 3: Store package (bbolt)

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/routes.go`
- Create: `internal/store/store_test.go`

- [ ] **Step 3.1 — Add bbolt dep**

```
go get go.etcd.io/bbolt@latest
```

- [ ] **Step 3.2 — Write failing tests**

Create `internal/store/store_test.go`:

```go
package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/store"
)

func openTest(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestRoutesPutGetDelete(t *testing.T) {
	s := openTest(t)

	r := store.Route{
		Domain:  "foo.local",
		Target:  "localhost:3000",
		TLD:     ".local",
		Enabled: true,
		Source:  "ad-hoc",
		AddedAt: time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, s.PutRoute(r))

	got, err := s.GetRoute("foo.local")
	require.NoError(t, err)
	require.Equal(t, r, got)

	all, err := s.ListRoutes()
	require.NoError(t, err)
	require.Len(t, all, 1)

	require.NoError(t, s.DeleteRoute("foo.local"))
	_, err = s.GetRoute("foo.local")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestGetRouteMissing(t *testing.T) {
	s := openTest(t)
	_, err := s.GetRoute("nope.local")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestListRoutesEmpty(t *testing.T) {
	s := openTest(t)
	all, err := s.ListRoutes()
	require.NoError(t, err)
	require.Empty(t, all)
}
```

- [ ] **Step 3.3 — Run tests, confirm fail**

```
go test ./internal/store/...
```

Expected: build fails.

- [ ] **Step 3.4 — Implement `store.go`**

Create `internal/store/store.go`:

```go
package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var bucketRoutes = []byte("routes")

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("store: not found")

// Store wraps a bbolt database.
type Store struct {
	db *bolt.DB
}

// Open opens (or creates) the database at path with 0600 perms.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("store: mkdir: %w", err)
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketRoutes)
		return err
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: init buckets: %w", err)
	}
	return &Store{db: db}, nil
}

// Close flushes and closes the database.
func (s *Store) Close() error { return s.db.Close() }
```

- [ ] **Step 3.5 — Implement `routes.go`**

Create `internal/store/routes.go`:

```go
package store

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Route describes a domain → upstream mapping.
type Route struct {
	Domain  string    `json:"domain"`
	Target  string    `json:"target"`  // host:port
	TLD     string    `json:"tld"`
	Enabled bool      `json:"enabled"`
	Source  string    `json:"source"` // "ad-hoc" or absolute project path
	AddedAt time.Time `json:"added_at"`
}

// PutRoute inserts or replaces a route keyed by Domain.
func (s *Store) PutRoute(r Route) error {
	if r.Domain == "" {
		return fmt.Errorf("store: route domain empty")
	}
	buf, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("store: marshal: %w", err)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketRoutes).Put([]byte(r.Domain), buf)
	})
}

// GetRoute returns the route for the given domain.
func (s *Store) GetRoute(domain string) (Route, error) {
	var r Route
	err := s.db.View(func(tx *bolt.Tx) error {
		buf := tx.Bucket(bucketRoutes).Get([]byte(domain))
		if buf == nil {
			return ErrNotFound
		}
		return json.Unmarshal(buf, &r)
	})
	return r, err
}

// DeleteRoute removes a route. Missing keys are not an error.
func (s *Store) DeleteRoute(domain string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketRoutes).Delete([]byte(domain))
	})
}

// ListRoutes returns all routes in lexicographic order by domain.
func (s *Store) ListRoutes() ([]Route, error) {
	out := []Route{}
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketRoutes).ForEach(func(_, v []byte) error {
			var r Route
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			out = append(out, r)
			return nil
		})
	})
	return out, err
}
```

> **Note:** v1 uses JSON encoding in bbolt. Spec mentions proto-encoded values; we switch to proto in Plan 2 once `api/proto` exists.

- [ ] **Step 3.6 — Run tests, confirm pass**

```
go test ./internal/store/...
```

Expected: PASS.

- [ ] **Step 3.7 — Commit**

```
git add internal/store go.mod go.sum
git commit -m "feat(store): bbolt-backed routes CRUD"
```

---

## Task 4: Hosts file editor

**Files:**
- Create: `internal/hosts/hosts.go`
- Create: `internal/hosts/hosts_darwin.go`
- Create: `internal/hosts/hosts_test.go`

- [ ] **Step 4.1 — Write failing tests**

Create `internal/hosts/hosts_test.go`:

```go
package hosts_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/hosts"
)

func TestParseSkipsCommentsAndBlanks(t *testing.T) {
	in := `# header
127.0.0.1   foo.local
   # indented comment
::1         bar.local
`
	got := hosts.Parse(strings.NewReader(in))
	require.Equal(t, []hosts.Entry{
		{IP: "127.0.0.1", Host: "foo.local"},
		{IP: "::1", Host: "bar.local"},
	}, got)
}

func TestAddIdempotent(t *testing.T) {
	existing := "127.0.0.1 foo.local\n"
	out, changed := hosts.AddEntry(existing, "127.0.0.1", "foo.local", "managed by mklocal")
	require.False(t, changed)
	require.Equal(t, existing, out)
}

func TestAddAppendsWhenMissing(t *testing.T) {
	out, changed := hosts.AddEntry("127.0.0.1 foo.local\n", "127.0.0.1", "bar.local", "managed by mklocal")
	require.True(t, changed)
	require.Contains(t, out, "127.0.0.1\tbar.local\t# managed by mklocal")
}

func TestRemoveDropsLine(t *testing.T) {
	in := "127.0.0.1 foo.local\n127.0.0.1\tbar.local\t# managed by mklocal\n"
	out, changed := hosts.RemoveEntry(in, "bar.local")
	require.True(t, changed)
	require.Equal(t, "127.0.0.1 foo.local\n", out)
}

func TestRemoveNoopWhenAbsent(t *testing.T) {
	in := "127.0.0.1 foo.local\n"
	out, changed := hosts.RemoveEntry(in, "missing.local")
	require.False(t, changed)
	require.Equal(t, in, out)
}

func TestValidHostnameStrict(t *testing.T) {
	require.True(t, hosts.ValidHostname("foo.local"))
	require.True(t, hosts.ValidHostname("api.foo.local"))
	require.False(t, hosts.ValidHostname(""))
	require.False(t, hosts.ValidHostname("-bad.local"))
	require.False(t, hosts.ValidHostname("bad..local"))
	require.False(t, hosts.ValidHostname("evil; rm -rf /"))
}
```

- [ ] **Step 4.2 — Run tests, confirm fail**

```
go test ./internal/hosts/...
```

Expected: build fails.

- [ ] **Step 4.3 — Implement `hosts.go`**

Create `internal/hosts/hosts.go`:

```go
package hosts

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

// Entry is a parsed host file line.
type Entry struct {
	IP   string
	Host string
}

var hostnameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)

// ValidHostname returns true if h is a syntactically valid lower-case DNS name.
// Used to guard /etc/hosts writes against injection.
func ValidHostname(h string) bool {
	if len(h) == 0 || len(h) > 253 {
		return false
	}
	return hostnameRE.MatchString(h)
}

// Parse extracts host entries from an /etc/hosts-formatted reader, skipping
// comments and blank lines. Multi-host lines yield one Entry per host.
func Parse(r io.Reader) []Entry {
	out := []Entry{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := fields[0]
		for _, h := range fields[1:] {
			out = append(out, Entry{IP: ip, Host: h})
		}
	}
	return out
}

// AddEntry appends "ip\thost\t# comment" to body if no line already maps host.
// Returns the new body and whether a change was made.
func AddEntry(body, ip, host, comment string) (string, bool) {
	for _, e := range Parse(strings.NewReader(body)) {
		if e.Host == host {
			return body, false
		}
	}
	if !strings.HasSuffix(body, "\n") && body != "" {
		body += "\n"
	}
	body += ip + "\t" + host + "\t# " + comment + "\n"
	return body, true
}

// RemoveEntry deletes any line that maps host. Returns the new body and
// whether a change was made.
func RemoveEntry(body, host string) (string, bool) {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	changed := false
	for _, line := range lines {
		stripped := line
		if i := strings.Index(line, "#"); i >= 0 {
			stripped = line[:i]
		}
		fields := strings.Fields(stripped)
		drop := false
		for _, h := range fields[min(1, len(fields)):] {
			if h == host {
				drop = true
				break
			}
		}
		if drop {
			changed = true
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n"), changed
}
```

- [ ] **Step 4.4 — Implement `hosts_darwin.go`**

Create `internal/hosts/hosts_darwin.go`:

```go
//go:build darwin

package hosts

import (
	"fmt"
	"os"
	"os/exec"
)

// HostsPath is the canonical path on macOS.
const HostsPath = "/etc/hosts"

// Editor mutates the system hosts file. Writes go through `sudo mklocal hosts-helper`.
type Editor struct {
	binPath string // absolute path to the mklocal binary
}

// NewEditor creates an editor that invokes the given mklocal binary via sudo.
func NewEditor(mklocalBin string) *Editor {
	return &Editor{binPath: mklocalBin}
}

// Read returns the current contents of /etc/hosts.
func (e *Editor) Read() (string, error) {
	b, err := os.ReadFile(HostsPath)
	if err != nil {
		return "", fmt.Errorf("hosts: read: %w", err)
	}
	return string(b), nil
}

// Add maps 127.0.0.1 → host if not already present. Requires sudo.
func (e *Editor) Add(host string) error {
	if !ValidHostname(host) {
		return fmt.Errorf("hosts: invalid hostname %q", host)
	}
	cmd := exec.Command("sudo", e.binPath, "hosts-helper", "add", host)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Remove deletes the mapping for host. Requires sudo.
func (e *Editor) Remove(host string) error {
	if !ValidHostname(host) {
		return fmt.Errorf("hosts: invalid hostname %q", host)
	}
	cmd := exec.Command("sudo", e.binPath, "hosts-helper", "remove", host)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
```

- [ ] **Step 4.5 — Run tests, confirm pass**

```
go test ./internal/hosts/...
```

Expected: PASS.

- [ ] **Step 4.6 — Commit**

```
git add internal/hosts
git commit -m "feat(hosts): cross-platform hosts editor + macOS sudo-helper backend"
```

---

## Task 5: Cert CA (vendored mkcert)

**Files:**
- Create: `internal/cert/ca.go`
- Create: `internal/cert/ca_test.go`

- [ ] **Step 5.1 — Write failing tests**

Create `internal/cert/ca_test.go`:

```go
package cert_test

import (
	"crypto/x509"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/cert"
)

func TestCreateAndLoadCA(t *testing.T) {
	dir := t.TempDir()
	ca, err := cert.CreateCA(dir, "mklocal local CA")
	require.NoError(t, err)
	require.NotNil(t, ca.Cert)
	require.NotNil(t, ca.Key)
	require.True(t, ca.Cert.IsCA)
	require.WithinDuration(t, time.Now().Add(10*365*24*time.Hour), ca.Cert.NotAfter, 24*time.Hour)

	loaded, err := cert.LoadCA(dir)
	require.NoError(t, err)
	require.Equal(t, ca.Cert.SerialNumber, loaded.Cert.SerialNumber)
}

func TestLoadCAMissing(t *testing.T) {
	_, err := cert.LoadCA(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
}

func TestCACertChain(t *testing.T) {
	ca, err := cert.CreateCA(t.TempDir(), "mklocal local CA")
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	_, err = ca.Cert.Verify(x509.VerifyOptions{Roots: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}})
	require.NoError(t, err)
}
```

- [ ] **Step 5.2 — Run tests, confirm fail**

```
go test ./internal/cert/...
```

Expected: build fails.

- [ ] **Step 5.3 — Implement `ca.go`**

Create `internal/cert/ca.go`:

```go
package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// CA holds the root certificate and its private key for the mklocal local CA.
type CA struct {
	Cert    *x509.Certificate
	CertPEM []byte
	Key     *ecdsa.PrivateKey
	KeyPEM  []byte
	dir     string
}

const (
	caCertFile = "rootCA.pem"
	caKeyFile  = "rootCA-key.pem"
)

// CreateCA generates a new ECDSA P-256 CA, writes the cert (0644) and key (0400)
// to dir, and returns it.
func CreateCA(dir, commonName string) (*CA, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("cert: mkdir: %w", err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("cert: gen key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("cert: serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName, Organization: []string{"mklocal local development"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("cert: create: %w", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("cert: parse: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("cert: marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err := os.WriteFile(filepath.Join(dir, caCertFile), certPEM, 0o644); err != nil {
		return nil, fmt.Errorf("cert: write cert: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, caKeyFile), keyPEM, 0o400); err != nil {
		return nil, fmt.Errorf("cert: write key: %w", err)
	}
	return &CA{Cert: parsed, CertPEM: certPEM, Key: key, KeyPEM: keyPEM, dir: dir}, nil
}

// LoadCA reads the CA cert + key from dir.
func LoadCA(dir string) (*CA, error) {
	certPEM, err := os.ReadFile(filepath.Join(dir, caCertFile))
	if err != nil {
		return nil, fmt.Errorf("cert: read cert: %w", err)
	}
	keyPEM, err := os.ReadFile(filepath.Join(dir, caKeyFile))
	if err != nil {
		return nil, fmt.Errorf("cert: read key: %w", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, errors.New("cert: invalid cert PEM")
	}
	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("cert: parse: %w", err)
	}
	kblock, _ := pem.Decode(keyPEM)
	if kblock == nil {
		return nil, errors.New("cert: invalid key PEM")
	}
	key, err := x509.ParseECPrivateKey(kblock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("cert: parse key: %w", err)
	}
	return &CA{Cert: parsed, CertPEM: certPEM, Key: key, KeyPEM: keyPEM, dir: dir}, nil
}

// Dir returns the directory the CA was created/loaded from.
func (c *CA) Dir() string { return c.dir }
```

- [ ] **Step 5.4 — Run tests, confirm pass**

```
go test ./internal/cert/...
```

Expected: PASS.

- [ ] **Step 5.5 — Commit**

```
git add internal/cert/ca.go internal/cert/ca_test.go
git commit -m "feat(cert): root CA generation and loading"
```

---

## Task 6: Cert leaf issuer

**Files:**
- Create: `internal/cert/issuer.go`
- Create: `internal/cert/issuer_test.go`

- [ ] **Step 6.1 — Write failing tests**

Create `internal/cert/issuer_test.go`:

```go
package cert_test

import (
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/cert"
)

func TestIssueLeafCert(t *testing.T) {
	ca, err := cert.CreateCA(t.TempDir(), "mklocal local CA")
	require.NoError(t, err)
	is := cert.NewIssuer(ca)

	leaf, err := is.Issue("foo.local")
	require.NoError(t, err)
	require.NotNil(t, leaf)

	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	parsed, err := x509.ParseCertificate(leaf.Certificate[0])
	require.NoError(t, err)
	_, err = parsed.Verify(x509.VerifyOptions{Roots: pool, DNSName: "foo.local"})
	require.NoError(t, err)
}

func TestIssueIsCached(t *testing.T) {
	ca, err := cert.CreateCA(t.TempDir(), "mklocal local CA")
	require.NoError(t, err)
	is := cert.NewIssuer(ca)
	a, err := is.Issue("foo.local")
	require.NoError(t, err)
	b, err := is.Issue("foo.local")
	require.NoError(t, err)
	require.Same(t, a, b, "issuer should return the same cached *tls.Certificate")
}

func TestGetCertificateBySNI(t *testing.T) {
	ca, err := cert.CreateCA(t.TempDir(), "mklocal local CA")
	require.NoError(t, err)
	is := cert.NewIssuer(ca)
	hello := &tls.ClientHelloInfo{ServerName: "bar.local"}
	leaf, err := is.GetCertificate(hello)
	require.NoError(t, err)
	require.NotNil(t, leaf)
}
```

- [ ] **Step 6.2 — Run tests, confirm fail**

```
go test ./internal/cert/...
```

Expected: build fails.

- [ ] **Step 6.3 — Implement `issuer.go`**

Create `internal/cert/issuer.go`:

```go
package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// Issuer mints leaf certs signed by the CA, caching by SNI hostname.
type Issuer struct {
	ca    *CA
	mu    sync.RWMutex
	cache map[string]*tls.Certificate
}

// NewIssuer creates an Issuer backed by ca.
func NewIssuer(ca *CA) *Issuer {
	return &Issuer{ca: ca, cache: map[string]*tls.Certificate{}}
}

// Issue returns (and caches) a leaf cert valid for host.
func (i *Issuer) Issue(host string) (*tls.Certificate, error) {
	i.mu.RLock()
	c, ok := i.cache[host]
	i.mu.RUnlock()
	if ok {
		return c, nil
	}

	leaf, err := i.mint(host)
	if err != nil {
		return nil, err
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	if existing, ok := i.cache[host]; ok {
		return existing, nil
	}
	i.cache[host] = leaf
	return leaf, nil
}

// GetCertificate is suitable as tls.Config.GetCertificate.
func (i *Issuer) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if hello.ServerName == "" {
		return nil, fmt.Errorf("cert: no SNI")
	}
	return i.Issue(hello.ServerName)
}

func (i *Issuer) mint(host string) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("cert: gen leaf key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("cert: serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(825 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{host},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, i.ca.Cert, &key.PublicKey, i.ca.Key)
	if err != nil {
		return nil, fmt.Errorf("cert: sign leaf: %w", err)
	}
	return &tls.Certificate{
		Certificate: [][]byte{der, i.ca.Cert.Raw},
		PrivateKey:  key,
	}, nil
}
```

- [ ] **Step 6.4 — Run tests, confirm pass**

```
go test ./internal/cert/...
```

Expected: PASS.

- [ ] **Step 6.5 — Commit**

```
git add internal/cert/issuer.go internal/cert/issuer_test.go
git commit -m "feat(cert): cached leaf cert issuer with SNI callback"
```

---

## Task 7: macOS Keychain trust install

**Files:**
- Create: `internal/cert/trust/darwin.go`
- Create: `internal/cert/trust/trust_darwin_test.go`

- [ ] **Step 7.1 — Implement `darwin.go`**

Create `internal/cert/trust/darwin.go`:

```go
//go:build darwin

package trust

import (
	"crypto/sha1" //nolint:gosec // SHA1 fingerprint for cert identification only
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const systemKeychain = "/Library/Keychains/System.keychain"

// Install adds the CA certificate at certPath to the macOS system keychain
// as a trusted root. Requires sudo (the caller is expected to have prompted).
func Install(certPath string) error {
	abs, err := filepath.Abs(certPath)
	if err != nil {
		return fmt.Errorf("trust: abs path: %w", err)
	}
	cmd := exec.Command("sudo", "security", "add-trusted-cert",
		"-d", "-r", "trustRoot",
		"-k", systemKeychain,
		abs,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("trust: add-trusted-cert: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Uninstall removes the CA certificate at certPath from the system keychain.
func Uninstall(certPath string) error {
	abs, err := filepath.Abs(certPath)
	if err != nil {
		return fmt.Errorf("trust: abs path: %w", err)
	}
	cmd := exec.Command("sudo", "security", "remove-trusted-cert", "-d", abs)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("trust: remove-trusted-cert: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// IsInstalled returns true if a cert with the same SHA1 fingerprint as the
// supplied x509 cert is present in any keychain.
func IsInstalled(c *x509.Certificate) (bool, error) {
	if c == nil {
		return false, errors.New("trust: nil cert")
	}
	sum := sha1.Sum(c.Raw) //nolint:gosec
	fp := strings.ToUpper(hex.EncodeToString(sum[:]))
	cmd := exec.Command("security", "find-certificate", "-Z", "-a", systemKeychain)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("trust: find-certificate: %w", err)
	}
	return strings.Contains(string(out), fp), nil
}
```

- [ ] **Step 7.2 — Add integration test (skipped by default)**

Create `internal/cert/trust/trust_darwin_test.go`:

```go
//go:build darwin

package trust_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/cert"
	"github.com/venkatkrishna07/mklocal/internal/cert/trust"
)

func TestInstallUninstallKeychain(t *testing.T) {
	if os.Getenv("MKLOCAL_TEST_KEYCHAIN") != "1" {
		t.Skip("set MKLOCAL_TEST_KEYCHAIN=1 to run; will prompt for sudo and mutate system keychain")
	}
	dir := t.TempDir()
	ca, err := cert.CreateCA(dir, "mklocal test CA")
	require.NoError(t, err)

	require.NoError(t, trust.Install(filepath.Join(dir, "rootCA.pem")))
	t.Cleanup(func() { _ = trust.Uninstall(filepath.Join(dir, "rootCA.pem")) })

	ok, err := trust.IsInstalled(ca.Cert)
	require.NoError(t, err)
	require.True(t, ok)
}
```

- [ ] **Step 7.3 — Verify build**

```
go build ./internal/cert/trust/...
go test ./internal/cert/trust/...
```

Expected: build OK, test skipped (env var not set).

- [ ] **Step 7.4 — Commit**

```
git add internal/cert/trust
git commit -m "feat(cert/trust): macOS Keychain install/uninstall"
```

---

## Task 8: Proxy router

**Files:**
- Create: `internal/proxy/router.go`
- Create: `internal/proxy/router_test.go`

- [ ] **Step 8.1 — Write failing tests**

Create `internal/proxy/router_test.go`:

```go
package proxy_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/proxy"
	"github.com/venkatkrishna07/mklocal/internal/store"
)

func TestRouterLookup(t *testing.T) {
	r := proxy.NewRouter()
	r.Set([]store.Route{
		{Domain: "foo.local", Target: "localhost:3000", Enabled: true},
		{Domain: "bar.local", Target: "localhost:4000", Enabled: true},
		{Domain: "off.local", Target: "localhost:5000", Enabled: false},
	})
	target, ok := r.Lookup("foo.local")
	require.True(t, ok)
	require.Equal(t, "localhost:3000", target)

	_, ok = r.Lookup("nope.local")
	require.False(t, ok)

	_, ok = r.Lookup("off.local")
	require.False(t, ok, "disabled routes must not resolve")
}

func TestRouterHotReload(t *testing.T) {
	r := proxy.NewRouter()
	r.Set([]store.Route{{Domain: "foo.local", Target: "old", Enabled: true}})
	r.Set([]store.Route{{Domain: "foo.local", Target: "new", Enabled: true}})
	target, ok := r.Lookup("foo.local")
	require.True(t, ok)
	require.Equal(t, "new", target)
}
```

- [ ] **Step 8.2 — Run tests, confirm fail**

```
go test ./internal/proxy/...
```

Expected: build fails.

- [ ] **Step 8.3 — Implement `router.go`**

Create `internal/proxy/router.go`:

```go
package proxy

import (
	"sync/atomic"

	"github.com/venkatkrishna07/mklocal/internal/store"
)

// Router is a hot-reloadable domain → target map.
// Concurrent reads are lock-free via atomic pointer swap.
type Router struct {
	table atomic.Pointer[map[string]string]
}

// NewRouter returns an empty router.
func NewRouter() *Router {
	r := &Router{}
	empty := map[string]string{}
	r.table.Store(&empty)
	return r
}

// Set replaces the routing table atomically. Disabled routes are dropped.
func (r *Router) Set(routes []store.Route) {
	next := make(map[string]string, len(routes))
	for _, rt := range routes {
		if !rt.Enabled {
			continue
		}
		next[rt.Domain] = rt.Target
	}
	r.table.Store(&next)
}

// Lookup returns the upstream for domain or "" / false if unknown/disabled.
func (r *Router) Lookup(domain string) (string, bool) {
	t := *r.table.Load()
	v, ok := t[domain]
	return v, ok
}
```

- [ ] **Step 8.4 — Run tests, confirm pass**

```
go test ./internal/proxy/...
```

Expected: PASS.

- [ ] **Step 8.5 — Commit**

```
git add internal/proxy/router.go internal/proxy/router_test.go
git commit -m "feat(proxy): atomic hot-reload domain router"
```

---

## Task 9: Proxy server (TLS + reverse proxy)

**Files:**
- Create: `internal/proxy/server.go`
- Create: `internal/proxy/server_test.go`

- [ ] **Step 9.1 — Write failing tests**

Create `internal/proxy/server_test.go`:

```go
package proxy_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/cert"
	"github.com/venkatkrishna07/mklocal/internal/proxy"
	"github.com/venkatkrishna07/mklocal/internal/store"
)

func TestProxyEndToEnd(t *testing.T) {
	// upstream backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "yes")
		fmt.Fprintln(w, "hello from", r.Host)
	}))
	defer backend.Close()

	// strip http:// prefix from backend URL
	target := strings.TrimPrefix(backend.URL, "http://")

	// CA + issuer
	ca, err := cert.CreateCA(t.TempDir(), "test")
	require.NoError(t, err)
	is := cert.NewIssuer(ca)

	// router with one route
	r := proxy.NewRouter()
	r.Set([]store.Route{{Domain: "foo.local", Target: target, Enabled: true}})

	// proxy on random port
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{GetCertificate: is.GetCertificate, MinVersion: tls.VersionTLS12})
	require.NoError(t, err)
	srv := proxy.NewServer(r, ln)
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() { _ = srv.Close() })

	// client trusts our CA, resolves foo.local to listener addr
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	hostPort := ln.Addr().String()
	dial := func(network, addr string) (net.Conn, error) {
		return net.Dial(network, hostPort)
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialTLS: func(network, addr string) (net.Conn, error) {
				return tls.Dial(network, hostPort, &tls.Config{RootCAs: pool, ServerName: "foo.local"})
			},
			Dial: dial,
		},
	}

	resp, err := client.Get("https://foo.local/")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "hello from")
	require.Equal(t, "yes", resp.Header.Get("X-Backend"))
}

func TestProxyUnknownHost404(t *testing.T) {
	ca, err := cert.CreateCA(t.TempDir(), "test")
	require.NoError(t, err)
	is := cert.NewIssuer(ca)
	r := proxy.NewRouter()

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{GetCertificate: is.GetCertificate, MinVersion: tls.VersionTLS12})
	require.NoError(t, err)
	srv := proxy.NewServer(r, ln)
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() { _ = srv.Close() })

	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	hostPort := ln.Addr().String()
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialTLS: func(network, addr string) (net.Conn, error) {
				return tls.Dial(network, hostPort, &tls.Config{RootCAs: pool, ServerName: "ghost.local"})
			},
		},
	}
	resp, err := client.Get("https://ghost.local/")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
```

- [ ] **Step 9.2 — Run tests, confirm fail**

```
go test ./internal/proxy/...
```

Expected: build fails.

- [ ] **Step 9.3 — Implement `server.go`**

Create `internal/proxy/server.go`:

```go
package proxy

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
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

// Close stops the server.
func (s *Server) Close() error { return s.srv.Close() }

// Addr returns the listener address.
func (s *Server) Addr() net.Addr { return s.ln.Addr() }

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	target, ok := s.router.Lookup(host)
	if !ok {
		slog.Info("proxy: no route", "host", host)
		http.Error(w, fmt.Sprintf("mklocal: no route for %s", host), http.StatusNotFound)
		return
	}
	upstream := &url.URL{Scheme: "http", Host: target}
	rp := httputil.NewSingleHostReverseProxy(upstream)
	rp.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
		slog.Warn("proxy: upstream error", "host", host, "target", target, "err", err)
		http.Error(rw, fmt.Sprintf("mklocal: upstream %s unreachable: %v", target, err), http.StatusBadGateway)
	}
	r.Host = target
	rp.ServeHTTP(w, r)
}
```

- [ ] **Step 9.4 — Run tests, confirm pass**

```
go test ./internal/proxy/...
```

Expected: PASS.

- [ ] **Step 9.5 — Commit**

```
git add internal/proxy/server.go internal/proxy/server_test.go
git commit -m "feat(proxy): TLS server with SNI cert and reverse proxy"
```

---

## Task 10: CLI root + cobra wiring

**Files:**
- Create: `internal/cli/root.go`
- Create: `internal/cli/cli_test.go`
- Modify: `cmd/mklocal/main.go`

- [ ] **Step 10.1 — Add cobra dep**

```
go get github.com/spf13/cobra@latest
```

- [ ] **Step 10.2 — Implement `internal/cli/root.go`**

Create:

```go
package cli

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mklocal/internal/version"
)

var (
	flagVerbose bool
	flagHome    string
)

// New returns the root cobra command.
func New() *cobra.Command {
	root := &cobra.Command{
		Use:           "mklocal",
		Short:         "Local HTTPS for your dev servers",
		Long:          "mklocal maps https://<name>.<tld> to local upstreams with auto-trusted TLS.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.String(),
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			lvl := slog.LevelInfo
			if flagVerbose {
				lvl = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))
		},
	}
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "enable debug logging")
	root.PersistentFlags().StringVar(&flagHome, "home", "", "override config dir (default ~/.mklocal)")

	root.AddCommand(
		newAddCmd(),
		newRemoveCmd(),
		newListCmd(),
		newServeCmd(),
		newInstallCmd(),
		newUninstallCmd(),
		newHostsHelperCmd(),
	)
	return root
}

// Execute runs the root command and returns its exit code.
func Execute() int {
	if err := New().Execute(); err != nil {
		slog.Error(err.Error())
		return 1
	}
	return 0
}

// HomeDir returns the resolved config directory. Honors --home and $MKLOCAL_HOME.
func HomeDir() (string, error) {
	if flagHome != "" {
		return flagHome, nil
	}
	if env := os.Getenv("MKLOCAL_HOME"); env != "" {
		return env, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".mklocal"), nil
}
```

- [ ] **Step 10.3 — Stub all subcommand constructors**

Create stub files so the build compiles. Real implementations land in later tasks.

`internal/cli/add.go`:

```go
package cli

import "github.com/spf13/cobra"

func newAddCmd() *cobra.Command {
	return &cobra.Command{Use: "add", Short: "placeholder", RunE: func(*cobra.Command, []string) error { return nil }}
}
```

Repeat for `remove.go`, `list.go`, `serve.go`, `install.go`, `uninstall.go`, `hostshelper.go` with the same shape but unique `Use` strings: `remove`, `list`, `serve`, `install`, `uninstall`, `hosts-helper`.

- [ ] **Step 10.4 — Wire `cmd/mklocal/main.go`**

Replace contents:

```go
package main

import (
	"os"

	"github.com/venkatkrishna07/mklocal/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
```

- [ ] **Step 10.5 — Write CLI smoke tests**

Create `internal/cli/cli_test.go`:

```go
package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/cli"
)

func TestRootHelp(t *testing.T) {
	root := cli.New()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--help"})
	require.NoError(t, root.Execute())
	require.Contains(t, buf.String(), "mklocal")
}

func TestRootListsSubcommands(t *testing.T) {
	root := cli.New()
	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"add", "remove", "list", "serve", "install", "uninstall", "hosts-helper"} {
		require.True(t, names[want], "missing subcommand %s", want)
	}
}
```

- [ ] **Step 10.6 — Build + test**

```
make build
go test ./internal/cli/...
./bin/mklocal --help
```

Expected: build OK, tests PASS, help text printed.

- [ ] **Step 10.7 — Commit**

```
git add internal/cli cmd/mklocal/main.go go.mod go.sum
git commit -m "feat(cli): cobra root + subcommand stubs"
```

---

## Task 11: CLI `add` / `remove` / `list`

**Files:**
- Modify: `internal/cli/add.go`
- Modify: `internal/cli/remove.go`
- Modify: `internal/cli/list.go`

These commands write to the store and mutate `/etc/hosts`. They do **not** require a running proxy — the proxy reads the store at serve time.

- [ ] **Step 11.1 — Implement `add.go`**

Replace contents:

```go
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mklocal/internal/config"
	"github.com/venkatkrishna07/mklocal/internal/hosts"
	"github.com/venkatkrishna07/mklocal/internal/store"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name> <host:port>",
		Short: "Map https://<name>.<tld> to a local upstream",
		Args:  cobra.ExactArgs(2),
		RunE:  runAdd,
	}
	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	home, err := HomeDir()
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(home, "config.toml"))
	if err != nil {
		return err
	}
	name, target := args[0], args[1]
	domain := strings.ToLower(name)
	if !strings.Contains(domain, ".") {
		domain += cfg.TLD
	}
	if !hosts.ValidHostname(domain) {
		return fmt.Errorf("invalid domain %q", domain)
	}

	s, err := store.Open(filepath.Join(home, "state.db"))
	if err != nil {
		return err
	}
	defer s.Close()

	if existing, err := s.GetRoute(domain); err == nil {
		return fmt.Errorf("route already exists: %s → %s", existing.Domain, existing.Target)
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}

	r := store.Route{
		Domain:  domain,
		Target:  target,
		TLD:     cfg.TLD,
		Enabled: true,
		Source:  "ad-hoc",
		AddedAt: time.Now().UTC(),
	}
	if err := s.PutRoute(r); err != nil {
		return err
	}

	binPath, err := os.Executable()
	if err != nil {
		return err
	}
	if err := hosts.NewEditor(binPath).Add(domain); err != nil {
		return fmt.Errorf("hosts: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "added: https://%s → %s\n", domain, target)
	return nil
}
```

- [ ] **Step 11.2 — Implement `remove.go`**

Replace contents:

```go
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mklocal/internal/config"
	"github.com/venkatkrishna07/mklocal/internal/hosts"
	"github.com/venkatkrishna07/mklocal/internal/store"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a domain mapping",
		Args:    cobra.ExactArgs(1),
		RunE:    runRemove,
	}
}

func runRemove(cmd *cobra.Command, args []string) error {
	home, err := HomeDir()
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(home, "config.toml"))
	if err != nil {
		return err
	}
	name := strings.ToLower(args[0])
	if !strings.Contains(name, ".") {
		name += cfg.TLD
	}
	s, err := store.Open(filepath.Join(home, "state.db"))
	if err != nil {
		return err
	}
	defer s.Close()
	if _, err := s.GetRoute(name); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("no such route: %s", name)
		}
		return err
	}
	if err := s.DeleteRoute(name); err != nil {
		return err
	}
	binPath, err := os.Executable()
	if err != nil {
		return err
	}
	if err := hosts.NewEditor(binPath).Remove(name); err != nil {
		return fmt.Errorf("hosts: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "removed: %s\n", name)
	return nil
}
```

- [ ] **Step 11.3 — Implement `list.go`**

Replace contents:

```go
package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mklocal/internal/store"
)

func newListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List domain mappings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := HomeDir()
			if err != nil {
				return err
			}
			s, err := store.Open(filepath.Join(home, "state.db"))
			if err != nil {
				return err
			}
			defer s.Close()
			rs, err := s.ListRoutes()
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(rs)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "DOMAIN\tTARGET\tENABLED\tSOURCE")
			for _, r := range rs {
				fmt.Fprintf(tw, "%s\t%s\t%v\t%s\n", r.Domain, r.Target, r.Enabled, r.Source)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output as JSON")
	return cmd
}
```

- [ ] **Step 11.4 — Build + smoke**

```
make build
./bin/mklocal list      # should print headers + no rows
```

Expected: empty table.

- [ ] **Step 11.5 — Commit**

```
git add internal/cli/add.go internal/cli/remove.go internal/cli/list.go
git commit -m "feat(cli): add, remove, list subcommands"
```

---

## Task 12: CLI `hosts-helper` (privileged subcommand)

**Files:**
- Modify: `internal/cli/hostshelper.go`

- [ ] **Step 12.1 — Implement `hostshelper.go`**

Replace contents:

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mklocal/internal/hosts"
)

const hostsComment = "managed by mklocal"

func newHostsHelperCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "hosts-helper",
		Short:  "Privileged hosts-file editor (invoked via sudo)",
		Hidden: true,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "add <host>",
		Args:  cobra.ExactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return helperAdd(args[0]) },
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "remove <host>",
		Args:  cobra.ExactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return helperRemove(args[0]) },
	})
	return cmd
}

func helperAdd(host string) error {
	if !hosts.ValidHostname(host) {
		return fmt.Errorf("invalid hostname %q", host)
	}
	body, err := os.ReadFile(hosts.HostsPath)
	if err != nil {
		return err
	}
	next, changed := hosts.AddEntry(string(body), "127.0.0.1", host, hostsComment)
	if !changed {
		return nil
	}
	return os.WriteFile(hosts.HostsPath, []byte(next), 0o644)
}

func helperRemove(host string) error {
	if !hosts.ValidHostname(host) {
		return fmt.Errorf("invalid hostname %q", host)
	}
	body, err := os.ReadFile(hosts.HostsPath)
	if err != nil {
		return err
	}
	next, changed := hosts.RemoveEntry(string(body), host)
	if !changed {
		return nil
	}
	return os.WriteFile(hosts.HostsPath, []byte(next), 0o644)
}
```

- [ ] **Step 12.2 — Verify build**

```
make build
./bin/mklocal hosts-helper --help
```

Expected: help text printed, command is `hidden` so absent from main `--help`.

- [ ] **Step 12.3 — Commit**

```
git add internal/cli/hostshelper.go
git commit -m "feat(cli): hosts-helper privileged subcommand"
```

---

## Task 13: CLI `install` / `uninstall`

**Files:**
- Modify: `internal/cli/install.go`
- Modify: `internal/cli/uninstall.go`

- [ ] **Step 13.1 — Implement `install.go`**

Replace contents:

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mklocal/internal/cert"
	"github.com/venkatkrishna07/mklocal/internal/cert/trust"
	"github.com/venkatkrishna07/mklocal/internal/config"
)

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Generate CA, trust it, prepare state dir",
		RunE:  runInstall,
	}
}

func runInstall(cmd *cobra.Command, _ []string) error {
	home, err := HomeDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	cfgPath := filepath.Join(home, "config.toml")
	if _, statErr := os.Stat(cfgPath); os.IsNotExist(statErr) {
		if err := config.Save(cfgPath, config.Default()); err != nil {
			return err
		}
	}

	caDir := filepath.Join(home, "ca")
	var ca *cert.CA
	if _, statErr := os.Stat(filepath.Join(caDir, "rootCA.pem")); os.IsNotExist(statErr) {
		fmt.Fprintln(cmd.OutOrStdout(), "generating local CA...")
		ca, err = cert.CreateCA(caDir, "mklocal local CA")
	} else {
		ca, err = cert.LoadCA(caDir)
	}
	if err != nil {
		return err
	}

	ok, err := trust.IsInstalled(ca.Cert)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(cmd.OutOrStdout(), "installing CA in macOS Keychain (you will be prompted for your password)...")
		if err := trust.Install(filepath.Join(caDir, "rootCA.pem")); err != nil {
			return err
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "install complete. next: `mklocal add foo localhost:3000 && mklocal serve`.")
	return nil
}
```

- [ ] **Step 13.2 — Implement `uninstall.go`**

Replace contents:

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mklocal/internal/cert/trust"
)

func newUninstallCmd() *cobra.Command {
	var purge bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Untrust CA and optionally purge state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := HomeDir()
			if err != nil {
				return err
			}
			rootCA := filepath.Join(home, "ca", "rootCA.pem")
			if _, err := os.Stat(rootCA); err == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "removing CA from Keychain...")
				if err := trust.Uninstall(rootCA); err != nil {
					return err
				}
			}
			if purge {
				fmt.Fprintln(cmd.OutOrStdout(), "purging", home)
				return os.RemoveAll(home)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "uninstalled. config preserved at", home)
			return nil
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "also delete config, state, certs")
	return cmd
}
```

- [ ] **Step 13.3 — Build**

```
make build
./bin/mklocal install --help
```

Expected: help OK.

- [ ] **Step 13.4 — Commit**

```
git add internal/cli/install.go internal/cli/uninstall.go
git commit -m "feat(cli): install (CA gen + trust) and uninstall"
```

---

## Task 14: CLI `serve` (foreground daemon)

**Files:**
- Modify: `internal/cli/serve.go`

- [ ] **Step 14.1 — Implement `serve.go`**

Replace contents:

```go
package cli

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mklocal/internal/cert"
	"github.com/venkatkrishna07/mklocal/internal/config"
	"github.com/venkatkrishna07/mklocal/internal/proxy"
	"github.com/venkatkrishna07/mklocal/internal/store"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the foreground reverse proxy on :<proxy_port>",
		RunE:  runServe,
	}
}

func runServe(cmd *cobra.Command, _ []string) error {
	home, err := HomeDir()
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(home, "config.toml"))
	if err != nil {
		return err
	}
	ca, err := cert.LoadCA(filepath.Join(home, "ca"))
	if err != nil {
		return fmt.Errorf("CA not found — run `mklocal install` first: %w", err)
	}
	is := cert.NewIssuer(ca)

	s, err := store.Open(filepath.Join(home, "state.db"))
	if err != nil {
		return err
	}
	defer s.Close()

	router := proxy.NewRouter()
	routes, err := s.ListRoutes()
	if err != nil {
		return err
	}
	router.Set(routes)

	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if rs, err := s.ListRoutes(); err == nil {
					router.Set(rs)
				}
			}
		}
	}()

	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(cfg.ProxyPort))
	ln, err := tls.Listen("tcp", addr, &tls.Config{
		GetCertificate: is.GetCertificate,
		MinVersion:     tls.VersionTLS12,
	})
	if err != nil {
		return fmt.Errorf("listen %s: %w (binding :443 requires sudo or CAP_NET_BIND_SERVICE)", addr, err)
	}
	defer ln.Close()
	srv := proxy.NewServer(router, ln)
	slog.Info("proxy: listening", "addr", ln.Addr().String(), "routes", len(routes))

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve() }()

	select {
	case <-ctx.Done():
		slog.Info("proxy: shutting down")
		return srv.Close()
	case err := <-errCh:
		return err
	}
}
```

> Note: until Plan 5 (Linux `setcap`), running `mklocal serve` on macOS requires `sudo` to bind `:443`. For local testing, set `proxy_port = 8443` in config to avoid sudo.

- [ ] **Step 14.2 — Build + verify help**

```
make build
./bin/mklocal serve --help
```

Expected: help OK.

- [ ] **Step 14.3 — Commit**

```
git add internal/cli/serve.go
git commit -m "feat(cli): foreground serve with poll-based router reload"
```

---

## Task 15: End-to-end smoke test (manual)

This task is manual verification on a real macOS box. No code, but the steps document what "Plan 1 done" means.

- [ ] **Step 15.1 — Run install**

```
make build
./bin/mklocal install
```

Expected: CA generated under `~/.mklocal/ca/`, Keychain prompt accepted.

- [ ] **Step 15.2 — Start an upstream**

In another shell:

```
python3 -m http.server 3000
```

- [ ] **Step 15.3 — Add a route**

```
./bin/mklocal add foo localhost:3000
```

Expected: `added: https://foo.local → localhost:3000`. `/etc/hosts` contains `127.0.0.1\tfoo.local\t# managed by mklocal`. `sudo` prompt accepted.

- [ ] **Step 15.4 — Set test port (avoid :443 sudo for first run)**

Edit `~/.mklocal/config.toml`:

```toml
proxy_port = 8443
```

- [ ] **Step 15.5 — Run the proxy**

```
./bin/mklocal serve
```

Expected: log line `proxy: listening addr=0.0.0.0:8443 routes=1`.

- [ ] **Step 15.6 — Curl through the proxy**

In another shell:

```
curl -v https://foo.local:8443/
```

Expected: TLS handshake succeeds (CA trusted), 200 response with the Python directory listing.

- [ ] **Step 15.7 — Remove the route**

```
./bin/mklocal remove foo
./bin/mklocal list
```

Expected: route gone, `/etc/hosts` line removed.

- [ ] **Step 15.8 — (Optional) Bind :443**

Switch `proxy_port = 443` in config, run `sudo ./bin/mklocal serve`, retry `curl https://foo.local/` without an explicit port.

Plan 1 is complete when steps 15.1 – 15.7 pass on a clean machine.

---

## Self-review notes

- All packages declared in the spec section 4.1 that Plan 1 touches are covered: `version`, `config`, `store`, `hosts`, `cert`, `cert/trust/darwin`, `proxy`, `cli`. Packages deferred to later plans (`daemon`, `tui`, `ipc`, `doctor`, `app`, `cert/trust/linux`, `cert/trust/windows`, `internal/daemon/service`) are explicitly listed under "Out of scope for Plan 1".
- Spec section 5.3 hosts-helper invocation pattern is implemented in Task 4 (`internal/hosts/hosts_darwin.go`) and Task 12 (`internal/cli/hostshelper.go`).
- Spec section 5.5 bbolt schema is partially implemented (`routes` bucket only); `projects`, `settings`, `stats`, `logs` buckets are created but unused in Plan 1.
- Spec uses proto-encoded values; Plan 1 substitutes JSON because protos do not exist until Plan 2. This is called out under Task 3 step 3.5.
- TUI work is entirely deferred.

## Definition of done

- All tests pass on macOS: `make test`.
- Lint clean: `golangci-lint run`.
- Manual smoke test (Task 15) passes end-to-end on a fresh `~/.mklocal/`.
- Documentation committed under `docs/superpowers/specs/` and `docs/superpowers/plans/`.
