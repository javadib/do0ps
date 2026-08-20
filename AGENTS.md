# AGENTS.md

Instructions for AI coding agents (Claude Code, Codex, or any other agent) working on this repository.

## 1. Project Overview

This project connects to multiple hosting/PaaS providers (starting with Iranian providers) and exposes their
capabilities through an **MCP (Model Context Protocol) server**, so chatbots such as Claude and ChatGPT/Codex
can perform infrastructure tasks (create a server, configure DNS, etc.) on behalf of the user through natural
language.

Two consumer-facing layers sit on top of this server:

1. **MCP Tools** — one tool per implemented provider operation, with a well-described JSON Schema. Most
   natural-language-to-parameter extraction (e.g. "a 2GB RAM server" → `ram_mb: 2048`) is expected to happen
   automatically by the calling model, driven by good tool/parameter descriptions — not by custom NLU code here.
2. **Skill / system prompt (separate artifact, not this file)** — cross-tool orchestration guidance, business
   rules, and workflows (e.g. "look up the DNS zone before creating a subdomain"). Written later, once the
   provider tool list is finalized. Do not conflate this with AGENTS.md — this file is for people/agents
   *building* the project, not for the end-user-facing chatbot behavior.

The project is intended to be **open source**, usable both for personal/team use and by the wider developer/DevOps
community, including non-technical end users who only ever interact with it through a chatbot.

## 2. Tech Stack

- **Language:** Go **1.26.2** — the version `go.mod` pins and the single source of truth. CI reads it via
  `go-version-file: go.mod`; the Dockerfile builder stage must name the same version. If you bump it, bump it
  in `go.mod` and the Dockerfile together, and update this line.
- **HTTP framework:** Fiber **v3** (built on fasthttp, not `net/http`).
- **Persistence:** SQLite, using a **pure-Go driver** (`modernc.org/sqlite`) — do NOT use `mattn/go-sqlite3` or
  any other cgo-based driver. This keeps builds fully static and cross-compilation/Docker builds simple.
- **Queue / background work:** Go channels + an in-process worker pool (goroutines). No Redis, no external
  message broker.
- **Retry/backoff:** a standard backoff library (e.g. `cenkalti/backoff`) wrapping outbound provider HTTP calls.
- **No ORM requirement** — plain SQL (`database/sql`) against SQLite is fine; keep it simple.

## 3. Deployment Target (important constraint)

- Phase 1 target is **self-hosted only**: Docker container, VPS, or a single static binary.
- **Vercel / serverless platforms are explicitly NOT a target** for phase 1. This was a deliberate decision:
  serverless platforms (Vercel and similar) don't support persistent processes, in-memory worker pools, or local
  SQLite file persistence across invocations. Do not introduce code that assumes a serverless runtime model
  (e.g. assuming `/tmp` persists, or that a background goroutine can outlive a single request).
- Do not add external managed services (Redis, managed Postgres, message queues) "just in case" — that
  reintroduces the operational complexity this architecture was specifically designed to avoid.

## 4. Architecture — Hexagonal (Ports & Adapters)

### 4.0 Layering Rule (read this before writing any package)

- **`internal/core`** (domain types + application use cases) must have **zero imports** of Fiber,
  `database/sql`, any MCP SDK, or any provider HTTP client. It only depends on **ports** — interfaces it
  defines for itself.
- **Adapters** depend on core (they implement its ports, or call its use cases). Core never imports an adapter
  package. Dependency direction is always `adapters → core`, never the reverse.
- **Primary/driving adapter:** the MCP tool layer — translates incoming MCP tool calls into calls on core
  application use cases.
- **Secondary/driven adapters:** SQLite job store, the channel-based queue/worker pool, and each provider
  client (Parspack, etc.) — each one implements a port that core defines.

### 4.1 Ports & Provider Adapters

- Core defines **one dedicated port per provider** for now, e.g. `ports.ParspackProvider`, listing exactly the
  operations core needs from that provider — **not** a generic shared `HostingProvider` interface across all
  providers. This preserves the earlier decision to not design a common interface prematurely: hexagonal
  architecture just means each provider's capabilities sit behind *some* interface owned by core, not that
  every provider must implement the *same* interface from day one.
- Each provider's secondary adapter (e.g. `internal/adapters/providers/parspack`) implements that provider's
  dedicated port by calling the real provider API.
