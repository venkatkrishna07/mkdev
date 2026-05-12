# Plan 2 — gRPC Daemon + IPC + launchd Service

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Git policy (project CLAUDE.md):** Do **not** run `git add`, `git commit`, `git push`, `git reset`, or `git checkout` without explicit confirmation from the user. Commit steps are written into the plan for reference; ask before executing them.

**Goal:** Split mklocal into a long-running user-level daemon (managed by launchd) and a thin gRPC client. After this plan, `mklocal install` registers a launchd agent; the daemon owns the proxy, cert issuer, hosts editor, and route store. CLI subcommands (`add`, `remove`, `list`) become gRPC clients talking to the daemon over a Unix socket. Foundation for the TUI in Plans 3–4.

**Architecture:** Single binary, two execution modes. (a) `mklocal daemon` runs the long-lived process: opens the bbolt store, generates leaf certs, mutates `/etc/hosts` via the existing `hosts-helper` sudo path, runs the TLS reverse proxy on `127.0.0.1:<proxy_port>`, and serves a gRPC API on `~/.mklocal/daemon.sock`. (b) Every other subcommand (`add`, `remove`, `list`, `status`, ...) is a gRPC client. `mklocal install` writes a launchd plist; `mklocal start`/`stop`/`status` drive `launchctl bootstrap`/`bootout`.

**Tech Stack:** Go 1.25, `google.golang.org/grpc`, `google.golang.org/protobuf`, `buf` (codegen), Unix domain socket (macOS), launchd LaunchAgent.

**Out of scope for Plan 2 (deferred):**
- TUI (Plans 3 & 4).
- Linux systemd-user / Windows service / Windows named pipe (Plan 5).
- HTTPS termination on `:443` without sudo via launchd port handoff (Plan 5 / 6).
- Goreleaser, signed binaries, GH Releases (Plan 6).
- `mklocal run <cmd>` ephemeral mode (deferred to Plan 7+).

---

## Design decisions

| Decision | Choice | Rationale |
|---|---|---|
| Codegen tool | **buf** (`buf.build`) | modern community standard; deterministic `buf.gen.yaml`; locks plugin versions |
| Generated stubs location | `internal/api/` | private to module — no public Go API surface for gRPC types |
| Wire transport | **Unix socket** at `~/.mklocal/daemon.sock` (mode 0600) | owner-only, fast, native gRPC |
| Daemon model | **Mandatory.** `add`/`remove`/`list` fail clearly if no daemon. | dev-bind/tailscale pattern; cleaner two-process model |
| Daemon start | `launchctl bootstrap` user domain LaunchAgent | first-class macOS lifecycle, RunAtLoad, KeepAlive=true |
| `serve` subcommand | Retired; replaced by `mklocal daemon` (foreground) | one source of truth for the daemon code path |
| Hot reload | Daemon owns the in-memory router; mutations go through gRPC and emit events on an in-proc bus. The 2-second polling ticker from Plan 1 is **deleted**. | event-driven beats polling |
| Event delivery to clients | gRPC server-streaming RPC `WatchEvents` | future-proofs Plan 3 TUI live tab |
| Daemon log | `~/.mklocal/logs/daemon.log` via `slog` JSON handler | parseable, rotatable in Plan 4 |
| Auth | None — Unix socket perms 0600 is the security boundary | sufficient for a single-user dev tool |

---

## File map

### New files

| Path | Responsibility |
|---|---|
| `api/proto/mklocal.proto` | gRPC service contract (proto3) |
| `api/proto/buf.yaml` | buf module config |
| `api/proto/buf.gen.yaml` | buf codegen config (Go + Go-gRPC plugins) |
| `api/proto/buf.lock` | locked plugin/dep versions |
| `internal/api/mklocal.pb.go` | generated message types (do not edit) |
| `internal/api/mklocal_grpc.pb.go` | generated gRPC client/server stubs (do not edit) |
| `internal/ipc/paths.go` | socket path resolution + perms |
| `internal/ipc/client.go` | gRPC `*grpc.ClientConn` factory over Unix socket |
| `internal/ipc/server.go` | gRPC `*grpc.Server` factory + listener helper |
| `internal/ipc/ipc_test.go` | round-trip test with a stub service |
| `internal/daemon/daemon.go` | top-level Daemon struct: wires store, cert, proxy, hosts, gRPC |
| `internal/daemon/service.go` | `api.MklocalServer` implementation (route CRUD, status, watch) |
| `internal/daemon/events.go` | in-proc event bus (subscribe / publish / fan-out) |
| `internal/daemon/log.go` | rotates + opens daemon log file, returns slog handler |
| `internal/daemon/daemon_test.go` | integration test: spin daemon on temp socket, call AddRoute, assert event |
| `internal/daemon/service/darwin.go` | launchd plist install/uninstall/status |
| `internal/daemon/service/darwin_test.go` | plist marshalling test (no real launchctl invocation) |
| `internal/cli/daemon.go` | `mklocal daemon` subcommand — runs the daemon in foreground |
| `internal/cli/start.go` | `mklocal start` — `launchctl bootstrap` |
| `internal/cli/stop.go` | `mklocal stop` — `launchctl bootout` |
| `internal/cli/status.go` | `mklocal status` — combines launchctl + gRPC Status |
| `internal/cli/client.go` | shared `dialDaemon(ctx) (*grpc.ClientConn, error)` helper |
| `scripts/proto-gen.sh` | invokes `buf generate` from repo root |

### Modified files

| Path | Why |
|---|---|
| `go.mod` | add `grpc` + `protobuf` deps |
| `Makefile` | add `proto` target |
| `internal/cli/root.go` | register `daemon`/`start`/`stop`/`status` subcommands |
| `internal/cli/add.go` | replace direct store/hosts calls with `client.AddRoute` |
| `internal/cli/remove.go` | replace with `client.RemoveRoute` |
| `internal/cli/list.go` | replace with `client.ListRoutes` |
| `internal/cli/serve.go` | **delete** — superseded by `daemon` subcommand |
| `internal/cli/install.go` | also install launchd plist (after Keychain trust) |
| `internal/cli/uninstall.go` | also `launchctl bootout` and remove plist |
| `internal/proxy/router.go` | expose `Subscribe`/`Routes` hooks for daemon — or keep router unchanged and let daemon maintain its own snapshot (decide in Task 6) |
| `.gitignore` | ignore generated `*.pb.go` if we decide to; in this plan we **commit** the generated files for reproducibility |

### Deleted files

- `internal/cli/serve.go` (replaced by `daemon.go`)

---

## Task 1: Add gRPC + protobuf dependencies and proto schema

**Files:**
- Create: `api/proto/mklocal.proto`
- Create: `api/proto/buf.yaml`
- Create: `api/proto/buf.gen.yaml`
- Create: `scripts/proto-gen.sh`
- Modify: `go.mod`
- Modify: `Makefile`

- [ ] **Step 1.1 — Add gRPC + protobuf dependencies**

```
go get google.golang.org/grpc@latest google.golang.org/protobuf@latest
```

Expected: `go.mod` gains `google.golang.org/grpc` and `google.golang.org/protobuf` (and a tail of transitives).

- [ ] **Step 1.2 — Write `api/proto/mklocal.proto`**

