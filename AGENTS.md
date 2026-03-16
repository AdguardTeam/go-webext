# AGENTS.md

## Project Overview

**go-webext** is a CLI tool for managing browser extensions across multiple web
stores: Chrome Web Store, Firefox Add-ons (AMO), and Microsoft Edge Add-ons.
It supports uploading, updating, publishing, signing, and checking the status of
extensions via each store's API.

## Technical Context

- **Language**: Go 1.26
- **Primary Dependencies**:
  - `urfave/cli/v2` — CLI framework
  - `golang-jwt/jwt/v4` — JWT signing (Firefox API auth)
  - `caarlos0/env/v6` — environment variable parsing
  - `joho/godotenv` — `.env` file loading
  - `AdguardTeam/golibs` — shared AdGuard utilities (logging, validation, HTTP)
  - `stretchr/testify` — test assertions
- **Storage**: None (stateless CLI; credentials via env vars / `.env`)
- **Testing**: `go test` with `testify`; HTTP interactions tested via
  `httptest.NewServer` mocks
- **Target Platform**: Cross-platform CLI binary (macOS, Linux, Windows)
- **Project Type**: Single binary CLI
- **Performance Goals**: N/A
- **Constraints**: Requires store API credentials provided via environment
  variables; Edge Store insert not supported via API
- **Scale/Scope**: Internal CI/CD tooling for AdGuard browser extension releases

## Project Structure

```text
├── main.go                    # Entry point — delegates to internal/cmd
├── go.mod                     # Module definition and dependencies
├── Makefile                   # Build, test, lint, format commands
├── .golangci.yml              # Linter configuration (golangci-lint v2)
├── bamboo-specs/              # CI/CD pipeline definitions
├── internal/
│   ├── cmd/                   # CLI command definitions and wiring
│   ├── chrome/                # Chrome Web Store client (v1 + v2 APIs)
│   ├── firefox/               # Firefox AMO client (v5 API)
│   │   └── api/               # Low-level Firefox HTTP API layer
│   ├── edge/                  # Microsoft Edge Add-ons client (v1 + v1.1 APIs)
│   ├── fileutil/              # ZIP archive helpers
│   └── urlutil/               # URL utility functions
├── specs/                     # Feature specifications (SDD workflow)
├── README.md                  # User-facing documentation
└── CHANGELOG.md               # Release history
```

## Build And Test Commands

| Action   | Command                    |
|----------|----------------------------|
| Build    | `make build`               |
| Test     | `make test`                |
| Lint     | `make lint`                |
| Format   | `make format`              |
| Clean    | `make clean`               |
| Coverage | `make coverage`            |

**Prerequisites**: Go 1.26+, `golangci-lint`, `gofumpt`.

## Contribution Instructions

1. **Run tests before committing**: `make test`
2. **Run linter before committing**: `make lint`
3. **Run formatter before committing**: `make format`
4. **All code must pass lint and tests** — CI enforces this.
5. **Do not commit generated or temporary files** (`tmp/`, `coverage.html`,
   built binary).
6. **Keep credentials out of code** — use environment variables or `.env` files.
   Never hard-code API keys, secrets, or tokens.
7. **Update CHANGELOG.md** for user-visible changes following Keep a Changelog
   format.
8. **Test files live next to the code they test** (e.g., `chrome_test.go`
   alongside `chrome.go`). Use `httptest.NewServer` for HTTP API mocking.

## Code Guidelines

### Architecture

- **Package per store**: Each browser store has its own package under
  `internal/` (`chrome`, `firefox`, `edge`). Store-specific types, API clients,
  and business logic live together.
- **CLI wiring in `internal/cmd`**: All CLI flag definitions, subcommand
  registration, and store initialization live in `cmd.go`. Action functions call
  into store packages.
- **Separate API layer for Firefox**: `internal/firefox/api/` contains the
  low-level HTTP client; `internal/firefox/firefox.go` contains higher-level
  orchestration (upload → validate → create version → sign → download).
- **Config via environment**: Store credentials are parsed from environment
  variables using `caarlos0/env` with struct tags. `.env` files are loaded
  automatically via `godotenv`.

### Code Quality

- **Structured logging**: Use `log/slog` with `slogutil` throughout. Every
  store method logs entry/exit with relevant IDs.
- **Error wrapping**: Always wrap errors with `fmt.Errorf("context: %w", err)`.
  Use `errors.WithDeferred` for deferred close errors.
- **Unexported fields**: Store and client structs use unexported fields; expose
  behavior through constructor functions (`NewStore`, `NewClient`) accepting
  config structs.
- **Linter rules**: See `.golangci.yml` — enabled linters include `errcheck`,
  `govet`, `staticcheck`, `revive`, `gosec`, `gocyclo` (max complexity 20).
  Formatter: `goimports`.

### Testing

- **Test files**: `*_test.go` next to production code.
- **HTTP mocking**: Use `net/http/httptest` to create mock servers. Define
  handler maps for different API endpoints.
- **Test data**: Static fixtures go in `testdata/` directories within each
  package.
- **Run with `-count=1`**: Tests run without caching (`go test ./internal/...
  -count=1`).

### Other

- **API versioning**: Chrome supports v1 and v2 APIs; Edge supports v1 and v1.1.
  Version selection is driven by environment variables
  (`CHROME_API_VERSION`, `EDGE_API_VERSION`).
- **No `internal/` test exclusion in lint**: Test files are excluded from lint
  via `.golangci.yml` exclusion paths.
