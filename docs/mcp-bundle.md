# do0ps MCP Bundle

An **MCP Bundle** (`.mcpb`) is a single installable file containing the do0ps MCP server and everything it
needs to run. Users install it into a chat client in one step instead of editing JSON config by hand,
installing Go, or running a server.

Each bundle holds one statically linked binary and a `manifest.json` describing the tools it offers:

```
do0ps-1.2.3-darwin-arm64.mcpb   (a zip)
├── manifest.json               required at the archive root; generated from the tool registry
├── server/
│   └── do0ps                   static binary, no runtime to install (do0ps.exe on Windows)
└── README.md                   this file
```

The bundled server runs over **stdio**: the chat client spawns it as a child process and talks JSON-RPC over
the pipe. That is the same tool surface the self-hosted Streamable HTTP server exposes — both transports share
one dispatcher — so nothing behaves differently depending on how you installed it.

## Two ways to run it

The same bundle covers both, chosen by what you type into the extension's settings:

| | **Local** (settings left empty) | **Connected** (server URL filled in) |
| --- | --- | --- |
| What runs | the whole server, inside the extension | a thin bridge to your own server |
| Needs a server | no | yes — see [README.md](README.md) |
| Needs a token | no | yes |
| Job state | on this machine | on the server, shared |
| Good for | one person, nothing to deploy | a team sharing one deployment and one job history |

Local is the default and needs no configuration at all. Switch to connected when several people should see the
same operations, or when the machine running the chat client should not talk to provider APIs directly.

## Building

### Prerequisites

**The Go toolchain, and nothing else.** The builder is a Go program
([`cmd/mcpb-build`](../cmd/mcpb-build)), not a shell script: it invokes `go build` once per target and writes
each archive with `archive/zip`. There is no bash, no `zip` binary, no `make`, and no Node/`mcpb` CLI in the
path — which is what lets the same command work on Windows, macOS and Linux.

| | Needed |
| --- | --- |
| Go | the version [`go.mod`](../go.mod) pins |
| git | optional — only used to derive the version from a tag; without it you get `0.0.0-dev`, or pass `-version` |
| Anything else | no |

Cross-compilation needs no extra setup either: `CGO_ENABLED=0` is set by the builder, and the SQLite driver is
pure Go, so **any one machine can build the bundles for all five targets**. A Windows laptop produces working
macOS bundles, and the unix execute bit is written into the archive explicitly rather than inherited from the
host filesystem — so a bundle packed on Windows still unpacks into a runnable binary on macOS and Linux.

### Commands

Everything is one command, identical on all three operating systems — PowerShell, cmd, Terminal, bash:

```
go run ./cmd/mcpb-build                             # every released target
go run ./cmd/mcpb-build -targets host               # just this machine, for a quick install test
go run ./cmd/mcpb-build -version 1.2.3
go run ./cmd/mcpb-build -targets linux/amd64,darwin/arm64
go run ./cmd/mcpb-build -print-manifest             # inspect manifest.json without building
```

Run it from anywhere inside the repository — it finds the module root itself.

On macOS and Linux the [`Makefile`](../Makefile) wraps the same commands, if you prefer:

```sh
make mcpb                                  # every released target
make mcpb VERSION=1.2.3 TARGETS=linux/amd64
make mcpb-local                            # -targets host
```

`make` is not standard on Windows; use the `go run` form there. The Makefile targets are thin wrappers, so
nothing is only reachable through them.

### Options

| Flag | Default | Meaning |
| --- | --- | --- |
| `-version` | current git tag, else `0.0.0-dev+<short sha>` | Bundle version, without a leading `v`. Stamped into both the binary and the manifest |
| `-targets` | every target in the table below | Comma-separated `GOOS/GOARCH` pairs, or `host` for this machine |
| `-out` | `dist/mcpb` | Directory to write the bundles into |
| `-print-manifest` | off | Print the `manifest.json` for the first target and exit, building nothing |

Released targets: `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`, `windows/amd64`. The bundle
format distinguishes operating systems but not CPU architectures, so each target gets its own bundle rather
than one archive that might hand an Intel binary to an Apple silicon machine.

The version is stamped into the binary (`-X main.version=...`) and into the manifest from the same value, so
`do0ps --version` and what a client reports after installing always agree.

