# mkdev

**Local HTTPS for your dev servers.**

> 🚧 **Pre-1.0 (macOS only).** Walking-skeleton release. API and on-disk layout may change.

[![build](https://img.shields.io/badge/build-pending-lightgrey)]() [![tests](https://img.shields.io/badge/tests-pending-lightgrey)]() [![license](https://img.shields.io/badge/license-MIT-blue)](./LICENSE)

---

## Why mkdev

- **Real certs in your Keychain.** No browser warnings, no `--insecure`, no per-project root.
- **No per-app config.** One TOML, one bbolt file. Your app stays untouched.
- **Single static Go binary.** No Node, no Docker, no nginx.conf to copy-paste.
- **No background daemon yet.** Plan 2 will add that; today `serve` runs in the foreground.

## What it does

```
mkdev install                    # generates CA, trusts in Keychain
mkdev add myapp localhost:3000   # routes https://myapp.local → localhost:3000
mkdev serve                      # foreground TLS proxy
curl https://myapp.local           # 200 from your local app
```

That's the whole pitch. Four lines, real HTTPS.

## Install

No prebuilt binaries yet. Build from source:

```sh
git clone https://github.com/venkatkrishna07/mkdev.git
cd mkdev
make build
cp bin/mkdev ~/bin/        # or /usr/local/bin
```

Requires **Go 1.25+** and macOS.

## Platform support

| Platform | Status      |
|----------|-------------|
| macOS    | Supported   |
| Linux    | Roadmap     |
| Windows  | Roadmap     |

## Commands

| Command            | Purpose                                                       |
|--------------------|---------------------------------------------------------------|
| `install`          | Generate the root CA, write defaults, trust in Keychain.      |
| `add <name> <h:p>` | Add route. Appends a `127.0.0.1` entry to `/etc/hosts`.       |
| `remove <name>`    | Remove route and its `/etc/hosts` entry.                      |
| `list`             | List routes in the store.                                     |
| `serve`            | Run the TLS reverse proxy in the foreground.                  |
| `uninstall`        | Untrust the CA. `--purge` also wipes `~/.mkdev/`.           |
| `hosts-helper`     | Hidden. Invoked via `sudo` to mutate `/etc/hosts` atomically. |

`hosts-helper` is not meant to be called directly. `add` / `remove` re-invoke the same binary under `sudo` to perform the privileged `/etc/hosts` write.

## Configuration

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
| `proxy_port`    | `443`       | Set to `8443` to run `serve` without sudo for dev testing.  |
| `theme`         | `auto`      | Reserved for the upcoming TUI.                              |
| `log_retention` | `7d`        | Reserved.                                                   |
| `log_max_size`  | `100MB`     | Reserved.                                                   |

Override the config directory with `--home <path>` or `MKDEV_HOME=...`.

## How it works

- Generates an **ECDSA P-256** root CA at `~/.mkdev/ca/`. The private key is mode `0o400`.
- Installs the CA in the macOS **System Keychain** via `security add-trusted-cert -d -r trustRoot -p ssl -p basic`.
- On `add`, writes a route to a **bbolt** KV at `~/.mkdev/state.db` and appends a `127.0.0.1 <name>.<tld>` line to `/etc/hosts` via a `sudo`-invoked helper subcommand.
- `serve` listens TLS on `127.0.0.1:<proxy_port>`, **mints leaf certs per SNI** on demand using the root CA, and reverse-proxies to the configured upstream.
- The route table is re-read every 2 seconds, so `add` / `remove` take effect without restarting `serve`.
- The proxy binds `127.0.0.1` only. It does **not** listen on `0.0.0.0`.

## Security

This tool installs a **private CA into your system trust store**. Anyone with read access to `~/.mkdev/ca/rootCA-key.pem` can mint TLS certs that your machine will trust. The key is created `0o400` (owner read only).

- The proxy binds `127.0.0.1`, never `0.0.0.0`.
- No telemetry. No remote calls. No update checks.
- `add` / `remove` invoke `sudo` to mutate `/etc/hosts`. See [SECURITY.md](./SECURITY.md) for the threat model and a known limit around `os.Executable()`-resolved helper paths.

## Uninstall

```sh
mkdev uninstall           # untrust CA, remove /etc/hosts entries
mkdev uninstall --purge   # also delete ~/.mkdev/
```

If something gets stuck, open **Keychain Access.app**, search for `mkdev`, and delete by hand. Then `grep mkdev /etc/hosts` and clean any leftovers.

## Comparisons

- **vs. [mkcert](https://github.com/FiloSottile/mkcert):** mkcert does one thing — issuing locally-trusted certs — and does it very well. mkdev also does that, plus `/etc/hosts` and the reverse proxy. If all you want is certs, use mkcert.
- **vs. [Caddy](https://caddyserver.com):** Caddy is a production-grade reverse proxy. You write a `Caddyfile` per project. mkdev is opinionated: one binary, four commands, no config per project.
- **vs. dnsmasq + nginx:** the classic stack. Powerful, configurable, and a half-day of YAML and `brew services`. mkdev trades flexibility for the 4-line quickstart.

mkdev is the **"all three jobs (cert, hosts, proxy) in one binary"** play.

## Roadmap

Internal design docs and phased plans live under [`docs/superpowers/specs/`](./docs/superpowers/specs/) and [`docs/superpowers/plans/`](./docs/superpowers/plans/). They are intentionally not gitignored — useful as context.

Planned, in order:

- **Plan 2** — background daemon with gRPC IPC; `serve` becomes `up`/`down`.
- **Plan 3-4** — TUI (route table, live logs, cert inspection).
- **Plan 5** — Linux (NSS / `update-ca-certificates`) and Windows (CryptoAPI) trust-store support.
- **Plan 6** — signed releases via goreleaser, CI matrix, Homebrew tap.

Nothing on the roadmap is implemented yet. If a feature isn't in the table above, it doesn't exist.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

[MIT](./LICENSE).

## Author

Venkatakrishna S.
