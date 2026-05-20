#!/usr/bin/env bash
# Shared configuration and helpers for the clapip curl scripts.
# Source this file; do not run it directly.
#
# Override defaults with environment variables:
#   CLAPIP_HOST     host clapip is bound to   (default: localhost)
#   CLAPIP_PORT     port clapip listens on    (default: 8999)
#   CLAPIP_API_KEY  bearer token, if required (default: empty — no auth header)
#   CLAPIP_MODEL    model to request          (default: sonnet)

CLAPIP_HOST="${CLAPIP_HOST:-localhost}"
CLAPIP_PORT="${CLAPIP_PORT:-8999}"
CLAPIP_API_KEY="${CLAPIP_API_KEY:-}"
CLAPIP_MODEL="${CLAPIP_MODEL:-sonnet}"
BASE_URL="http://${CLAPIP_HOST}:${CLAPIP_PORT}"

command -v jq >/dev/null 2>&1 || {
  echo "error: jq is required by these scripts (brew install jq)" >&2
  exit 1
}

# capi runs curl against the proxy, attaching the bearer token when one is set.
capi() {
  if [ -n "$CLAPIP_API_KEY" ]; then
    curl -sS -H "Authorization: Bearer ${CLAPIP_API_KEY}" "$@"
  else
    curl -sS "$@"
  fi
}

# chat_payload MODEL STREAM PROMPT -> prints a chat completion request body.
chat_payload() {
  jq -n --arg m "$1" --argjson s "$2" --arg p "$3" \
    '{model:$m, stream:$s, messages:[{role:"user", content:$p}]}'
}
