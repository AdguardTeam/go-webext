# Development Guide

This document explains how to set up the development environment, build, test,
and contribute to `go-webext`.

## Prerequisites

| Tool              | Version | Purpose                |
|-------------------|---------|------------------------|
| Go                | 1.26+   | Language runtime       |
| golangci-lint     | v2+     | Linting                |
| gofumpt           | latest  | Code formatting        |

Install Go from <https://go.dev/dl/>.

Install development tools:

```bash
# golangci-lint (see https://golangci-lint.run/welcome/install/)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin

# gofumpt
go install mvdan.cc/gofumpt@latest
```

## Getting Started

### 1. Clone the repository

```bash
git clone <repository-url>
cd go-webext
```

### 2. Install dependencies

```bash
go mod download
```

### 3. Configure environment

Store API credentials are loaded from a `.env` file in the project root (or from
environment variables). Copy the template below and fill in your values:

```dotenv
# Chrome Web Store
CHROME_CLIENT_ID=<client_id>
CHROME_CLIENT_SECRET=<client_secret>
CHROME_REFRESH_TOKEN=<refresh_token>
CHROME_PUBLISHER_ID=<publisher_id>
CHROME_API_VERSION=v1

# Firefox AMO
FIREFOX_CLIENT_ID=<client_id>
FIREFOX_CLIENT_SECRET=<client_secret>

# Microsoft Edge Add-ons (v1.1 recommended)
EDGE_CLIENT_ID=<client_id>
EDGE_API_KEY=<api_key>
EDGE_API_VERSION=v1.1
```

The `.env` file is git-ignored. See [README.md](README.md) for detailed
instructions on obtaining credentials for each store.

### 4. Build and run

```bash
make build
./go-webext --help
```

## Development Commands

All commands are defined in the `Makefile`:

| Command         | Description                                      |
|-----------------|--------------------------------------------------|
| `make build`    | Build the binary (runs `make clean` first)       |
| `make clean`    | Remove the built binary                          |
| `make test`     | Run all tests (`go test ./internal/... -count=1`) |
| `make lint`     | Run `golangci-lint run ./...`                    |
| `make format`   | Format code with `gofumpt -w .`                  |
| `make coverage` | Generate HTML coverage report (`coverage.html`)  |
| `make start`    | Live-reload via CompileDaemon (requires install) |

### Running a single package's tests

```bash
go test ./internal/chrome/ -count=1 -v
```

### Running tests with race detection

CI runs tests with the race detector. To match CI locally:

```bash
go test -race ./...
```

## Development Workflow

### Branching

The CI pipeline (Bamboo) automatically creates builds for pull requests. Create
a feature branch, push it, and open a PR.

### Before committing

Run the full check suite:

```bash
make format
make lint
make test
```

All three must pass — CI enforces this.

### Code style

Code guidelines, architecture patterns, and package conventions are documented
in [AGENTS.md](AGENTS.md). Key points:

- **Error wrapping**: `fmt.Errorf("context: %w", err)`
- **Structured logging**: `log/slog` with `slogutil`
- **Unexported fields**: expose behavior through constructors
- **Formatter**: `goimports` (enforced by golangci-lint)

### Testing

- Test files live next to the code they test (e.g., `chrome_test.go` beside
  `chrome.go`).
- HTTP API interactions are tested with `net/http/httptest` mock servers.
- Static test fixtures go in `testdata/` directories within each package.
- Tests run without caching (`-count=1`).

### CI pipeline

The Bamboo pipeline runs two stages:

1. **Test** — `go test -race ./...` in `golang:1.26.1` Docker image
2. **Lint** — `golangci-lint run -v` in `golangci/golangci-lint:v2.10.1` image

Builds are created automatically for pull requests and deleted after inactivity.

## Common Tasks

### Adding support for a new store API version

1. Add types and client methods in the store's package under `internal/`.
2. Wire the new version in `internal/cmd/cmd.go` — add a config constant, update
   the store constructor switch, and add CLI flags if needed.
3. Add environment variable(s) for version selection (e.g.,
   `CHROME_API_VERSION`).
4. Write tests using `httptest.NewServer` mocks.
5. Update `README.md` with usage examples and credential setup.

### Adding a new CLI flag

Flags are defined in `internal/cmd/cmd.go`. Define a `*cli.XxxFlag` variable and
add it to the relevant subcommand's `Flags` slice.

### Verbose / debug logging

Pass `-v` (or `--verbose`) as a global flag to enable debug-level logging:

```bash
./go-webext -v update chrome -a <id> -f ./chrome.zip
```

## Troubleshooting

### `golangci-lint` fails with "error obtaining VCS status"

This happens when the Git history is incomplete (e.g., shallow clone). Disable
VCS stamping:

```bash
GOFLAGS="-buildvcs=false" golangci-lint run ./...
```

### `.env` file values with special characters

The Edge API key may contain characters that need escaping. The project uses
[godotenv](https://github.com/joho/godotenv) to load `.env` files — refer to
its documentation for quoting rules.

### Tests fail with `-count=1` but pass without it

`-count=1` disables test caching. If tests are non-deterministic or depend on
external state, they will surface only with caching disabled. Check for shared
mutable state between test cases.

### Build produces no binary

`make build` outputs the binary as `./go-webext` in the project root. Ensure
`make clean` did not fail silently and that you have write permissions.

## Additional Resources

- [README.md](README.md) — user-facing documentation, CLI usage, credential
  setup
- [AGENTS.md](AGENTS.md) — code guidelines, architecture, project structure
- [CHANGELOG.md](CHANGELOG.md) — release history
