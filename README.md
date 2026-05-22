# clapip

**Claude API Proxy** — a lightweight, high-performance Go proxy that exposes an
OpenAI-compatible HTTP API but executes inferences by wrapping the Anthropic
`claude` CLI as a local subprocess.

This lets AI IDEs (Cursor, Continue, and other OpenAI-compatible clients) use a
flat-rate Claude subscription without direct API keys. `clapip` never reads or
stores Anthropic tokens — it relies entirely on the `claude` CLI's native
keychain authentication.

## How it works

1. `clapip` receives a standard OpenAI HTTP request.
2. It flattens the `messages` array into a single prompt and maps any client
   `tools` definitions to native `claude` permissions.
3. It spawns `claude --print` as a subprocess, passing the prompt as an
   explicit argument (never via a shell).
4. It translates the subprocess `stdout` back into OpenAI JSON or SSE.

## Installation

Requires Go 1.21+ and the Anthropic [`claude` CLI](https://claude.com/claude-code)
installed and authenticated (`claude` must be on your `$PATH`).

```sh
go install github.com/nealhardesty/clapip@latest
```

Or build from source:

```sh
make build      # produces ./bin/clapip
```

## Usage

```sh
clapip [flags]
```

| Flag | Alias | Default  | Description                                   |
|------|-------|----------|-----------------------------------------------|
| `--port`        | `-p` | `8999`   | Port to listen on                  |
| `--bind-all`    | `-a` | _(off)_  | Bind to all network interfaces (default: localhost only) |
| `--model`       | `-m` | `sonnet` | Default model when a request omits one        |
| `--claude-path` | `-c` | `claude` | Path to the `claude` CLI binary               |
| `--api-key`     | `-k` | _(none)_ | Bearer token required from clients             |
| `--version`     | `-v` |          | Print version and exit                         |

By default clapip listens on `127.0.0.1` only, so it is reachable solely from
the local machine. Pass `-a`/`--bind-all` to bind to all interfaces (`0.0.0.0`)
when the proxy needs to be reachable from other hosts.

**API key precedence:** `-k` flag → `PROXY_API_KEY` environment variable → no
key required (open proxy).

On startup `clapip` runs `claude --version`; if the CLI is missing or fails it
logs a fatal error and exits.

### Running at login (macOS)

On macOS, `clapip` can be installed as a per-user `launchd` agent so it starts
automatically at login and is restarted if it exits:

```sh
./scripts/install-launchd.sh   # install and start the agent
./scripts/remove-launchd.sh    # stop and uninstall the agent
```

The installer resolves an absolute path to the `clapip` binary, captures the
current `$PATH` (so the agent can locate the `claude` CLI), and writes
`~/Library/LaunchAgents/com.nealhardesty.clapip.plist`. Logs are written to
`~/Library/Logs/clapip.log`. Re-running the installer reinstalls cleanly. The
agent runs `clapip` with default flags (port `8999`, localhost only).

## Endpoints

### `GET /health`
Returns `200` with the current version. Dynamically verifies the `claude`
binary is still reachable and returns `503` if it is not.

### `GET /v1/models`
Returns an OpenAI-formatted list of supported models: `sonnet`, `opus`, `haiku`.

### `POST /v1/chat/completions`
The core inference endpoint. Supports both standard JSON responses and
`stream: true` Server-Sent Events. When an API key is configured, requests must
include an `Authorization: Bearer <key>` header.

#### Example

```sh
curl http://localhost:8999/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"sonnet","messages":[{"role":"user","content":"Hello!"}]}'
```

## Tool proxying

`clapip` is stateless and does not execute tools itself. When a client sends
OpenAI `tools` definitions, the proxy strips them from the prompt and instead
maps recognized tools to native `claude` permissions via `--allowedTools`:

| OpenAI tool                          | claude permission |
|--------------------------------------|-------------------|
| `run_command`, `execute_command`, `bash` | `Bash`        |
| `read_file`, `read`, `view_file`     | `Read`            |
| `edit_file`, `edit`, `apply_diff`    | `Edit`            |
| `write_file`, `create_file`, `write` | `Write`           |

Unrecognized tools are ignored. File and shell operations are delegated to the
`claude` CLI natively.

## Development

`clapip` uses only the Go standard library. Common tasks run through the
Makefile:

```sh
make help          # list all targets
make build         # compile into ./bin
make run           # build and run with default flags
make test          # run all tests with the race detector
make smoke         # run curl smoke tests against a running server
make vet           # run go vet
make fmt           # gofmt the tree
make clean         # remove build artifacts
```

### Curl test scripts

[`scripts/`](scripts/) contains `curl`-based scripts for exercising a running
server (they require `jq`). Each honors the environment variables
`CLAPIP_HOST`, `CLAPIP_PORT`, `CLAPIP_API_KEY`, and `CLAPIP_MODEL`.

```sh
./scripts/health.sh                 # GET /health
./scripts/models.sh                 # GET /v1/models
./scripts/chat.sh "Hello there"     # non-streaming chat completion
./scripts/chat-stream.sh "Count to 5"   # streaming (SSE) chat completion
./scripts/test.sh                   # full smoke test with pass/fail assertions
```

`scripts/test.sh` (also `make smoke`) checks every endpoint plus the error and
authentication paths, and exits non-zero if any check fails. To include the
auth checks, set `CLAPIP_API_KEY` to match the server's key:

```sh
CLAPIP_PORT=8999 CLAPIP_API_KEY=secret ./scripts/test.sh
```

### Versioning & releases

The version is the `Version` constant in [`version.go`](version.go) — the
single source of truth.

```sh
make version       # print the current version
make bump-patch    # increment patch (also bump-minor / bump-major)
make release       # commit, tag, push, and publish a GitHub release
make push          # bump-patch + build + release
```

> `make release` pushes to the current branch and publishes a GitHub release
> via the `gh` CLI. Per project rules, always publish changes with `make push`
> rather than running `git` commands directly.

## Project layout

```
clapip/
  main.go                 entrypoint (thin wrapper)
  version.go              Version constant
  internal/config/        flag and environment parsing
  internal/openai/        OpenAI types and translation helpers
  internal/claude/        claude CLI subprocess runner
  internal/server/        HTTP server and handlers
```

## Changelog

See [CHANGELOG.md](CHANGELOG.md). Current version: **v0.1.0**.