- Once 2–3 providers are implemented and real overlap between their ports is visible, refactor toward a shared
  port that multiple provider adapters implement. Do this later, not now.
- Even before that refactor, keep domain data shapes (e.g. a `Server` type, a `DNSRecord` type in
  `internal/core/domain`) as consistent as reasonably possible across providers, so the eventual port
  unification is easier.
- First provider to implement: **Parspack**. Later: ArvanCloud (ابرآروان), Liara.
- **DNS is not a standalone product for Parspack (confirmed, and also true for ArvanCloud):** DNS records are
  managed *inside* the CDN zone/domain configuration, not through a separate DNS API. Do not design a
  standalone `ports.ParspackDNS`-style port — DNS record operations belong on the same `ParspackProvider` CDN
  capability, scoped to a CDN zone (`zone_uuid`). The confirmed endpoint shape lives in
  `docs/api-specs/parspack-cdn.openapi.yaml` (see §4.5) and is spelled out in issue #19.

### 4.2 Authentication (two distinct, unrelated auth layers — do not conflate them)

1. **MCP server access auth:** every incoming request to the MCP server must carry a valid Bearer token from an
   allow-list, checked in Fiber middleware sitting in front of the MCP primary adapter (`internal/auth`, not
   inside `internal/core`). Requests without a valid token are dropped. Reserve a `client_id` field associated
   with each token now (even if unused today) — this keeps the door open for a future multi-tenant/SaaS mode
   without reworking the auth layer.
2. **Provider credentials (API key / secret key):** NOT stored server-side, no credential store, no encryption
   at rest needed for this. The calling chatbot session holds the user's provider credentials and sends them as
   parameters on every relevant MCP tool call. Every tool that talks to a provider must accept credentials as
   explicit input parameters.

### 4.3 Application Use Cases, Ports, and Two Classes of Operations

- `internal/core/app` holds one use case per business operation (e.g. `ProvisionServer`, `SetupDNS`,
  `GetOperationStatus`). The MCP primary adapter's job is thin: parse/validate the tool call, call the matching
  use case, translate the result back to an MCP response. All orchestration logic belongs in the use case, not
  in the MCP handler.
- Core defines two outbound ports for this: **`ports.Queue`** (enqueue/dispatch work) and
  **`ports.JobRepository`** (persist/read job state). The use cases depend only on these ports.
- `internal/adapters/queue` implements `ports.Queue` using Go channels + an in-process worker pool.
  `internal/adapters/sqlite` implements `ports.JobRepository`. Neither the MCP adapter nor the core use cases
  talk to channels or SQL directly — they go through the ports.
- Provider operations fall into two classes; each use case must declare which class it is:
    - **Fast** (seconds — e.g. DNS record changes, listing resources): the use case calls `Queue.Dispatch(...)`
      and blocks on the returned result channel while a worker executes it (via the provider port) with
      retry/backoff. The result returns synchronously — the MCP caller never sees the queue.
    - **Long** (minutes or more — e.g. server provisioning): the use case returns immediately with a generated
      `operation_id` and status `pending` (persisted via `JobRepository`). A worker performs the initial provider
      call (with retry) in the background and polls until the operation reaches a terminal state. A separate
      `GetOperationStatus` use case / `get_operation_status` MCP tool lets the caller check progress later. Never
      block an MCP tool call for minutes — that risks client-side timeouts.
- Retry/backoff around the actual outbound HTTP call is a concern of each provider's secondary adapter (it
  knows that provider's rate limits/quirks); attempt counts and scheduling live in the `Job` domain data via
  `JobRepository`.

### 4.4 SQLite Job Store & Recovery (implements `ports.JobRepository`)

- A `jobs` table persists job state: `id`, `type`, `payload` (JSON), `status`
  (`pending`/`running`/`done`/`failed`), `attempts`, `next_retry_at`, `result`, and a reserved `tenant_id`
  column (unused for now, kept for future multi-tenant support).
- On process startup, any job left in `pending` or `running` status must be recovered.
- **Idempotency / reconciliation is critical:** a `running` job at recovery time might mean the provider call
  actually succeeded right before the process crashed. Before blindly retrying a `running` job's create-style
  call, query the provider to check whether the resource already exists. Never assume a "running" job simply
  needs to be replayed from scratch — that can create duplicate resources on the provider side.

### 4.5 Parspack API Surfaces (confirmed)

