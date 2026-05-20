#!/usr/bin/env bash
# Check the clapip /health endpoint.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
. ./common.sh

echo "GET ${BASE_URL}/health"
capi "${BASE_URL}/health" | jq .
