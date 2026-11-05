# blcli Usage Guide

This document provides detailed usage examples and best practices for using `blcli`.
For Chinese documentation, see [USAGE_zh.md](./USAGE_zh.md).

## Table of Contents

- [Quick Start](#quick-start)
- [Command Reference](#command-reference)
- [Configuration Guide](#configuration-guide)
- [Template Repository Guide](#template-repository-guide)
- [Common Workflows](#common-workflows)
- [Agent Workflows](#agent-workflows)
- [Troubleshooting](#troubleshooting)

## Quick Start

### Step 1: Generate Configuration

```bash
# Generate args.yaml from template repository
blcli init-args -r github.com/ggsrc/bl-template -o args.yaml

# Or use local template repository
blcli init-args -r /path/to/bl-template -o args.yaml
```

This creates an `args.yaml` file with all available parameters organized hierarchically.

### Step 2: Customize Configuration

Edit `args.yaml` to set your values:

```yaml
global:
  GlobalName: "my-org"

terraform:
  version: "1.0.0"
  global:
    # Use "0" to disable org_id in Terraform init output (no org_id in variable/resources)
    OrganizationID: "123456789012"
    BillingAccountID: "01ABCD-2EFGH3-4IJKL5"
    GlobalName: "my-org"
  
  init:
    components:
      projects:
        ProjectServices:
          "${project.prd.id}":
            - "compute.googleapis.com"
            - "container.googleapis.com"
  
  projects:
    - name: "prd"
      global:
        project_name: "prd"
      components:
        - name: "backend"
        - name: "variables"
          parameters:
            project_id: "my-project-123"
            region: "us-west1"
```

### Step 3: Initialize Projects

```bash
# Initialize terraform projects
blcli init terraform -r github.com/ggsrc/bl-template -a args.yaml

# Or use local template repository
blcli init terraform -r /path/to/bl-template -a args.yaml
```

## Command Reference

### `blcli contract`

Print the v2 AI-agent tool contract.

```bash
blcli contract --format json
blcli contract "apply terraform" --format yaml
```

Use this when an automation system needs stable command inputs, outputs, JSON schemas, exit codes, examples, compatibility policy, and guidance.

### `blcli diagnose`

Classify a failed command output and suggest repair steps.

```bash
blcli diagnose --message "Error 409: already exists" --format json
blcli diagnose --file execution_stage5.log
```

Use JSON output for agents. The result includes `category`, `confidence`, `matched_keywords`, `next_steps`, and `repair_commands`.
Known failures also print a concise diagnosis to stderr when normal commands fail.
Offline failure samples are available under `integration/fixtures/failures/` for safe replay.
Agent replay playbooks are available under `integration/fixtures/agent-replay/`.

### `blcli runs`

List and inspect persisted run records from `~/.blcli/progress`.

```bash
blcli runs list --format json
blcli runs list --status failed
blcli runs show op-20260529-103000-app --format yaml
```

Use this to look up run ids, inspect step status, and retrieve stored error messages. Step records include timestamps, duration, status, command/action, output excerpt, error location, and error message when available.

### `blcli init-args`

Generates a configuration file from template repository parameter definitions.

#### Basic Usage

```bash
# Generate YAML format (default)
blcli init-args -r github.com/user/repo -o args.yaml

# Generate TOML format
blcli init-args -r github.com/user/repo -o args.toml --format toml
```

#### Template Repository Options

```bash
# GitHub repository (public or private)
blcli init-args -r github.com/ggsrc/bl-template -o args.yaml

# GitHub repository with specific branch/tag
blcli init-args -r github.com/ggsrc/bl-template@v1.0.0 -o args.yaml

# Local absolute path
blcli init-args -r /Users/username/code/bl-template -o args.yaml

# Local relative path
blcli init-args -r ./bl-template -o args.yaml

# Local path with file:// protocol
blcli init-args -r file:///Users/username/code/bl-template -o args.yaml
```

#### Flags

- `-r, --template-repo`: Template repository URL or local path (required)
- `-o, --output`: Output file path (default: `args.yaml`)
- `--format`: Output format: `yaml` or `toml` (default: `yaml`)
- `-f, --force-update`: Force update templates from remote repository

#### What Gets Generated

The `init-args` command collects parameter definitions from multiple levels:

1. **Repository-level** (`bl-template/args.yaml`):
   ```yaml
   global:
     GlobalName: "my-org"
   ```

2. **Terraform-level** (`bl-template/terraform/args.yaml`):
   ```yaml
   terraform:
     global:
       OrganizationID: "123456789012"
       BillingAccountID: "01ABCD-2EFGH3-4IJKL5"
   ```

3. **Project-level** (`bl-template/terraform/project/args.yaml`):
   ```yaml
   terraform:
     projects:
       - name: "prd"
         global:
           project_name: "prd"
   ```

4. **Component-level** (individual `*-args.yaml` files):
   ```yaml
   terraform:
     projects:
       - name: "prd"
         components:
           - name: "backend"
             parameters: {}
   ```

5. **Init components** (`bl-template/terraform/init/args.yaml`):
   ```yaml
   terraform:
     init:
       components:
         projects:
        ProjectServices: {...}  # keys: ${project.<name>.id} placeholders
           ProjectServices: {...}
   ```

### `blcli init`

Initializes infrastructure projects using templates.

#### Basic Usage

```bash
# Initialize all modules (terraform, kubernetes, gitops)
blcli init -r github.com/user/repo -a args.yaml

# Initialize only terraform
blcli init terraform -r github.com/user/repo -a args.yaml

# Initialize multiple modules
blcli init terraform kubernetes -r github.com/user/repo -a args.yaml
```

#### Template Repository Options

Same as `init-args` command - supports GitHub repositories and local paths.

#### Multiple Args Files

You can specify multiple args files, with earlier files overriding later ones:

```bash
blcli init terraform -r github.com/user/repo -a base.yaml -a override.yaml -a local-override.yaml
```

This is useful for:
- Base configuration shared across environments
- Environment-specific overrides
- Local development overrides

#### Flags

- `-r, --template-repo`: Template repository URL or local path (required)
- `-a, --args`: Args file path (YAML or TOML), can be specified multiple times (required)
- `-w, --overwrite`: Overwrite existing blcli-managed directories
- `-f, --force-update`: Force update templates from remote repository
- `--cache-expiry`: Cache expiry duration (default: 24h, 0 = no expiry)

#### What Gets Generated

1. **Init Files** (`terraform/init/`):
   - Based on `terraform.init.components` in `args.yaml`
   - Only components listed in `args.yaml` are generated
   - Output directories are specified by `destination` in `config.yaml`

2. **Modules** (`terraform/gcp/modules/`):
   - All modules defined in `config.yaml` are copied
   - Shared across all projects

3. **Projects** (`terraform/gcp/{project-name}/`):
   - Only projects listed in `terraform.projects[]` are generated
   - Only components explicitly listed in `args.yaml` are rendered
   - Each project gets its own directory with rendered files

#### Component Filtering

Only components explicitly listed in `args.yaml` are rendered:

```yaml
terraform:
  projects:
    - name: "prd"
      components:
        - name: "backend"      # ✅ Rendered
        - name: "variables"    # ✅ Rendered
        # "gke" not listed     # ❌ Skipped
        # "main" not listed    # ❌ Skipped
```

This allows you to:
- Generate minimal configurations for testing
- Gradually add components as needed
- Use different component sets for different projects

#### GitOps Output

When `args.yaml` contains a `gitops` section (`argocd.project` and `apps`), `blcli init` generates GitOps configuration under `{workspace}/gitops/{project}/{app_name}/`, including Deployment/StatefulSet, Service, ConfigMap, and ArgoCD Application (`app.yaml`) manifests.

### `blcli apply`

Deploy generated configuration or initialize repositories. Subcommands include: `terraform`, `kubernetes`, `gitops`, `all`, and `init-repos`.

#### apply init-repos

For `terraform`, `kubernetes`, and `gitops` directories, this command runs: `git init` -> confirm `gh repo create` -> confirm `git add` / `commit` / `push`. It requires `gh` to be installed and authenticated.

```bash
# Required: `-d` root directory (contains `terraform`, `kubernetes`, `gitops`), `-o/--org` GitHub organization or username
blcli apply init-repos --org myorg -d ./workspace/output

# Optional: custom subdirectory paths
blcli apply init-repos -o myorg -d ./out --terraform-dir ./out/tf --kubernetes-dir ./out/k8s --gitops-dir ./out/gitops
```

**Flags:**
- `-d, --dir`: Root working directory (required)
- `-o, --org`: GitHub organization or username (required)
- `--terraform-dir`: Default `{dir}/terraform`
- `--kubernetes-dir`: Default `{dir}/kubernetes`
- `--gitops-dir`: Default `{dir}/gitops`

**Private keys are never committed.** Before any git or `gh` command, init-repos scans the three
directories (skipping `.git`) and stops with a list of findings if it sees private-key material:
a PEM/OpenSSH private key header in any file, file names `id_*` (except `*.pub`), `*.pem`, `*.key`,
`*.p12`, `*.pfx`, or a `keys/` directory that holds more than public keys. Move those files out of
the directory and run the command again; a `.gitignore` entry is not enough. If a key was ever
committed or pushed, rotate it: removing it from the latest commit does not remove it from history.
init-repos also adds `keys/` to `kubernetes/.gitignore`.

#### Generated keys

`blcli init` runs the template's `kubernetes/init.sh` with `BLCLI_KEYS_DIR=<workspace>/keys`, where
`<workspace>` is the output directory that contains `terraform/`, `kubernetes/` and `gitops/`.
The directory is created with mode 0700 and a `.gitignore` containing `*`, and is not part of any of
the generated repositories. Templates write generated
private keys there, for example the ArgoCD GitHub deploy key `argocd_github`. Set `BLCLI_KEYS_DIR`
in the environment to use another directory outside the generated repositories.
`blcli apply kubernetes` passes the same directory to custom install scripts as `BLCLI_KEYS_DIR`
and, unless it is already set, `ARGOCD_KEYS_DIR`.

The Dex GitHub OAuth client secret is not an arg: `argocd-cm` references a sealed Secret, and the
ArgoCD install seals the value from `DEX_GITHUB_CLIENT_SECRET` (`gen_dex_sealed_secret.sh`). Export it in
the shell that runs `blcli apply kubernetes`; blcli passes the environment through and never sets it
from args. `.env` variables named `*_DEX_GITHUB_CLIENT_SECRET` / `*_GITHUB_AUTH_CLIENT_SECRET` are
ignored with a notice.

Workspaces generated by older blcli versions have the key in `<workspace>/kubernetes/keys`.
`blcli init` moves it to `<workspace>/keys`; until then `apply kubernetes` keeps using the
old location and prints a warning.

#### apply gitops

Runs `kubectl apply -f` for all GitOps `app.yaml` (ArgoCD Application) files. Actual app rollout is handled by ArgoCD sync.

```bash
blcli apply gitops -d ./workspace/output/gitops --args args.yaml

# Specify kubeconfig and context
blcli apply gitops -d ./generated/gitops --args args.yaml --kubeconfig ~/.kube/config --context my-cluster
```

## Configuration Guide

### Hierarchical Parameter Structure

Parameters are organized in a hierarchical structure that matches the template repository layout:

```yaml
# Level 1: Repository-level global
global:
  GlobalName: "my-org"

# Level 2: Terraform-level global
terraform:
  global:
    OrganizationID: "123456789012"
    BillingAccountID: "01ABCD-2EFGH3-4IJKL5"
    GlobalName: "my-org"  # Overrides repository-level GlobalName

# Level 3: Project-level global
terraform:
  projects:
    - name: "prd"
      global:
        project_name: "prd"  # Project-specific

# Level 4: Component-level parameters
terraform:
  projects:
    - name: "prd"
      components:
        - name: "variables"
          parameters:
            project_id: "my-project-123"  # Component-specific
            region: "us-west1"
```

### Parameter Inheritance

Parameters are inherited from higher levels and can be overridden:

1. **Repository global** → Base for everything
2. **Terraform global** → Overrides repository global for terraform
3. **Project global** → Overrides terraform global for specific project
4. **Component parameters** → Component-specific, merged with project global

### Parameter Flattening

All global parameters are automatically flattened to the top level for template access:

```yaml
# In args.yaml
global:
  GlobalName: "my-org"
terraform:
  global:
    OrganizationID: "123456789012"
  projects:
    - name: "prd"
      global:
        project_name: "prd"
```

```hcl
# In template (backend.tf.tmpl)
terraform {
  backend "gcs" {
    bucket = "{{ .GlobalName }}-terraform-backend"        # From global or terraform.global
    prefix = "terraform/gcp/{{ .GlobalName }}/{{ .project_name }}"  # project_name from project.global
  }
}
```

When the same key is set at several levels, the more specific level always wins, in this
order (lowest first): repository `global`, `terraform.global`, project `global`, component
`parameters`.

### Parameter Names in Templates

Args files and `default.yaml` usually spell parameters in kebab-case (`hosted-domain`) or
snake_case (`project_id`), while templates read PascalCase or lowerCamelCase. blcli makes every
component parameter readable under its own key and under these spellings:

| Key in args | Also readable in templates as |
|-------------|-------------------------------|
| `hosted-domain` | `.HostedDomain`, `.hostedDomain` |
| `root-url` | `.RootURL`, `.rootUrl`, `.rootURL` |
| `project_id` | `.ProjectID`, `.projectId`, `.projectID` |
| `imageTag` | `.ImageTag` |
| `IstioVersion` | `.istioVersion` |

PascalCase keeps common abbreviations in upper case (`ID`, `URL`, `API`, `ArgoCD`, ...). A key
that is spelled out in the args file always keeps its own value; aliases only fill in the other
spellings.

For Kubernetes components, the `default`/`example` values in the component's `args.yaml` fill in
parameters the args file does not set. A value set in the args file replaces the args.yaml default
whatever its spelling, so `image-tag: v2.0.1` in args replaces `imageTag: ""` from args.yaml.
A component that a project only pulls in as a dependency (it is not listed under that project's
`components`) renders with its args.yaml defaults; it does not borrow parameters that another
project set for the same component.

### Template Functions

Templates are Go `text/template` files with three extra functions:

- `default`: Sprig argument order. `{{ default "d" .Value }}` and `{{ .Value | default "d" }}`
  both return `.Value` unless it is unset, `nil` or the empty string, in which case they return
  `"d"`. An explicit `false` or `0` is kept, so `enabled: false` in args survives
  `{{ .Enabled | default true }}`.
- `quote`: wraps a value in double quotes, escaping quotes inside.
- `replace`: `{{ replace .name "_" "-" }}` replaces every occurrence.

### Init Components Configuration

Init components are configured under `terraform.init.components`:

```yaml
terraform:
  init:
    components:
      backend: {}  # Empty means use defaults from args.yaml
      projects:
        ProjectServices:  # keys: ${project.<name>.id} placeholders
          "${project.prd.id}":
            - "compute.googleapis.com"
            - "container.googleapis.com"
      atlantis:
        AtlantisName: "my-atlantis"
```

Only components listed here will be generated. If a component is not listed, it will be skipped.

## Template Repository Guide

The canonical sample template repository is **github.com/ggsrc/bl-template**. For the full specification of directory structure, `config.yaml`, `default.yaml`, and `args.yaml` formats (including [parameter types](ARGS_YAML_TYPES.md)), see [Template Repository Protocol](TEMPLATE_REPO_PROTOCOL.md).

### Repository Structure

A template repository should follow this structure:

```
bl-template/
├── args.yaml                    # Repository-level global parameters
├── terraform/
│   ├── config.yaml              # Defines structure (init, modules, projects)
│   ├── args.yaml                # Terraform-level global parameters
│   ├── init/
│   │   ├── args.yaml            # Init components parameter definitions
│   │   ├── main.tf.tmpl
│   │   ├── variable.tf.tmpl
│   │   └── projects.tf.tmpl
│   ├── modules/
│   │   └── gke/
│   │       ├── main.tf.tmpl
│   │       └── variables.tf.tmpl
│   └── project/
│       ├── args.yaml            # Project-level global parameters
│       ├── backend-args.yaml    # Backend component parameters
│       ├── backend.tf.tmpl
│       └── variables.tf.tmpl
```

### config.yaml Structure

```yaml
version: "1.0.0"

# Init items (org-level initialization)
init:
  - name: "backend"
    path:
      - terraform/init/main.tf.tmpl
      - terraform/init/variable.tf.tmpl
    destination: terraform/init/0-terraform-backend/
    args: terraform/init/args.yaml
  
  - name: "projects"
    path:
      - terraform/init/main.tf.tmpl
      - terraform/init/projects.tf.tmpl
    destination: terraform/init/1-{{.GlobalName}}-projects/
    args: terraform/init/args.yaml

# Shared modules
modules:
  - name: "gke"
    path:
      - terraform/modules/gke

# Project components
projects:
  - name: "backend"
    path:
      - terraform/project/backend.tf.tmpl
    args: terraform/project/backend-args.yaml
    dependencies: []
  
  - name: "variables"
    path:
      - terraform/project/variables.tf.tmpl
    args: terraform/project/variables-args.yaml
    dependencies: ["backend"]
```

**Key Fields:**
- `path`: List of template files to render
- `destination`: Output directory path (relative to workspace root), supports template variables
- `args`: Parameter definition file path
- `dependencies`: Component dependency order (not currently enforced, but documented)

### args.yaml Structure (Parameter Definitions)

Each `args.yaml` file defines parameters for that level:

```yaml
version: "1.0.0"

parameters:
  # Global parameters (flattened to top level)
  global:
    GlobalName:
      type: string
      description: "Global name used for resource naming"
      required: true
      example: "my-org"
  
  # Component parameters
  components:
    backend:
      description: "Terraform backend configuration"
      # GlobalName and project_name are inherited from global
```

## Common Workflows

### Workflow 1: New Project Setup

```bash
# 1. Generate configuration
blcli init-args -r github.com/ggsrc/bl-template -o args.yaml

# 2. Edit args.yaml with your values
vim args.yaml

# 3. Initialize projects
blcli init terraform -r github.com/ggsrc/bl-template -a args.yaml

# 4. Review generated files
ls -la terraform/gcp/prd/
```

### Workflow 2: Local Development

```bash
# 1. Clone template repository locally
git clone github.com/ggsrc/bl-template /path/to/bl-template

# 2. Make changes to templates
vim /path/to/bl-template/terraform/project/backend.tf.tmpl

# 3. Generate args.yaml from local template
blcli init-args -r /path/to/bl-template -o args.yaml

# 4. Initialize with local template
blcli init terraform -r /path/to/bl-template -a args.yaml
```

### Workflow 3: Multiple Environments

```bash
# 1. Create base configuration
blcli init-args -r github.com/ggsrc/bl-template -o base.yaml

# 2. Create environment-specific overrides
cp base.yaml prod.yaml
cp base.yaml dev.yaml

# Edit prod.yaml and dev.yaml with environment-specific values

# 3. Initialize production
blcli init terraform -r github.com/ggsrc/bl-template -a base.yaml -a prod.yaml

# 4. Initialize development
blcli init terraform -r github.com/ggsrc/bl-template -a base.yaml -a dev.yaml
```

### Workflow 4: Adding New Components

```bash
# 1. Add component to args.yaml
vim args.yaml
# Add new component to terraform.projects[].components[]

# 2. Re-run init (only new components will be rendered)
blcli init terraform -r github.com/ggsrc/bl-template -a args.yaml
```

### Workflow 5: GitOps Generation and Deployment

```bash
# 1. init-args includes gitops section (if template has gitops/config.yaml and default.yaml)
blcli init-args -r github.com/ggsrc/bl-template -o args.yaml

# 2. Fill gitops.argocd.project and gitops.apps in args.yaml
vim args.yaml

# 3. Generate GitOps config into {workspace}/gitops/{project}/{app_name}/
blcli init -r github.com/ggsrc/bl-template -a args.yaml -w

# 4. Apply ArgoCD Applications
blcli apply gitops -d ./workspace/output/gitops --args args.yaml
```

### Workflow 6: Create and Push GitHub Repositories with init-repos

```bash
# 1. Generate terraform/kubernetes/gitops directories first
blcli init -r github.com/ggsrc/bl-template -a args.yaml -w --output ./workspace/output

# 2. Run git init, create GitHub repos, and push for all three directories (interactive confirmation)
blcli apply init-repos -o myorg -d ./workspace/output
# Requires authenticated GitHub CLI: gh auth login
```

## Agent Workflows

### Inspect the tool contract

```bash
blcli contract --format json
blcli contract "apply terraform" --format json
blcli contract "runs list" --format json
```

Agents should read the contract before planning command calls and validate arguments against `input_schema` when possible. Prefer commands with `--dry-run` and `--format json` when available.

### Diagnose a failed run

```bash
# Capture the failed command output first
blcli apply terraform -d ./workspace/output/terraform --project prd > execution_stage5.log 2>&1

# Classify and repair
blcli diagnose --file execution_stage5.log --format json

# Inspect stored run details
blcli runs list --status failed --format json
blcli runs show <operation-id> --format json
```

Use the returned `repair_commands` as candidates. Confirm destructive or production-impacting commands with a human before execution.
When inspecting `runs show`, steps from `apply all` include the actual Terraform, Kubernetes, and GitOps subprocess command plus the captured output excerpt.

## Troubleshooting

### Issue: Template not found

**Error:** `failed to load template: terraform/project/gke.tf.tmpl: template not found`

**Solution:**
- Check if the template file exists in the template repository
- Verify the path in `config.yaml` is correct
- Use `--force-update` to refresh cache

### Issue: project_name is empty in generated files

**Error:** Generated `backend.tf` shows `prefix = "terraform/gcp/my-org/"` (missing project_name)

**Solution:**
- Ensure `project_name` is set in `terraform.projects[].global.project_name` in `args.yaml`
- Verify the project name matches the `name` field in `terraform.projects[]`

### Issue: Component not rendered

**Error:** Component exists in `config.yaml` but not generated

**Solution:**
- Check if the component is explicitly listed in `terraform.projects[].components[]` in `args.yaml`
- Only components listed in `args.yaml` are rendered (component filtering)

### Issue: Module initialization fails

**Error:** `failed to copy module security-policy-corp-ip-whitelist: no template files found`

**Solution:**
- This is a warning, not an error - other modules will continue to be processed
- Check if the module directory exists in the template repository
- Use `--force-update` to refresh cache

### Issue: Local path not working

**Error:** `local path does not exist` or `local path is not a directory`

**Solution:**
- Use absolute paths: `/Users/username/code/bl-template`
- Or relative paths: `./bl-template` or `../bl-template`
- Ensure the path points to a directory, not a file

### Issue: Cache issues

**Error:** Templates are outdated or missing

**Solution:**
- Use `--force-update` to bypass cache
- Clear cache manually: `rm -rf ~/.blcli/templates/`
- For local paths, no cache is used - changes are immediate

## Best Practices

1. **Use version control for args.yaml**: Commit your `args.yaml` files to version control
2. **Use multiple args files**: Separate base configuration from environment-specific overrides
3. **Test with local templates**: Use local template repository for development and testing
4. **Component filtering**: Only list components you need in `args.yaml` to keep generated code minimal
5. **Regular updates**: Use `--force-update` periodically to get latest templates
6. **Review generated files**: Always review generated files before applying to production
