# Changelog

All notable changes to blcli are documented in this file.

## [Unreleased]

> **Rendered output changes.** Args that blcli used to ignore now take effect. Re-render with
> `blcli init` into a scratch directory, diff the result against what is deployed, and review every
> difference before applying (for example a version, namespace or Helm release name that now
> follows your args instead of the template literal).

### Fixed

- **Template `default` function**: uses Sprig argument order, so `{{ default "d" .v }}` and
  `{{ .v | default "d" }}` return `.v` when it is set. It used to return the literal every time.
  Only unset, `nil` and `""` count as empty; an explicit `false` or `0` is kept.
- **Arg names**: component parameters are readable under their own key, PascalCase and
  lowerCamelCase (`hosted-domain` -> `.HostedDomain`, `.hostedDomain`), for Kubernetes components,
  Terraform components and init items, and GitOps app parameters. kebab-case args from
  `default.yaml` (for example `image-tag`, `remote-write-url`) used to miss lowerCamelCase template
  variables, and the args.yaml default was used instead.
- **Component defaults**: an args value replaces the args.yaml default whatever its spelling.
- **Dependency components**: a component that a project pulls in only as a dependency renders with
  its args.yaml defaults instead of borrowing another project's parameters.
- **Generated private keys stay out of git**: `blcli init` gives `kubernetes/init.sh`
  `BLCLI_KEYS_DIR=<workspace>/.blcli-keys` (mode 0700, ignored by git) instead of
  `<workspace>/kubernetes/keys`, moves keys from that old location, and adds `keys/` to
  `kubernetes/.gitignore`. `apply kubernetes` passes the directory to custom install scripts as
  `BLCLI_KEYS_DIR` and `ARGOCD_KEYS_DIR`. If `kubernetes/keys` was ever committed or pushed, rotate
  the ArgoCD deploy key.
- **`apply init-repos` refuses to commit private keys**: before any git or `gh` command it scans
  the terraform, kubernetes and gitops directories for private-key material (PEM/OpenSSH private key
  headers, `id_*`, `*.pem`, `*.key`, `*.p12`, `*.pfx`, `keys/` directories) and stops with the list
  of findings.
- **Render precedence**: when a key is set at several levels, the more specific level always wins
  (repository global < `terraform.global` < project global < component parameters). The result used
  to depend on Go map iteration order.

## [2.0.0] - 2026-06-26

> **Versioning:** this release is **v2.0** on the product roadmap (earlier plans called it v1.5). v1.0 was the Phase 1 core workflow.

### Added

- **`init-args --profile`**: merge template profile overlays from `profiles/<name>/`.
- **`init-args --wizard`**: interactive prompts for org, profile, workspace, and GCP billing settings.
- **`init-args --preview`**: preview generated args without writing files.
- **`init --preview`**: preview init output paths and modules without writing files.
- **`blcli check args`**: validate args files against template parameter definitions.
- **`blcli contract`**: machine-readable Agent tool contract (json/yaml/table).
- **`blcli diagnose`**: classify failures and suggest repair steps.
- **`blcli runs`**: list/show persisted progress runs under `~/.blcli/progress`.
- **Failure fixtures**: `integration/fixtures/failures` for offline diagnosis tests.
- **GitHub Action**: composite action at `.github/actions/blcli` for CI workflows.
- **CI documentation**: `docs/zh/CI.md`.

### Changed

- **Resume**: `init` and `apply all` detect incomplete operations and offer resume (`--no-resume` to skip).
- **Failure hints**: `PrintFailureHints` integrates `agent.DiagnoseFailure` for richer suggestions.
- **Progress tracking**: apply subprocess steps record command output excerpts in progress files.

### Documentation

- Updated README product positioning and slogan.
- Revised `FEATURE_STATUS.md`, `Roadmap.md`, and `V1.0_STATUS_ANALYSIS.md` to match code.

## [1.0.0] - 2026-06-25

### Added

- Phase 1 GCP-first core loop: `init`, `init-args`, `apply`, `status`, `rollback`, `check`, `destroy`, `explain`.
- Terraform / Kubernetes / GitOps dependency ordering, execution plans, and `--dry-run`.
- Args validation before init, template loading from GitHub or local paths.
- Progress persistence to `~/.blcli/progress/`.