```protobuf
syntax = "proto3";

package mklocal.v1;

option go_package = "github.com/venkatkrishna07/mklocal/internal/api;api";

import "google/protobuf/timestamp.proto";
import "google/protobuf/empty.proto";

// Route describes a single domain → upstream mapping.
message Route {
  string domain = 1;
  string target = 2;
  string tld = 3;
  bool   enabled = 4;
  string source = 5;
  google.protobuf.Timestamp added_at = 6;
}

message RouteRef {
  string domain = 1;
}

message RouteList {
  repeated Route routes = 1;
}

message DaemonStatus {
  string version = 1;
  string addr = 2;            // proxy bind address
  int32  route_count = 3;
  google.protobuf.Timestamp started_at = 4;
}

message Event {
  oneof kind {
    Route        route_added    = 1;
    RouteRef     route_removed  = 2;
    Route        route_updated  = 3;
    DaemonStatus daemon_status  = 4;
  }
}

message EventFilter {
  bool routes_only = 1;
}

service Mklocal {
  rpc Status(google.protobuf.Empty)   returns (DaemonStatus);
  rpc ListRoutes(google.protobuf.Empty) returns (RouteList);
  rpc AddRoute(Route)                  returns (Route);
  rpc RemoveRoute(RouteRef)            returns (google.protobuf.Empty);
  rpc UpdateRoute(Route)               returns (Route);
  rpc ToggleRoute(RouteRef)            returns (Route);
  rpc WatchEvents(EventFilter)         returns (stream Event);
  rpc Shutdown(google.protobuf.Empty)  returns (google.protobuf.Empty);
}
```

- [ ] **Step 1.3 — Write `api/proto/buf.yaml`**

```yaml
version: v2
modules:
  - path: .
lint:
  use:
    - STANDARD
  except:
    - PACKAGE_VERSION_SUFFIX  # we use mklocal.v1 already
breaking:
  use:
    - FILE
```

- [ ] **Step 1.4 — Write `api/proto/buf.gen.yaml`**

```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: ../../internal/api
    opt:
      - paths=source_relative
  - remote: buf.build/grpc/go
    out: ../../internal/api
    opt:
      - paths=source_relative
      - require_unimplemented_servers=true
inputs:
  - directory: .
```

- [ ] **Step 1.5 — Write `scripts/proto-gen.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../api/proto"

if ! command -v buf >/dev/null 2>&1; then
  echo "buf not installed. Install: brew install bufbuild/buf/buf" >&2
  exit 1
fi

buf generate

# Flatten: buf places files relative to module root; ensure they land directly
# inside internal/api with no package subdirs.
echo "generated:"
ls -1 ../../internal/api/*.pb.go
```

Make executable:

```
chmod +x scripts/proto-gen.sh
```

- [ ] **Step 1.6 — Add `proto` target to Makefile**

Append to `Makefile`:

```make
proto:
	./scripts/proto-gen.sh
```

Add `proto` to `.PHONY` line at top of file.

- [ ] **Step 1.7 — Run codegen**

```
make proto
```

Expected: `internal/api/mklocal.pb.go` and `internal/api/mklocal_grpc.pb.go` created. If `buf` is not installed, follow the message to install it, then re-run.

- [ ] **Step 1.8 — Verify generated code builds**

```
go build ./internal/api/...
```

Expected: clean.

- [ ] **Step 1.9 — Commit (ask user first)**

```
git add api/proto Makefile scripts/proto-gen.sh internal/api go.mod go.sum
git commit -m "feat(api): add gRPC proto schema and buf codegen pipeline"
```

---

## Task 2: IPC paths + transport helpers

**Files:**
- Create: `internal/ipc/paths.go`
- Create: `internal/ipc/paths_test.go`

- [ ] **Step 2.1 — Write failing tests**

Create `internal/ipc/paths_test.go`:

```go
package ipc_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/ipc"
)

func TestSocketPath(t *testing.T) {
	got := ipc.SocketPath("/home/foo/.mklocal")
	require.Equal(t, filepath.Join("/home/foo/.mklocal", "daemon.sock"), got)
}

func TestPidPath(t *testing.T) {
	got := ipc.PidPath("/home/foo/.mklocal")
	require.Equal(t, filepath.Join("/home/foo/.mklocal", "daemon.pid"), got)
}
```

- [ ] **Step 2.2 — Implement `paths.go`**

Create `internal/ipc/paths.go`:

```go
package ipc

import "path/filepath"

// SocketPath returns the Unix socket path used by the daemon.
func SocketPath(home string) string { return filepath.Join(home, "daemon.sock") }

// PidPath returns the path where the daemon writes its PID for liveness checks.
func PidPath(home string) string { return filepath.Join(home, "daemon.pid") }
```

- [ ] **Step 2.3 — Run tests**

```
go test ./internal/ipc/...
```

Expected: PASS.

- [ ] **Step 2.4 — Commit**

```
git add internal/ipc/paths.go internal/ipc/paths_test.go
git commit -m "feat(ipc): socket and pid path helpers"
```

---

## Task 3: IPC server (Unix socket gRPC listener)

**Files:**
- Create: `internal/ipc/server.go`
- Create: `internal/ipc/ipc_test.go`

- [ ] **Step 3.1 — Write failing tests**

Create `internal/ipc/ipc_test.go`:

```go
package ipc_test

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/ipc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestServerListenAndClient(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "test.sock")

	srv, ln, err := ipc.NewServer(sock)
	require.NoError(t, err)
	healthpb.RegisterHealthServer(srv, health.NewServer())

	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { srv.GracefulStop() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		"unix:"+sock,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	require.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)
}

func TestServerSocketPerms(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "test.sock")

	_, ln, err := ipc.NewServer(sock)
	require.NoError(t, err)
	defer ln.Close()

	addr := ln.Addr().(*net.UnixAddr)
	require.Equal(t, sock, addr.Name)

	info, err := osStatPerm(sock)
	require.NoError(t, err)
	require.Equal(t, "-rw-------", info, "socket must be 0600")
}
```

Add a small helper to the same file:

```go
import "os"

func osStatPerm(p string) (string, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	return fi.Mode().String(), nil
}
```

- [ ] **Step 3.2 — Run tests, confirm fail**

```
go test ./internal/ipc/...
```

Expected: build fails — `ipc.NewServer` undefined.

- [ ] **Step 3.3 — Implement `server.go`**

Create `internal/ipc/server.go`:

```go
package ipc

import (
	"fmt"
	"net"
	"os"

	"google.golang.org/grpc"
)

// NewServer creates a gRPC server bound to a Unix socket at sock.
// The socket file is removed if it already exists and chmod'd to 0600 after
// the listener is created.
func NewServer(sock string) (*grpc.Server, net.Listener, error) {
	_ = os.Remove(sock) // ignore missing
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return nil, nil, fmt.Errorf("ipc: listen %s: %w", sock, err)
	}
	if err := os.Chmod(sock, 0o600); err != nil {
		_ = ln.Close()
		return nil, nil, fmt.Errorf("ipc: chmod %s: %w", sock, err)
	}
	return grpc.NewServer(), ln, nil
}
```

- [ ] **Step 3.4 — Run tests**

```
go test ./internal/ipc/...
```

Expected: PASS.

- [ ] **Step 3.5 — Commit**

