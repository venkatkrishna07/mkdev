# Security Policy

## Threat Model

mkdev installs a private root CA into your system trust store. The CA private key lives at `~/.mkdev/ca/rootCA-key.pem` (mode `0o400`, owner-read only). Anyone who obtains that key can mint TLS certificates your machine trusts. Treat it like an SSH private key.

### Known limitations

- The sudo-invoked `hosts-helper` binary path is resolved via `os.Executable()` and additionally validated by `internal/safeexec` (owner check, group/other-writable check, symlink resolution). Do not place `mkdev` in a directory whose parent is group/other-writable.
- The TLS proxy binds `0.0.0.0:<proxy_port>`. Routes not marked **shared** are ACL-rejected for non-loopback connections via `r.RemoteAddr` (no `X-Forwarded-For` trust). The port itself is reachable by any local process.
- mDNS responses on the LAN can be spoofed by any peer. mkdev's CA-bound TLS still protects confidentiality — an attacker who redirects `mkdev.local` cannot mint a cert your machine trusts — but they can cause TLS handshake failure (DoS).

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.x (pre-1.0) | Yes — latest tagged release |

Pre-1.0 carries no API or on-disk stability guarantees.

## Verifying Releases

Each tagged release publishes `checksums.txt` plus a cosign keyless signature (`checksums.txt.sig`) and certificate (`checksums.txt.pem`) bound to the GitHub Actions workflow OIDC identity.

```sh
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature checksums.txt.sig \
  --certificate-identity-regexp 'https://github.com/venkatkrishna07/mkdev/.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
sha256sum -c checksums.txt
```
