# Redis Controller

A Kubernetes controller for managing Redis key-value pairs using Custom Resources.

## Overview

This controller watches for `RedisEntry` custom resources and synchronizes their key-value pairs with a Redis database. It provides a Kubernetes-native way to manage Redis entries.

## Features

- Create and manage Redis key-value pairs using Kubernetes Custom Resources
- Automatic synchronization between CR state and Redis database
- Optional TTL support for Redis entries
- Status conditions for tracking Redis operations
- Helm charts for easy deployment of both the controller and Redis

## Prerequisites

- Kubernetes cluster (1.19+)
- Helm 3
- kubectl

## Installation

### 1. Install Redis

```bash
helm install redis ./helm/redis
```

### 2. Install the Controller

```bash
helm install redis-ctrl ./helm/redis-ctrl
```

## Usage

### Creating a Redis Entry

```yaml
apiVersion: redis.aaspcodes.github.io/v1alpha1
kind: RedisEntry
metadata:
  name: example-entry
spec:
  key: mykey
  value: myvalue
  ttl: 3600  # Optional: TTL in seconds
```

### Checking Status

```bash
kubectl get redisentry
```

## Development

### Requirements

- Go 1.21+
- Docker
- make

### Building

```bash
# Build the controller
make

# Run tests
make test

# Build docker image
make docker-build
```

### Running Locally

1. Install CRDs:
```bash
make install
```

2. Run controller:
```bash
make run
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

Apache 2.0