```
git add internal/ipc/server.go internal/ipc/ipc_test.go go.mod go.sum
git commit -m "feat(ipc): unix socket gRPC server"
```

---

## Task 4: IPC client

**Files:**
- Create: `internal/ipc/client.go`

- [ ] **Step 4.1 — Implement `client.go`**

```go
package ipc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ErrDaemonNotRunning indicates no daemon is listening on the socket.
var ErrDaemonNotRunning = errors.New("ipc: daemon not running")

// Dial connects to the daemon's gRPC server at sock. Returns
// ErrDaemonNotRunning if the socket file is missing or refuses connection.
func Dial(ctx context.Context, sock string) (*grpc.ClientConn, error) {
	if _, err := os.Stat(sock); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrDaemonNotRunning
		}
		return nil, fmt.Errorf("ipc: stat %s: %w", sock, err)
	}
	dialCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(
		"unix:"+sock,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("ipc: dial: %w", err)
	}
	// Probe with a no-op connect to catch dead socket files left behind.
	conn.Connect()
	state := conn.GetState()
	deadline := time.Now().Add(time.Second)
	for state.String() != "READY" && time.Now().Before(deadline) {
		if !conn.WaitForStateChange(dialCtx, state) {
			break
		}
		state = conn.GetState()
	}
	if state.String() != "READY" {
		_ = conn.Close()
		// Distinguish stale socket from genuine disconnect.
		if _, derr := net.Dial("unix", sock); derr != nil {
			return nil, ErrDaemonNotRunning
		}
		return nil, fmt.Errorf("ipc: connection not ready: %s", state)
	}
	return conn, nil
}
```

- [ ] **Step 4.2 — Test by extending `ipc_test.go`**

Append:

```go
func TestDialNoDaemon(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "nope.sock")
	_, err := ipc.Dial(t.Context(), sock)
	require.ErrorIs(t, err, ipc.ErrDaemonNotRunning)
}
```

- [ ] **Step 4.3 — Run tests**

```
go test ./internal/ipc/...
```

Expected: PASS.

- [ ] **Step 4.4 — Commit**

```
git add internal/ipc/client.go internal/ipc/ipc_test.go
git commit -m "feat(ipc): unix socket gRPC client with daemon-not-running sentinel"
```

---

## Task 5: Daemon log setup

**Files:**
- Create: `internal/daemon/log.go`
- Create: `internal/daemon/log_test.go`

- [ ] **Step 5.1 — Write failing test**

Create `internal/daemon/log_test.go`:

```go
package daemon_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/daemon"
)

func TestNewFileLogger(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "daemon.log")
	logger, close, err := daemon.NewFileLogger(logPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = close() })
	logger.Info("hello", "k", "v")

	require.NoError(t, close())

	b, err := os.ReadFile(logPath)
	require.NoError(t, err)
	require.NotEmpty(t, b)

	var rec map[string]any
	require.NoError(t, json.Unmarshal(b[:len(b)-1], &rec), "must be valid JSON line")
	require.Equal(t, "hello", rec["msg"])
	require.Equal(t, "v", rec["k"])
}
```

- [ ] **Step 5.2 — Run tests, confirm fail**

```
go test ./internal/daemon/...
```

Expected: build fails.

- [ ] **Step 5.3 — Implement `log.go`**

Create `internal/daemon/log.go`:

```go
package daemon

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// NewFileLogger opens (creates) a JSON-line log file at path with 0600 perms
// and returns an *slog.Logger writing to it. The returned close func flushes
// and closes the file.
func NewFileLogger(path string) (*slog.Logger, func() error, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, nil, fmt.Errorf("daemon: log mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: open log %s: %w", path, err)
	}
	logger := slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return logger, f.Close, nil
}
```

- [ ] **Step 5.4 — Run tests**

```
go test ./internal/daemon/...
```

Expected: PASS.

- [ ] **Step 5.5 — Commit**

```
git add internal/daemon/log.go internal/daemon/log_test.go
git commit -m "feat(daemon): JSON file logger"
```

---

## Task 6: Event bus

**Files:**
- Create: `internal/daemon/events.go`
- Create: `internal/daemon/events_test.go`

- [ ] **Step 6.1 — Write failing test**

Create `internal/daemon/events_test.go`:

```go
package daemon_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	api "github.com/venkatkrishna07/mklocal/internal/api"
	"github.com/venkatkrishna07/mklocal/internal/daemon"
)

func TestBusFanOut(t *testing.T) {
	bus := daemon.NewBus()
	a := bus.Subscribe()
	b := bus.Subscribe()
	defer bus.Unsubscribe(a)
	defer bus.Unsubscribe(b)

	ev := &api.Event{Kind: &api.Event_RouteAdded{RouteAdded: &api.Route{Domain: "foo.local"}}}
	bus.Publish(ev)

	for _, ch := range []<-chan *api.Event{a, b} {
		select {
		case got := <-ch:
			require.Equal(t, "foo.local", got.GetRouteAdded().Domain)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("subscriber missed event")
		}
	}
}

func TestBusUnsubscribeStopsDelivery(t *testing.T) {
	bus := daemon.NewBus()
	a := bus.Subscribe()
	bus.Unsubscribe(a)

	bus.Publish(&api.Event{Kind: &api.Event_RouteAdded{RouteAdded: &api.Route{Domain: "x"}}})

	select {
	case _, ok := <-a:
		require.False(t, ok, "channel must be closed after Unsubscribe")
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Unsubscribe did not close channel")
	}
}
```

- [ ] **Step 6.2 — Run tests, confirm fail**

```
go test ./internal/daemon/...
```

Expected: build fails.

- [ ] **Step 6.3 — Implement `events.go`**

Create `internal/daemon/events.go`:

```go
package daemon

import (
	"sync"

	api "github.com/venkatkrishna07/mklocal/internal/api"
)

// Bus is an in-process fan-out for daemon events. Subscribers receive every
// event published after they subscribe; slow subscribers drop events rather
// than blocking the publisher.
type Bus struct {
	mu   sync.Mutex
	subs map[chan *api.Event]struct{}
}

// NewBus creates an empty Bus.
func NewBus() *Bus { return &Bus{subs: map[chan *api.Event]struct{}{}} }

// Subscribe returns a channel that receives every published event until
// Unsubscribe is called.
func (b *Bus) Subscribe() <-chan *api.Event {
	ch := make(chan *api.Event, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe stops delivery and closes the channel.
func (b *Bus) Unsubscribe(ch <-chan *api.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for c := range b.subs {
		if c == ch {
			delete(b.subs, c)
			close(c)
			return
		}
	}
}

// Publish delivers ev to every subscriber. Subscribers with full buffers
// drop the event silently.
func (b *Bus) Publish(ev *api.Event) {
	b.mu.Lock()
	subs := make([]chan *api.Event, 0, len(b.subs))
	for c := range b.subs {
		subs = append(subs, c)
	}
	b.mu.Unlock()
	for _, c := range subs {
		select {
		case c <- ev:
		default:
		}
	}
}
```

- [ ] **Step 6.4 — Run tests**

```
go test ./internal/daemon/...
```

Expected: PASS.

- [ ] **Step 6.5 — Commit**

```
git add internal/daemon/events.go internal/daemon/events_test.go
git commit -m "feat(daemon): in-proc event bus with non-blocking fan-out"
```

