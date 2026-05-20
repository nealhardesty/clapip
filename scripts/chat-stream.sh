#!/usr/bin/env bash
# Send a streaming (SSE) chat completion request and print the raw events.
# Usage: ./chat-stream.sh [prompt words...]
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
. ./common.sh

PROMPT="${*:-Count from one to five.}"

echo "POST ${BASE_URL}/v1/chat/completions  (stream, model=${CLAPIP_MODEL})"
echo "prompt: ${PROMPT}"
echo
capi -N "${BASE_URL}/v1/chat/completions" \
  -H 'Content-Type: application/json' \
  -d "$(chat_payload "$CLAPIP_MODEL" true "$PROMPT")"
