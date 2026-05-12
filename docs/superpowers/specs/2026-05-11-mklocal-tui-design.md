# mklocal — Local HTTPS Dev Proxy with TUI

**Date:** 2026-05-11
**Status:** Approved (brainstorm)
**Owner:** venkatkrishna07

## 1. Summary

`mklocal` is a developer tool that lets you reach local dev servers via real HTTPS URLs (e.g. `https://myapp.local` → `http://localhost:3000`). It is delivered as a **single Go binary** with three modes:

1. **TUI** (default) — a full-screen, k9s-style interactive interface for managing domains, projects, logs, diagnostics, and settings.
2. **CLI** — classic subcommands (`add`, `remove`, `list`, `start`, `stop`, `install`) for scripts and automation.
3. **Daemon** (`mklocal daemon`) — long-running reverse proxy that owns `:443`, terminates TLS, and forwards to local upstreams.

The tool ships for **macOS, Linux, and Windows** via GitHub Releases. Its differentiator versus `mkcert` + manual `nginx` is the polished TUI and zero-friction lifecycle (install once, manage entirely from the TUI).

## 2. Goals and non-goals

### Goals (v1)

- Five-tab TUI: Domains, Projects, Logs, Doctor, Settings
- Add/remove/edit/toggle domain → upstream mappings
- Per-project `mklocal.toml` for declarative routes
- Auto-issued, locally-trusted TLS certs (vendored mkcert)
- Live request counters and access logs streamed from daemon to TUI
- User-level service install (launchd / systemd user / Windows service)
- Cross-platform single binary distributed via GitHub Releases

### Non-goals (v1, deferred)

- Wildcard DNS / `*.test` resolution (needs embedded DNS — v1.1)
- Per-route middleware (auth, rate limit, header rewrites)
- HTTP/3, IPv6 binding
- Remote daemon (always loopback)
- Multi-user shared daemon
- Public tunneling / ngrok-style sharing

## 3. Locked technical decisions

| Area | Decision | Rationale |
|------|----------|-----------|
| Language | Go (latest stable, ≥ 1.25) | stdlib networking, TUI ecosystem (Charm), cross-compile via `goreleaser` |
| Module path | `github.com/venkatkrishna07/mklocal` | user GH handle |
| Binary name | `mklocal` | single multi-mode binary |
| Layout | Go std module layout | matches Go community standard |
| Daemon model | User-level service (launchd / systemd user / Windows service) | dev-bind pattern, avoids root daemon |
| Privilege elevation | One-time `sudo`/Administrator at `mklocal install`; on-demand helper subprocess for `/etc/hosts` writes | minimum-privilege; UAC/sudo prompt only when needed |
| Platforms | macOS + Linux + Windows | full cross-platform |
| Cert backend | Vendored mkcert (MIT) in `internal/cert/` | single-binary distribution requirement |
| Distribution | GitHub Releases via `goreleaser` | matches "ship via GH releases, single binary" |
| DNS strategy | `/etc/hosts` entries everywhere (v1); embedded DNS + `/etc/resolver/` deferred to v1.1 | simplest cross-platform start |
| Config format | TOML — global `~/.mklocal/config.toml`, per-project `mklocal.toml` | git-committable, human-friendly |
| State storage | bbolt (`go.etcd.io/bbolt`) at `~/.mklocal/state.db` | pure Go KV, single file, ACID, fits routes + counters |
| IPC | gRPC over Unix socket (mac/linux) / Windows named pipe | typed contract, native server streaming for live events |
| TUI stack | Bubble Tea + Lipgloss + Bubbles (Charm) | modern, animated, best-in-class Go TUI ecosystem |
| CLI framework | `spf13/cobra` | de-facto standard (kubectl, gh, hugo) |
| Logging | `log/slog` stdlib (JSON handler) | structured, zero deps |
| Reverse proxy | `net/http/httputil.ReverseProxy` | stdlib, battle-tested |
| TOML parser | `github.com/BurntSushi/toml` | de-facto Go TOML |
| License | MIT | matches mkcert, dev-bind, Charm |

