# mkdev

**Real HTTPS for local dev — with a TUI and LAN sharing.**

[![ci](https://github.com/venkatkrishna07/mkdev/actions/workflows/ci.yml/badge.svg)](https://github.com/venkatkrishna07/mkdev/actions/workflows/ci.yml) [![release](https://github.com/venkatkrishna07/mkdev/actions/workflows/release.yml/badge.svg)](https://github.com/venkatkrishna07/mkdev/actions/workflows/release.yml) [![license](https://img.shields.io/badge/license-MIT-blue)](./LICENSE)

---

mkdev runs trusted HTTPS on `*.local`.  A single Go binary: cert authority + reverse proxy + `/etc/hosts` + mDNS broadcast + a full TUI.

What makes it different:

- **LAN sharing** Mark a route shared, hit `https://app.local` from your phone or any device on the same Wi-Fi.
- **TUI, not just a CLI.** Live route table, request logs, cert inspection, health doctor. `mkdev` with no args drops you in.
- **Hardened privilege boundary.** Owner, writability, and symlink checks on the sudo helper binary before any elevated call ([`internal/safeexec`](./internal/safeexec/safeexec.go)). No PATH-based shadowing, no group-writable shortcuts.
- **Per-SNI cert minting.** Leaves are issued on demand and gated by an explicit `knownHost` allow-list. Not wildcard, not pre-baked.

## What it does

```
mkdev install                    # CA + trust + daemon service + menu bar autostart
mkdev add myapp localhost:3000   # routes https://myapp.local → localhost:3000
curl https://myapp.local         # 200 from your local app
```

`install` is one-shot: generates the CA, trusts it in the system store, installs and enables the daemon user-service (launchd / systemd), registers the menu bar to launch on login, and spawns the bar immediately when a GUI session is present. The daemon owns the proxy; the TUI (`mkdev` no args) and `mkdev add | remove | list` talk to it over `~/.mkdev/daemon.sock`.

![mkdev demo](assets/demo-add.gif)

## LAN sharing

mkdev's headline feature. Share a route to any device on the same Wi-Fi with real TLS — no warnings, no tunnel service.

1. In the TUI Domains tab, select a route and press `s` to flip the **SHARE** column to `LAN`.
2. The route is advertised via mDNS as `<name>.local` → this machine's LAN IP.
3. On the phone / second laptop, browse to `https://<name>.local`. Once the device trusts the mkdev CA (one-time), no warnings.

### Caveats

- Only `.local` routes broadcast over mDNS. Other TLDs still proxy but aren't LAN-reachable by name.
- Corporate / cloud Wi-Fi often blocks multicast. Home and office Wi-Fi work.
- Toggling `s` is live — mDNS advertising and the LAN-side ACL update on the next request. No restart.
- Non-shared routes 403 non-loopback requests as defense-in-depth.
- Anyone on the LAN can hit your shared routes. Don't enable on untrusted Wi-Fi.

## Install

### Homebrew (macOS, Linux)

```sh
brew install venkatkrishna07/tap/mkdev
```

Upgrade later:

```sh
brew update
brew upgrade mkdev
```

### Go

```sh
go install github.com/venkatkrishna07/mkdev/cmd/mkdev@latest
```

Upgrade to a specific version:

```sh
go install github.com/venkatkrishna07/mkdev/cmd/mkdev@v0.2.0
```

### Direct download

Pre-built binaries for macOS (Intel + Apple Silicon), Linux (amd64 + arm64), and Windows (amd64) are published on the [Releases page](https://github.com/venkatkrishna07/mkdev/releases). Each release includes `checksums.txt` plus a cosign keyless signature (`checksums.txt.sig` + `.pem`) — see [SECURITY.md#verifying-releases](./SECURITY.md) for the verify command.

On macOS, if Gatekeeper blocks a direct-download binary:

```sh
xattr -d com.apple.quarantine ./mkdev
```

### From source

```sh
git clone https://github.com/venkatkrishna07/mkdev.git
cd mkdev
task build
cp bin/mkdev ~/bin/        # or /usr/local/bin
```

Requires **Go 1.25+**.

## First run

```sh
mkdev install   # CA, trust, daemon service, bar autostart — one command
mkdev           # launch TUI
```

After `install`, the daemon runs in the background and the menu bar appears (macOS, Linux GNOME/KDE). The bar shows daemon status, route list, and per-route enable / LAN-share toggles; `Quit` exits the bar without stopping the daemon.

## Upgrading

Replace the binary (brew upgrade / `go install ...@latest` / new download). The next time you run any `mkdev` subcommand, the binary reconciles the parts that live outside it: rewrites the daemon plist / systemd unit and bar autostart entry to point at the new binary, re-asserts `/etc/hosts` for enabled routes, and re-trusts the CA if it was dropped. Sudo prompts run inline. If you only have the daemon running and never touch the CLI, the daemon does the safe (no-sudo) bits on its own startup and queues the rest until the next CLI command.

Re-running `mkdev install` does the same thing explicitly and is always safe — every step is idempotent.

## Platform support

| Platform | Trust store                                                       | Elevation       |
|----------|-------------------------------------------------------------------|-----------------|
| macOS    | System Keychain (`security add-trusted-cert`)                     | `sudo` / `osascript` |
| Linux    | `update-ca-trust` / `update-ca-certificates` / `trust extract-compat` | `sudo` / `pkexec` |
| Windows  | `ROOT` system store via `crypt32.dll`                             | UAC (PowerShell `RunAs`) |

Linux distros detected: Debian/Ubuntu (`/usr/local/share/ca-certificates`), RHEL/Fedora (`/etc/pki/ca-trust/source/anchors`), Arch (`/etc/ca-certificates/trust-source/anchors`), openSUSE (`/usr/share/pki/trust/anchors`).

Firefox uses its own NSS store and is **not yet covered** — system Chrome/Safari/Edge/curl/wget all work.

## Commands

| Command                              | Purpose                                                       |
|--------------------------------------|---------------------------------------------------------------|
| `install`                            | One-shot: CA + trust + daemon service + bar autostart. Pass `--no-service` for CA-only. |
| `add <name> <target>`                | Add route. Appends a `127.0.0.1` entry to `/etc/hosts`.       |
| `remove <name>`                      | Remove route and its `/etc/hosts` entry.                      |
| `list`                               | List routes in the store.                                     |
| `tui`                                | Launch the TUI (also the default when run with no args).      |
| `daemon serve`                       | Run the daemon in the foreground (normally driven by the service unit). |
| `daemon stop`                        | Stop the running daemon and disable the user-service.         |
| `daemon status`                      | Report installed / enabled / running state of the service.    |
| `bar`                                | Launch the menu bar app. Autostarted by `install`.            |
| `uninstall`                          | Untrust CA, remove `/etc/hosts` entries, uninstall daemon + bar autostart, wipe `~/.mkdev/`. |
| `version`                            | Print version, commit, build date.                            |
| `completion <bash\|zsh\|fish\|powershell>` | Emit shell completion script.                                 |
| `hosts-helper`                       | Hidden. Invoked via `sudo` to mutate `/etc/hosts` atomically. |

### Flags and environment

| Flag / env                    | Effect                                                       |
|-------------------------------|--------------------------------------------------------------|
| `--home <path>`               | Override `~/.mkdev` state directory.                         |
| `--verbose`, `-v`             | Debug-level logging to stderr.                               |
| `--version`                   | Print version and exit.                                      |
| `MKDEV_HOME=<path>`           | Equivalent to `--home`.                                      |

### Target formats

`<target>` accepts any of:

```
host:port                  e.g. localhost:3000
http://host[:port]/path    e.g. http://localhost:3000/api
https://host[:port]/path   e.g. https://gitlab.example.com
```

For HTTPS upstreams (e.g., a private GitLab on a corporate VPN) the upstream's TLS cert must verify against the system trust store. Private CAs need their root added to the OS keychain.

`hosts-helper` is not meant to be called directly. `add` / `remove` re-invoke the same binary under `sudo` to perform the privileged `/etc/hosts` write.

## Configuration

> **TLD note.** `.local` routes need an mDNS responder (always-on on macOS, available on Linux when `nss-mdns` is installed). `.test` / `.dev` / `.localhost` work everywhere via `/etc/hosts` alone. Set `tld` in config to match.

Config lives at `~/.mkdev/config.toml`. Defaults:

```toml
tld           = ".local"   # appended to bare names in `add`
proxy_port    = 443        # binding :443 requires sudo on serve
theme         = "auto"     # reserved for future TUI
log_retention = "7d"       # reserved
log_max_size  = "100MB"    # reserved
```

| Field           | Default     | Notes                                                       |
|-----------------|-------------|-------------------------------------------------------------|
| `tld`           | `.local`    | Auto-appended when `add <name>` has no dot.                 |
| `proxy_port`    | `443`       | Set to `8443` if your platform won't let the user-service bind :443. |
| `theme`         | `auto`      | Reserved for the upcoming TUI.                              |
| `log_retention` | `7d`        | Reserved.                                                   |
| `log_max_size`  | `100MB`     | Reserved.                                                   |

Override the config directory with `--home <path>` or `MKDEV_HOME=...`.

## How it works

- Generates an **ECDSA P-256** root CA at `~/.mkdev/ca/`. The private key is mode `0o400`.
- Installs the CA in the OS-native trust store: macOS Keychain (`security`), Linux CA-bundle directory + `update-ca-*`, Windows `ROOT` store via `crypt32.dll`. Trust-store integration is adapted from [mkcert](https://github.com/FiloSottile/mkcert) (BSD-3) — see [`LICENSE-MKCERT`](./LICENSE-MKCERT).
- On `add`, the daemon writes a route to a **bbolt** KV at `~/.mkdev/state.db` and appends a `127.0.0.1 <name>.<tld>` line to `/etc/hosts` via a `sudo`-invoked helper subcommand.
- The daemon listens TLS on `0.0.0.0:<proxy_port>`, **mints leaf certs per SNI** on demand using the root CA, and reverse-proxies to the configured upstream.
- The proxy's route table reloads in-process on every `add` / `remove`, no restart.
- The proxy binds `0.0.0.0`, but non-loopback requests are 403'd unless the matching route is marked **shared** — see [LAN sharing](#lan-sharing).
- Cross-host upstream redirects (e.g. a GitLab that 301s to its public hostname) have their `Location` and `Set-Cookie Domain` rewritten back to the proxy domain so clients don't drop credentials.

## Security

This tool installs a **private CA into your system trust store**. Anyone with read access to `~/.mkdev/ca/rootCA-key.pem` can mint TLS certs that your machine will trust. The key is created `0o400` (owner read only).

- The proxy binds `0.0.0.0`, but a connection-source ACL 403s LAN requests to any route not explicitly marked **shared**. Loopback always passes.
- No telemetry. No remote calls. No update checks.
- `add` / `remove` invoke `sudo` to mutate `/etc/hosts`. See [SECURITY.md](./SECURITY.md) for the threat model and a known limit around `os.Executable()`-resolved helper paths.

## Uninstall

```sh
mkdev uninstall   # stops + disables daemon, untrusts CA, clears /etc/hosts,
                  # removes daemon service + bar autostart, wipes ~/.mkdev/
```

If something gets stuck, open **Keychain Access.app**, search for `mkdev`, and delete by hand. Then `grep mkdev /etc/hosts` and clean any leftovers.


## Daemon and API

The daemon owns the proxy and the route store. It exposes a local HTTP API on `~/.mkdev/daemon.sock` (owner-only `0600`). The TUI, CLI subcommands, and menu bar are all clients of the daemon — only the daemon holds the bbolt lock.

`mkdev install` installs the daemon as a user-service (launchd `LaunchAgent` on macOS, systemd `--user` on Linux) and enables it. It runs in the background and survives logout / login.

```sh
mkdev daemon status                # installed / enabled / running
mkdev daemon stop                  # stop + disable (prevents respawn)
curl --unix-socket ~/.mkdev/daemon.sock http://x/v1/status
curl --unix-socket ~/.mkdev/daemon.sock http://x/v1/routes
curl --unix-socket ~/.mkdev/daemon.sock -X POST http://x/v1/routes \
     -H 'Content-Type: application/json' \
     -d '{"name":"foo","target":"localhost:3000"}'
curl --unix-socket ~/.mkdev/daemon.sock -X DELETE http://x/v1/routes/foo
```

## Roadmap

Next:

- Project config file (`.mkdev.yaml` checked into the repo).
- Firefox / NSS trust store integration.
- Per-path routing (`/api` → 8080, `/ws` → 9000 on a single domain).
- Windows menu bar app (today macOS + Linux only).

## Troubleshooting

- **Firefox shows a red bar.** Firefox uses its own NSS store; system trust doesn't reach it. NSS integration is on the roadmap. For now, import `~/.mkdev/ca/rootCA.pem` manually under Settings → Privacy & Security → Certificates → View Certificates → Authorities → Import.
- **Daemon fails to bind :443.** macOS/Linux let non-root user-services bind :443 via launchd / systemd capabilities. If your distro restricts this, set `proxy_port = 8443` in `~/.mkdev/config.toml` and use `https://name.local:8443`.
- **`mkdev add` keeps asking for sudo.** Sudo's per-session cache expires (default 5 min). Use the TUI Domains tab instead — it elevates via `osascript` (macOS GUI prompt) or `pkexec` (Linux Polkit).
- **`/etc/hosts` already has an entry for that name.** `mkdev add` is idempotent and only appends when no `mkdev`-managed entry exists. Remove the prior entry by hand or pick a different name.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md).

## Acknowledgements

mkdev's trust-store integration — Keychain/`security` on macOS, CA-bundle + `update-ca-*` on Linux, `crypt32.dll` `ROOT` store on Windows, and the NSS adjacent paths — is adapted from [**mkcert**](https://github.com/FiloSottile/mkcert) by Filippo Valsorda, BSD-3. Without that prior art, this project would be substantially harder. See [`LICENSE-MKCERT`](./LICENSE-MKCERT) for the upstream license.

The TUI is built with [Charmbracelet's](https://charm.sh) Bubble Tea / Bubbles / Lipgloss. mDNS via [`hashicorp/mdns`](https://github.com/hashicorp/mdns). Local KV via [`bbolt`](https://github.com/etcd-io/bbolt).

## License

MIT   [LICENSE](./LICENSE).
