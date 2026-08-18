# Self-hosted deployment

Phase-1 target is self-hosted only: Docker, a VPS, or a single static binary.
There is no serverless support (see `AGENTS.md` §3).

## One-command Docker run

Prereqs: Docker with the Compose plugin.

```sh
cp .env.example .env     # then set MCP_AUTH_TOKENS (the only required var)
docker compose up -d
```

The server is reachable at:

- `GET  /healthz` — liveness probe, no auth: `curl http://localhost:8080/healthz`
- `POST /mcp` — Streamable HTTP MCP endpoint (requires an `Authorization: Bearer <token>` from `MCP_AUTH_TOKENS`)
- `GET  /mcp` — SSE transport for the MCP endpoint

`HTTP_PORT` in `.env` (default `8080`) is both the container's listen port and
the host port it is published on.

## Volume mounts and DB persistence

The SQLite job-store database — the only record of pending/running operations —
lives in the `do0ps-data` named volume, mounted at `/data` in the container
(`DB_PATH=/data/do0ps.db`). **That volume is the only copy of job history.**
Losing it loses every job record.

- `docker compose down` keeps the volume.
- `docker compose down -v` **deletes it** — make sure you have a backup first.
- The server runs as UID 65532; a fresh named volume is initialized writable
  by that UID automatically.

To keep the database as a plain file on the host instead of a named volume,
mount a host directory and chown it to the container user:

```yaml
volumes:
  - ./data:/data
```

```sh
mkdir -p data && chown 65532:65532 data
docker compose up -d
```

Back up the `do0ps.db` file (and its `-wal`/`-shm` sidecars, if present).

## Configuration

All runtime configuration comes from environment variables — see
`.env.example`. Only `MCP_AUTH_TOKENS` is required. `DB_PATH` inside the
container is always `/data/do0ps.db` (set by the compose file), so do not
change it in `.env` for containerized runs; change the mount instead.

## Image

The `Dockerfile` is multi-stage: a `golang:1.26-alpine` build stage compiles a
fully static binary (`CGO_ENABLED=0`; the pure-Go `modernc.org/sqlite` driver
needs no cgo), and the final `runner` stage is `scratch` with just the binary
and its CA bundle. Build it manually with:

```sh
docker build -t do0ps --target runner --build-arg APP_VERSION=dev .
```

The final stage name `runner` and the `APP_VERSION` build arg are required by
`.github/workflows/docker-publish.yml`, which publishes the image to GHCR on
release tags.