## 4. Architecture

### 4.1 Project layout

```
mklocal/
├── cmd/
│   └── mklocal/main.go              # cobra root, dispatches subcommands
├── internal/
│   ├── app/                         # wires components, lifecycle
│   ├── cli/                         # cobra handlers: add, remove, list, install, start, stop, daemon, hosts-helper
│   ├── tui/
│   │   ├── program.go               # bubble tea bootstrap, root Model
│   │   ├── tabs/                    # domains, projects, logs, doctor, settings
│   │   ├── modals/                  # add, confirm, help
│   │   ├── components/              # tabbar, footer, statuspill, table
│   │   └── styles/                  # lipgloss themes
│   ├── daemon/
│   │   ├── server.go                # gRPC server, request lifecycle
│   │   ├── service/                 # launchd plist / systemd unit / winsvc install
│   │   └── events.go                # in-proc event bus → grpc subscribers
│   ├── proxy/
│   │   ├── server.go                # tls.Listener on :443, SNI dispatch
│   │   ├── router.go                # domain → upstream lookup
│   │   └── metrics.go               # per-route counters
│   ├── cert/
│   │   ├── ca.go                    # CA gen + load
│   │   ├── issuer.go                # leaf cert per domain, cached
│   │   └── trust/                   # OS-specific trust store install
│   │       ├── darwin.go            # Keychain
│   │       ├── linux.go             # update-ca-trust / nss
│   │       └── windows.go           # certutil
│   ├── hosts/                       # /etc/hosts editor, cross-platform
│   ├── store/                       # bbolt buckets: routes, settings, stats, logs
│   ├── config/                      # TOML load/save, defaults
│   ├── ipc/
│   │   ├── client.go
│   │   ├── server.go
│   │   └── transport/
│   │       ├── unix_unix.go         # //go:build !windows
│   │       └── pipe_windows.go      # //go:build windows
│   ├── doctor/                      # diagnostic checks
│   └── version/                     # ldflags-injected build info
├── api/proto/mklocal.proto          # gRPC service contract
├── docs/superpowers/specs/
├── scripts/proto-gen.sh
├── .goreleaser.yaml
├── go.mod
├── LICENSE
├── README.md
└── CLAUDE.md
```

### 4.2 Package responsibilities and dependency direction

- `cmd/mklocal` — entrypoint only, no logic; calls `internal/cli`.
- `internal/cli` — cobra subcommand handlers. Depends on `app`, `daemon` (for `daemon` subcommand only), `ipc` (client for everything else).
- `internal/tui` — Bubble Tea program. Depends on `ipc` (client). Never imports `proxy`, `cert`, or `hosts`.
- `internal/daemon` — composes `proxy`, `cert`, `hosts`, `store`, `ipc/server`. Owns full lifecycle.
- `internal/proxy`, `internal/cert`, `internal/hosts`, `internal/store`, `internal/config`, `internal/doctor` — leaf packages, no inter-dependencies except via interfaces.
- `internal/ipc` — transport abstraction over Unix socket / named pipe + generated gRPC stubs from `api/proto`.

Dependency direction is strictly one-way: `cli`/`tui` → `ipc` (client) → gRPC → `ipc` (server) → `daemon` → leaf packages. No cycles.

## 5. Data flow

### 5.1 Request path (browser → upstream)

```
Browser → https://myapp.local
  │ /etc/hosts resolves to 127.0.0.1
  ▼
TLS handshake on :443 (SNI = "myapp.local")
  ▼
proxy.Server.ServeHTTP
  ├─ cert.Issuer.GetCert(sni) → cached leaf cert (or mint + cache)
  ├─ router.Lookup(host) → upstream "localhost:3000"
  ├─ httputil.ReverseProxy.ServeHTTP → forwards request
  ├─ metrics.Record(domain, status, latency)
  └─ events.Publish(RequestServed{...}) → gRPC subscribers
Upstream localhost:3000 → response → browser
```

### 5.2 Control path (TUI Add modal → daemon → state change)

