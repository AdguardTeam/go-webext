#start:
#	CompileDaemon -exclude-dir=".git" -exclude-dir="tmp"
#
#build: clean
#	go build
#
#clean:
#	rm -f go-webext
#
#lint:
#	 golangci-lint run ./...
#
#format:
#	gofumpt -w .
#
#test:
#	go test ./... -count=1
#
#coverage:
#	go test ./... --coverprofile "coverage.html" && go tool cover --html "coverage.html"

#start:
#	CompileDaemon -exclude-dir=".git" -exclude-dir="tmp"
#
#build: clean
#	go build
#
#clean:
#	rm -f go-webext
#
#lint:
#	 golangci-lint run ./...
#
#format:
#	gofumpt -w .
#
#test:
#	go test ./... -count=1
#
#coverage:
#	go test ./... --coverprofile "coverage.html" && go tool cover --html "coverage.html"

# Keep the Makefile POSIX-compliant.  We currently allow hyphens in
# target names, but that may change in the future.
#
# See https://pubs.opengroup.org/onlinepubs/9699919799/utilities/make.html.
.POSIX:

# Don't name this macro "GO", because GNU Make apparenly makes it an
# exported environment variable with the literal value of "${GO:-go}",
# which is not what we need.  Use a dot in the name to make sure that
# users don't have an environment variable with the same name.
#
# See https://unix.stackexchange.com/q/646255/105635.
GO.MACRO = $${GO:-go}
GOPROXY = https://goproxy.cn|https://proxy.golang.org|direct

RACE = 0
VERBOSE = 0

BRANCH = $$( git rev-parse --abbrev-ref HEAD )
VERSION = 0
REVISION = $$( git rev-parse --short HEAD )

ENV = env\
	BRANCH="$(BRANCH)"\
	GO="$(GO.MACRO)"\
	GOPROXY='$(GOPROXY)'\
	PATH="$${PWD}/bin:$$( "$(GO.MACRO)" env GOPATH )/bin:$${PATH}"\
	RACE='$(RACE)'\
	REVISION="$(REVISION)"\
	VERBOSE='$(VERBOSE)'\
	VERSION="$(VERSION)"\

# Keep the line above blank.

# Keep this target first, so that a naked make invocation triggers a
# full build.
build: go-deps go-build

init: ; git config core.hooksPath ./scripts/hooks

test: go-test

go-build: ; $(ENV)          "$(SHELL)" ./scripts/make/go-build.sh
go-deps:  ; $(ENV)          "$(SHELL)" ./scripts/make/go-deps.sh
go-lint:  ; $(ENV)          "$(SHELL)" ./scripts/make/go-lint.sh
go-test:  ; $(ENV) RACE='1' "$(SHELL)" ./scripts/make/go-test.sh
go-tools: ; $(ENV)          "$(SHELL)" ./scripts/make/go-tools.sh

go-check: go-tools go-lint go-test

# A quick check to make sure that all operating systems relevant to the
# development of the project can be typechecked and built successfully.
go-os-check:
	env GOOS='darwin'  "$(GO.MACRO)" vet ./internal/...
	env GOOS='linux'   "$(GO.MACRO)" vet ./internal/...
	env GOOS='windows' "$(GO.MACRO)" vet ./internal/...

txt-lint: ; $(ENV) "$(SHELL)" ./scripts/make/txt-lint.sh