---

## Task 7: gRPC service implementation (route CRUD + Status)

**Files:**
- Create: `internal/daemon/service.go`
- Create: `internal/daemon/service_test.go`

This task wires the route operations through gRPC. Hosts file mutation is **not** in scope of the service implementation itself — the daemon executes hosts edits via the existing `hosts-helper` sudo path when serving an AddRoute/RemoveRoute (Task 8). For now, the service mutates only the store + emits events.

- [ ] **Step 7.1 — Write failing test**

Create `internal/daemon/service_test.go`:

```go
package daemon_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	api "github.com/venkatkrishna07/mklocal/internal/api"
	"github.com/venkatkrishna07/mklocal/internal/daemon"
	"github.com/venkatkrishna07/mklocal/internal/store"
	"google.golang.org/protobuf/types/known/emptypb"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestServiceAddListRemove(t *testing.T) {
	s := openStore(t)
	bus := daemon.NewBus()
	svc := daemon.NewService(daemon.ServiceOpts{
		Store:     s,
		Bus:       bus,
		TLD:       ".local",
		HostsAdd:  func(string) error { return nil },
		HostsRm:   func(string) error { return nil },
		Version:   "test",
	})

	added, err := svc.AddRoute(t.Context(), &api.Route{Domain: "foo.local", Target: "localhost:3000"})
	require.NoError(t, err)
	require.Equal(t, "foo.local", added.Domain)
	require.True(t, added.Enabled)

	list, err := svc.ListRoutes(t.Context(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, list.Routes, 1)

	_, err = svc.RemoveRoute(t.Context(), &api.RouteRef{Domain: "foo.local"})
	require.NoError(t, err)

	list, err = svc.ListRoutes(t.Context(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Empty(t, list.Routes)
}

func TestServiceEventsEmitted(t *testing.T) {
	s := openStore(t)
	bus := daemon.NewBus()
	svc := daemon.NewService(daemon.ServiceOpts{
		Store:    s,
		Bus:      bus,
		TLD:      ".local",
		HostsAdd: func(string) error { return nil },
		HostsRm:  func(string) error { return nil },
		Version:  "test",
	})
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	_, err := svc.AddRoute(t.Context(), &api.Route{Domain: "foo.local", Target: "localhost:3000"})
	require.NoError(t, err)

	select {
	case ev := <-ch:
		require.NotNil(t, ev.GetRouteAdded())
	case <-time.After(200 * time.Millisecond):
		t.Fatal("RouteAdded not emitted")
	}
}

func TestServiceStatus(t *testing.T) {
	s := openStore(t)
	svc := daemon.NewService(daemon.ServiceOpts{
		Store:    s,
		Bus:      daemon.NewBus(),
		TLD:      ".local",
		HostsAdd: func(string) error { return nil },
		HostsRm:  func(string) error { return nil },
		Version:  "v0.2.0",
	})
	got, err := svc.Status(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Equal(t, "v0.2.0", got.Version)
}
```

- [ ] **Step 7.2 — Run tests, confirm fail**

```
go test ./internal/daemon/...
```

Expected: build fails.

- [ ] **Step 7.3 — Implement `service.go`**

Create `internal/daemon/service.go`:

```go
package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	api "github.com/venkatkrishna07/mklocal/internal/api"
	"github.com/venkatkrishna07/mklocal/internal/hosts"
	"github.com/venkatkrishna07/mklocal/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ServiceOpts is the constructor argument for Service.
type ServiceOpts struct {
	Store    *store.Store
	Bus      *Bus
	TLD      string
	Version  string
	HostsAdd func(domain string) error
	HostsRm  func(domain string) error
}

// Service implements api.MklocalServer.
type Service struct {
	api.UnimplementedMklocalServer
	opts      ServiceOpts
	startedAt time.Time
}

// NewService constructs a Service.
func NewService(opts ServiceOpts) *Service {
	if opts.Store == nil || opts.Bus == nil {
		panic("daemon: NewService requires Store and Bus")
	}
	return &Service{opts: opts, startedAt: time.Now().UTC()}
}

func (s *Service) Status(_ context.Context, _ *emptypb.Empty) (*api.DaemonStatus, error) {
	routes, err := s.opts.Store.ListRoutes()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list: %v", err)
	}
	return &api.DaemonStatus{
		Version:    s.opts.Version,
		RouteCount: int32(len(routes)),
		StartedAt:  timestamppb.New(s.startedAt),
	}, nil
}

func (s *Service) ListRoutes(_ context.Context, _ *emptypb.Empty) (*api.RouteList, error) {
	rs, err := s.opts.Store.ListRoutes()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list: %v", err)
	}
	out := &api.RouteList{Routes: make([]*api.Route, 0, len(rs))}
	for _, r := range rs {
		out.Routes = append(out.Routes, toProto(r))
	}
	return out, nil
}

func (s *Service) AddRoute(_ context.Context, in *api.Route) (*api.Route, error) {
	if in == nil || in.Domain == "" || in.Target == "" {
		return nil, status.Error(codes.InvalidArgument, "domain and target required")
	}
	domain := strings.ToLower(in.Domain)
	if !strings.Contains(domain, ".") {
		domain += s.opts.TLD
	}
	if !hosts.ValidHostname(domain) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid domain %q", domain)
	}
	if _, err := s.opts.Store.GetRoute(domain); err == nil {
		return nil, status.Errorf(codes.AlreadyExists, "route exists: %s", domain)
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, status.Errorf(codes.Internal, "get: %v", err)
	}
	if err := s.opts.HostsAdd(domain); err != nil {
		return nil, status.Errorf(codes.Internal, "hosts: %v", err)
	}
	r := store.Route{
		Domain:  domain,
		Target:  in.Target,
		TLD:     s.opts.TLD,
		Enabled: true,
		Source:  store.SourceAdHoc,
		AddedAt: time.Now().UTC(),
	}
	if err := s.opts.Store.PutRoute(r); err != nil {
		_ = s.opts.HostsRm(domain)
		return nil, status.Errorf(codes.Internal, "put: %v", err)
	}
	proto := toProto(r)
	s.opts.Bus.Publish(&api.Event{Kind: &api.Event_RouteAdded{RouteAdded: proto}})
	return proto, nil
}

func (s *Service) RemoveRoute(_ context.Context, in *api.RouteRef) (*emptypb.Empty, error) {
	if in == nil || in.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain required")
	}
	domain := strings.ToLower(in.Domain)
	if _, err := s.opts.Store.GetRoute(domain); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "no such route: %s", domain)
		}
		return nil, status.Errorf(codes.Internal, "get: %v", err)
	}
	if err := s.opts.HostsRm(domain); err != nil {
		return nil, status.Errorf(codes.Internal, "hosts: %v", err)
	}
	if err := s.opts.Store.DeleteRoute(domain); err != nil {
		_ = s.opts.HostsAdd(domain)
		return nil, status.Errorf(codes.Internal, "delete: %v", err)
	}
	s.opts.Bus.Publish(&api.Event{Kind: &api.Event_RouteRemoved{RouteRemoved: &api.RouteRef{Domain: domain}}})
	return &emptypb.Empty{}, nil
}

func (s *Service) UpdateRoute(_ context.Context, in *api.Route) (*api.Route, error) {
	if in == nil || in.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain required")
	}
	cur, err := s.opts.Store.GetRoute(strings.ToLower(in.Domain))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "no such route: %s", in.Domain)
		}
		return nil, status.Errorf(codes.Internal, "get: %v", err)
	}
	if in.Target != "" {
		cur.Target = in.Target
	}
	cur.Enabled = in.Enabled
	if err := s.opts.Store.PutRoute(cur); err != nil {
		return nil, status.Errorf(codes.Internal, "put: %v", err)
	}
	out := toProto(cur)
	s.opts.Bus.Publish(&api.Event{Kind: &api.Event_RouteUpdated{RouteUpdated: out}})
	return out, nil
}

func (s *Service) ToggleRoute(_ context.Context, in *api.RouteRef) (*api.Route, error) {
	if in == nil || in.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain required")
	}
	cur, err := s.opts.Store.GetRoute(strings.ToLower(in.Domain))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "no such route: %s", in.Domain)
		}
		return nil, status.Errorf(codes.Internal, "get: %v", err)
	}
	cur.Enabled = !cur.Enabled
	if err := s.opts.Store.PutRoute(cur); err != nil {
		return nil, status.Errorf(codes.Internal, "put: %v", err)
	}
	out := toProto(cur)
	s.opts.Bus.Publish(&api.Event{Kind: &api.Event_RouteUpdated{RouteUpdated: out}})
	return out, nil
}

func (s *Service) WatchEvents(in *api.EventFilter, stream api.Mklocal_WatchEventsServer) error {
	ch := s.opts.Bus.Subscribe()
	defer s.opts.Bus.Unsubscribe(ch)
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			if in.GetRoutesOnly() {
				switch ev.Kind.(type) {
				case *api.Event_RouteAdded, *api.Event_RouteRemoved, *api.Event_RouteUpdated:
				default:
					continue
				}
			}
			if err := stream.Send(ev); err != nil {
				return fmt.Errorf("send event: %w", err)
			}
		}
	}
}

func (s *Service) Shutdown(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	// Caller (Daemon) wires the actual shutdown signal; here we just publish.
	s.opts.Bus.Publish(&api.Event{Kind: &api.Event_DaemonStatus{DaemonStatus: &api.DaemonStatus{Version: s.opts.Version}}})
	return &emptypb.Empty{}, nil
}

func toProto(r store.Route) *api.Route {
	return &api.Route{
		Domain:  r.Domain,
		Target:  r.Target,
		Tld:     r.TLD,
		Enabled: r.Enabled,
		Source:  r.Source,
		AddedAt: timestamppb.New(r.AddedAt),
	}
}
```