```
TUI Add modal submit
  │ grpc AddRoute{domain, target, tld}
  ▼
daemon.Server.AddRoute
  ├─ validate (domain syntax, target reachable, no duplicate)
  ├─ store.Routes.Put(route)                           [bbolt]
  ├─ hosts.Add("127.0.0.1 myapp.local")                [via sudo helper]
  ├─ cert.Issuer.Preheat(domain)                       [mint cert now]
  ├─ router.Reload()                                   [hot swap routing table]
  └─ events.Publish(RouteAdded{...})
TUI receives RouteAdded over WatchEvents stream → table refreshes
```

### 5.3 Elevation model

- **`mklocal install`** — runs once with `sudo` / Administrator. Steps:
  1. Install user service (launchd plist / systemd user unit / Windows service).
  2. Grant `CAP_NET_BIND_SERVICE` on the binary (Linux only) — lets daemon bind `:443` as user.
  3. Generate CA, install in OS trust store, plus Firefox NSS / Java if present.
  4. Pre-create `~/.mklocal/` with `0700` perms.
- **Daemon runs as the user** — cannot edit `/etc/hosts` directly.
- **Hosts edits** are proxied via a small helper subprocess `mklocal hosts-helper add|remove <line>` invoked with `sudo`/UAC on demand. `sudo` caches credentials, so users only authenticate once per session.

### 5.4 gRPC contract

```proto
service Mklocal {
  // Routes
  rpc ListRoutes(Empty) returns (RouteList);
  rpc AddRoute(Route) returns (Route);
  rpc RemoveRoute(RouteRef) returns (Empty);
  rpc UpdateRoute(Route) returns (Route);
  rpc ToggleRoute(RouteRef) returns (Route);

  // Projects
  rpc ListProjects(Empty) returns (ProjectList);
  rpc LoadProject(ProjectPath) returns (Project);
  rpc UnloadProject(ProjectPath) returns (Empty);

  // Daemon lifecycle
  rpc Status(Empty) returns (DaemonStatus);
  rpc Reload(Empty) returns (Empty);
  rpc Shutdown(Empty) returns (Empty);

  // Streams
  rpc WatchEvents(EventFilter) returns (stream Event);
  rpc TailLogs(LogFilter) returns (stream LogLine);

  // Cert + doctor + settings
  rpc TrustCA(Empty) returns (Empty);
  rpc UntrustCA(Empty) returns (Empty);
  rpc CertInfo(RouteRef) returns (Cert);
  rpc RunChecks(Empty) returns (CheckResult);
  rpc GetSettings(Empty) returns (Settings);
  rpc UpdateSettings(Settings) returns (Settings);
}

message Event {
  oneof kind {
    RouteAdded     route_added    = 1;
    RouteRemoved   route_removed  = 2;
    RouteUpdated   route_updated  = 3;
    RequestServed  request_served = 4;
    UpstreamUp     upstream_up    = 5;
    UpstreamDown   upstream_down  = 6;
    DaemonStatus   daemon_status  = 7;
  }
}
```

### 5.5 bbolt schema

| Bucket | Key | Value (proto-encoded) |
|--------|-----|-----------------------|
| `routes` | domain | `Route{target, tld, enabled, source, added_at}` |
| `projects` | abs_path | `Project{path, services[]}` |
| `settings` | `"global"` | `Settings{...}` |
| `stats` | domain | `Stats{requests, last_hit_at, last_status}` |
| `logs` | ULID | `LogLine{ts, domain, method, path, status, latency_ms, err}` (capped ring, ~10k entries) |

## 6. TUI architecture

### 6.1 Root model

```go
type Model struct {
    width, height int
    theme         styles.Theme

    activeTab Tab
    tabs      [5]tea.Model

    modals    []tea.Model        // LIFO stack; top captures input
    showHelp  bool

    daemon   *ipc.Client          // grpc client, auto-reconnect
    daemonUp bool
}
```

### 6.2 Update routing rules

