.PHONY: build clean linux darwin windows all release

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

# Release target
release:
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make release VERSION=vX.Y.Z"; \
		exit 1; \
	fi
	@echo "Creating release $(VERSION)..."
	@if ! git diff --quiet; then \
		echo "Error: Working directory is not clean. Please commit your changes first."; \
		exit 1; \
	fi
	@if ! git diff --cached --quiet; then \
		echo "Error: Staged changes found. Please commit your changes first."; \
		exit 1; \
	fi
	@echo "Ensuring we're on main branch..."
	@if [ "$$(git branch --show-current)" != "main" ]; then \
		echo "Error: Not on main branch. Please switch to main first."; \
		exit 1; \
	fi
	@echo "Pulling latest changes..."
	@git pull origin main
	@echo "Creating and pushing tag $(VERSION)..."
	@git tag -a $(VERSION) -m "Release $(VERSION)"
	@git push origin $(VERSION)
	@echo "Release $(VERSION) created successfully!"
	@echo "GitHub Actions will now build binaries and create the release."
	@echo "Check: https://github.com/$(shell git config --get remote.origin.url | sed 's/.*github.com[:/]\(.*\)\.git/\1/')/actions"