CI builds bundles on every pull request as a smoke test — packing them, validating each manifest, and driving
a real `initialize` handshake against the packed binary — and `release.yml` attaches them to the GitHub release
after a version is cut. The released bundles are on the repository's
[Releases](https://github.com/javadib/do0ps/releases) page.

## Installing

### Claude Desktop (bundle install)

Download the `.mcpb` for your platform, then either double-click it or go to **Settings → Extensions** and drop
the file in. Claude reads `manifest.json`, registers the tools, and manages the process from then on.

The extension's settings then offer two fields, both optional:

| Setting | Leave empty to… | Fill in to… |
| --- | --- | --- |
| **do0ps server URL** | run the server inside the extension | point at your own server, e.g. `https://do0ps.example.com/mcp` — include the `/mcp` path |
| **Access token** | — | authenticate against it: one token from that server's `MCP_AUTH_TOKENS` |

Neither is your hosting provider's API key. That one is never stored anywhere — you give it to the assistant
per request, and it lives only in the chat session.

Changing either setting restarts the extension; there is nothing else to configure.

### Any other MCP client

Clients that do not read `.mcpb` files still run the same binary — unpack the bundle and point the client at
`server/do0ps --stdio`. A `.mcpb` is a plain zip, so any unzip tool works; rename it to `.zip` first if your
tool insists on the extension.

```sh
# macOS / Linux
unzip do0ps-*.mcpb -d ~/do0ps
```

```powershell
# Windows PowerShell
Expand-Archive -Path .\do0ps-1.2.3-windows-amd64.mcpb -DestinationPath $HOME\do0ps
```

No bearer token is involved in any of these: the allow-list guards the HTTP listener, and there is no listener
here.

**Claude Code:**

```sh
claude mcp add do0ps -- ~/do0ps/server/do0ps --stdio
```

**Codex CLI** (`~/.codex/config.toml`):

```toml
[mcp_servers.do0ps]
command = "/home/you/do0ps/server/do0ps"
args = ["--stdio"]

# Optional: bridge to your own server instead of running one locally.
[mcp_servers.do0ps.env]
DO0PS_SERVER_URL = "https://do0ps.example.com/mcp"
DO0PS_AUTH_TOKEN = "your-token"
```

**Cursor, Windsurf, and other `mcpServers`-style clients:**

```json
{
  "mcpServers": {
    "do0ps": {
      "command": "/home/you/do0ps/server/do0ps",
      "args": ["--stdio"]
    }
  }
}
```

On Windows the same entry points at the `.exe`, with backslashes escaped for JSON:

```json
{
  "mcpServers": {
    "do0ps": {
      "command": "C:\\Users\\you\\do0ps\\server\\do0ps.exe",
      "args": ["--stdio"]
    }
  }
}
```

Use an absolute path in every case: clients spawn the server from a working directory you do not control.

### Self-hosted instead of bundled

If you would rather run one shared server for a team than install a bundle per machine, run the same binary
over HTTP (`docker compose up`, or `do0ps` with no flags) and connect clients to `https://<host>/mcp` with a
bearer token from `MCP_AUTH_TOKENS`. The bundle and the server are the same build; only the transport differs
— see [README.md](README.md) for the self-hosted setup.

`MCP_TRANSPORT=stdio` is the environment-variable equivalent of `--stdio`, for clients that configure
environment rather than arguments. `DO0PS_SERVER_URL` and `DO0PS_AUTH_TOKEN` are what the two settings fields
above map onto, so any client that can set environment variables can use connected mode.

## Using it

Ask in plain language — "spin up a 2GB Ubuntu server called web-01", "point api.example.com at 1.2.3.4". The
assistant fills in the parameters from the tool schemas.

Provider credentials are **not stored by the server**. You supply your provider API key as a parameter on each
tool call, and it lives only in the chat session (see AGENTS.md §4.2). Nothing is written to disk except job
state.

In local mode the bundled server keeps job state in a SQLite file, so a long provisioning operation survives a
restart:

| Path | Default |
| --- | --- |
| macOS | `~/Library/Application Support/do0ps/jobs.db` |
| Linux | `~/.config/do0ps/jobs.db` |
| Windows | `%AppData%\do0ps\jobs.db` |

Set `DB_PATH` to move it. (Under HTTP the default stays `./data/do0ps.db`, relative to the working directory
the deployment chose.) In connected mode nothing is written here at all — the server owns the job store.

## Troubleshooting

**The extension installs but no tools appear.** Check the client's MCP log. The server writes structured logs
to stderr (never stdout, which carries the protocol), and clients capture stderr into their own log files. In
connected mode the first line names the endpoint it is bridging to.

**`spawn … ENOENT` in the log.** The client could not find the binary inside the extension. Every bundle this
repository builds spells the path out as `${__dirname}/server/do0ps` (`.exe` on Windows), so this means the
bundle was hand-edited or built elsewhere — rebuild it with `go run ./cmd/mcpb-build`.

**"rejected the access token" or "cannot reach the do0ps server".** Connected mode is configured but the server
is not answering. The message names the endpoint it tried; check the URL includes `/mcp`, that the token
matches an entry in the server's `MCP_AUTH_TOKENS`, and that the server is reachable from this machine. To fall
back to local mode, clear the server URL field.

**macOS refuses to run the binary.** The binaries are not code-signed or notarized yet. Right-click the
binary → Open once, or `xattr -d com.apple.quarantine server/do0ps`.

**The binary will not start.** Run it yourself: `server/do0ps --version` should print the bundle's version.

**Verify a bundle by hand** — unpack it and drive a handshake the way a client would:

```sh
# macOS / Linux
unzip -q do0ps-*.mcpb -d /tmp/do0ps-check
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}' \
               '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
  | /tmp/do0ps-check/server/do0ps --stdio
```

```powershell
# Windows PowerShell
Expand-Archive .\do0ps-1.2.3-windows-amd64.mcpb -DestinationPath $env:TEMP\do0ps-check -Force
'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}',
'{"jsonrpc":"2.0","id":2,"method":"tools/list"}' |
  & "$env:TEMP\do0ps-check\server\do0ps.exe" --stdio
```

You should get an `initialize` result naming the server and version, then the tool list.
