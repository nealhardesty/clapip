#!/usr/bin/env bash
# List the models advertised by clapip via /v1/models.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
. ./common.sh

echo "GET ${BASE_URL}/v1/models"
capi "${BASE_URL}/v1/models" | jq .