Parspack exposes **three separate API surfaces**, same host, same Bearer-token auth scheme, different path
prefixes. Do not assume they share a client config beyond auth — base paths differ:

| Surface | Base URL | Spec file | Phase-1 issue |
| --- | --- | --- | --- |
| Cloud Server (VM/network, Abrha-based) | `https://my.parspack.com/cserver` | not committed (see #9's references — cross-check against `github.com/abrhacom/go-api-abrha`) | #9, #10, #11, #12, #13, #14, #15 |
| CDN (zones, **DNS records live here**, +18 other tags out of phase-1 scope) | `https://my.parspack.com/cdnapi` | `docs/api-specs/parspack-cdn.openapi.yaml` | #19 (in scope), #24 (backlog) |
| SSL (certificate ordering workflow) | `https://my.parspack.com/sslv2` | `docs/api-specs/parspack-ssl.openapi.yaml` | #18 |

The CDN and SSL OpenAPI spec files were provided directly by the project owner and should be treated as
authoritative — prefer them over re-deriving endpoint shapes from `docs.parspack.com`, which is a JS-rendered
SPA that tooling generally cannot scrape.

## 5. MCP Server Details

- Remote transport: Streamable HTTP, served over Fiber.
- **Local transport: stdio**, for the MCP Bundle (`.mcpb`) distribution — the chat client spawns the binary
  itself and talks JSON-RPC over the pipes, so there is no listener and no bearer token in that mode (the OS
  process boundary is the trust boundary). Both transports go through the same `dispatch` in
  `internal/adapters/mcp`; never implement a protocol method for only one of them. Under stdio, stdout is the
  protocol channel — logs go to stderr, and a single stray `fmt.Println` breaks the connection.
- An installed bundle can also run as a **bridge** (`mcp.Proxy`): the user fills a server URL and token into
  the extension's settings, and the binary forwards stdio JSON-RPC to a self-hosted server's `/mcp` instead of
  running the adapters in-process. A bridge builds no job store, queue or provider client — keep it that way,
  and keep both stdio paths on the shared loop in `stdio.go` so framing and concurrency stay identical.
- Use Fiber v3's official **`github.com/gofiber/fiber/v3/middleware/sse`** package for the streaming side of
  the transport (`sse.New(sse.Config{Handler: ...})`) rather than hand-rolling `SetBodyStreamWriter` logic.
  Client disconnect is handled via `stream.Context()`, which is canceled when the stream ends or a write fails.
- Still worth a minimal end-to-end round-trip test early (MCP client ↔ this SSE endpoint) before building the
  rest of the transport layer on top of it, since this middleware is fairly new (Fiber v3.3.0+).
- Every implemented provider operation must be exposed as an MCP Tool with:
    - A clear, unambiguous name and description.
    - A JSON Schema where every parameter has a human-readable description, including units and example values
      (e.g. `ram_mb: "RAM in megabytes, e.g. 2048 for 2GB"`). This is what allows the calling chatbot to correctly
      extract structured parameters from natural language — schema quality matters more than any custom prompt.

## 6. Repository Layout (hexagonal)

```
cmd/server/                    main.go — composition root: builds adapters, wires them into core via ports,
                                starts Fiber. This is the ONLY place allowed to know about every package at once.

internal/core/
  domain/                      Server, DNSRecord, Job, Operation — plain Go types, zero external dependencies
  ports/                       interfaces core depends on: JobRepository, Queue, per-provider ports
                                (e.g. ParspackProvider)
  app/                         use cases / application services: ProvisionServer, SetupDNS,
                                GetOperationStatus, ...

internal/adapters/
  mcp/                         primary adapter — MCP tool registration/handlers, calls into core/app
  sqlite/                      secondary adapter — implements ports.JobRepository
  queue/                       secondary adapter — channel-based worker pool, implements ports.Queue
  system/                      secondary adapter — wall clock and ID generation (ports.Clock, ports.IDGenerator)
  providers/
    parspack/                  secondary adapter — implements ports.ParspackProvider against the real API

internal/auth/                 Bearer token middleware, sits in front of the mcp primary adapter

cmd/mcpb-build/                build tooling — cross-compiles the server and packs dist/mcpb/*.mcpb, one
                                bundle per GOOS/GOARCH, with the manifest generated from the mcp adapter's
                                tool registry so the two cannot drift. Not part of the running server.
                                See docs/mcp-bundle.md.

docs/api-specs/                OpenAPI specs for external Parspack APIs (see §4.5) — reference material, not
                                code Claude generates; treat as ground truth for adapter implementation.
```

## 7. Coding Conventions

- **Dependency direction is one-way: `adapters → core`, never `core → adapters`.** If you find yourself adding
  an import of Fiber, `database/sql`, or a provider SDK inside `internal/core`, stop — that logic belongs in an
  adapter, or core needs a new port instead.
- Format with `gofmt`/`goimports`; keep `go vet` (and ideally `golangci-lint`) clean.
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`. Avoid panics in library code — return errors.
- **All comments, log output, and identifiers must be in English**, regardless of the language used in project
  discussions or issue descriptions.
- **Build and release tooling is written in Go, not shell.** Contributors build on Windows as well as macOS
  and Linux, and a `.sh` file needs a POSIX shell plus whatever binaries it shells out to (`zip`, `make`) that
  Windows does not have. A `go run ./cmd/<tool>` command works identically on all three with only the
  toolchain the project already requires — and it can be tested. Makefile targets are welcome as thin
  conveniences over those commands, never as the only way to run them.
- No secrets or credentials committed to the repository, ever — provider credentials only ever flow through
  MCP tool call parameters at runtime (see §4.2).

## 8. Explicitly Open / Not Yet Decided — do not assume an answer

- **Monolith vs. microservice(s) / module boundaries** for the long term: undecided. Build phase 1 as a single
  deployable service; do not prematurely split into multiple services or over-engineer module boundaries for a
  hypothetical microservice future.
- **Multi-tenant SaaS / dashboard**: a possible future direction, not committed to. Auth and job-store schema
  leave room for it (see `client_id` / `tenant_id` above), but do not build tenant UI, billing, or onboarding
  flows now.
- **Integration with other existing systems** (e.g. an existing agent/software-factory system): not decided.
  This project should remain a clean, independently usable subsystem exposed via the standard MCP protocol —
  do not add ad hoc coupling to any other system.

## 9. Branching — read this before creating a branch

**The active development branch is `step/ph1`, not `master`.** Phase-1 work happens on `step/ph1`; `master`
is far behind it and is only what releases are cut from.

- **Always branch from `step/ph1`** (`git fetch origin step/ph1 && git checkout -b <name> origin/step/ph1`),
  and open pull requests **against `step/ph1`**.
- Never branch from, or target, `master` or `develop` for feature work. A branch cut from `master` is built on
  a stale tree — it will miss most of the phase-1 code and produce conflicts or duplicate work.
- If a working copy arrives checked out on `master` (a fresh clone often does), do not take that as the base
  to work from — fetch and switch to `step/ph1` first.

## 10. Working from GitHub Issues

Phase-1 work is tracked entirely as GitHub Issues on this repo (`javadib/do0ps`), not as a separate task list
anywhere else. Every issue carries three kinds of labels:

- `phase-1` — always present for phase-1 scope. Issues labeled `backlog` instead (e.g. #24) are explicitly
  post-phase-1 and should not be picked up under this workflow.
- `area:*` — which part of the system it touches (`area:core`, `area:sqlite`, `area:queue`, `area:auth`,
  `area:mcp`, `area:parspack`, `area:infra`, `area:docs`, `area:research`).
- `status:*` — current pickup state:
    - `status:ready` — available, no unmet dependencies, safe to start.
    - `status:in-progress` — an agent is already working on it. **Do not start work on an issue in this
      state**, even if it looks unassigned — this is the mechanism that prevents two agents from doing the same
      work twice out of a shared queue.
    - `status:blocked` — has an explicit unmet dependency (stated in the issue body). Do not start until the
      blocking issue is closed.

**Before writing any code against an issue:** change its label from `status:ready` to `status:in-progress`
first. This is a hard rule, not a suggestion — it's the only thing standing between this repo and duplicate
work when multiple agents pull from the open-issue queue concurrently.

Issue bodies reference dependencies by number (e.g. "Depends on #4"). Treat that as a hard prerequisite — read
the referenced issue and confirm it's actually closed/merged before starting, don't assume the numbering implies
it's done.

When an issue is complete (PR merged, acceptance criteria met), close it. If a PR doesn't fully close an issue,
leave `status:in-progress` and note in a comment what's left, rather than silently reopening the pickup queue to
other agents mid-work.
