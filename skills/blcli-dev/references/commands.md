# Command Notes

## High-value entrypoints

- `pkg/cli/init.go` and `pkg/bootstrap/manager.go`: main `blcli init` flow.
- `pkg/cli/init_args.go`: args generation flow and schema shaping.
- `pkg/cli/apply_kubernetes.go` and `pkg/bootstrap/apply_kubernetes.go`: Kubernetes apply order and custom install execution.
- `pkg/cli/check.go` and `pkg/bootstrap/check_kubernetes.go`: repo and manifest checks.
- `pkg/cli/status.go` and `pkg/bootstrap/status.go`: status collection.
- `pkg/cli/rollback.go` and `pkg/bootstrap/rollback.go`: reverse execution logic.

## Typical local commands

```bash
./bin/blcli init-args /path/to/bl-template -o args.yaml
./bin/blcli init /path/to/bl-template -a args.yaml -m kubernetes --overwrite
./bin/blcli check kubernetes -d kubernetes --template-repo /path/to/bl-template
./bin/blcli apply kubernetes -d kubernetes --project prd
./bin/blcli status -a args.yaml --type kubernetes
```

## Build and test shortcuts

```bash
go build -o ./bin/blcli ./cmd/blcli
GOCACHE=/tmp/gocache go test ./pkg/bootstrap/...
GOCACHE=/tmp/gocache go test -run '^TestName$' ./pkg/bootstrap
```

## Practical usage rules

- Rebuild `./bin/blcli` after CLI or bootstrap changes before manual verification.
- When validating template repo changes, point commands at the local `bl-template` path instead of GitHub.
- For Kubernetes failures, inspect the rendered workspace under `workspace/kubernetes/...` before changing code.
- For apply order issues, compare args component names, config component names, and normalized names.
