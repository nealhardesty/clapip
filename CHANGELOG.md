# Changelog

All notable changes to clapip are documented in this file.

## Unreleased

### Changed
- The server now binds to `127.0.0.1` (localhost) only by default instead of
  all interfaces, limiting access to the local machine.

### Added
- `-a`/`--bind-all` flag to bind to all network interfaces (`0.0.0.0`) when
  the proxy needs to be reachable from other hosts.
- `scripts/remove-launchd.sh` to stop and uninstall the macOS `launchd` agent.

### Fixed
- `scripts/install-launchd.sh` now uses an absolute path to the `clapip`
  binary, sets the agent's `PATH` so it can locate the `claude` CLI, creates
  `~/Library/LaunchAgents` if missing, writes logs to `~/Library/Logs`,
  enables `KeepAlive`, and is idempotent on re-runs. Previously the agent
  failed to launch because `clapip` was not on `launchd`'s minimal `PATH`.

## v0.1.0

Initial release.

### Added
- OpenAI-compatible HTTP proxy that executes inferences by wrapping the
  Anthropic `claude` CLI as a local subprocess.
- Endpoints: `GET /health`, `GET /v1/models`, `POST /v1/chat/completions`.
- Streaming (SSE) and non-streaming chat completion responses.
- Startup verification of the `claude` CLI via `claude --version`.
- Dynamic `claude` reachability check on `/health` (503 when unavailable).
- Configuration via flags (`-p/--port`, `-m/--model`, `-c/--claude-path`,
  `-k/--api-key`) with `PROXY_API_KEY` environment-variable fallback.
- Optional bearer-token authentication with constant-time comparison.
- Tool interception: OpenAI `tools` definitions are mapped to native
  `--allowedTools` flags (`Bash`, `Read`, `Edit`, `Write`).
- Request-context-bound subprocesses: client disconnects kill the child.
- `version.go` single-source-of-truth versioning and Makefile release tooling
  (`bump-patch`/`bump-minor`/`bump-major`, `release`, `push`).
- `scripts/` curl-based test scripts (`health.sh`, `models.sh`, `chat.sh`,
  `chat-stream.sh`, and a `test.sh` smoke suite) plus a `make smoke` target.
