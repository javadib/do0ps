# do0ps

[![CI](https://github.com/javadib/do0ps/actions/workflows/ci.yml/badge.svg?branch=step/ph1)](https://github.com/javadib/do0ps/actions/workflows/ci.yml)

> An MCP server that lets a chatbot run your infrastructure.

**[فارسی / Persian →](README_FA.md)**

---

## Table of contents

1. [Introduction](#1-introduction)
2. [Concepts](#2-concepts)
3. [Tools and technologies](#3-tools-and-technologies)
4. [Architecture](#4-architecture)
5. [Repository layout](#5-repository-layout)
6. [Installation and setup](#6-installation-and-setup)
7. [Configuration reference](#7-configuration-reference)
8. [The MCP interface](#8-the-mcp-interface)
9. [Connecting an MCP client](#9-connecting-an-mcp-client)
10. [Skills](#10-skills)
11. [Connected MCP servers](#11-connected-mcp-servers)
12. [Provider APIs (Parspack)](#12-provider-apis-parspack)
13. [Development workflow](#13-development-workflow)
14. [Project status and known gaps](#14-project-status-and-known-gaps)
15. [Roadmap](#15-roadmap)
16. [Contributing](#16-contributing)

> **Which branch is this?** Phase-1 development happens on **`step/ph1`**, not on the default branch
> `master`. This document describes `step/ph1`, which is far ahead of `master`. See
> [Development workflow](#13-development-workflow).

---

## 1. Introduction

**do0ps** connects hosting and PaaS providers — starting with the Iranian provider **Parspack** — to chatbots
through the **Model Context Protocol (MCP)**. It exposes each provider capability as an MCP tool, so an
assistant such as Claude or ChatGPT/Codex can perform real infrastructure work on the user's behalf from
natural language: *"give me a 2GB Ubuntu server in Tehran and point api.example.com at it."*

The design bet is that **no custom NLU code is needed**. The calling model already turns "2GB of RAM" into
`ram_mb: 2048` — provided the tool and parameter descriptions are precise enough. So schema quality, not prompt
engineering, is where the effort goes.

Two layers sit on top of the server, and both now exist:

| Layer | What it is | Where it lives |
| --- | --- | --- |
| **MCP tools** | 147 tools across VM, network, CDN and SSL operations, each with a fully described JSON Schema | `../internal/adapters/mcp` |
| **Skill / system prompt** | Cross-tool orchestration and business rules ("look up the DNS zone before creating a subdomain") | [`../skills/parspack-infra/SKILL.md`](../skills/parspack-infra/SKILL.md) |

The project is intended to be open source and usable both by DevOps teams and by non-technical end users who
only ever touch it through a chatbot.

**Current status:** phase 1 is substantially built — the Parspack adapters, use cases and tools are
implemented and unit-tested across every package, `go test ./...` is green, and the server runs from a plain
binary or from Docker. See [Project status and known gaps](#14-project-status-and-known-gaps) for what is
still missing.

---

## 2. Concepts

### MCP (Model Context Protocol)

An open protocol that lets a chatbot discover and call external tools. A client asks the server for its tool
list (`tools/list`), then invokes one with structured arguments (`tools/call`). do0ps is an MCP **server**: it
publishes infrastructure tools and executes them.

### Tools and schema quality

Every provider operation becomes one tool with a JSON Schema in which every property carries a human-readable
description, its unit, and an example — for instance `ram_mb: "RAM in megabytes, e.g. 2048 for 2GB"`. This is
what allows a model to extract correct parameters from a sentence.

### Two unrelated authentication layers

These are constantly confused, so the project keeps them strictly apart:

| | MCP access auth | Provider credentials |
| --- | --- | --- |
| **Protects** | The do0ps server itself | Your Parspack account |
| **Carried in** | `Authorization: Bearer <token>` header | Tool call arguments (`api_key`, `secret_key`) |
| **Configured by** | The operator, via `MCP_AUTH_TOKENS` | The end user, held by their chatbot session |
| **Stored server-side** | Yes — as SHA-256 digests, in memory | **Never.** No credential store, no encryption at rest, never written to the database or logs |

Provider credentials arrive on every provider-touching call, are used, and are dropped. `ProviderCredentials`
even overrides `String()`/`GoString()` to render as `REDACTED`, so an accidental `%v` cannot leak them.

### Fast versus long operations

Every use case declares which class it belongs to:

- **Fast** (seconds — DNS changes, listings, most CDN settings): the use case dispatches the work onto a worker
  and blocks on the result inside the same tool call. The caller never learns a queue exists.
- **Long** (minutes — server provisioning, load balancer creation, snapshots, VM restore): the tool returns
  immediately with an `operation_id` and status `pending`. A worker performs the provider call in the
  background and polls until the resource is ready. The caller checks progress with `get_operation_status`.

Four job types are long: `provision_server`, `provision_load_balancer`, `create_snapshot`, `restore_vm`.
Long operations never block the tool call, because a multi-minute MCP response is a client-side timeout
waiting to happen.

### Jobs, operations, and reconciliation

A **job** is the internal record (status, attempts, retry schedule, payload); an **operation** is the smaller,
caller-facing projection of it (`pending` / `running` / `succeeded` / `failed`, plus a result). Jobs are
persisted in SQLite so a restart does not lose them.

**Idempotency is treated as a correctness problem, not an optimization.** A job found in `running` state at
startup might mean the provider call actually succeeded a moment before the crash. Replaying it would create a
duplicate server that someone pays for. So do0ps never blindly retries:

1. At startup, `Recovery` marks every unfinished job as *interrupted* and leaves it non-terminal.
2. Credentials to check with the provider are gone (they were never persisted, by design).
3. The next `get_operation_status` call that carries credentials reconciles the job — it asks the provider
   whether the resource exists, then settles the operation as succeeded or as safely retryable.

Retries within a live process work the same way: `createOrAdopt` looks for an existing resource by name before
issuing a second create.

### Retry and backoff, at two levels

- **Inside one provider call** — the Parspack client retries a request that failed with a retryable status,
  with exponential backoff, before giving up.
- **Around a whole background job** — the worker pool retries a failed job handler up to `maxAttempts`
  (default **5**), persisting `Reschedule(...)` with the next attempt time and re-enqueueing it on a timer.
  Because `NextRetryAt` is persisted before the wait, a crash mid-backoff does not lose the retry.
- **A recovery sweep** every 30s (`WithSweepInterval`) re-reads the store via `JobRepository.ListDue` and
  re-enqueues pending work the pool has dropped — which happens when re-enqueueing a retry hits a full queue,
  leaving a job with no timer behind it. The sweep deliberately skips *interrupted* jobs: those lost their
  credentials to a restart, so running them would fail on authentication, burn the retry budget, and end in a
  terminal failure — destroying the reconciliation path above. They are the caller's to resolve through
  `get_operation_status`.

A job's credentials are held in memory for as long as it can still be re-attempted, and released through a
`ports.JobSettled` callback the pool invokes once the job reaches a terminal state — never between attempts.

### Ports and adapters (hexagonal architecture)

Core owns interfaces; adapters implement them. The dependency arrow points **inward only** — `../internal/core`
imports no Fiber, no `database/sql`, no MCP SDK, no provider client. See [Architecture](#4-architecture).

---

## 3. Tools and technologies

| Concern | Choice | Why |
| --- | --- | --- |
| Language | **Go** (`../go.mod` pins `go 1.26.2`) | Static binaries, easy self-hosting |
| HTTP | **Fiber v3** (`github.com/gofiber/fiber/v3` v3.5.0) | fasthttp-based; v3's `middleware/sse` serves the streaming half of the transport |
| Persistence | **SQLite** via **`modernc.org/sqlite`** | Pure-Go driver. cgo drivers (`mattn/go-sqlite3`) are explicitly banned so builds stay static and cross-compilation/Docker stay trivial |
| Queue | Go channels + bounded in-process worker pool | No Redis, no broker, no extra operational surface |
| Retry/backoff | **`github.com/cenkalti/backoff/v4`** in the queue; hand-rolled retry in the provider client | Job-level and request-level retry are different concerns |
| Config | Environment variables + **`github.com/joho/godotenv`** | 12-factor, with a `.env` for local runs |
| Logging | `log/slog`, level-configurable | Structured logs, standard library |
| IDs | `crypto/rand`, 128-bit hex | Operation IDs are handed to callers, so they must not be guessable |
| Tests | `go test ./...` — every package has tests | Race detector on in CI |
| Lint | `go vet` + **golangci-lint v2.12** with a committed `../.golangci.yml` | Runs as a separate CI job |
| Container | Multi-stage `../Dockerfile`, `CGO_ENABLED=0`, **distroless nonroot** final image | No shell, no libc, no root in the runtime image |
| CI/CD | GitHub Actions | Build/test, lint, semantic release, GHCR image publish |
| Release | **go-semantic-release** | Go-native; avoids pulling a Python/Node toolchain in just to cut releases |

**Deployment target is self-hosted only:** Docker container, VPS, or a single static binary.
**Vercel and serverless platforms are explicitly not a target.** They have no persistent process, no
in-memory worker pool, and no durable local SQLite file — every architectural decision here assumes those
exist. Do not write code that assumes a serverless runtime (that `/tmp` persists, or that a goroutine can
outlive a request).

Equally deliberate: no managed Redis, Postgres, or message queue is added "just in case." Avoiding that
operational complexity is the point of this design.

---

## 4. Architecture

do0ps follows **hexagonal architecture (ports & adapters)**.

```
                          ┌──────────────────────────────────┐
   MCP client             │           internal/core          │
   (Claude / Codex)       │                                  │
        │                 │  domain/   pure types & rules    │
        ▼                 │  ports/    interfaces core owns  │
  ┌──────────┐  Bearer    │  app/      use cases             │
  │  Fiber   │  auth      │                                  │
  │ + auth   │────────────┼──► ProvisionServer   (long)      │
  └──────────┘            │    CreateSnapshot    (long)      │
        │                 │    RestoreVM         (long)      │
        ▼                 │    ProvisionLoadBalancer (long)  │
  ┌──────────┐            │    ~60 fast use cases            │
  │   mcp    │  primary   │    GetOperationStatus / Recovery │
  │ adapter  │────────────└────┬──────────┬──────────┬───────┘
  │ 147 tools│                 │          │          │
  └──────────┘         ports.Queue  ports.JobRepository
        │                        │          │   ports.ParspackProvider
   POST /mcp  (JSON-RPC)         ▼          ▼               │
   GET  /mcp  (SSE)         ┌────────┐ ┌────────┐    ┌──────▼──────┐
                            │ queue  │ │ sqlite │    │  parspack   │
                            │ pool   │ │ store  │    │  client     │
                            └────────┘ └────────┘    └─────────────┘
                                 secondary (driven) adapters
```

**The layering rule.** `../internal/core` has zero imports of Fiber, `database/sql`, any MCP SDK, or any provider
HTTP client. It depends only on interfaces it defines for itself in `../internal/core/ports`. Adapters depend on
core; core never imports an adapter. If you find yourself adding such an import inside core, the logic belongs
in an adapter — or core needs a new port.

**One port per provider, for now.** Core defines `ports.ParspackProvider` — a large interface (~157 methods)
listing exactly the operations it needs. It is deliberately *not* a generic `HostingProvider` shared across
every provider; a common interface gets designed once two or three providers make the real overlap visible.
Meanwhile the domain data shapes (`Server`, `DNSRecord`, `CDNZone`, `SSLOrder`, …) are kept consistent, so
that eventual unification is a mechanical change rather than a data-model rewrite.

**Shared long-operation scaffolding.** `../internal/core/app/longop.go` holds the machinery common to long
operations — the memory-only credentials map and the polling knobs — used by `CreateSnapshot` and `RestoreVM`.
`ProvisionServer` predates that helper and still carries its own copy.

**The composition root.** `../cmd/server/main.go` is the only file allowed to know about every package at once.
It builds the adapters, injects them into ~70 use cases through ports, registers the four job handlers, runs
startup recovery, and starts Fiber. Shutdown is bounded by one signal-derived context: Fiber stops accepting
requests, then the worker pool drains, then the database closes.

---

## 5. Repository layout

```
cmd/server/                    main.go — composition root (+ main_test.go, an end-to-end server test)

internal/config/               env-var loading + slog construction. Plain glue: no Fiber, no adapters, no core
internal/auth/                 Bearer token middleware, sits in front of the mcp adapter

internal/core/
  domain/                      Server, DNSRecord, CDNZone (+ 10 more cdn_*.go), SSL types, Job, Operation,
                                 ProviderCredentials, sentinel errors — plain Go types, no dependencies
  ports/                       Queue, JobRepository, ParspackProvider, Clock, IDGenerator
  app/                         ~70 use cases, one file per business operation, each with a _test.go;
                                 longop.go holds the shared long-operation scaffolding

internal/adapters/
  mcp/                         primary adapter — tools.go plus 13 cdn_*/ssl_*/vpc_*/snapshot_* tool files,
                                 JSON-RPC framing, SSE stream, Fiber routes
  sqlite/                      job store + migrations/0001_jobs.sql
  queue/                       bounded channel worker pool with backoff-driven job retry
  system/                      wall clock and ID generation
  providers/
    parspack/                  client.go (three API surfaces, auth, retry, error mapping) plus one file per
                                 capability area (vms, keys, firewalls, loadbalancers, snapshots, vpcs,
                                 reserved_ips, ssl, cdn*), each with tests

skills/parspack-infra/         the end-user Skill: orchestration guidance for the chatbot (§10.2)
docs/api-specs/                Parspack OpenAPI specs — reference material, treated as ground truth
.claude/skills/, .agents/skills/   vendored coding-agent skills (§10.1)
.github/workflows/             ci, release, docker-publish, jiffy, close-linked-issues
Dockerfile, docker-compose.yml, Makefile, .golangci.yml, .env.example
AGENTS.md                      the authoritative guide for agents/contributors working on this repo
CLAUDE.md                      a pointer to AGENTS.md, so the two never drift
```

---

## 6. Installation and setup

Configuration comes from the environment. A `.env` file in the working directory is loaded when present and
is the convenient way to run locally; it is optional, so containers and CI can supply the variables directly.

### Prerequisites

- **Go 1.26.2** — the version `../go.mod` pins, CI reads via `go-version-file: go.mod`, and the Dockerfile
  builder stage names. Bump the three together
- No cgo toolchain, no external database, no message broker
- Optional: Docker + Compose, `golangci-lint` v2.12 for linting

### Generate an access token

Tokens must be at least **16 characters**; the server refuses to start otherwise.

```bash
openssl rand -hex 32
```

### Quick start: go run / binary

```bash
git clone https://github.com/javadib/do0ps.git
cd do0ps
git checkout step/ph1

cp .env.example .env
# edit .env and set MCP_AUTH_TOKENS, e.g.
#   MCP_AUTH_TOKENS="<32-hex-token>:client-a:Ops Team"

make run          # go run ./cmd/server
```

`make build` compiles everything; for a binary you can copy to a VPS, build it explicitly:

```bash
go build -o do0ps ./cmd/server
```

Run it on the target host with the same environment variables, from a `.env` file or exported directly.
`make test`, `make vet` and `make lint`
run the test suite, `go vet`, and golangci-lint; `make install-tools` installs the exact golangci-lint version
CI uses.

Startup emits a log line such as:

```
level=INFO msg=listening addr=:8080 tools=147 version=dev
```

### Quick start: Docker Compose

```bash
cp .env.example .env
# edit .env — at minimum set MCP_AUTH_TOKENS
docker compose up -d --build
```

This builds the `runner` target from the committed `../Dockerfile` and starts do0ps on `HTTP_PORT` (8080 by
default), with the SQLite job store on the `do0ps-data` named volume. Losing that volume loses job history, so
back it up like any other stateful container.

The image is `gcr.io/distroless/static-debian12:nonroot` — no shell, no package manager, runs as uid 65532.
That is why `../docker-compose.yml` defines no container healthcheck: there is no `curl` or `wget` inside to run
one. Probe `GET /healthz` from outside instead.

`.env` is listed in `../.dockerignore`, so the image never carries your secrets; Compose passes the variables in
through `env_file` instead.

### Verify

```bash
export TOKEN=<the token half of your MCP_AUTH_TOKENS entry>

# Liveness — deliberately outside the token allow-list, so orchestrators can probe it
curl -s localhost:8080/healthz
# {"status":"ok"}

# Tool discovery — requires a valid bearer token
curl -s localhost:8080/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Smoke-test the transport end to end with the built-in no-op tool
curl -s localhost:8080/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ping","arguments":{}}}'
```

A missing or unknown token gets `401` with a deliberately uninformative body — the server never hints at *why*
a token was rejected.

---

## 7. Configuration reference

All configuration is read from environment variables, with `.env` loaded at startup.
[`.env.example`](../.env.example) is the source of truth for defaults; this table summarizes it.

| Variable | Required | Default | Meaning |
| --- | --- | --- | --- |
| `MCP_AUTH_TOKENS` | **yes** for HTTP | — | Bearer allow-list: `token:client_id[:name]`, comma-separated. Tokens must be ≥16 chars. Not used under `stdio`, which has no listener to guard |
| `MCP_TRANSPORT` | no | `http` | `http` (Streamable HTTP over Fiber) or `stdio` (the chat client spawns this binary — how an installed MCP bundle runs). The `--stdio` flag does the same |
| `DB_PATH` | no | `../data/do0ps.db` | SQLite job-store file. Its parent directory is created automatically. Under `stdio` the default moves to the per-user config directory |
| `HTTP_PORT` | no | `8080` | Port the server listens on |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, or `error` |
| `DO0PS_QUEUE_WORKERS` | no | `8` | Worker goroutines in the pool |
| `DO0PS_QUEUE_DEPTH` | no | `256` | Bounded work-channel depth. When full, work is rejected rather than buffered without limit |
| `DO0PS_POLL_INTERVAL` | no | `10s` | How often a long operation polls the provider (Go duration syntax) |
| `DO0PS_POLL_TIMEOUT` | no | `20m` | Cap on a single long-operation job before it is recorded as failed |
| `DO0PS_SHUTDOWN_WAIT` | no | `30s` | How long graceful shutdown waits for in-flight jobs to drain |

Example:

```
MCP_AUTH_TOKENS="change-me-0123456789:client-a,change-me-9876543210:client-b"
```

**`client_id` is reserved.** It is parsed and attached to each authenticated request today but otherwise
unused — it exists so a future multi-tenant mode does not require reworking the auth layer. The `jobs` table
carries an equivalent unused `tenant_id` column for the same reason.

---

## 8. The MCP interface

### Transport

Streamable HTTP over Fiber, on one path, with both halves implemented:

| Route | Auth | Purpose |
| --- | --- | --- |
| `GET /healthz` | none | Liveness probe |
| `POST /mcp` | Bearer | JSON-RPC 2.0 request/response |
| `GET /mcp` | Bearer | SSE stream for server-initiated messages |

The SSE half emits a `ready` event (`{"protocol":"mcp-streamable-http"}`) and then holds the connection open,
using `stream.Context()` to notice a disconnect. No use case pushes anything down the stream yet — it proves
the transport round-trip rather than carrying traffic.

JSON-RPC methods currently answered: **`tools/list`** and **`tools/call`**. Anything else returns `-32601`.

### Error mapping

Failures are translated from domain sentinels to JSON-RPC codes, so callers get something actionable while
internals stay private:

| Condition | Code | Body |
| --- | --- | --- |
| Malformed JSON-RPC | `-32600` | `malformed JSON-RPC request` (HTTP 400) |
| Unknown method or unknown tool | `-32601` / `-32602` | `unsupported method X` / `unknown tool X` |
| Invalid input, bad provider credentials, not found | `-32602` | The specific error message |
| Anything else | `-32603` | `tool X failed` — the full error is logged server-side, not returned |

### The tool inventory

**147 tools** are registered. Every provider-touching tool accepts `api_key` and, where the surface uses a key
pair, `secret_key`.

| Family | Tools | Examples |
| --- | --- | --- |
| CDN rule engines | 17 | `create_cdn_origin_rule`, `update_cdn_page_rule`, `toggle_cdn_transform_rule` |
| CDN ModSec WAF | 12 | `update_cdn_modsec_status`, `create_cdn_modsec_rule` |
| CDN edge load balancing | 10 | `create_cdn_load_balance`, `update_cdn_load_balance_server` |
| CDN zone settings | 10 | `update_cdn_antivirus_status`, `update_cdn_developer_mode`, `update_cdn_dnssec_status` |
| CDN edge firewall | 9 | `create_cdn_access_rule`, `update_cdn_ip_reputation`, `update_cdn_ddos_actions` |
| CDN logs & analytics | 8 | `get_cdn_access_log`, `get_cdn_waf_log`, `get_cdn_monthly_traffic_usage` |
| CDN network settings | 8 | `update_cdn_https_convertor`, `update_cdn_web_socket`, `update_cdn_www_redirection` |
| SSL certificate ordering | 8 | `create_ssl_order`, `process_ssl_order`, `verify_ssl_challenge`, `reissue_ssl_certificate` |
| CDN cache | 7 | `purge_cdn_cache`, `update_cdn_cache_ttl`, `list_cdn_cache_entries` |
| CDN bulklists | 6 | `create_cdn_bulklist`, `list_cdn_firewall_countries` |
| CDN rate limiting | 6 | `create_cdn_rate_limit_rule`, `update_cdn_rate_limit_rule_priority` |
| CDN zones | 5 | `create_cdn_zone`, `list_cdn_zones`, `list_cdn_plans` |
| CDN zone SSL | 5 | `update_cdn_min_tls_version`, `update_cdn_hsts`, `list_cdn_certificates` |
| DNS records | 5 | `create_dns_record`, `update_dns_record`, `get_nameserver_records` |
| VM firewalls | 5 | `create_firewall`, `update_firewall` |
| VM load balancers | 5 | `create_load_balancer` (long), `update_load_balancer` |
| Reserved IPs | 4 | `reserve_ip`, `assign_ip_to_server`, `release_ip` |
| Snapshots & restore | 4 | `create_snapshot` (long), `restore_vm` (long) |
| VM lifecycle | 4 | `create_server` (long), `list_servers`, `get_server`, `delete_server` |
| VPCs | 4 | `create_vpc`, `list_vpcs` |
| SSH keys | 3 | `register_ssh_key`, `list_ssh_keys` |
| Operations | 1 | `get_operation_status` |
| Transport smoke test | 1 | `ping` — a built-in no-op tool with no use case or provider behind it |

Two firewall families and two load-balancer families exist and **must not be conflated**: the `*_firewall` /
`*_load_balancer` tools act on the Abrha-based cloud-server network, while the `*_cdn_*` equivalents act at
the CDN edge. They live on different API surfaces.

### Example: a long operation, end to end

```bash
TOKEN=<your do0ps token>

# 1. Start it — returns immediately
curl -s localhost:8080/mcp -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{
  "jsonrpc":"2.0","id":1,"method":"tools/call",
  "params":{"name":"create_server","arguments":{
    "api_key":"<provider key>","name":"web-01","region":"tehran",
    "image":"ubuntu-24.04","cpu_cores":2,"ram_mb":2048,"disk_gb":40}}}'

# 2. Poll it. Passing api_key lets this call reconcile with the provider
#    if the server restarted while the operation was in flight.
curl -s localhost:8080/mcp -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{
  "jsonrpc":"2.0","id":2,"method":"tools/call",
  "params":{"name":"get_operation_status","arguments":{
    "operation_id":"<id from step 1>","api_key":"<provider key>"}}}'
```

---

## 9. Connecting an MCP client

There are two ways to connect, and they serve the same 147 tools — the transports share one dispatcher.

### Option A: install the MCP Bundle (no server to run)

Download the `.mcpb` for your platform from
[Releases](https://github.com/javadib/do0ps/releases) and install it into your chat client — in Claude
Desktop, double-click it or drop it into **Settings → Extensions**. The client spawns the bundled binary over
stdio and manages it from then on; there is no host, no port and no bearer token involved.

For clients that do not read `.mcpb` files, unzip the bundle and point them at `server/do0ps --stdio`.
[docs/mcp-bundle.md](mcp-bundle.md) has the per-client configuration and the build instructions.

### Option B: connect to a self-hosted server

Point your client at the `/mcp` route with a bearer token from `MCP_AUTH_TOKENS`:

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

Put that in your client's MCP configuration (for Claude Code, a `.mcp.json` in the project or the user-level
config; for Claude Desktop, `claude_desktop_config.json`). Terminate TLS in front of the server — the token is
a bearer credential and must never cross a plaintext connection.

Provider credentials are **not** configured here. The chatbot session supplies them per tool call.

---

## 10. Skills

"Skill" means two different things in this project. Keep them apart.

### 10.1 Coding-agent skills — for people building do0ps

The repo vendors two Go skills from [`samber/cc-skills-golang`](https://github.com/samber/cc-skills-golang) so
that any agent working on this codebase writes consistent Go:

| Skill | What it covers |
| --- | --- |
| `golang-code-style` | Line length and breaking, variable declarations, control-flow clarity, when a comment helps versus hurts |
| `golang-design-patterns` | Functional options, constructor APIs, error flow, resource lifecycle, graceful shutdown, resilience, dependency injection, hexagonal/clean architecture references |

They are checked in twice, at `../.claude/skills` and `../.agents/skills`, so both Claude Code and other agent
tooling pick them up from their own conventional location. `../skills-lock.json` at the repo root pins each one
by source repo, path, and content hash — that is what makes an update visible in a diff instead of silent.

Their fingerprints show up all over the codebase: functional options with eager validation (`WithWorkers`,
`WithPollTimeout`, `WithCDNBaseURL` all fail at construction rather than under load), constructors returning
`(T, error)`, wrapped errors with `%w`, and a shutdown path that drains workers under a bounded context.

### 10.2 The end-user Skill — for the chatbot using do0ps

[`../skills/parspack-infra/SKILL.md`](../skills/parspack-infra/SKILL.md) is the artifact that tells the *consuming*
assistant how to behave. It is written for a model talking to a non-technical user, and covers:

- A tool table marking each tool **fast** or **long**
- Rules that apply to every call (never dump raw tool output; summarize)
- The `operation_id` + polling pattern for long operations
- Per-area workflows: creating a server, SSH keys, CDN zones and DNS records, firewalls, load balancers,
  reserved IPs, VPCs, snapshots and restore, SSL certificates
- How to hand errors to the user

This is deliberately **not** AGENTS.md: AGENTS.md is for people and agents *building* this project, while the
Skill governs end-user-facing chatbot behavior.

Note that the Skill documents the phase-1 tool set (roughly 50 tools) and has not been extended to cover the
CDN capabilities added later under issue #24 — so a chatbot loading it gets guidance for the VM/DNS/SSL core,
but none for the ~90 CDN edge tools.

---

## 11. Connected MCP servers

### 11.1 The MCP server this project exposes

do0ps is itself an MCP server — see [The MCP interface](#8-the-mcp-interface) for its 147 tools and
[Connecting an MCP client](#9-connecting-an-mcp-client) for wiring it into a chatbot.

### 11.2 MCP servers used while developing do0ps

No `.mcp.json` is committed, so nothing is auto-connected by cloning this repo — these are configured
per-developer or per-agent-environment. The ones used in this project's agent workflow:

| Server | Used for |
| --- | --- |
| **GitHub MCP** | Reading and writing issues, pull requests, reviews, branches, and CI status. This is how the issue-driven workflow in §13 is actually executed |
| **Notion MCP** | Project notes and documentation pages kept outside the repo |
| **Google Drive MCP** | Provider documents and specs shared by the project owner |

If you want the same setup, add them to your own client configuration; nothing in the build depends on them.

### 11.3 Jiffy

`../.github/workflows/jiffy.yml` forwards any issue or comment mentioning `@jiffy` to a self-hosted Jiffy
gateway, which turns the issue thread into a pull request. It is gated by a `JIFFY_USER_WHITELIST` repository
variable — with no whitelist configured, the workflow posts setup instructions on the issue and stops instead
of dispatching.

---

## 12. Provider APIs (Parspack)

Parspack exposes **three separate API surfaces** — same host, same Bearer-token scheme, different path
prefixes. The client holds all three base URLs separately, each overridable for tests:

| Surface | Base URL | Client option | Spec in this repo |
| --- | --- | --- | --- |
| Cloud Server (VM/network, Abrha-based) | `https://my.parspack.com/cserver` | `WithBaseURL` | Not committed — cross-check against `github.com/abrhacom/go-api-abrha` |
| CDN (zones — **DNS records live here**) | `https://my.parspack.com/cdnapi` | `WithCDNBaseURL` | `api-specs/parspack-cdn.openapi.yaml` |
| SSL (certificate ordering workflow) | `https://my.parspack.com/sslv2` | `WithSSLBaseURL` | `api-specs/parspack-ssl.openapi.yaml` |

The two committed OpenAPI specs came directly from the project owner and are **authoritative** — prefer them
over re-deriving endpoint shapes from `docs.parspack.com`, which is a JS-rendered SPA that tooling generally
cannot scrape.

**DNS is not a standalone product at Parspack** (and the same holds for ArvanCloud): DNS records are managed
*inside* the CDN zone configuration, scoped to a `zone_uuid`, not through a separate DNS API. That is why
there is no `ports.ParspackDNS` — DNS operations sit on `ParspackProvider` alongside the CDN capability.

Providers planned after Parspack: **ArvanCloud** (ابرآروان) and **Liara**.

---

## 13. Development workflow

### Branching

The repository's default branch is `master`, but **phase-1 work is developed and merged on `step/ph1`**.
That has one consequence worth knowing: GitHub's native "Closes #N" auto-close only fires for the default
branch, so `../.github/workflows/close-linked-issues.yml` re-implements the closing-keyword scan for merges into
non-default branches. Without it, an issue merged on `step/ph1` would stay open and `status:in-progress`,
silently deadlocking every issue that lists it as a dependency.

### Conventions

- Dependency direction is one-way: `adapters → core`, never the reverse
- Format with `gofmt`/`goimports` (`../.golangci.yml` sets the local import prefix); keep `go vet` and
  golangci-lint clean
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`. Avoid panics in library code — return errors
- **All comments, log output, and identifiers are in English**, regardless of the language used in issues and
  project discussion
- No secrets or credentials in the repository, ever. Provider credentials only ever flow through MCP tool call
  parameters at runtime

### Working from GitHub issues

Phase-1 work is tracked entirely as GitHub issues on `javadib/do0ps`. Each issue carries three label kinds:

- `phase-1` — in scope. `backlog` instead means post-phase-1
- `area:*` — `core`, `sqlite`, `queue`, `auth`, `mcp`, `parspack`, `infra`, `docs`, `research`
- `status:*` — `ready` (safe to start), `in-progress` (**someone is already on it — do not start**),
  `blocked` (has an unmet dependency stated in the body)

**Before writing any code against an issue, move its label from `status:ready` to `status:in-progress`.** This
is a hard rule: it is the only thing preventing two agents pulling from the same open-issue queue from doing
the same work twice. Dependencies referenced as "Depends on #4" are hard prerequisites — confirm the
referenced issue is actually closed rather than assuming the numbering implies it.

### CI

| Workflow | Trigger | What it does |
| --- | --- | --- |
| `ci.yml` | PR / push to `master` or `step/ph1` | `go build`, `go vet`, `go test ./... -race -cover`; golangci-lint v2.12 as a parallel job |
| `release.yml` | push to `master` | go-semantic-release; on a release, calls the docker workflow |
| `docker-publish.yml` | `v*.*.*` tag, manual, or called by release | Builds the `runner` target with `APP_VERSION` and pushes to GHCR |
| `jiffy.yml` | issue opened / comment containing `@jiffy` | Dispatches the issue thread to the Jiffy gateway |
| `close-linked-issues.yml` | PR merged into a non-default branch | Closes issues referenced by closing keywords |

### Testing

```bash
make test          # go test ./...
go test ./... -race -cover
```

Every package now has tests, including an end-to-end server test in `../cmd/server/main_test.go` and per-file
adapter tests for each Parspack capability area. **One package currently fails** — see below.

---

## 14. Project status and known gaps

| Area | Status |
| --- | --- |
| Hexagonal structure, domain, ports | Implemented |
| ~70 use cases across VM, network, CDN, SSL | Implemented, unit tested |
| SQLite job store + migration + startup recovery | Implemented, tested |
| Worker pool: bounded, graceful drain, backoff job retry (max 5 attempts), recovery sweep over `ListDue` | Implemented, tested |
| Bearer auth middleware (hashed allow-list, constant-time compare) | Implemented, tested |
| MCP tool registry: 147 tools, `tools/list` + `tools/call` | Implemented, tested |
| Streamable HTTP: POST + SSE `GET /mcp` | Implemented, tested |
| stdio transport (`--stdio`), for installed MCP bundles | Implemented, tested |
| Parspack adapter across all three API surfaces | Implemented, tested against fake servers |
| Dockerfile (distroless, nonroot, static) + docker-compose | Committed |
| Makefile, `../.golangci.yml`, `.env.example` | Committed |
| End-user Skill (`../skills/parspack-infra`) | Covers all 147 tools |
| MCP `initialize` handshake, `ping` | Implemented on both transports |
| stdio transport + `.mcpb` bundle build | Implemented, smoke-tested in CI |
| LICENSE | Not committed yet, despite the open-source intent |

### Recently resolved

The following were fixed on this branch and are recorded here so the change is easy to trace:

- **`.env` was mandatory.** `config.Load()` called `godotenv.Load()` and `log.Fatal`'d on a missing file,
  which failed `go test ./...`, kept CI red, and prevented the Docker image from starting at all (`.env` is
  dockerignored, so the image can never contain one). Loading is now best-effort: `_ = godotenv.Load()`, with
  the required-value checks that follow deciding whether the configuration is usable.
- **Go version drift.** `../AGENTS.md` said 1.25+ and the Dockerfile built on the floating `golang:1.26-alpine`.
  Both now name **1.26.2**, matching `../go.mod`.
- **`godotenv` was marked indirect** in `../go.mod` despite being imported directly — `go mod tidy` moved it.
- **The `../Makefile`'s `run` comment** named `DO0PS_TOKENS`; the variable is `MCP_AUTH_TOKENS`.
- **The end-user Skill covered only the phase-1 tools.** It now documents all 147, with the CDN edge families
  and a workflow section for them.
- **Retries could never authenticate.** Every long-operation handler released the caller's credentials with a
  `defer` on its first attempt, so attempt 2 failed with `ErrInvalidCredentials`, attempts 3-5 did the same,
  and the job ended terminally `failed` — with the provider having been called exactly once, and the
  reconciliation path closed off. Credentials now survive until the job settles, via `ports.JobSettled`.
- **`JobRepository.ListDue` had no caller.** A recovery sweep now uses it (see
  [Retry and backoff](#2-concepts)), with an explicit guard against touching interrupted jobs.

---

## 15. Roadmap

Phase-1 scope as tracked in the issue tracker:

| Group | Work |
| --- | --- |
| A — foundations | CI & tooling (#1), config + structured logging (#2), domain types (#3), core ports (#4) |
| B — adapters | SQLite store & recovery (#5), queue worker pool (#6), bearer auth (#7) |
| C — transport | MCP bootstrap: Fiber v3 + Streamable HTTP + SSE, tool registry (#8) |
| D/E — Parspack compute | VM lifecycle (#9), SSH keys (#10), firewall (#11), load balancer (#12), reserved IP (#13), VPC (#14), snapshots (#15) |
| F — Parspack CDN & SSL | SSL API spike (#16), CDN API spike (#17), SSL ordering workflow (#18), CDN zones + DNS records (#19) |
| G — shipping | Composition root & graceful shutdown (#20), Dockerfile + compose (#21), README (#22) |
| H — end-user layer | The Skill / system prompt for the tool set (#23) |
| Post-phase-1 | CDN capabilities beyond zone/DNS (#24) — since delivered on `step/ph1` |

Beyond phase 1: ArvanCloud and Liara adapters, and — once two or three providers exist and the real overlap is
visible — a shared provider port replacing the per-provider one. `ports.ParspackProvider` at ~157 methods is
itself an argument for splitting it by capability area when that refactor happens.

### Deliberately undecided

Do not assume an answer to these; they are open by choice:

- **Monolith versus microservices.** Phase 1 is a single deployable service. Do not pre-split it or
  over-engineer module boundaries for a hypothetical future
- **Multi-tenant SaaS / dashboard.** Possible, not committed. `client_id` and `tenant_id` leave room for it;
  do not build tenant UI, billing, or onboarding now
- **Integration with other existing systems.** Undecided. do0ps should stay a clean, independently usable
  subsystem exposed through standard MCP — no ad hoc coupling to anything else

---

## 16. Contributing

1. Read [`../AGENTS.md`](../AGENTS.md) first — it is the authoritative guide for both humans and coding agents, and
   this README summarizes it rather than replacing it. (`../CLAUDE.md` is just a pointer to it, so the two cannot
   drift.)
2. Branch from **`step/ph1`**, not `master`.
3. Pick an issue labelled `phase-1` + `status:ready`, and flip it to `status:in-progress` before writing code.
4. Keep the dependency direction intact, keep `go vet` and golangci-lint clean, and add tests next to what you
   change — every package here has them.
5. Close the issue when the PR is merged and the acceptance criteria are met. If a PR doesn't fully close it,
   leave `status:in-progress` and comment what's left — don't silently reopen the pickup queue mid-work.

No license file has been committed yet; the project is intended to be open source.
