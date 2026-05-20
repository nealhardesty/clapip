
# PRD: `clapip` - Go-based Claude CLI API Proxy

## Overview

Build `clapip`, a high-performance, lightweight Go proxy that exposes an OpenAI-compatible HTTP API but executes inferences by wrapping the Anthropic `claude` CLI as a local subprocess. This enables AI IDEs to utilize a flat-rate Claude Max subscription without requiring direct API keys, while safely proxying IDE tool requests to the CLI's native tools.

## 1. Core Architecture & Stack

* **Language:** Go (1.21+)
* **Dependencies:** Primarily standard library (`net/http`, `os/exec`, `encoding/json`, `flag`, `context`, `runtime/debug`).
* **Execution Model:** The server parses incoming OpenAI HTTP requests, formats them, spawns `claude --print` as a subprocess, reads `stdout`, and translates the output back into OpenAI JSON or SSE formats.

## 2. Initialization & Sanity Checks

* **Startup Verification:** On boot, `clapip` must verify that the Claude CLI is installed and executable (e.g., by checking the resolved path or running `<claude-path> --version`). If it is not found or fails, log a fatal error and exit immediately.

## 3. Configuration & CLI Arguments

Configuration should be driven by command-line flags, with environment variable fallbacks where appropriate.

* **`-p, --port`**: The port number to listen on. (Default: `8999`).
* **`-m, --model`**: The default model to use if one isn't provided by the client, or to force a specific model. (Default: `sonnet`).
* **`-c, --claude-path`**: Explicit path to the Claude CLI binary. (Default: `claude`, relying on the system `$PATH`).
* **`-k, --api-key`**: Bearer token required by clients to use the proxy.
* **Precedence:** 1. CLI Flag (`-k`) -> 2. Environment Variable (`PROXY_API_KEY`) -> 3. No key required (open proxy).



## 4. API Endpoints

* `GET /health`
* Returns HTTP 200 OK along with the current `clapip` version.
* **Dynamic Check:** Must actively verify that the `claude` executable is still accessible at the configured path and return a 503 if the binary is missing or unreachable.


* `GET /v1/models`
* Returns a hardcoded OpenAI-formatted list of supported models mapped to CLI equivalents.


* `POST /v1/chat/completions`
* The core inference engine supporting standard JSON and SSE formats.



## 5. Tool Mapping & Proxying (Stateless)

* **Built-in Tool Interception:** The proxy must gracefully handle IDEs (Cursor, Continue) that pass system tool definitions via the standard OpenAI `tools` array.
* **Translation to CLI Flags:** Map standard client tools to Claude Code's native permissions. If the client sends a `run_command`, `read_file`, or `edit_file` tool, strip the tool definitions from the OpenAI JSON payload so as not to confuse the LLM. Instead, append the `--allowedTools "Bash,Read,Edit"` flag to the CLI subprocess command.
* **Delegated Execution:** The proxy does not execute tools itself or manage state. It delegates file and bash operations to the Claude CLI natively. Custom or unrecognized tools should be ignored to maintain a stateless architecture.

## 6. Subprocess & Lifecycle Management

* **Context Binding:** Tie the `os/exec.CommandContext` directly to the `http.Request.Context()`. If the HTTP client disconnects, the child `claude` process **must** receive a SIGKILL/SIGTERM immediately.
* **Safe Execution:** Pass user prompts explicitly as arguments to the `exec.Command` array. **Do not** use `sh -c` to prevent shell injection.
* *Command Structure Example:* `<claude-path> --print --model <mapped_or_default_model> --allowedTools "<mapped_tools>" "<merged_prompt>"`

## 7. Input & Output Translation

* **Input Handling:** Parse the standard OpenAI `messages` array. Combine the roles into a single formatted string suitable for the CLI.
* **Output Streaming (SSE):** If `stream: true`, continuously read from the subprocess `stdout` pipe. Wrap chunks in the OpenAI ChatCompletion format (`data: {"choices": [{"delta": {"content": "..."}}]} \n\n`) and flush the writer.
* **Non-Streaming:** If `stream: false`, accumulate `stdout` until exit, then return the standard OpenAI ChatCompletion JSON object.

## 8. Security & Error Handling

* **Authentication:** If an API key is configured, enforce a middleware check for the `Authorization: Bearer <key>` header using constant-time string comparison (`crypto/subtle`).
* **Error Bubbling:** Capture `stderr` from the CLI. If the process exits with a non-zero status, return an HTTP 500 containing the `stderr` string so the user knows if the CLI requires re-authentication (e.g., `claude auth login`).
* **Zero Token Storage:** The proxy must never attempt to read or store Anthropic OAuth tokens, relying entirely on the CLI's native keychain.

## 9. Build, Release, & Version Management

To support clean installations via `go install [github.com/nealhardesty/clapip@latest](https://github.com/nealhardesty/clapip@latest)`, the project must include a standard Makefile and automated version management driven by a static file.

* **Version Tracking:**
* Maintain a `version.go` file in the main package containing a hardcoded constant (e.g., `const Version = "v0.1.0"`).
* This ensures that users installing via `go install` receive an accurate version string when starting the app or hitting `/health`.


* **Makefile Targets:**
* `make build`: Compiles the binary into a local `./bin` directory.
* `make run`: Builds and immediately executes the binary with default flags.
* `make test`: Runs `go test ./...`.
* **Version Bump Targets:**
* `make bump-patch`, `make bump-minor`, `make bump-major`: Scripts (using `awk` or `sed`) that automatically read `version.go`, increment the corresponding semantic version, and overwrite `version.go` with the new value.


* **`make release`**: A zero-argument release automation target that performs the following sequence:
1. Extracts the *current* version string directly from `version.go`.
2. Commits the file (`git commit -am "chore: bump version to <extracted-version>"`).
3. Tags the commit (`git tag <extracted-version>`).
4. Pushes the commit and tags to the repository (`git push origin main && git push origin --tags`).
5. Uses the GitHub CLI (`gh release create <extracted-version> --generate-notes`) to publish a formal release so users can download pre-compiled binaries or rely on the Go module proxy.
