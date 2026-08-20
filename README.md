# do0ps

[![CI](https://github.com/javadib/do0ps/actions/workflows/ci.yml/badge.svg?branch=develop)](https://github.com/javadib/do0ps/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26.2-00ADD8?logo=go&logoColor=white)](go.mod)
[![MCP](https://img.shields.io/badge/MCP-2025--06--18-5A45FF)](https://modelcontextprotocol.io)

> An MCP server that lets a chatbot run your infrastructure.

**[فارسی / Persian →](docs/README_FA.md)**

do0ps (Do Ops) connects AI assistants — Claude, ChatGPT/Codex, Cursor, or anything else that speaks the
[Model Context Protocol](https://modelcontextprotocol.io) — to hosting and PaaS providers, starting with the
Iranian provider [Parspack](https://parspack.com). Ask for a server in plain language and it calls the
provider's API for you:

> **"Spin up a 2GB Ubuntu server called web-01, then point api.example.com at it."**

The assistant turns that into `create_server(name: "web-01", ram_mb: 2048, image: "ubuntu-24.04")`, polls the
operation until the machine is ready, and follows up with `create_dns_record`. No dashboard, no CLI flags, no
Terraform.

---

## Contents

- [Why do0ps](#why-do0ps) · [What it can do](#what-it-can-do) · [Install](#install)
- [Configuration](#configuration) · [Architecture](#architecture) · [Development](#development)
- [Releases](#releases) · [Documentation](#documentation) · [Contributing](#contributing)

---

## Why do0ps

**Natural language is the interface.** Parameter extraction ("a 2GB server" → `ram_mb: 2048`) is left to the
calling model and driven by careful JSON Schema descriptions on every tool — units, examples, constraints.
There is no custom NLU code here, because schema quality does that job better.

**Your provider keys are never stored.** do0ps has no credential store and no encryption-at-rest to get
wrong. The chat session holds your API key and passes it as a parameter on each tool call, where it lives in
memory for the life of the request and is never written to disk.

**Self-hosted, single binary.** One static Go binary, one SQLite file. No Redis, no managed Postgres, no
message broker, no serverless runtime. Run it as a container, on a VPS, or let a chat client spawn it for you.

**Long operations do not block the chat.** Provisioning a server takes minutes, which no MCP client will wait
for. Those calls return an `operation_id` immediately and finish on a background worker; the assistant asks
`get_operation_status` when it wants to know. Fast operations — DNS records, listings — return inline.

---

## What it can do

**147 MCP tools** across three Parspack API surfaces:

| Area | Tools | Examples |
| --- | --- | --- |
| CDN edge — WAF, cache, rules, analytics | 99 | ModSecurity rules, rate limits, page/origin/transform rules, cache purge, access and security logs |
| Networking — VPC, reserved IPs, firewalls, load balancers | 17 | `create_vpc`, `reserve_ip`, `assign_ip_to_server`, `update_firewall`, `provision_load_balancer` |
| CDN zones and DNS records | 9 | `create_cdn_zone`, `create_dns_record`, `get_nameserver_records` |
| Servers and snapshots | 9 | `create_server`, `list_servers`, `create_snapshot`, `restore_vm` |
| SSL certificates | 8 | `create_ssl_order`, `get_ssl_challenge`, `verify_ssl_challenge`, `reissue_ssl_certificate` |
| SSH keys | 3 | `register_ssh_key`, `list_ssh_keys`, `delete_ssh_key` |
| Operations and health | 2 | `get_operation_status`, `ping` |

DNS is not a standalone product at Parspack — records live inside a CDN zone, and the tools are scoped that
way. Providers planned next: **ArvanCloud** (ابرآروان) and **Liara**.

---

## Install

Three ways in, all running the same build. Pick one.

### 1. MCP Bundle — nothing to run

Download the `.mcpb` for your platform from [Releases](https://github.com/javadib/do0ps/releases) and drop it
into your chat client. In Claude Desktop, double-click it or use **Settings → Extensions**. The client spawns
the bundled binary over stdio and manages it from then on — no server, no port, no token.

The extension's settings carry two optional fields. Leave them empty to run the whole server locally; fill in
a **server URL** and **access token** and the same bundle becomes a thin bridge to a self-hosted instance, so
a team shares one deployment and one job history.

Other clients can use the bundle too — it is a plain zip. See **[docs/mcp-bundle.md](docs/mcp-bundle.md)** for
per-client configuration and for building bundles yourself.

### 2. Docker

```bash
git clone https://github.com/javadib/do0ps.git && cd do0ps
cp .env.example .env          # set MCP_AUTH_TOKENS to a token of 16+ characters
docker compose up -d
curl -s localhost:8080/healthz
```

The image is also published to `ghcr.io/javadib/do0ps` on each release. The SQLite job store lives on a named
volume so in-flight operations survive a restart.

### 3. From source

```bash
git clone https://github.com/javadib/do0ps.git && cd do0ps
git checkout develop
cp .env.example .env
make run                      # or: go run ./cmd/server
```

Requires only the Go toolchain in [`go.mod`](go.mod) — no cgo, no external database.

### Connecting a client to a self-hosted server

```json
{
  "mcpServers": {
    "do0ps": {
      "type": "http",
      "url": "https://your-host.example.com/mcp",
      "headers": { "Authorization": "Bearer <your do0ps token>" }
    }
  }
}
```

Terminate TLS in front of the server — that token is a bearer credential.

---

## Configuration

Everything comes from the environment; a `.env` file is loaded when present and is optional, so containers and
CI can supply variables directly. [`.env.example`](.env.example) is the source of truth.

| Variable | Required | Default | Meaning |
| --- | --- | --- | --- |
| `MCP_AUTH_TOKENS` | for HTTP | — | Bearer allow-list: `token:client_id[:name]`, comma-separated, 16+ chars each |
| `MCP_TRANSPORT` | no | `http` | `http` (Streamable HTTP) or `stdio` (spawned by a chat client) |
| `HTTP_PORT` | no | `8080` | Listen port |
| `DB_PATH` | no | `./data/do0ps.db` | SQLite job store. Parent directory is created automatically |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, `error` |
| `DO0PS_SERVER_URL` | no | — | Bundle only: bridge to this server instead of running in-process |
| `DO0PS_AUTH_TOKEN` | no | — | Bearer token for `DO0PS_SERVER_URL` |

Queue and timeout tuning (`DO0PS_QUEUE_WORKERS`, `DO0PS_POLL_INTERVAL`, `DO0PS_SHUTDOWN_WAIT`, …) is
documented in [`.env.example`](.env.example) and the [full reference](docs/README.md#7-configuration-reference).

### Two unrelated auth layers

Do not conflate them:

1. **Server access** — a bearer token from `MCP_AUTH_TOKENS`, checked in middleware in front of the MCP
   endpoint. Guards the HTTP listener only; the stdio transport has no listener, so it needs no token.
2. **Provider credentials** — your Parspack API key, supplied per tool call by the chat session. Never stored,
   never logged, never persisted with a job.

---

## Architecture

Hexagonal (ports & adapters). `internal/core` holds domain types and use cases and imports no Fiber, no
`database/sql`, no MCP SDK, and no provider client — only interfaces it defines for itself. Dependencies point
inward, always.

```
cmd/server/          composition root — builds adapters, wires them into core, serves the transport
cmd/mcpb-build/      build tooling — cross-compiles and packs the .mcpb bundles

internal/core/
  domain/            plain Go types: Server, DNSRecord, Job, Operation
  ports/             interfaces core depends on: JobRepository, Queue, ParspackProvider
  app/               use cases: ProvisionServer, SetupDNS, GetOperationStatus, …

internal/adapters/
  mcp/               primary adapter — tool registry, JSON-RPC, both transports
  sqlite/            job store (pure-Go driver, no cgo)
  queue/             channel-based worker pool
  providers/parspack/  the real provider client

internal/auth/       bearer middleware, in front of the MCP adapter
internal/config/     environment loading and the structured logger
```

**Transports.** Streamable HTTP over Fiber v3 for self-hosted deployments, and stdio for installed bundles.
Both go through one dispatcher, so the tool surface is identical either way.

**Jobs and recovery.** Long operations persist to SQLite with attempts and retry scheduling. On startup, jobs
left `pending` or `running` are recovered — and a `running` job is reconciled against the provider before any
retry, because it may have succeeded moments before the crash. Replaying it blindly would create a duplicate
server.

Full write-up: **[docs/README.md](docs/README.md)**.

---

## Development

```bash
make build      # go build ./...
make test       # go test ./...
make vet        # go vet ./...
make lint       # golangci-lint (v2.12, matching CI)
make fmt        # gofmt -l -w .
make mcpb       # build installable .mcpb bundles into dist/mcpb/
```

Every target is a one-line wrapper around a `go` command, so anything here also runs directly with the Go
toolchain when `make` is unavailable — which is how the project is built on Windows.

CI runs `go build`, `go vet`, `go test ./... -race -cover`, and golangci-lint as a parallel job on every pull
request. Conventions worth knowing before opening one:

- Dependency direction is one-way: `adapters → core`, never the reverse
- All comments, log output, and identifiers are in English, whatever language the discussion happened in
- Build and release tooling is written in Go, not shell, so it runs the same on Windows, macOS and Linux
- No secrets in the repository, ever

---

## Releases

| Branch | Publishes | Pre-release | Docker image | MCP bundles |
| --- | --- | --- | --- | --- |
| `master` | `vX.Y.Z` | no | yes | yes |
| `develop` | `vX.Y.Z-RC.N` | yes | no | no |

Versions are derived from [Conventional Commits](https://www.conventionalcommits.org) by
[go-semantic-release](https://github.com/go-semantic-release/semantic-release), and stay in 0.x until 1.0 is a
deliberate decision rather than something a commit message causes.

**`develop` is the active development branch.** Branch from it, open pull requests against it; `master` is
where stable releases are cut.

---

## Documentation

| Document | What it covers |
| --- | --- |
| **[docs/README.md](docs/README.md)** | The full reference — concepts, architecture, every configuration knob, the tool surface, known gaps |
| **[docs/README_FA.md](docs/README_FA.md)** | همان مستند به فارسی |
| **[docs/mcp-bundle.md](docs/mcp-bundle.md)** | Building, installing and troubleshooting the `.mcpb` bundles |
| **[AGENTS.md](AGENTS.md)** | The authoritative guide for humans and coding agents working *on* do0ps |
| **[skills/parspack-infra](skills/parspack-infra)** | End-user Skill: how a chatbot should orchestrate these tools |
| **[docs/api-specs/](docs/api-specs)** | Provider OpenAPI specs, treated as ground truth for adapters |

---

## Contributing

1. Read [AGENTS.md](AGENTS.md) — it is authoritative, and this README summarizes it rather than replacing it.
2. Branch from `develop` and open the pull request against `develop`.
3. Pick an issue labelled `status:ready` and move it to `status:in-progress` before writing code — that label
   is what stops two contributors doing the same work.
4. Keep the dependency direction intact, keep `go vet` and golangci-lint clean, and add tests next to what you
   change. Every package here has them.

Issues and discussion: [github.com/javadib/do0ps/issues](https://github.com/javadib/do0ps/issues).

---

## Status and license

The Parspack adapters, use cases and tools are implemented and unit-tested across every package, `go test ./...`
is green, and the server runs from a binary, from Docker, or as an installed bundle. Known gaps are listed in
[docs/README.md](docs/README.md#14-project-status-and-known-gaps).

**No license file has been committed yet.** The project is intended to be open source; until a `LICENSE` is
added, default copyright applies and reuse is not granted.