1. `tea.WindowSizeMsg` propagates to active tab + all modals.
2. Global keys (`q`, `?`, `1`–`5`, `Tab`, `Shift+Tab`, `Ctrl+R`, `Ctrl+C`) handled at root.
3. If a modal is open, the top modal owns input; `Esc` closes it.
4. Otherwise input is forwarded to the active tab.
5. `events.Event` from the daemon stream is broadcast to all tabs; each tab decides whether to consume it.

### 6.3 View composition

```
Top bar:    [ mklocal ]                                  [ ● daemon running ]
Tab bar:    [ Domains ]  Projects  Logs  Doctor  Settings
Body:       active tab .View()
Footer:     context-aware keybind hints from activeTab.Keybinds()
Overlay:    top modal centered via lipgloss.Place, base layer dimmed
```

### 6.4 Per-tab model contract

```go
type Tab interface {
    tea.Model
    Title() string
    Keybinds() []Keybind
    HandleEvent(events.Event) tea.Cmd
}
```

### 6.5 Daemon event subscription

```go
func subscribeEvents(c *ipc.Client) tea.Cmd {
    return func() tea.Msg {
        ev := <-c.Events()
        return DaemonEventMsg{Event: ev}
    }
}
// Re-issued from Update after each event → continuous stream
```

### 6.6 Reconnect behavior

`ipc.Client` runs a reconnect goroutine. On disconnect it emits `DaemonStatusChanged{Up: false}`. TUI shows a banner: `[!] Daemon stopped — press 's' to start`. Pressing `s` calls service-start API; if the service is not installed, the TUI walks the user through `mklocal install` interactively.

### 6.7 Layout breakpoints

| Width | Behavior |
|-------|----------|
| ≥ 100 | Full 5-tab bar, detail panel to the right of the table |
| 80–99 | Full tabs, detail panel below the table |
| < 80  | Compact tab labels (icons only), detail panel hidden |
| < 60  | Render warning: "terminal too narrow — resize to at least 60 columns" |

### 6.8 Theming

`styles.Theme` is a struct of lipgloss styles. Four themes ship: `auto` (detect terminal background), `dark`, `light`, `mono`. Switching from the Settings tab applies live without restart.

## 7. Tabs in detail

### 7.1 Domains (default)

Table columns: `Domain`, `Target`, `Status`, `Source`. Detail panel below shows cert path, added timestamp, last hit, request count.

Keys: `a` add · `e` edit · `d` delete · `t` toggle · `r` reload · `enter` open in browser · `c` copy URL · `/` filter.

### 7.2 Projects

Lists every directory whose `mklocal.toml` has been loaded. Selecting a project shows the parsed TOML inline.

Keys: `u` up project · `D` down project · `o` open folder · `r` reload from disk.

### 7.3 Logs

Live-tails daemon + per-domain access logs via `TailLogs` gRPC stream. Filter dropdown (per-domain, level), pause/resume.

Keys: `space` pause/resume · `/` filter · `c` clear · `s` save to file.

### 7.4 Doctor

Re-runnable diagnostic checks: mkcert installed, CA in trust store, Firefox NSS, `:443` available, hosts file writable, daemon log size, config dir perms.

Keys: `r` re-run · `L` rotate log · `enter` show check details.

### 7.5 Settings

Form-style editor for `~/.mklocal/config.toml`. Live-applies non-restart-required settings; flags ones that need daemon restart.

Keys: `Tab` field nav · `Enter` cycle option · `s` save · `Esc` cancel.

## 8. Error handling

### 8.1 Philosophy

- **Daemon** — log structured error, return `status.Error` with appropriate gRPC code; never crash on per-request failures.
- **TUI** — surface error inline in detail panel + toast at footer; never swallow.
- **CLI** — print to stderr, exit non-zero. Exit codes: `1` usage, `2` daemon down, `3` operation failed, `4` permission.
- **Validation** only at boundaries (user input, gRPC ingress, hosts file lines). No paranoid internal asserts.

### 8.2 Critical edge cases

