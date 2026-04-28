# Contributing to lfk

Contributions are welcome! Here is how to get started.

## Prerequisites

- Go 1.26.2 or later
- Access to a Kubernetes cluster (for testing)
- `kubectl` configured and working
- `golangci-lint` ([install](https://golangci-lint.run/welcome/install/))

## Development Setup

```bash
# Clone the repository
git clone https://github.com/janosmiko/lfk.git
cd lfk

# Set up git hooks and install dependencies
make setup
go mod download

# Build the binary
make build

# Run it
./lfk
```

## Building and Testing

```bash
# Build
go build -o lfk .

# Run tests (if available)
go test ./...

# Run with race detector
go build -race -o lfk . && ./lfk

# Lint (if you have golangci-lint installed)
golangci-lint run
```

## Project Structure

The application follows a standard Go project layout:

- `main.go` - Entry point, initializes the Kubernetes client, loads config, and starts the Bubbletea program
- `internal/app/` - Core application logic: the Bubbletea model, update loop, async commands, and bookmarks
- `internal/k8s/` - Kubernetes client wrapper handling API calls, owner resolution, and CRD discovery
- `internal/model/` - Shared types, actions, navigation state, and resource templates
- `internal/ui/` - All rendering: columns, overlays, styles, themes, help screen, and log viewer
- `internal/logger/` - Application logging

## Submitting Changes

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Ensure the project builds cleanly (`go build ./...`)
5. Commit your changes with a descriptive message
6. Push to your fork and open a Pull Request
