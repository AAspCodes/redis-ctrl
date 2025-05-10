#!/usr/bin/env bash

set -euo pipefail

# Check if kind cluster exists
if kind get clusters | grep -q "kind"; then
    echo "Cleaning up kind cluster..."
    kind delete cluster --name kind
else
    echo "No kind cluster found, nothing to clean up."
fi 