| Case | Behavior |
|------|----------|
| `:443` already bound by another process | Daemon refuses start; doctor identifies the process via `lsof -i:443` (mac/linux) or `netstat -ano` (win) |
| Hosts entry exists with wrong IP | `install` warns and offers overwrite (with confirm) |
| Daemon binary missing during service start | Service auto-disabled; TUI banner offers reinstall |
| Upstream returns 5xx repeatedly | Marked `UpstreamDown`, event pushed, row dimmed in TUI |
| Cert expired (mkcert default 825 days) | Auto-reissue at 30 days remaining; logged |
| Concurrent `add` from CLI + TUI | bbolt single-writer + gRPC handler mutex serializes |
| User runs second daemon instance | Socket bind conflict → exit with "daemon already running" |
| Trust store wiped (e.g. mac Keychain reset) | Doctor detects via cert verify; prompts re-trust |
| Terminal resize while modal open | Modal recenters; no layout break |
| TUI launched without daemon installed | Interactive walkthrough invokes `mklocal install` |
| Project `mklocal.toml` malformed | Project tab shows error row, does not load |
| Daemon log > 100 MB | Auto-rotate to `.1`, keep last 3 rotations, GC older |

## 9. Testing strategy

| Layer | Tooling | Coverage target |
|-------|---------|-----------------|
| Unit | stdlib `testing` + `testify/require` | `store`, `config`, `router`, `hosts`, `cert`, `doctor` ≥ 80 % |
| Integration | Spawn real daemon on temp socket + bbolt in temp dir | Full gRPC contract, route lifecycle, event delivery |
| TUI | `tea.NewProgram(WithInput(strings.NewReader(...)))` driven by golden-file snapshots | Every tab + every modal happy path |
| End-to-end | GitHub Actions matrix on mac + linux + win runners | install → add → `curl https://test.local` → remove |
| Lint | `golangci-lint` (`gofmt`, `govet`, `staticcheck`, `errcheck`, `gosec`) | zero warnings |

## 10. Build, release, distribution

### 10.1 Build matrix (goreleaser)

```
darwin/amd64, darwin/arm64
linux/amd64, linux/arm64
windows/amd64, windows/arm64
```

Six artifacts. `.tar.gz` for unix, `.zip` for windows. SHA-256 checksums and sigstore (cosign) signatures published alongside.

### 10.2 GitHub Actions pipeline

1. Lint (`golangci-lint`)
2. Test (matrix per OS)
3. Build (matrix per OS)
4. On tag `v*` → `goreleaser release` → GitHub Release with binaries + checksums + signatures

### 10.3 Install UX (end-user)

```
# Download tarball from GitHub Release, extract
$ mklocal install        # interactive: sudo prompt once, registers service, trusts CA
$ mklocal                # launches TUI
```

## 11. Observability and security

- Daemon writes structured logs to `~/.mklocal/logs/daemon.log` via `slog` JSON handler (persistence, offline grep, post-mortem).
- For the live Logs tab the daemon also publishes log records to in-memory subscribers; the TUI consumes them via the `TailLogs` gRPC server stream (section 5.4). The file and the stream emit the same records — file is the canonical store, stream is the realtime fan-out.
- `mklocal doctor` checks log size, rotates if needed.
- **No telemetry, no analytics, no remote calls.** Strictly local.
- gRPC server listens on Unix socket with `0600` perms (owner-only).
- `hosts-helper` validates input strictly (regex-locked to `IP DOMAIN` lines) before any `/etc/hosts` write.
- Vendored mkcert preserves Filo's CA-on-disk perms (`rootCA-key.pem` → `0400`).
- No external network ingress beyond the `:443` reverse proxy.

## 12. Out of scope for v1 (revisit for v1.1+)

- Embedded DNS server + `/etc/resolver/<tld>` for wildcard `*.test`
- Per-route middleware (auth, headers, rewrites)
- HTTP/3, IPv6 binding
- `mklocal run <name> <cmd...>` ephemeral mode (à la dev-bind)
- Remote / multi-user daemon
- Public tunneling (ngrok-equivalent)

## 13. Open questions

None at brainstorm close. Implementation plan (next phase) may surface platform-specific subtleties (Windows named-pipe permissions, Firefox NSS on Snap Ubuntu, Keychain prompt UX) that we'll handle as they arise.
