# Contributing

Thanks for considering a contribution to mkdev.

## Development setup

```sh
git clone https://github.com/venkatkrishna07/mkdev
cd mkdev
make build   # requires Go 1.25+
make test    # race detector enabled
make lint    # requires golangci-lint
```

## Commit style

Conventional Commits. The release changelog auto-filters out `docs:`, `chore:`, `test:`, and `ci:` prefixes.

Examples:

- `feat(cli): add completion subcommand`
- `fix(proxy): close upstream on shutdown`
- `docs: clarify install steps`

## Pull requests

1. Fork → branch off `main` → PR back to `main`.
2. All CI checks must pass (lint, vet, test) across macOS, Linux, and Windows.
3. Add tests for behavior changes.
4. Update README if user-facing flags or commands change.

## Security-sensitive changes

If a change touches the CA, sudo helper, trust-store integration, or TLS minting path: flag it in the PR description and consider a private disclosure first. See [SECURITY.md](./SECURITY.md).

## Releasing

Maintainer-only. Push a `vX.Y.Z` tag to `main`; GoReleaser handles binaries, checksums, signing, and Homebrew tap formula.
