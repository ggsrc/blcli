<div align="center">

# blcli

**Infrastructure-as-Code CLI for GCP · Kubernetes · GitOps**

*Generate, apply, and destroy multi-environment cloud infrastructure from a single config file.*

[![Go Version](https://img.shields.io/badge/go-1.21%2B-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)
[![Build](https://img.shields.io/badge/build-passing-brightgreen?style=flat-square)]()
[![Template](https://img.shields.io/badge/template-bl--template-orange?style=flat-square)](https://github.com/NFTGalaxy/bl-template)

</div>

---

## What is blcli?

`blcli` is a CLI tool that bootstraps and manages multi-environment GCP infrastructure end-to-end — from Terraform project creation through Kubernetes add-on deployment to GitOps application rollout — driven by a single `args.yaml` configuration file and a reusable template repository.

```
args.yaml  +  bl-template  →  blcli  →  GCP projects + GKE clusters + ArgoCD apps
```

**Why blcli?**

| Without blcli | With blcli |
|---|---|
| Manually copy Terraform per project | One template → N environments |
| Hand-wire Kubernetes add-on install scripts | Dependency-ordered component graph |
| Separately manage GitOps Application CRDs | `blcli apply gitops` in one command |
| No standard destroy path | Two-prompt safety confirmation + full teardown |

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Quick Start (5 minutes)](#quick-start-5-minutes)
- [Full Workflow](#full-workflow)
- [Commands Reference](#commands-reference)
  - [blcli init-args](#blcli-init-args)
  - [blcli init](#blcli-init)
  - [blcli apply init](#blcli-apply-init)
  - [blcli apply terraform](#blcli-apply-terraform)
  - [blcli apply kubernetes](#blcli-apply-kubernetes)
  - [blcli apply gitops](#blcli-apply-gitops)
  - [blcli apply init-repos](#blcli-apply-init-repos)
  - [blcli destroy](#blcli-destroy)
  - [blcli check](#blcli-check)
- [Configuration Reference](#configuration-reference)
  - [args.yaml structure](#argsyaml-structure)
  - [Inheritance model](#inheritance-model)
  - [.env overrides](#env-overrides)
- [Template Repository](#template-repository)
  - [Directory layout](#directory-layout)
  - [config.yaml format](#configyaml-format)
- [Generated Output Structure](#generated-output-structure)
- [Architecture](#architecture)
- [Logging and Debugging](#logging-and-debugging)
- [Troubleshooting](#troubleshooting)
  - [GCP projects in DELETE_REQUESTED state](#gcp-projects-in-delete_requested-state)
  - [409 resource-already-exists conflicts](#409-resource-already-exists-conflicts)
  - [Billing disabled on restored projects](#billing-disabled-on-restored-projects)
  - [kubectl config.lock file error](#kubectl-configlock-file-error)
  - [prevent_destroy error](#terraform-state-bucket-prevent_destroy-error)
  - [GKE node pool CPU quota exceeded](#gke-node-pool-cpu-quota-exceeded)
  - [Missing backend configuration](#missing-backend-configuration)
  - [ArgoCD SSH authentication failures](#argocd-ssh-authentication-failures)
- [Contributing](#contributing)

---

## Prerequisites

| Tool | Purpose | Required |
|------|---------|----------|
| `go 1.21+` | Build blcli | Build only |
| `terraform` | GCP infrastructure | `apply terraform` |
| `kubectl` | Kubernetes resources | `apply kubernetes` |
| `helm` | Helm chart installs | `apply kubernetes` |
| `kustomize` | Kustomize overlays | `apply kubernetes` |
| `istioctl` | Istio mesh install | `apply kubernetes` |
| `gcloud` | GCP auth & credentials | `apply terraform` |
| `gh` | GitHub repo creation | `apply init-repos` |

Verify all tools at once:

```bash
blcli check
```

**GCP Prerequisites:**
- `gcloud auth login` with a project-creation-capable account
- A valid GCP Billing Account ID
- Sufficient CPU / address quota per region (recommend requesting increases before deploying large clusters)

**ArgoCD / GitOps Prerequisites:**
- A GitOps repository on GitHub (e.g. `my-org/infra-gitops`) — **use the actual repo name**, not a generic placeholder like `gitops`
- Repository must contain the application manifest directories referenced in `args.yaml`:
  - `stg/hello-world/`
  - `stg/hello-world-2/`
  - `prd/hello-world/`
- An SSH deploy key (`argocd_github.pub`) added to that repository
- GitOps repo URLs in **SSH format** (`git@github.com:org/repo.git`) in `.env` — HTTP URLs will cause ArgoCD auth failures
- ArgoCD sealed secret generated **without** the `useSshAgent` field (its presence breaks SSH authentication)

---

## Installation

### From source (recommended)

```bash
git clone https://github.com/NFTGalaxy/blcli-go
cd blcli-go

# Build binary in current directory
go build -o blcli ./cmd/blcli

# Or install globally to $GOPATH/bin
go install ./cmd/blcli
```

### Verify installation

```bash
blcli version
blcli check
```

---

## Quick Start (5 minutes)

```bash
# 1. Generate a starter args.yaml from the template repository
blcli init-args ../bl-template

# 2. Fill in your GCP credentials in args.yaml
#    OrganizationID, BillingAccountID, GlobalName, TerraformBackendBucket

# 3. Generate all infrastructure code
blcli init -a args.yaml -f ../bl-template

# 4. Create GCP projects + remote state bucket
blcli apply init -d ./terraform/

# 5. Deploy GCP resources (VPCs, GKE, DNS, certs…)
blcli apply terraform -d ./terraform/

# 6. Deploy Kubernetes add-ons (Istio, ArgoCD, monitoring…)
gcloud container clusters get-credentials corp-cluster --region us-west1 --project <corp-project-id>
blcli apply kubernetes -d ./kubernetes/ -r ../bl-template -p corp \
  --context gke_<corp-project-id>_us-west1_corp-cluster

# 7. Register ArgoCD Application CRDs
blcli apply gitops -d ./gitops/ --args args.yaml \
  --context gke_<corp-project-id>_us-west1_corp-cluster

# 8. Tear everything down (interactive — requires double confirmation)
blcli destroy terraform --args=args.yaml
```

---

## Full Workflow

### Stage overview

| # | Stage | Command | Idempotent |
|---|-------|---------|-----------|
| 1 | Build & install | `go build -o blcli ./cmd/blcli` | ✅ |
| 2 | Generate config scaffold | `blcli init-args -f ../bl-template` | ✅ |
| 3 | Edit config & generate code | `blcli init -a args.yaml -f ../bl-template` | ✅ (with `--overwrite`) |
| 4 | Create GCP projects + state bucket | `blcli apply init -d ./terraform/` | ✅ |
| 5 | Dry-run validation | `blcli apply terraform -d ./terraform/ --dry-run` | ✅ |
| 6 | Deploy GCP resources | `blcli apply terraform -d ./terraform/` | ✅ |
| 7 | Deploy Kubernetes add-ons | `blcli apply kubernetes -d ./kubernetes/ -p corp` | ✅ |
| 8 | Deploy GitOps apps | `blcli apply gitops -d ./gitops/ --args args.yaml` | ✅ |
| 9 | Destroy all resources | `blcli destroy terraform --args=args.yaml` | — |
| 10 | Clean up & E2E idempotency check | Re-run init + apply from scratch | — |

---

### Stage 1: Build

```bash
cd blcli-go
go build -o blcli ./cmd/blcli
go install ./cmd/blcli     # optional: make globally available
```

---

### Stage 2: Generate config scaffold

```bash
# From a local template repository
blcli init-args -f ../bl-template

# From a remote GitHub repository
blcli init-args github.com/NFTGalaxy/bl-template
```

This writes `args.yaml` and `.env` to the current directory with all available parameters pre-populated.

---

### Stage 3: Edit config and generate infrastructure code

Edit `args.yaml` to set your organisation-specific values:

```yaml
global:
  GlobalName: "my-org"                         # Used as prefix in all resource names

terraform:
  global:
    OrganizationID: "123456789012"             # GCP Organisation ID (set "0" to omit)
    BillingAccountID: "01ABCD-2EFGH3-4IJKL5"  # GCP Billing Account
    TerraformBackendBucket: "my-org-tfstore"   # GCS bucket for Terraform state

  projects:
    - name: prd
      components:
        - name: vpc
          parameters:
            enable_nat: true
            enable_private_pool_route: true
            ip_cidr_range: 10.12.0.0/16
        - name: gke
          parameters:
            cluster_name: prd-cluster
            machine_type: e2-small
            deletion_protection: true
```

Edit `.env` to provide secrets and SSH-format GitOps URLs:

```bash
BLCLI_GITOPS_SOURCE_REPO_URL=git@github.com:my-org/infra-gitops.git
BLCLI_ARGOCD_GIT_REPOSITORY_URL="git@github.com:my-org/infra-gitops.git"
```

Generate all infrastructure code:

```bash
blcli init -a args.yaml -f ../bl-template

# Regenerate only Terraform files (overwrite existing)
blcli init -a args.yaml -f ../bl-template -m terraform --overwrite
```

---

### Stage 4: Create GCP projects and remote state

`blcli apply init` runs the `init/` Terraform directories in the order defined by `.blcli.marker` — `prepare` items first (state bucket), then the rest (project creation).

```bash
# First run (creates state bucket and GCP projects)
blcli apply init -d ./terraform/

# Idempotency check — expect "No changes" on second run
blcli apply init -d ./terraform/
```

> **If projects already exist (409 errors):** blcli auto-imports them into Terraform state via `google_project.projects["<id>"]`. If projects are in `DELETE_REQUESTED` state, undelete them first:
> ```bash
> gcloud projects undelete <project-id>
> ```

---

### Stage 5: Dry-run validation

Validate plans before spending real GCP resources:

```bash
# Terraform plan (no apply)
blcli apply terraform -d ./terraform/ --dry-run > dryrun_tf.log 2>&1

# Kubernetes dry-run (requires cluster already running)
blcli apply kubernetes -d ./kubernetes/ -r ../bl-template -p corp \
  --context gke_<corp-project-id>_us-west1_corp-cluster \
  --dry-run > dryrun_k8s.log 2>&1

# GitOps dry-run (requires ArgoCD deployed)
blcli apply gitops -d ./gitops/ --args args.yaml \
  --context gke_<corp-project-id>_us-west1_corp-cluster \
  --dry-run > dryrun_gitops.log 2>&1
```

---

### Stage 6: Deploy GCP resources

```bash
blcli apply terraform -d ./terraform/
```

`blcli apply terraform` runs both `init/` directories and `gcp/` project directories in full:

```
init phase:
  [1/2] init/0-terraform-statestore  (GCS backend bucket — prepare:true)
  [2/2] init/1-my-org-projects       (GCP project creation)

apply phase (parallel-safe, dependency-ordered):
  gcp/corp  →  gcp/prd  →  gcp/stg
```

Each directory runs: `terraform init` → `validate` → `plan` → `apply`.

> **Cross-project dependency note:** On first apply, IAM bindings referencing service accounts in other projects may fail. Re-run `blcli apply terraform -d ./terraform/` — it converges in 2–3 passes.

---

### Stage 7: Deploy Kubernetes add-ons

Get cluster credentials for all environments, then apply each:

```bash
REGION=us-west1
gcloud container clusters get-credentials corp-cluster --region $REGION --project <corp-project-id>
gcloud container clusters get-credentials prd-cluster  --region $REGION --project <prd-project-id>
gcloud container clusters get-credentials stg-cluster  --region $REGION --project <stg-project-id>

# Corp
blcli apply kubernetes -d ./kubernetes/ -r ../bl-template -p corp \
  --context gke_<corp-project-id>_${REGION}_corp-cluster

# Prd
blcli apply kubernetes -d ./kubernetes/ -r ../bl-template -p prd \
  --context gke_<prd-project-id>_${REGION}_prd-cluster

# Stg
blcli apply kubernetes -d ./kubernetes/ -r ../bl-template -p stg \
  --context gke_<stg-project-id>_${REGION}_stg-cluster
```

Components are applied in dependency order per project (e.g. `external-secrets-operator` before `argocd`).

**Verify ArgoCD:**

```bash
# Get initial admin password
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" \
  --context gke_<corp-project-id>_us-west1_corp-cluster | base64 -d && echo

# Port-forward to access UI
kubectl port-forward svc/argocd-server -n argocd 8080:80 \
  --context gke_<corp-project-id>_us-west1_corp-cluster
# Open: http://localhost:8080/argocd  (user: admin)
```

---

### Stage 8: Deploy GitOps apps

```bash
blcli apply gitops -d ./gitops/ --args args.yaml \
  --context gke_<corp-project-id>_us-west1_corp-cluster

# --skip-sync: apply Application CRDs without waiting for ArgoCD sync
blcli apply gitops -d ./gitops/ --args args.yaml \
  --context gke_<corp-project-id>_us-west1_corp-cluster --skip-sync
```

ArgoCD syncs the actual application manifests from your `infra-gitops` repository automatically once Applications are registered.

---

### Stage 9: Destroy

> ⚠️ **Destructive — test/non-production environments only.**

**Before destroying**, disable GKE deletion protection if `deletion_protection = true`:

```bash
# 1. Set deletionProtection: false in args.yaml for each project
# 2. Regenerate terraform output
blcli init -a args.yaml -f ../bl-template -m terraform --overwrite
# 3. Apply the change (updates cluster metadata only — no resource deletion)
blcli apply terraform -d ./terraform/ -p corp
# 4. Now destroy
blcli destroy terraform --args=args.yaml
```

```bash
# Destroy (requires two interactive confirmations)
blcli destroy terraform --args=args.yaml

# Verify
gcloud projects list --filter="projectId:my-org-*"
```

**Expected behaviour:**
- `init/0-terraform-statestore` will report "Instance cannot be destroyed" — this is intentional (`prevent_destroy = true` protects Terraform state). Record but do not treat as a blocker.
- All three GCP projects (`corp`, `prd`, `stg`) and their resources are destroyed.

> **If destroy reports "No changes / 0 destroyed" but projects/clusters still exist in GCP**, this indicates a backend or state mismatch. Diagnose with:
> ```bash
> terraform -chdir=terraform/gcp/prd state pull | jq '.resources | length'
> ```
> If state is empty (0), the GCS backend connection is lost — re-run `blcli init -m terraform --overwrite`, reinitialise each project directory with `-backend-config=config.gcs.tfbackend`, then retry destroy. As a last resort (test environments only):
> ```bash
> gcloud projects delete my-org-corp-<suffix>
> gcloud projects delete my-org-prd-<suffix>
> gcloud projects delete my-org-stg-<suffix>
> # Keep the tfstore bucket: my-org-tfstore-<suffix>
> ```

---

### Stage 10: Clean up and E2E idempotency check

```bash
# Remove all generated output
rm -rf terraform/ kubernetes/ gitops/
```

Re-run the full pipeline from scratch to confirm self-healing behaviour:

```bash
# Regenerate all infrastructure code
blcli init -a args.yaml -f ../bl-template

# Re-apply init (must produce same GCP projects, same state bucket)
blcli apply init -d ./terraform/

# Re-apply terraform (must converge to "No changes" on second pass)
blcli apply terraform -d ./terraform/
```

A successful E2E idempotency run confirms that:
- Template rendering is deterministic
- Terraform state is intact in GCS
- No manual fixups are required on a clean second run

---

## Logging and Debugging

Redirect each stage's output to a numbered log file for easier post-mortem analysis:

```bash
# Stage-by-stage log naming convention
blcli apply init -d ./terraform/             > execution_stage4.log   2>&1
blcli apply terraform -d ./terraform/        > execution_stage5.log   2>&1
blcli apply kubernetes -d ./kubernetes/ ...  > execution_stage6.log   2>&1
blcli apply gitops -d ./gitops/ ...          > execution_stage7.log   2>&1
blcli destroy terraform --args=args.yaml     > execution_stage9.log   2>&1

# After a fix, append to a _fix variant
blcli apply init -d ./terraform/             > execution_stage4_fix.log 2>&1
```

When a stage fails, match error keywords against the [Troubleshooting](#troubleshooting) section and re-run only the affected stage. Do not start over from stage 1.

**Quick error triage:**

| Error keyword | Likely cause | Section |
|---|---|---|
| `DELETE_REQUESTED` | Projects soft-deleted | [GCP projects in DELETE_REQUESTED state](#gcp-projects-in-delete_requested-state) |
| `already exists` / 409 | Resource exists outside state | [409 conflicts](#409-resource-already-exists-conflicts) |
| `BILLING_DISABLED` | Billing detached after project restore | [Billing disabled](#billing-disabled-on-restored-projects) |
| `prevent_destroy` | State bucket plan forces replace | [prevent_destroy error](#terraform-state-bucket-prevent_destroy-error) |
| `resource address "google_project.main" does not exist` | Wrong import address | [google_project.main import error](#google_projectmain-import-error) |
| `config.lock` | Stale kubectl lock file | [config.lock error](#kubectl-configlock-file-error) |
| `Missing backend configuration` | Project lacks `backend "gcs" {}` block | [Missing backend](#missing-backend-configuration) |
| `useSshAgent` | ArgoCD sealed secret field present | [ArgoCD SSH auth](#argocd-ssh-authentication-failures) |
| `No changes / 0 destroyed` but resources exist | State lost or backend mismatch | [Stage 9 note](#stage-9-destroy) |

---

## Commands Reference

### `blcli init-args`

Generate a starter `args.yaml` from a template repository's parameter definitions.

```bash
blcli init-args [template-repo] [flags]

# Examples
blcli init-args                                          # uses default template
blcli init-args github.com/NFTGalaxy/bl-template        # from GitHub
blcli init-args ../bl-template                          # from local path
blcli init-args github.com/org/repo@v2.0.0             # pinned tag
blcli init-args ../bl-template -o myconfig.yaml        # custom output path
blcli init-args ../bl-template --format toml           # TOML output
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--output` | `-o` | `args.yaml` | Output file path |
| `--format` | | `yaml` | Output format: `yaml` or `toml` |
| `--force-update` | `-f` | false | Force pull latest template |
| `--cache-expiry` | | `24h` | Template cache TTL (`0` = no expiry) |

---

### `blcli init`

Render infrastructure code from templates using your `args.yaml`.

```bash
blcli init [template-repo] -a args.yaml [flags]

# Examples
blcli init ../bl-template -a args.yaml                      # all modules
blcli init ../bl-template -a args.yaml -m terraform         # terraform only
blcli init ../bl-template -a args.yaml -m kubernetes -m gitops
blcli init ../bl-template -a args.yaml -m terraform --overwrite  # regenerate
blcli init ../bl-template -a base.yaml -a override.yaml     # multi-file merge
blcli init ../bl-template -a args.yaml -o workspace/output  # custom output dir
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--args` | `-a` | required | Args file (YAML/TOML); repeatable, earlier files win |
| `--modules` | `-m` | all | Modules to render: `terraform`, `kubernetes`, `gitops` |
| `--output` | `-o` | from config | Output directory |
| `--overwrite` | `-w` | false | Overwrite blcli-managed directories |
| `--force-update` | `-f` | false | Force pull latest template |
| `--cache-expiry` | | `24h` | Template cache TTL |

**What `blcli init` does:**
1. Loads and merges all `--args` files (earlier overrides later)
2. Validates merged config against template parameter definitions (`required`, `pattern`, `enum`, `numberRange`, `stringLength`, `validation.unique`)
3. Renders only the components explicitly listed in `args.yaml`
4. Writes `terraform/.blcli.marker` with ordered `init_prepare_dirs` and `init_dirs`
5. Copies shared modules to `terraform/gcp/modules/`

---

### `blcli apply init`

Run only the `init/` Terraform directories (state bucket first, then project creation). Use this before `blcli apply terraform` when setting up a fresh environment.

```bash
blcli apply init -d ./terraform/ [flags]

# With options
blcli apply init -d ./terraform/ --auto-approve --timeout 30m
blcli apply init -d ./terraform/ --init-delay=0       # skip inter-directory wait
```

| Flag | Default | Description |
|------|---------|-------------|
| `-d, --dir` | required | Terraform root directory |
| `--auto-approve` | false | Skip interactive confirmation |
| `--timeout` | `1h` | Overall timeout |
| `--init-delay` | `30s` | Wait between directories (allows GCP API propagation) |
| `--skip-backend` | false | Skip backend init (testing only) |

---

### `blcli apply terraform`

Run `init/` directories then all `gcp/` project directories. Each directory runs `terraform init → validate → plan → apply`.

```bash
blcli apply terraform -d ./terraform/ [flags]

# Apply a single project only
blcli apply terraform -d ./terraform/ -p prd

# Use template repo for dependency-ordered project execution
blcli apply terraform -d ./terraform/ -r ../bl-template

# Dry-run (plan only, no apply)
blcli apply terraform -d ./terraform/ --dry-run
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dir` | `-d` | required | Terraform root directory |
| `--project` | `-p` | all | Apply only this project name |
| `--template-repo` | `-r` | — | Template repo path for dependency ordering |
| `--auto-approve` | | false | Skip confirmation prompts |
| `--timeout` | | `1h` | Overall timeout |
| `--dry-run` | | false | Plan only, no apply |
| `--skip-backend` | | false | Skip GCS backend init |

---

### `blcli apply kubernetes`

Deploy Kubernetes components (Helm charts, kustomize overlays, install scripts) to a cluster in dependency order.

```bash
blcli apply kubernetes -d ./kubernetes/ -r ../bl-template -p corp \
  --context gke_<project>_<region>_corp-cluster [flags]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dir` | `-d` | required | Kubernetes root directory |
| `--project` | `-p` | required | Project name (`corp`, `prd`, `stg`) |
| `--template-repo` | `-r` | required | Template repo path (for component ordering) |
| `--context` | | current | kubectl context to use |
| `--dry-run` | | false | Print plan, do not apply |

---

### `blcli apply gitops`

Apply ArgoCD `Application` CRDs from the generated `gitops/` directory. ArgoCD handles the actual application sync from the GitOps repository.

```bash
blcli apply gitops -d ./gitops/ --args args.yaml \
  --context gke_<project>_<region>_corp-cluster [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--args` | required | Path to `args.yaml` |
| `--context` | current | kubectl context |
| `--skip-sync` | false | Apply CRDs without waiting for ArgoCD sync |
| `--kubeconfig` | `~/.kube/config` | Path to kubeconfig |

---

### `blcli apply init-repos`

Initialise generated directories as git repositories, create GitHub repos via `gh`, and push initial commits.

```bash
blcli apply init-repos -d ./workspace/output --org my-github-org
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dir` | `-d` | required | Root directory (contains `terraform/`, `kubernetes/`, `gitops/`) |
| `--org` | `-o` | required | GitHub organisation or username |
| `--terraform-dir` | | `{dir}/terraform` | Override terraform path |
| `--kubernetes-dir` | | `{dir}/kubernetes` | Override kubernetes path |
| `--gitops-dir` | | `{dir}/gitops` | Override gitops path |

---

### `blcli destroy`

Destroy all resources managed by blcli. Requires double interactive confirmation.

```bash
# Destroy terraform resources only
blcli destroy terraform --args=args.yaml

# Destroy all modules
blcli destroy --args=args.yaml
```

**Safety prompts:**
1. Type `yes` to confirm destruction
2. If `global.organization_id` or `global.name` is set in args, type its value as a second confirmation

> ⚠️ This removes GCP projects, GKE clusters, VPCs, DNS zones, and cleans up local generated Terraform directories. **Use only in non-production environments.**

---

### `blcli check`

Verify all required external tools are installed:

```bash
blcli check
```

---

## Configuration Reference

### `args.yaml` structure

```yaml
# ── Global (repo-level) ────────────────────────────────────────────────────
global:
  GlobalName: "my-org"          # Prefix for all resource names

# ── Terraform ─────────────────────────────────────────────────────────────
terraform:
  global:
    OrganizationID: "123456789012"          # GCP Org ID; "0" = omit org_id
    BillingAccountID: "01ABCD-2EFGH3-4IJKL5"
    GlobalName: "my-org"
    TerraformBackendBucket: "my-org-tfstore-abc123"
    TerraformVersion: "1.9.0"

  init:
    components:
      projects:                             # GCP project creation
        ProjectServices:
          "${project.prd.id}":
            - compute.googleapis.com
            - container.googleapis.com
            - dns.googleapis.com

  projects:
    - name: prd
      global:
        project_name: prd
        region: us-west1
      components:
        - name: backend
          parameters: {}
        - name: variables
          parameters:
            zone: us-west1-a
        - name: vpc
          parameters:
            enable_nat: true
            enable_private_pool_route: true
            ip_cidr_range: 10.12.0.0/16
        - name: gke
          parameters:
            cluster_name: prd-cluster
            machine_type: e2-small
            deletion_protection: true
            min_node_count: 1
            max_node_count: 50

    - name: stg
      global:
        project_name: stg
        region: us-west1
      components:
        - name: vpc
          parameters:
            enable_nat: true          # ← must be true for private GKE egress
            enable_private_pool_route: true
            ip_cidr_range: 10.11.0.0/16

    - name: corp
      global:
        project_name: corp
        region: us-west1
      components:
        - name: vpc
          parameters:
            enable_nat: true
            enable_private_pool_route: true
            ip_cidr_range: 10.10.0.0/16

# ── GitOps ─────────────────────────────────────────────────────────────────
gitops:
  argocd:
    - project: stg
      apps:
        - name: hello-world
        - name: hello-world-2
    - project: prd
      apps:
        - name: hello-world
```

### Inheritance model

Parameters are merged from broadest → narrowest scope:

```
global.*
  └── terraform.global.*
        └── terraform.projects[n].global.*
              └── terraform.projects[n].components[m].parameters.*
```

All global parameters are **flattened** for template access:

```
global.GlobalName                    → {{ .GlobalName }}
terraform.global.OrganizationID      → {{ .OrganizationID }}
projects[n].global.project_name      → {{ .project_name }}
components[m].parameters.enable_nat  → {{ .enable_nat }}
```

> Setting `OrganizationID: "0"` suppresses `org_id` from all generated Terraform resources and variables — useful when deploying without a GCP Organisation.

### `.env` overrides

Place a `.env` file next to `args.yaml` for secrets and environment-specific overrides:

```bash
# GitOps repository — must use SSH format for ArgoCD
BLCLI_GITOPS_SOURCE_REPO_URL=git@github.com:my-org/infra-gitops.git
BLCLI_GITOPS_STG_HELLO_WORLD_APPLICATION_REPO=git@github.com:my-org/infra-gitops.git
BLCLI_GITOPS_PRD_HELLO_WORLD_APPLICATION_REPO=git@github.com:my-org/infra-gitops.git
BLCLI_ARGOCD_GIT_REPOSITORY_URL="git@github.com:my-org/infra-gitops.git"
```

> Values already set in `args.yaml` are **not** overridden by `.env`. Use `.env` only for secrets and values absent from `args.yaml`.

---

## Template Repository

### Directory layout

```
bl-template/
├── args.yaml                        # Repo-level parameter definitions
├── terraform/
│   ├── config.yaml                  # Init items, modules, projects, dependencies
│   ├── args.yaml                    # Terraform-level parameters
│   ├── default.yaml                 # Default values for generated args.yaml
│   ├── init/
│   │   ├── args.yaml
│   │   ├── terraform-statestore.tf.tmpl
│   │   ├── projects.tf.tmpl
│   │   └── config.gcs.tfbackend.tmpl
│   ├── modules/
│   │   ├── gke/
│   │   ├── gke-node-pool/
│   │   └── gke-sm-accessor-sa/
│   └── project/
│       ├── args.yaml
│       ├── vpc.tf.tmpl
│       ├── gke.tf.tmpl
│       ├── dns.tf.tmpl
│       └── ...
├── kubernetes/
│   └── config.yaml
└── gitops/
    └── config.yaml
```

### `config.yaml` format

```yaml
version: "1.0.0"

# Init items — executed by `blcli apply init` and `blcli apply terraform`
init:
  - name: statestore
    prepare: true                    # prepare:true = run FIRST (creates state bucket)
    path:
      - terraform/init/terraform-statestore.tf.tmpl
      - terraform/init/config.gcs.tfbackend.tmpl
    destination: terraform/init/0-terraform-statestore/
    args: terraform/init/args.yaml

  - name: projects
    path:
      - terraform/init/projects.tf.tmpl
    destination: terraform/init/1-{{.GlobalName}}-projects/
    args: terraform/init/args.yaml

# Shared Terraform modules (copied verbatim)
modules:
  - name: gke
    path:
      - terraform/modules/gke

# Project components (rendered per project listed in args.yaml)
projects:
  - name: backend
    path:
      - terraform/project/backend.tf.tmpl
    args: terraform/project/backend-args.yaml
    dependencies: []

  - name: vpc
    path:
      - terraform/project/vpc.tf.tmpl
    args: terraform/project/vpc-args.yaml
    dependencies: [backend, variables]

  - name: gke
    path:
      - terraform/project/gke.tf.tmpl
    args: terraform/project/gke-args.yaml
    dependencies: [vpc]
```

**Key points:**
- `prepare: true` marks an init item to run before all others (e.g. state bucket must exist before backends can be configured)
- `destination` supports Go template variables: `1-{{.GlobalName}}-projects/`
- `dependencies` defines DAG ordering within each module; components with no unmet dependencies run in parallel
- Only components **explicitly listed** in `args.yaml` are rendered

---

## Generated Output Structure

```
./
├── args.yaml
├── .env
├── terraform/
│   ├── .blcli.marker                 # Ordered init_prepare_dirs + init_dirs
│   ├── init/
│   │   ├── 0-terraform-statestore/   # GCS backend bucket (prepare:true)
│   │   │   ├── terraform-statestore.tf
│   │   │   └── config.gcs.tfbackend
│   │   └── 1-my-org-projects/        # GCP project creation
│   │       ├── projects.tf
│   │       └── config.gcs.tfbackend
│   └── gcp/
│       ├── modules/                  # Copied from bl-template/terraform/modules/
│       │   ├── gke/
│       │   └── gke-node-pool/
│       ├── corp/
│       │   ├── backend.tf
│       │   ├── vpc.tf
│       │   ├── gke.tf
│       │   ├── dns.tf
│       │   └── config.gcs.tfbackend
│       ├── prd/
│       └── stg/
├── kubernetes/
│   ├── corp/
│   │   ├── external-secrets-operator/
│   │   ├── 0-external-secrets/
│   │   ├── 1-istio/
│   │   ├── 2-argocd/
│   │   ├── 3-victoria-metrics-operator/
│   │   └── 4-grafana/
│   ├── prd/
│   └── stg/
└── gitops/
    ├── stg/
    │   ├── hello-world/
    │   │   ├── deployment.yaml
    │   │   ├── service.yaml
    │   │   └── app.yaml              # ArgoCD Application CRD
    │   └── hello-world-2/
    └── prd/
        └── hello-world/
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                         blcli                           │
│  ┌──────────┐  ┌──────────┐  ┌───────────────────────┐ │
│  │  CLI     │  │ Template │  │      Bootstrap        │ │
│  │ (Cobra)  │→ │ Loader   │→ │  terraform / k8s /    │ │
│  │          │  │ (cache)  │  │  gitops / destroy     │ │
│  └──────────┘  └──────────┘  └───────────────────────┘ │
│                     ↓                  ↓                │
│              bl-template          args.yaml             │
│              config.yaml          .env                  │
│              *.tf.tmpl                                  │
└─────────────────────────────────────────────────────────┘
         ↓                    ↓                ↓
   terraform/gcp/      kubernetes/corp/   gitops/stg/
   (generated HCL)     (generated YAML)   (ArgoCD apps)
```

**Design principles:**

| Principle | Implementation |
|-----------|---------------|
| Self-describing templates | Each template repo defines its own structure via `config.yaml` |
| Render plan before write | Builds a complete plan before touching the filesystem; fails fast on validation errors |
| Layered configuration | 4-level inheritance: global → terraform.global → project.global → component.parameters |
| Smart caching | `git clone` once, `git pull` on expiry; `--force-update` to bypass |
| Component filtering | Only components listed in `args.yaml` are rendered — no surprise files |
| Idempotent apply | Every `apply` subcommand is safe to re-run; converges to desired state |

---

## Troubleshooting

### GCP projects in `DELETE_REQUESTED` state

Projects soft-deleted within the last 30 days cannot be imported by Terraform. Restore them first:

```bash
gcloud projects undelete <project-id>
# Restore all three at once
gcloud projects undelete my-org-corp-<suffix>
gcloud projects undelete my-org-prd-<suffix>
gcloud projects undelete my-org-stg-<suffix>
```

### `google_project.main` import error

If Terraform reports `resource address "google_project.main" does not exist`, your template uses `for_each` — import with the keyed address:

```bash
terraform import 'google_project.projects["my-org-prd-abc123"]' my-org-prd-abc123
```

### 409 resource-already-exists conflicts

When existing GCP resources are not in Terraform state, `terraform apply` returns 409. Import them manually:

```bash
# VPC network
terraform -chdir=terraform/gcp/corp import google_compute_network.main \
  projects/my-org-corp-<suffix>/global/networks/main

# Subnet
terraform -chdir=terraform/gcp/corp import google_compute_subnetwork.default \
  my-org-corp-<suffix>/us-west1/us-west-1

# GKE cluster
terraform -chdir=terraform/gcp/corp import "module.gke.google_container_cluster.main" \
  projects/my-org-corp-<suffix>/locations/us-west1/clusters/corp-cluster

# GKE node pool
terraform -chdir=terraform/gcp/corp import "module.gke.google_container_node_pool.main" \
  projects/my-org-corp-<suffix>/locations/us-west1/clusters/corp-cluster/nodePools/main
```

Re-run `blcli apply terraform -d ./terraform/` after each batch of imports — it converges progressively.

### Billing disabled on restored projects

Re-attach billing after undeleting a project:

```bash
gcloud billing projects link <project-id> --billing-account=<BILLING_ACCOUNT_ID>
# Verify
gcloud billing projects describe <project-id>
```

### `kubectl config.lock` file error

A concurrent `kubectl` config write left a stale lock:

```bash
rm -f ~/.kube/config.lock
```

### Terraform state bucket `prevent_destroy` error

The GCS state bucket has `lifecycle.prevent_destroy = true` by design. If a plan tries to replace it (e.g. after adding an explicit `project =` attribute to `google_storage_bucket`), remove that attribute from the statestore template — the provider default project is sufficient and avoids a forced replacement.

### GKE node pool CPU quota exceeded

Large node pools (`e2-standard-8`) consume quota quickly. Scale down idle pools before Terraform creates new ones:

```bash
gcloud container clusters resize <cluster> --node-pool=on-demand \
  --num-nodes=0 --region=<region> --project=<project-id> --quiet
# Verify quota freed
gcloud compute project-info describe --project=<project-id> \
  --format="json" | jq '.quotas[] | select(.metric | test("CPU"))'
```

### Missing backend configuration

If you see `Warning: Missing backend configuration`, the generated project Terraform is missing the `terraform { backend "gcs" {} }` block, so `-backend-config` is silently ignored and state falls back to local.

**Fix:** Ensure the template's `backend.tf.tmpl` contains:
```hcl
terraform {
  backend "gcs" {}
}
```
Then regenerate and re-initialise:
```bash
blcli init -a args.yaml -f ../bl-template -m terraform --overwrite
terraform -chdir=terraform/gcp/prd init -input=false -backend-config=config.gcs.tfbackend
terraform -chdir=terraform/gcp/prd state pull | jq '.resources | length'
```

### ArgoCD SSH authentication failures

ArgoCD fails to clone the GitOps repository if:

1. **HTTPS URL used instead of SSH** — all GitOps repo URLs in `.env` must be `git@github.com:org/repo.git`
2. **Wrong repository name** — use the real repo name (e.g. `infra-gitops`), not a placeholder like `gitops`
3. **`useSshAgent` present in the sealed secret** — regenerate the ArgoCD SSH secret and ensure `useSshAgent` is absent from the secret YAML before sealing

### Verify Terraform is using GCS backend

```bash
terraform init -input=false -backend-config=config.gcs.tfbackend
terraform state pull | jq '.resources | length'
# Must be > 0 if resources were previously created
```

---

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Make changes and add tests in `workspace/`
4. Build and run the verification suite: `go build -o blcli ./cmd/blcli && blcli check`
5. Open a Pull Request against `main`

**Conventional commit prefixes:** `feat:`, `fix:`, `docs:`, `refactor:`, `test:`

---

## License

MIT © NFTGalaxy
