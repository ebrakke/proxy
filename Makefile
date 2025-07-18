.PHONY: build clean linux darwin windows all

# Default target
build:
	go build -o proxy .

# Cross-compilation targets
linux:
	GOOS=linux GOARCH=amd64 go build -o proxy-linux .

darwin:
	GOOS=darwin GOARCH=amd64 go build -o proxy-darwin .

windows:
	GOOS=windows GOARCH=amd64 go build -o proxy-windows.exe .

# ARM targets for newer Macs and ARM servers
linux-arm64:
	GOOS=linux GOARCH=arm64 go build -o proxy-linux-arm64 .

darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o proxy-darwin-arm64 .

# Build all platforms
all: linux darwin windows linux-arm64 darwin-arm64

# Clean build artifacts
clean:
	rm -f proxy proxy-* 

# Install locally
install: build
	cp proxy /usr/local/bin/

# Quick test
test:
	go test ./...

# Format code
fmt:
	go fmt ./...

# Static builds (useful for containers/older systems)
static-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-extldflags "-static"' -o proxy-linux-static .

static-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -ldflags '-extldflags "-static"' -o proxy-linux-arm64-static .