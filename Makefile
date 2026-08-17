# do0ps build tooling.
#
# Targets:
#   build  - compile all packages
#   run    - run the MCP server locally (needs DO0PS_TOKENS, see .env.example)
#   test   - run the test suite
#   vet    - go vet
#   lint   - golangci-lint (v2.12.0 matches CI)
#   fmt    - gofmt all Go sources

GOLANGCI_LINT_VERSION := v2.12.0

.PHONY: build run test vet lint fmt

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

# Convenience for installing the exact golangci-lint version CI uses.
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)