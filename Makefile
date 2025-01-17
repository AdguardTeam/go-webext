start:
	CompileDaemon -exclude-dir=".git" -exclude-dir="tmp"

build: clean
	go build

clean:
	rm -f go-webext

lint:
	 golangci-lint run ./...

format:
	gofumpt -w .

test:
	go test ./internal/... -count=1

coverage:
	go test ./internal/... --coverprofile "coverage.html" && go tool cover --html "coverage.html"
