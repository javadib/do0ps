# do0ps build tooling.
#
# Targets:
#   build  - compile all packages
#   run    - run the MCP server locally (needs MCP_AUTH_TOKENS, see .env.example)
#   test   - run the test suite
#   vet    - go vet
#   lint   - golangci-lint (v2.12.0 matches CI)
#   fmt    - gofmt all Go sources
#   mcpb   - build installable MCP bundles into dist/mcpb/ (see docs/mcp-bundle.md)
#
# Every target here is a one-line wrapper around a `go` command, so anything
# in this file can be run directly with the Go toolchain when make is not
# available (Windows).

GOLANGCI_LINT_VERSION := v2.12.0

.PHONY: build run test vet lint fmt mcpb mcpb-local clean

build:
	go build ./...

run:
	go run ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run

fmt:
	gofmt -l -w .

# Build the .mcpb bundles end users install into a chat client. VERSION
# defaults to the current git tag; TARGETS defaults to every released
# GOOS/GOARCH pair. See docs/mcp-bundle.md.
#
# These are conveniences: the builder is a Go program, so `go run
# ./cmd/mcpb-build` works identically without make -- which is how it is built
# on Windows.
mcpb:
	go run ./cmd/mcpb-build $(if $(VERSION),-version $(VERSION)) $(if $(TARGETS),-targets $(TARGETS))

# One bundle for this machine, for a quick local install test.
mcpb-local:
	go run ./cmd/mcpb-build -targets host $(if $(VERSION),-version $(VERSION))

clean:
	rm -rf dist

# Convenience for installing the exact golangci-lint version CI uses.
# v2 moved the module to github.com/golangci/golangci-lint/v2; the v1 path
# does not resolve v2 tags.
install-tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
