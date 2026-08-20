# syntax=docker/dockerfile:1

# ---- builder ----------------------------------------------------------------
# Compiles a static do0ps binary. CGO is disabled: the SQLite job store uses
# the pure-Go modernc.org/sqlite driver (AGENTS.md §2), so no C toolchain or
# libc is required at build or run time.
FROM golang:1.26.2-alpine AS builder

# Baked into the binary via -ldflags below. .github/workflows/docker-publish.yml
# passes this on tagged releases; defaults to "dev" for local/manual builds.
ARG APP_VERSION=dev

WORKDIR /src

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${APP_VERSION}" \
    -o /out/do0ps ./cmd/server

# The distroless nonroot user below (uid/gid 65532) needs write access to the
# SQLite job-store directory. Pre-create it here with the right ownership so
# the COPY into the final stage lands with correct permissions -- that image
# has no shell to chown with.
RUN mkdir -p /data && chown 65532:65532 /data

# ---- runner -------------------------------------------------------------
# Named "runner": .github/workflows/docker-publish.yml builds this target
# specifically and passes the APP_VERSION build arg above.
FROM gcr.io/distroless/static-debian12:nonroot AS runner

WORKDIR /

COPY --from=builder /out/do0ps /do0ps
COPY --from=builder --chown=65532:65532 /data /data

# Matches .env.example's HTTP_PORT default; override via the HTTP_PORT env
# var (and the docker-compose.yml port mapping) if you change it.
EXPOSE 8080

# Matches .env.example's DB_PATH default layout, rooted at this volume.
# Mount a named volume or bind mount here -- losing it loses job history
# (AGENTS.md §4.4).
VOLUME ["/data"]
ENV DB_PATH=/data/do0ps.db

USER nonroot:nonroot

ENTRYPOINT ["/do0ps"]
