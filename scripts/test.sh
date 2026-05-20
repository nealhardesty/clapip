#!/usr/bin/env bash
# Smoke test for a running clapip server. Exercises every endpoint and the
# error/auth paths, then exits non-zero if any check fails.
#
# Start the server first (e.g. `make run`), then run this script.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
. ./common.sh

PASS=0
FAIL=0
ok()  { echo "  ok   - $1"; PASS=$((PASS + 1)); }
bad() { echo "  FAIL - $1"; FAIL=$((FAIL + 1)); }

# req METHOD PATH [DATA] — populates RESP_CODE and RESP_BODY.
req() {
  local method="$1" path="$2" data="${3:-}" tmp
  tmp="$(mktemp)"
  if [ -n "$data" ]; then
    RESP_CODE="$(capi -o "$tmp" -w '%{http_code}' -X "$method" \
      -H 'Content-Type: application/json' -d "$data" "${BASE_URL}${path}")"
  else
    RESP_CODE="$(capi -o "$tmp" -w '%{http_code}' -X "$method" "${BASE_URL}${path}")"
  fi
  RESP_BODY="$(cat "$tmp")"
  rm -f "$tmp"
}

expect_code() { # expect_code DESC WANT
  if [ "$RESP_CODE" = "$2" ]; then ok "$1 (HTTP $RESP_CODE)"
  else bad "$1 (HTTP $RESP_CODE, want $2) body=$RESP_BODY"; fi
}

expect_contains() { # expect_contains DESC NEEDLE
  case "$RESP_BODY" in
    *"$2"*) ok "$1" ;;
    *) bad "$1 (missing '$2') body=$RESP_BODY" ;;
  esac
}

echo "clapip smoke test against ${BASE_URL}"
echo

if ! curl -sS -o /dev/null "${BASE_URL}/health" 2>/dev/null; then
  echo "error: cannot reach clapip at ${BASE_URL}" >&2
  echo "start the server first, e.g.:  make run" >&2
  exit 1
fi

echo "[health]"
req GET /health
expect_code "health responds 200" 200
expect_contains "health reports status ok" '"status":"ok"'
expect_contains "health reports a version" '"version"'

echo "[models]"
req GET /v1/models
expect_code "models responds 200" 200
expect_contains "models lists sonnet" '"sonnet"'
expect_contains "models is an OpenAI list" '"object":"list"'

echo "[chat: non-streaming]"
req POST /v1/chat/completions "$(chat_payload "$CLAPIP_MODEL" false 'Reply with exactly the word PONG.')"
expect_code "chat responds 200" 200
expect_contains "chat returns a chat.completion" '"object":"chat.completion"'
expect_contains "chat returns choices" '"choices"'
expect_contains "chat finishes with stop" '"finish_reason":"stop"'

echo "[chat: streaming]"
STREAM="$(capi -N "${BASE_URL}/v1/chat/completions" \
  -H 'Content-Type: application/json' \
  -d "$(chat_payload "$CLAPIP_MODEL" true 'Reply with exactly the word PONG.')")"
case "$STREAM" in
  *"chat.completion.chunk"*) ok "stream emits chunk events" ;;
  *) bad "stream emits chunk events (got: $STREAM)" ;;
esac
case "$STREAM" in
  *"data: [DONE]"*) ok "stream terminates with [DONE]" ;;
  *) bad "stream terminates with [DONE] (got: $STREAM)" ;;
esac

echo "[error handling]"
req POST /v1/chat/completions 'this is not json'
expect_code "malformed JSON rejected" 400
req POST /v1/chat/completions '{"messages":[]}'
expect_code "empty messages rejected" 400
req GET /v1/chat/completions
expect_code "wrong method rejected" 405

echo "[auth]"
if [ -n "$CLAPIP_API_KEY" ]; then
  CODE="$(curl -sS -o /dev/null -w '%{http_code}' "${BASE_URL}/v1/models")"
  if [ "$CODE" = "401" ]; then ok "request without bearer token rejected (HTTP 401)"
  else bad "request without bearer token rejected (HTTP $CODE, want 401)"; fi
  CODE="$(curl -sS -o /dev/null -w '%{http_code}' \
    -H 'Authorization: Bearer wrong-token' "${BASE_URL}/v1/models")"
  if [ "$CODE" = "401" ]; then ok "request with wrong token rejected (HTTP 401)"
  else bad "request with wrong token rejected (HTTP $CODE, want 401)"; fi
else
  echo "  skip - auth checks (set CLAPIP_API_KEY to enable)"
fi

echo
echo "results: ${PASS} passed, ${FAIL} failed"
[ "$FAIL" -eq 0 ]
