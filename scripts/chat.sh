#!/usr/bin/env bash
# Send a non-streaming chat completion request.
# Usage: ./chat.sh [prompt words...]
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
. ./common.sh

PROMPT="${*:-Reply with a short friendly greeting.}"

echo "POST ${BASE_URL}/v1/chat/completions  (model=${CLAPIP_MODEL})"
echo "prompt: ${PROMPT}"
echo
capi "${BASE_URL}/v1/chat/completions" \
  -H 'Content-Type: application/json' \
  -d "$(chat_payload "$CLAPIP_MODEL" false "$PROMPT")" | jq .
