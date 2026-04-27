FROM golang:1.26.1 AS go-base
WORKDIR /app

# =============================================================================
# Test plan
# =============================================================================

FROM go-base AS test
RUN --mount=type=cache,target=/root/go/pkg/mod,id=go-webext-gomod \
    --mount=type=cache,target=/root/.cache/go-build,id=go-webext-gobuild \
    --mount=type=bind,source=go.mod,target=go.mod \
    --mount=type=bind,source=go.sum,target=go.sum \
    go mod download

COPY . .

# Disable VCS stamping to prevent "error obtaining VCS status" — Docker builds
# do not carry full git history, so Go's VCS stamping feature fails without this flag.
RUN --mount=type=cache,target=/root/go/pkg/mod,id=go-webext-gomod \
    --mount=type=cache,target=/root/.cache/go-build,id=go-webext-gobuild \
    GOFLAGS="-buildvcs=false" go test -race ./...

FROM scratch AS test-output
COPY --from=test /app/go.sum /

# =============================================================================
# Lint plan
# =============================================================================

FROM golangci/golangci-lint:v2.10.1 AS lint
WORKDIR /app
COPY . .

# Disable VCS stamping to prevent "error obtaining VCS status" — Docker builds
# do not carry full git history, so Go's VCS stamping feature fails without this flag.
RUN --mount=type=cache,target=/root/go/pkg/mod,id=go-webext-gomod \
    --mount=type=cache,target=/root/.cache/go-build,id=go-webext-gobuild \
    GOFLAGS="-buildvcs=false" golangci-lint run -v

FROM scratch AS lint-output
COPY --from=lint /app/go.sum /
