# syntax=docker/dockerfile:1
#
# Multi-stage build for the do0ps MCP server.
#
# Stage 1 (build): compile a fully static Go binary. The pure-Go SQLite
# driver (modernc.org/sqlite) means CGO_ENABLED=0 builds cleanly -- no cgo,
# no musl/glibc concerns, no cross-compile headaches. The result is a
# standalone ELF with zero dynamic dependencies.
# Stage 2 (runner): scratch -- the binary, its CA bundle (the server talks
# HTTPS to provider APIs), and nothing else.
#
# The final stage MUST be named "runner": .github/workflows/docker-publish.yml
# builds with `--target runner` and passes APP_VERSION via build args.

FROM golang:1.26-alpine AS build

ARG APP_VERSION=dev

# Provider adapters make outbound HTTPS calls; ship the CA bundle into the
# scratch image.
RUN apk add --no-cache ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${APP_VERSION}" \
    -o /out/do0ps ./cmd/server

# Pre-create the database directory owned by the non-root UID. Docker
# initializes a fresh named volume from the image directory at the mount
# point, inheriting this ownership, so the server can write the SQLite file
# without the classic root-owned-volume permission failure. 65532 is the
# distroless "nonroot" convention.
RUN mkdir -p /out/data && chown 65532:65532 /out/data

FROM scratch AS runner

COPY --from=build /out/do0ps /do0ps
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build --chown=65532:65532 /out/data /data

# Docker Compose points DB_PATH at this volume-mounted path (see
# docker-compose.yml); setting it here gives a sensible default for bare
# `docker run` too.
ENV DB_PATH=/data/do0ps.db

EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/do0ps"]