- [ ] **Step 7.4 — Run tests**

```
go test ./internal/daemon/...
```

Expected: PASS (Add/List/Remove, events, status).

- [ ] **Step 7.5 — Commit**

```
git add internal/daemon/service.go internal/daemon/service_test.go
git commit -m "feat(daemon): gRPC service implementation (routes + events + status)"
```

---

## Task 8: Daemon top-level (wires proxy + service + IPC + log)

**Files:**
- Create: `internal/daemon/daemon.go`
- Create: `internal/daemon/daemon_test.go`

The Daemon owns:
- `slog.Logger` writing to `~/.mklocal/logs/daemon.log`
- The bbolt store
- The cert CA + leaf issuer
- The proxy server (TLS listener + reverse proxy)
- The gRPC server (route ops + events)
- The hosts editor (via `hosts.NewEditor` + the `mklocal hosts-helper` subprocess)

- [ ] **Step 8.1 — Write integration test**

Create `internal/daemon/daemon_test.go`:

```go
package daemon_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	api "github.com/venkatkrishna07/mklocal/internal/api"
	"github.com/venkatkrishna07/mklocal/internal/daemon"
	"github.com/venkatkrishna07/mklocal/internal/ipc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestDaemonAddRouteEndToEnd(t *testing.T) {
	home := t.TempDir()
	sock := filepath.Join(home, "daemon.sock")

	d, err := daemon.New(daemon.Opts{
		Home:         home,
		SocketPath:   sock,
		ProxyEnabled: false, // skip TLS bind in unit test
		HostsAdd:     func(string) error { return nil },
		HostsRm:      func(string) error { return nil },
		Version:      "test",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	go func() { _ = d.Run(ctx) }()

	// Wait until socket appears.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := ipc.Dial(ctx, sock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon never came up")
		}
		time.Sleep(20 * time.Millisecond)
	}

	conn, err := grpc.NewClient("unix:"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	cli := api.NewMklocalClient(conn)

	added, err := cli.AddRoute(ctx, &api.Route{Domain: "foo.local", Target: "localhost:3000"})
	require.NoError(t, err)
	require.Equal(t, "foo.local", added.Domain)

	list, err := cli.ListRoutes(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, list.Routes, 1)

	status, err := cli.Status(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Equal(t, int32(1), status.RouteCount)
}
```

- [ ] **Step 8.2 — Run tests, confirm fail**

```
go test ./internal/daemon/...
```

Expected: build fails.

- [ ] **Step 8.3 — Implement `daemon.go`**

Create `internal/daemon/daemon.go`:

```go
package daemon

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
	"sync"

	api "github.com/venkatkrishna07/mklocal/internal/api"
	"github.com/venkatkrishna07/mklocal/internal/cert"
	"github.com/venkatkrishna07/mklocal/internal/ipc"
	"github.com/venkatkrishna07/mklocal/internal/proxy"
	"github.com/venkatkrishna07/mklocal/internal/store"
)

// Opts configures a Daemon.
type Opts struct {
	Home         string                // e.g. ~/.mklocal
	SocketPath   string                // path to unix socket; if empty, derived from Home
	ProxyPort    int                   // TLS proxy port (e.g. 443 or 8443)
	ProxyEnabled bool                  // false in tests to skip binding
	TLD          string                // e.g. ".local"
	Version      string
	HostsAdd     func(domain string) error
	HostsRm      func(domain string) error
}

// Daemon wires every long-running component.
type Daemon struct {
	opts    Opts
	logger  *slog.Logger
	logShut func() error
	store   *store.Store
	ca      *cert.CA
	issuer  *cert.Issuer
	router  *proxy.Router
	bus     *Bus
	svc     *Service
}

// New constructs (but does not start) a Daemon.
func New(opts Opts) (*Daemon, error) {
	if opts.SocketPath == "" {
		opts.SocketPath = ipc.SocketPath(opts.Home)
	}
	if opts.TLD == "" {
		opts.TLD = ".local"
	}

	logger, closeLog, err := NewFileLogger(filepath.Join(opts.Home, "logs", "daemon.log"))
	if err != nil {
		return nil, err
	}

	s, err := store.Open(filepath.Join(opts.Home, "state.db"))
	if err != nil {
		_ = closeLog()
		return nil, err
	}

	var ca *cert.CA
	var issuer *cert.Issuer
	router := proxy.NewRouter()

	if opts.ProxyEnabled {
		ca, err = cert.LoadCA(filepath.Join(opts.Home, "ca"))
		if err != nil {
			_ = s.Close()
			_ = closeLog()
			return nil, fmt.Errorf("daemon: load CA: %w", err)
		}
		issuer = cert.NewIssuer(ca, router.Has)
	}

	routes, err := s.ListRoutes()
	if err != nil {
		_ = s.Close()
		_ = closeLog()
		return nil, err
	}
	router.Set(routes)

	bus := NewBus()
	svc := NewService(ServiceOpts{
		Store:    s,
		Bus:      bus,
		TLD:      opts.TLD,
		Version:  opts.Version,
		HostsAdd: opts.HostsAdd,
		HostsRm:  opts.HostsRm,
	})

	d := &Daemon{
		opts:    opts,
		logger:  logger,
		logShut: closeLog,
		store:   s,
		ca:      ca,
		issuer:  issuer,
		router:  router,
		bus:     bus,
		svc:     svc,
	}

	// Refresh router on every relevant event so the proxy sees changes.
	go d.routerReloader()

	return d, nil
}

func (d *Daemon) routerReloader() {
	ch := d.bus.Subscribe()
	defer d.bus.Unsubscribe(ch)
	for ev := range ch {
		switch ev.Kind.(type) {
		case *api.Event_RouteAdded, *api.Event_RouteRemoved, *api.Event_RouteUpdated:
			if rs, err := d.store.ListRoutes(); err == nil {
				d.router.Set(rs)
			}
		}
	}
}

// Run starts the gRPC server (and the proxy if ProxyEnabled). It blocks
// until ctx is canceled, then performs a graceful shutdown.
func (d *Daemon) Run(ctx context.Context) error {
	grpcSrv, ln, err := ipc.NewServer(d.opts.SocketPath)
	if err != nil {
		return err
	}
	api.RegisterMklocalServer(grpcSrv, d.svc)

	var wg sync.WaitGroup

	// gRPC server
	wg.Go(func() {
		d.logger.Info("daemon: gRPC listening", "sock", d.opts.SocketPath)
		if err := grpcSrv.Serve(ln); err != nil {
			d.logger.Error("daemon: grpc serve", "err", err)
		}
	})

	// Optional proxy
	var proxySrv *proxy.Server
	if d.opts.ProxyEnabled {
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(d.opts.ProxyPort))
		pln, err := tls.Listen("tcp", addr, &tls.Config{
			GetCertificate: d.issuer.GetCertificate,
			MinVersion:     tls.VersionTLS13,
		})
		if err != nil {
			grpcSrv.GracefulStop()
			return fmt.Errorf("daemon: tls listen: %w", err)
		}
		proxySrv = proxy.NewServer(d.router, pln)
		wg.Go(func() {
			d.logger.Info("daemon: proxy listening", "addr", pln.Addr().String())
			if err := proxySrv.Serve(); err != nil {
				d.logger.Error("daemon: proxy serve", "err", err)
			}
		})
	}

	<-ctx.Done()
	d.logger.Info("daemon: shutting down")
	grpcSrv.GracefulStop()
	if proxySrv != nil {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = proxySrv.Shutdown(shutCtx)
		cancel()
	}
	wg.Wait()
	return nil
}

// Close releases all resources. Safe to call after Run returns.
func (d *Daemon) Close() error {
	_ = d.store.Close()
	return d.logShut()
}
```

Add `"time"` to imports if not already there (it is for `context.WithTimeout`).

- [ ] **Step 8.4 — Run tests**

```
go test ./internal/daemon/...
```

Expected: PASS, including the end-to-end test that adds a route over gRPC.

- [ ] **Step 8.5 — Commit**

```
git add internal/daemon/daemon.go internal/daemon/daemon_test.go
git commit -m "feat(daemon): top-level Daemon wiring proxy + gRPC + log + store"
```

---

## Task 9: CLI `daemon` subcommand

**Files:**
- Modify: `internal/cli/serve.go` → delete
- Create: `internal/cli/daemon.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 9.1 — Delete `serve.go`**

```
rm /Users/venkatkrishnas/Downloads/Projects/Personal/mklocal/internal/cli/serve.go
```

- [ ] **Step 9.2 — Create `internal/cli/daemon.go`**

```go
package cli

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mklocal/internal/config"
	"github.com/venkatkrishna07/mklocal/internal/daemon"
	"github.com/venkatkrishna07/mklocal/internal/hosts"
	"github.com/venkatkrishna07/mklocal/internal/version"
)

func newDaemonCmd() *cobra.Command {
	var foreground bool
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run the mklocal daemon in the foreground (launchd invokes this)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := HomeDir()
			if err != nil {
				return err
			}
			cfg, err := config.Load(filepath.Join(home, "config.toml"))
			if err != nil {
				return err
			}
			binPath, err := os.Executable()
			if err != nil {
				return err
			}
			editor := hosts.NewEditor(binPath)

			d, err := daemon.New(daemon.Opts{
				Home:         home,
				ProxyPort:    cfg.ProxyPort,
				ProxyEnabled: true,
				TLD:          cfg.TLD,
				Version:      version.String(),
				HostsAdd:     editor.Add,
				HostsRm:      editor.Remove,
			})
			if err != nil {
				return err
			}
			defer d.Close()

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			return d.Run(ctx)
		},
	}
	cmd.Flags().BoolVar(&foreground, "foreground", false, "alias for default behavior; kept for symmetry")
	return cmd
}
```

- [ ] **Step 9.3 — Wire into `root.go`**

In `internal/cli/root.go`, replace `newServeCmd()` with `newDaemonCmd()` in the AddCommand block. Also delete the import of `serve.go`'s references — none should remain after the file deletion.

- [ ] **Step 9.4 — Build + smoke**

```
make build
./bin/mklocal daemon --help
```

Expected: usage prints.

Do NOT run `./bin/mklocal daemon` yet — that needs `install` to have created the CA. Tested below in the manual smoke (Task 14).

- [ ] **Step 9.5 — Commit**

```
git rm internal/cli/serve.go
git add internal/cli/daemon.go internal/cli/root.go
git commit -m "feat(cli): replace 'serve' with 'daemon' subcommand"
```

---

## Task 10: CLI client helper + migrate add/remove/list

**Files:**
- Create: `internal/cli/client.go`
- Modify: `internal/cli/add.go`
- Modify: `internal/cli/remove.go`
- Modify: `internal/cli/list.go`

- [ ] **Step 10.1 — Create `internal/cli/client.go`**

```go
package cli

import (
	"context"
	"fmt"

	api "github.com/venkatkrishna07/mklocal/internal/api"
	"github.com/venkatkrishna07/mklocal/internal/ipc"
	"google.golang.org/grpc"
)

// dialDaemon returns a connected gRPC client + close func, or an error
// that the CLI translates to a clear "daemon not running" message.
func dialDaemon(ctx context.Context, home string) (api.MklocalClient, func() error, error) {
	conn, err := ipc.Dial(ctx, ipc.SocketPath(home))
	if err != nil {
		if errors.Is(err, ipc.ErrDaemonNotRunning) {
			return nil, nil, fmt.Errorf("daemon not running. run `mklocal start` first")
		}
		return nil, nil, err
	}
	return api.NewMklocalClient(conn), conn.Close, nil
}

