#!/usr/bin/env bash

set -euo pipefail

# Check if kind is installed
if ! command -v kind &> /dev/null; then
    echo "kind is not installed. Installing..."
    curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-darwin-amd64
    chmod +x ./kind
    sudo mv ./kind /usr/local/bin/kind
fi

# Check if a kind cluster exists
if ! kind get clusters | grep -q "kind"; then
    echo "No kind cluster found. Creating one..."
    kind create cluster --name kind --wait 5m
else
    echo "Using existing kind cluster..."
fi

# Ensure kubectl is configured to use the kind cluster
kind export kubeconfig --name kind



# Run the E2E tests
echo "Running E2E tests..."
go test -v ./test/e2e/... -timeout 30m 