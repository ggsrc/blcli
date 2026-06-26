# Changelog

All notable changes to blcli are documented in this file.

## [2.0.0] - 2026-06-26

> **版本说明：** 本 release 对应产品路线图 **v2.0**（原规划称 v1.5）。v1.0 为 Phase 1 核心闭环。

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