// keep grpc import alive for ToolSearch / linters
var _ = grpc.NewClient
```

Fix import: add `"errors"` to the import block.

- [ ] **Step 10.2 — Rewrite `internal/cli/add.go`**

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	api "github.com/venkatkrishna07/mklocal/internal/api"
)

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <host:port>",
		Short: "Map https://<name>.<tld> to a local upstream",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := HomeDir()
			if err != nil {
				return err
			}
			cli, closeFn, err := dialDaemon(cmd.Context(), home)
			if err != nil {
				return err
			}
			defer closeFn()
			r, err := cli.AddRoute(cmd.Context(), &api.Route{Domain: args[0], Target: args[1]})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added: https://%s → %s\n", r.Domain, r.Target)
			return nil
		},
	}
}
```

- [ ] **Step 10.3 — Rewrite `internal/cli/remove.go`**

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	api "github.com/venkatkrishna07/mklocal/internal/api"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a domain mapping",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := HomeDir()
			if err != nil {
				return err
			}
			cli, closeFn, err := dialDaemon(cmd.Context(), home)
			if err != nil {
				return err
			}
			defer closeFn()
			if _, err := cli.RemoveRoute(cmd.Context(), &api.RouteRef{Domain: args[0]}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed: %s\n", args[0])
			return nil
		},
	}
}
```

- [ ] **Step 10.4 — Rewrite `internal/cli/list.go`**

```go
package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
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
			cli, closeFn, err := dialDaemon(cmd.Context(), home)
			if err != nil {
				return err
			}
			defer closeFn()
			resp, err := cli.ListRoutes(cmd.Context(), &emptypb.Empty{})
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(resp.Routes)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "DOMAIN\tTARGET\tENABLED\tSOURCE")
			for _, r := range resp.Routes {
				fmt.Fprintf(tw, "%s\t%s\t%v\t%s\n", r.Domain, r.Target, r.Enabled, r.Source)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output as JSON")
	return cmd
}
```

- [ ] **Step 10.5 — Build**

```
make build
./bin/mklocal list
```

Expected: prints `daemon not running. run \`mklocal start\` first` (no daemon yet).

- [ ] **Step 10.6 — Run cli tests**

The existing `cli_test.go` tests `TestListWorksOnFreshHome`/`TestListJSONWorksOnFreshHome` exercise the list command without a daemon and previously succeeded against the local store. After this migration they will fail with the new "daemon not running" error. Update them:

In `internal/cli/cli_test.go`, replace those two tests with:

```go
func TestListErrorsWithoutDaemon(t *testing.T) {
	t.Setenv("MKLOCAL_HOME", t.TempDir())
	root := cli.New()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"list"})
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "daemon not running")
}
```

Remove the JSON variant (covered by the same error path).

- [ ] **Step 10.7 — Run all tests**

```
go test -race -count=1 ./...
```

Expected: PASS.

- [ ] **Step 10.8 — Commit**

```
git add internal/cli/client.go internal/cli/add.go internal/cli/remove.go internal/cli/list.go internal/cli/cli_test.go
git commit -m "feat(cli): migrate add/remove/list to gRPC client"
```

---

## Task 11: launchd plist install/uninstall (`internal/daemon/service/darwin.go`)

**Files:**
- Create: `internal/daemon/service/darwin.go`
- Create: `internal/daemon/service/darwin_test.go`

- [ ] **Step 11.1 — Implement `darwin.go`**

```go
//go:build darwin

package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

const (
	label = "dev.mklocal.daemon"
)

// PlistPath returns the location of the LaunchAgent plist for the calling user.
func PlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

// Args holds template inputs for Render.
type Args struct {
	BinPath string
	Home    string
	LogPath string
}

const plistTmpl = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>` + label + `</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{ .BinPath }}</string>
    <string>daemon</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>MKLOCAL_HOME</key>
    <string>{{ .Home }}</string>
  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>{{ .LogPath }}</string>
  <key>StandardErrorPath</key><string>{{ .LogPath }}</string>
  <key>ProcessType</key><string>Background</string>
</dict>
</plist>
`

