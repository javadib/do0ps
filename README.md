# do0ps

[![CI](https://github.com/javadib/do0ps/actions/workflows/ci.yml/badge.svg?branch=step/ph1)](https://github.com/javadib/do0ps/actions/workflows/ci.yml)

do0ps connects to hosting/PaaS providers -- starting with the Iranian provider
**Parspack** -- and exposes their capabilities through an **MCP (Model
Context Protocol) server**, so a chatbot such as Claude or Codex can create a
server, configure DNS, order a certificate, and similar infrastructure tasks
on your behalf, driven by natural language instead of a provider console.

## Architecture

do0ps is a single self-hosted Go service, built as hexagonal (ports &
adapters): an MCP tool layer turns incoming tool calls into calls on core
application use cases, which reach the outside world only through ports --
a channel-based job queue, a SQLite job store, and one adapter per provider.
For the full breakdown (tech stack, layering rules, repository layout,
open questions) see [`AGENTS.md`](AGENTS.md); this README only covers running
the server and connecting a client to it.

**Not a target:** Vercel or other serverless platforms. do0ps needs a
persistent process (in-memory worker pool, a local SQLite file) -- run it as
a Docker container, on a VPS, or as a plain binary.

## Quick start: Docker Compose (recommended)

```bash
git clone https://github.com/javadib/do0ps.git
cd do0ps
cp .env.example .env
# edit .env -- at minimum, set MCP_AUTH_TOKENS (see below)
docker compose up -d --build
```

This builds the image from the committed `Dockerfile` (`runner` target) and
starts do0ps on `HTTP_PORT` (`8080` by default), with the SQLite job store
persisted in the `do0ps-data` named volume -- losing that volume loses job
history, so back it up like you would any other stateful container.

Check it's up:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

## Quick start: go run / binary

Requires Go 1.26+ (see `go.mod`).

```bash
git clone https://github.com/javadib/do0ps.git
cd do0ps
cp .env.example .env
# edit .env -- at minimum, set MCP_AUTH_TOKENS
export $(grep -v '^#' .env | xargs)   # or use a tool like direnv/dotenv
make run                              # go run ./cmd/server
```

`make build` compiles everything as a sanity check; to get a binary you can
copy to a VPS, build it explicitly: `go build -o do0ps ./cmd/server`. Run it
there with the same environment variables. `make test`, `make vet`, and
`make lint` run the test suite, `go vet`, and `golangci-lint` respectively.

## Environment variables

Copy [`.env.example`](.env.example) to `.env` and fill it in -- it is the
source of truth for defaults; the table below summarizes it.

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `MCP_AUTH_TOKENS` | **yes** | -- | Comma-separated list of allowed MCP access tokens, each `token:client_id[:name]`. Each token must be at least 16 characters. The server refuses to start without this. |
| `DB_PATH` | no | `./data/do0ps.db` | Path to the SQLite job-store file. Its parent directory is created automatically. |
| `HTTP_PORT` | no | `8080` | Port the MCP server (Streamable HTTP) listens on. |
| `LOG_LEVEL` | no | `info` | Structured logger level: `debug`, `info`, `warn`, or `error`. |
| `DO0PS_QUEUE_WORKERS` | no | `8` | Number of worker goroutines processing queued jobs. |
| `DO0PS_QUEUE_DEPTH` | no | `256` | Maximum number of jobs buffered in the in-process queue. |
| `DO0PS_POLL_INTERVAL` | no | `10s` | How often a running long-operation job is polled against the provider. |
| `DO0PS_POLL_TIMEOUT` | no | `20m` | How long a long operation is allowed to run before it's marked failed. |
| `DO0PS_SHUTDOWN_WAIT` | no | `30s` | How long graceful shutdown waits for in-flight jobs to drain. |

`MCP_AUTH_TOKENS` example:

```
MCP_AUTH_TOKENS="change-me-0123456789:client-a,change-me-9876543210:client-b"
```

## Connecting an MCP client (Claude, Codex, ...)

do0ps speaks MCP over **Streamable HTTP** at a single `/mcp` route (JSON-RPC
over `POST`, with an SSE stream over `GET` for server-initiated messages).
Point your MCP client at:

```
https://<your-host>:<HTTP_PORT>/mcp
```

with an `Authorization: Bearer <token>` header, using one of the tokens you
put in `MCP_AUTH_TOKENS`. `GET /healthz` is intentionally unauthenticated so
orchestrators/uptime checks can probe liveness without a token; every other
route requires a valid bearer token.

### Provider credentials are not configured server-side

do0ps stores **no** provider credentials. Every tool call that talks to
Parspack takes the API key/secret as explicit parameters, supplied by the
calling chatbot session at call time -- not read from server-side
configuration or a credential store. This is a deliberate design choice (see
`AGENTS.md` §4.2): the MCP bearer token above only controls access to the
do0ps server itself, and is unrelated to your Parspack account credentials.

## What you can do with it

Once connected, the assistant has tools covering (Parspack, phase 1):

- **VM / cloud server lifecycle** -- create, list, get, delete
- **VPC** -- create, list, get, delete
- **Firewalls** -- create, list, get, delete
- **Load balancers** -- create, list, get, update, delete
- **Reserved IPs** -- reserve, release, assign to/unassign from a server
- **SSH keys** -- register, list, delete
- **Snapshots** -- create, list, delete, restore a VM from one
- **SSL certificates** -- order, verify domain challenge, issue, reissue
- **CDN zones and DNS records** -- create/get/delete zones, manage DNS
  records and nameservers within a zone
- **Long-running operations** -- `get_operation_status` to poll anything
  that returns an `operation_id` instead of blocking (server provisioning,
  load balancer creation, etc.)

Ask in plain language ("give me a 2GB server in Tehran with a public IP") --
turning that into the right tool call and parameters is the calling model's
job, driven by each tool's JSON Schema description, not something you need
to script yourself.

## Contributing

Phase-1 work is tracked as GitHub Issues on this repo; see `AGENTS.md` §9
for the label-driven workflow agents (and humans) follow to pick up work.
