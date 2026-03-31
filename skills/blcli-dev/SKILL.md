---
name: blcli-dev
description: Use this skill when working on blcli-go infrastructure workflows, template-repo changes, or operational commands driven by `blcli`. Apply it for `init`, `init-args`, `check`, `apply`, `status`, `rollback`, debugging generated Terraform/Kubernetes/GitOps output, and keeping `blcli-go` behavior aligned with `bl-template` conventions.
---

# blcli Dev

Operate `blcli` as the primary interface for infra workflows in this repository. Treat `blcli-go` and `bl-template` as a coupled system: template changes can break command behavior, and command behavior often depends on template layout, naming, and install scripts.

## Core Workflow

1. Identify the module and command family involved: `init-args`, `init`, `check`, `apply`, `status`, or `rollback`.
2. Read the relevant CLI entrypoint under `pkg/cli/` and bootstrap implementation under `pkg/bootstrap/`.
3. If the task touches generated infrastructure, inspect the corresponding files in `bl-template` before changing `blcli-go`.
4. Prefer reproducing the problem with `./bin/blcli ...` or `go test` before patching behavior.
5. After changes, validate the narrowest useful scope first, then broader integration flows if the environment allows it.

## Command Selection

- Use `init-args` when the task is about generating or understanding `args.yaml` or `args.toml`.
- Use `init` when the task is about rendering workspace files from template repositories.
- Use `check` when the task is about preflight validation, repo checks, or Kubernetes manifest checks.
- Use `apply` when the task is about execution order, install commands, cluster actions, or Terraform/Kubernetes/GitOps rollout behavior.
- Use `status` when the task is about inspecting existing workspace or cluster state.
- Use `rollback` when the task is about uninstall or reverse-order cleanup behavior.

Detailed command notes live in [references/commands.md](references/commands.md).

## Repo Conventions

- Treat `bl-template` as the source of truth for renderable files, `config.yaml`, `args.yaml`, and custom install scripts.
- Keep `config.yaml` `path` entries consistent with files that actually exist in the template repo.
- For Kubernetes components, distinguish renderable `.tmpl` files from plain files copied as-is.
- Respect component naming rules, including numeric prefixes used in args and dependency ordering.
- When changing custom install behavior, inspect both rendered output and runtime execution in `apply_kubernetes`.
- When debugging missing or stale files, check overwrite behavior before assuming rendering failed.

Detailed conventions and common pitfalls live in [references/conventions.md](references/conventions.md).

## Debugging Rules

- If a generated file is missing, verify the component is present in args, the template path exists, and the file is not being skipped by overwrite or optional/init filtering.
- If a command executes the wrong script or wrong path, inspect both rendered file location and the working directory used by the subprocess.
- If a Kubernetes apply order looks wrong, trace `ResolveKubernetesDependencies` and any name normalization between args and config.
- If a custom install step fails, inspect the rendered script in the workspace, not just the template source.
- If tests fail due to missing external template repos or sandbox limits, report that explicitly and use the narrowest local validation available.

Detailed debugging checklists live in [references/diagnostics.md](references/diagnostics.md).

## Validation

- Prefer focused tests such as `go test ./pkg/bootstrap/...` or a single named test before running the entire suite.
- When sandboxed Go cache permissions interfere, use a local cache override such as `GOCACHE=/tmp/gocache`.
- If changing template behavior, validate both the render step and the runtime step that consumes the rendered output.
- Do not assume green unit tests cover external template drift; verify the template files referenced by config still exist.
