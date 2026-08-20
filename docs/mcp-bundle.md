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

## Building

```sh
make mcpb                                  # all default targets, version from the current git tag
make mcpb VERSION=1.2.3                    # explicit version
make mcpb VERSION=1.2.3 TARGETS=linux/amd64
make mcpb-local                            # just this machine, for a quick install test
```

Bundles land in `dist/mcpb/`. Requirements are Go (the version in `go.mod`) and `zip` — no Node, no `mcpb`
CLI.

Default targets: `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`, `windows/amd64`. The bundle
format distinguishes operating systems but not CPU architectures, so each target gets its own bundle rather
than one archive that might pick the wrong binary on Apple silicon.

The version is stamped into the binary (`-X main.version=...`) and into the manifest from the same value, so
`do0ps --version` and what a client reports after installing always agree.

CI builds bundles on every pull request as a smoke test — packing them, validating each manifest, and driving
a real `initialize` handshake against the packed binary — and `release.yml` attaches them to the GitHub release
after a version is cut — the released bundles are on the repository's
[Releases](https://github.com/javadib/do0ps/releases) page.

## Installing

### Claude Desktop (bundle install)

Download the `.mcpb` for your platform, then either double-click it or go to **Settings → Extensions** and drop
the file in. Claude reads `manifest.json`, registers the tools, and manages the process from then on.

### Any other MCP client

Clients that do not read `.mcpb` files still run the same binary — unzip the bundle and point the client at
`server/do0ps --stdio`. The bundle is a plain zip, so `unzip do0ps-*.mcpb -d ~/do0ps` is all it takes.

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

Use an absolute path in every case: clients spawn the server from a working directory you do not control.

### Self-hosted instead of bundled

If you would rather run one shared server for a team than install a bundle per machine, run the same binary
over HTTP (`docker compose up`, or `do0ps` with no flags) and connect clients to `https://<host>/mcp` with a
bearer token from `MCP_AUTH_TOKENS`. The bundle and the server are the same build; only the transport differs
— see [README.md](README.md) for the self-hosted setup.

`MCP_TRANSPORT=stdio` is the environment-variable equivalent of `--stdio`, for clients that configure
environment rather than arguments.

## Using it

Ask in plain language — "spin up a 2GB Ubuntu server called web-01", "point api.example.com at 1.2.3.4". The
assistant fills in the parameters from the tool schemas.

Provider credentials are **not stored by the server**. You supply your provider API key as a parameter on each
tool call, and it lives only in the chat session (see AGENTS.md §4.2). Nothing is written to disk except job
state.

The bundled server keeps that job state in a SQLite file so a long provisioning operation survives a restart:

| Path | Default |
| --- | --- |
| macOS | `~/Library/Application Support/do0ps/jobs.db` |
| Linux | `~/.config/do0ps/jobs.db` |
| Windows | `%AppData%\do0ps\jobs.db` |

Set `DB_PATH` to move it. (Under HTTP the default stays `./data/do0ps.db`, relative to the working directory
the deployment chose.)

## Troubleshooting

**The extension installs but no tools appear.** Check the client's MCP log. The server writes structured logs
to stderr (never stdout, which carries the protocol), and clients capture stderr into their own log files.

**macOS refuses to run the binary.** The binaries are not code-signed or notarized yet. Right-click the
binary → Open once, or `xattr -d com.apple.quarantine server/do0ps`.

**Verify a bundle by hand** — unpack it and drive a handshake the way a client would:

```sh
unzip -q do0ps-*.mcpb -d /tmp/do0ps-check
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}' \
               '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
  | /tmp/do0ps-check/server/do0ps --stdio
```

You should get an `initialize` result naming the server and version, then the tool list.
