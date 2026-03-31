# Makefile Guide

English is the primary documentation language in this repository.
For Chinese documentation, see [MAKEFILE_zh.md](./MAKEFILE_zh.md).

This guide explains how to use the `blcli` Makefile for local development, testing, and builds.

## Quick Start

### List all available commands

```bash
make help
```

### Setup development environment

```bash
# Install developer tools (kind, golangci-lint, etc.)
make install-tools

# Or do one-shot setup
make dev-setup
```

## Build Commands

```bash
# Build binary (output: ../workspace/bin/blcli)
make build

# Development build (race detector + debug symbols)
make build-dev

# Install to $GOPATH/bin
make install
```

## Test Commands

### Unit tests

```bash
make test-unit
make test-unit-verbose
make test-unit-coverage   # Generates coverage.html
```

### Integration / E2E

```bash
make test-integration
make test-e2e
make test-e2e-verbose
```

### Full test sets

```bash
make test-all   # unit + integration (no E2E)
make test       # default test target (usually unit only)
```

### Manual test targets

```bash
make test-manual
make test-manual-init-k8s
make test-manual-apply-k8s
make test-manual-init-tf
make test-manual-init-args
```

## Kubernetes Cluster Management

### Create local Kubernetes cluster (Mac, kind)

```bash
make k8s-create
make k8s-status
make k8s-kubeconfig

# Use cluster (or follow command printed by make k8s-kubeconfig)
export KUBECONFIG=$(kind get kubeconfig-path --name blcli-test)
```

### Delete/reset cluster

```bash
make k8s-delete
make k8s-reset
```

### Run tests against local cluster

```bash
make k8s-create
export KUBECONFIG=$(kind get kubeconfig-path --name blcli-test)
kubectl get nodes
make test-e2e
make test-manual-apply-k8s
```

## Code Quality

```bash
make fmt
make vet
make lint
make clean
```

## CI Commands

```bash
make ci-test
make ci-build
```

## Example Developer Workflow

```bash
# First-time setup
make dev-setup

# Create local cluster
make k8s-create
export KUBECONFIG=$(kind get kubeconfig-path --name blcli-test)

# Develop and validate
make test-unit
make fmt
make build
```

## Troubleshooting

### `kind` not installed

```bash
brew install kind
# or
make install-tools
```

### `golangci-lint` not installed

```bash
brew install golangci-lint
# or
make install-tools
```

### Docker not running

`kind` requires Docker. Ensure Docker Desktop is running.

### Cluster creation failed

```bash
docker ps
make k8s-delete
make k8s-create
```

## Environment Variables

```bash
# Custom version
VERSION=v1.0.0 make build
```

## Output Files

- `../workspace/bin/blcli`: built binary
- `coverage.out`: coverage raw data
- `coverage.html`: coverage HTML report

## Notes

1. E2E tests require a Kubernetes cluster.
2. Some manual tests are interactive.
3. Reconfigure `KUBECONFIG` if needed after restart.
4. Default test timeout can be changed via `TEST_TIMEOUT` in Makefile.