// Render returns the plist bytes for the given args.
func Render(a Args) ([]byte, error) {
	t, err := template.New("plist").Parse(plistTmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, a); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Install writes the plist and `launchctl bootstrap`s it.
func Install(a Args) error {
	plist, err := Render(a)
	if err != nil {
		return err
	}
	p, err := PlistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, plist, 0o644); err != nil {
		return fmt.Errorf("service: write plist: %w", err)
	}
	uid := os.Getuid()
	cmd := exec.Command("launchctl", "bootstrap", "gui/"+strconv.Itoa(uid), p)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "already bootstrapped") {
			return nil
		}
		return fmt.Errorf("service: bootstrap: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Uninstall stops the agent and removes the plist.
func Uninstall() error {
	p, err := PlistPath()
	if err != nil {
		return err
	}
	uid := os.Getuid()
	cmd := exec.Command("launchctl", "bootout", "gui/"+strconv.Itoa(uid)+"/"+label)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "not loaded") {
		return fmt.Errorf("service: bootout: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsLoaded reports whether launchctl knows about the agent.
func IsLoaded() (bool, error) {
	uid := os.Getuid()
	cmd := exec.Command("launchctl", "print", "gui/"+strconv.Itoa(uid)+"/"+label)
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 113 {
			return false, nil // not loaded
		}
		return false, fmt.Errorf("service: print: %w", err)
	}
	return true, nil
}
```

Add `"errors"` import.

- [ ] **Step 11.2 — Test the template rendering**

Create `internal/daemon/service/darwin_test.go`:

```go
//go:build darwin

package service_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/venkatkrishna07/mklocal/internal/daemon/service"
)

func TestRender(t *testing.T) {
	got, err := service.Render(service.Args{
		BinPath: "/usr/local/bin/mklocal",
		Home:    "/Users/foo/.mklocal",
		LogPath: "/Users/foo/.mklocal/logs/launchd.log",
	})
	require.NoError(t, err)
	s := string(got)
	require.Contains(t, s, "<string>dev.mklocal.daemon</string>")
	require.Contains(t, s, "<string>/usr/local/bin/mklocal</string>")
	require.Contains(t, s, "<string>daemon</string>")
	require.Contains(t, s, "/Users/foo/.mklocal")
	require.True(t, strings.HasPrefix(s, "<?xml"))
}
```

- [ ] **Step 11.3 — Run tests**

```
go test ./internal/daemon/service/...
```

Expected: PASS.

- [ ] **Step 11.4 — Commit**

```
git add internal/daemon/service
git commit -m "feat(daemon/service): macOS launchd plist install/uninstall"
```

---

## Task 12: CLI `start` / `stop` / `status`

**Files:**
- Create: `internal/cli/start.go`
- Create: `internal/cli/stop.go`
- Create: `internal/cli/status.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/install.go`
- Modify: `internal/cli/uninstall.go`

- [ ] **Step 12.1 — `start.go`**

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mklocal/internal/daemon/service"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Load and start the mklocal launchd agent",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := HomeDir()
			if err != nil {
				return err
			}
			binPath, err := os.Executable()
			if err != nil {
				return err
			}
			args := service.Args{
				BinPath: binPath,
				Home:    home,
				LogPath: filepath.Join(home, "logs", "launchd.log"),
			}
			if err := service.Install(args); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "daemon started")
			return nil
		},
	}
}
```

- [ ] **Step 12.2 — `stop.go`**

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mklocal/internal/daemon/service"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop and unload the mklocal launchd agent",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := service.Uninstall(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "daemon stopped")
			return nil
		},
	}
}
```

- [ ] **Step 12.3 — `status.go`**

```go
package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mklocal/internal/daemon/service"
	"google.golang.org/protobuf/types/known/emptypb"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon launchctl + gRPC status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := HomeDir()
			if err != nil {
				return err
			}
			loaded, err := service.IsLoaded()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "launchd:  loaded=%v\n", loaded)

			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
			defer cancel()
			cli, closeFn, err := dialDaemon(ctx, home)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "grpc:     down (%v)\n", err)
				return nil
			}
			defer closeFn()
			st, err := cli.Status(ctx, &emptypb.Empty{})
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "grpc:     unreachable (%v)\n", err)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "grpc:     up\n")
			fmt.Fprintf(cmd.OutOrStdout(), "version:  %s\n", st.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "routes:   %d\n", st.RouteCount)
			return nil
		},
	}
}
```

- [ ] **Step 12.4 — Register in root.go**

In `internal/cli/root.go`, add `newStartCmd()`, `newStopCmd()`, `newStatusCmd()` to the AddCommand block.

- [ ] **Step 12.5 — Update `install.go`**

After the existing trust-install + CA-gen flow, append a call to `service.Install(...)` so `mklocal install` also registers (and starts) the launchd agent. Print "daemon installed and started".

Specifically: at the bottom of `runInstall` (before the final completion `Fprintln`), insert:

```go
binPath, err := os.Executable()
if err != nil {
	return err
}
if err := service.Install(service.Args{
	BinPath: binPath,
	Home:    home,
	LogPath: filepath.Join(home, "logs", "launchd.log"),
}); err != nil {
	return fmt.Errorf("launchd install: %w", err)
}
```

Add the `service` import.

- [ ] **Step 12.6 — Update `uninstall.go`**

Before removing CA from Keychain, call `service.Uninstall()`. Tolerate "not loaded" errors silently (the helper already does).

```go
if err := service.Uninstall(); err != nil {
	slog.Warn("launchd uninstall", "err", err)
}
```

Add the `service` and `log/slog` imports.

- [ ] **Step 12.7 — Build + smoke**

```
make build
./bin/mklocal --help
```

Expected: `start`, `stop`, `status` listed.

- [ ] **Step 12.8 — Commit**

```
git add internal/cli
git commit -m "feat(cli): start/stop/status subcommands and launchd-aware install/uninstall"
```

---

## Task 13: Wire daemon into install/serve auto-start

Already covered in Task 12.5/12.6. This task only adds a doc note in the README's quickstart that `mklocal install` now starts the daemon automatically and that `mklocal start`/`stop` exist.

- [ ] **Step 13.1 — Update README**

Replace the quickstart block in `README.md` with:

```
mklocal install                  # generates CA, trusts in Keychain, starts daemon
mklocal add myapp localhost:3000 # routes https://myapp.local → localhost:3000
curl https://myapp.local         # 200 from your local app (or :<proxy_port> if changed)
```

Drop any reference to `mklocal serve`. Add a "Daemon control" section under Commands listing `start`, `stop`, `status`, `daemon`.

- [ ] **Step 13.2 — Commit**

```
git add README.md
git commit -m "docs: update quickstart for daemon-mode lifecycle"
```

---

## Task 14: End-to-end smoke test (manual)

This is a manual gate. No code.

- [ ] **Step 14.1 — Clean state**

```
./bin/mklocal stop || true
./bin/mklocal uninstall --purge || true
rm -rf ~/.mklocal
```

- [ ] **Step 14.2 — Install**

```
./bin/mklocal install
```

Expect: CA generated, Keychain trust prompt, "daemon started".

- [ ] **Step 14.3 — Confirm daemon is up**

```
./bin/mklocal status
```

Expect: `launchd: loaded=true`, `grpc: up`, `routes: 0`.

- [ ] **Step 14.4 — Lower port if you want to skip sudo on bind**

Edit `~/.mklocal/config.toml` and set `proxy_port = 8443`. Restart daemon:

```
./bin/mklocal stop && ./bin/mklocal start
```

- [ ] **Step 14.5 — Start backend**

```
python3 -m http.server 3000 &
```

- [ ] **Step 14.6 — Add route**

```
./bin/mklocal add foo localhost:3000
./bin/mklocal list
grep foo.local /etc/hosts
```

Expect: route listed, hosts entry present.

- [ ] **Step 14.7 — Curl through proxy**

```
curl -v https://foo.local:8443/
```

Expect: 200.

- [ ] **Step 14.8 — Watch events while modifying routes**

In a second terminal:

```
./bin/mklocal status   # confirm grpc up
```

(Plan 2 does not ship a CLI for `WatchEvents`; the gRPC stream is consumed by Plan 3's TUI. You can verify the stream with `grpcurl -plaintext -unix ~/.mklocal/daemon.sock mklocal.v1.Mklocal/WatchEvents` if you have `grpcurl` installed.)

- [ ] **Step 14.9 — Tear down**

```
./bin/mklocal remove foo
./bin/mklocal uninstall --purge
killall python3 || true
```

Expect: route removed, hosts line gone, Keychain CA removed, `~/.mklocal/` purged, launchd agent unloaded.

Plan 2 is complete when steps 14.1 – 14.9 pass on a clean macOS box.

---

## Self-review notes

- Spec section 5.4 gRPC contract: covered by Tasks 1, 7, 8 (Status, ListRoutes, AddRoute, RemoveRoute, UpdateRoute, ToggleRoute, WatchEvents, Shutdown). `ListProjects`, `LoadProject`, `UnloadProject`, `TailLogs`, `TrustCA`, `UntrustCA`, `CertInfo`, `RunChecks`, `GetSettings`, `UpdateSettings` are deferred to later plans — only the route + status + watch + shutdown subset lands in Plan 2.
- Spec section 5.5 bbolt schema: still single `routes` bucket. `projects`, `stats`, `logs` buckets stay deferred.
- Spec section 4 layout: `internal/daemon`, `internal/daemon/service/`, `internal/ipc`, `api/proto`, `internal/api` all land in Plan 2 as designed. `internal/tui` remains deferred.
- `mklocal serve` removal is documented in Task 9 — README quickstart drops it in Task 13.
- Polling reloader from Plan 1 is replaced by the event-bus-driven `routerReloader` in `internal/daemon/daemon.go` (Task 8).
- TLS 1.3 minimum and `127.0.0.1` bind from the Plan 1 post-audit fixes carry forward into the daemon's proxy listener (Task 8).
- The `hosts-helper` subcommand from Plan 1 is unchanged; the daemon shells out to it the same way Plan 1's CLI did.

## Definition of done

- All gRPC RPCs (Status, ListRoutes, AddRoute, RemoveRoute, UpdateRoute, ToggleRoute, WatchEvents, Shutdown) implemented and tested.
- `mklocal install` registers and starts the launchd agent.
- `mklocal add/remove/list` operate via gRPC; fail clearly if daemon is down.
- `mklocal status` reports launchctl + gRPC state.
- All unit + integration tests pass under `-race`.
- `gofmt -l .` empty, `go vet ./...` clean.
- Manual smoke (Task 14) passes on a fresh `~/.mklocal/`.
- `internal/cli/serve.go` deleted; no caller references it.
- README quickstart no longer mentions `serve`.
