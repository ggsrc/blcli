# blcli Usage Guide

This document provides detailed usage examples and best practices for using `blcli`.

## Table of Contents

- [Quick Start](#quick-start)
- [Command Reference](#command-reference)
- [Configuration Guide](#configuration-guide)
- [Template Repository Guide](#template-repository-guide)
- [Common Workflows](#common-workflows)
- [Troubleshooting](#troubleshooting)

## Quick Start

### Step 1: Generate Configuration

```bash
# Generate args.yaml from template repository
blcli init-args -r github.com/NFTGalaxy/bl-template -o args.yaml

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
blcli init terraform -r github.com/NFTGalaxy/bl-template -a args.yaml

# Or use local template repository
blcli init terraform -r /path/to/bl-template -a args.yaml
```

## Command Reference

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
blcli init-args -r github.com/NFTGalaxy/bl-template -o args.yaml

# GitHub repository with specific branch/tag
blcli init-args -r github.com/NFTGalaxy/bl-template@v1.0.0 -o args.yaml

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

#### GitOps 输出

当 `args.yaml` 含有 `gitops` 段（`argocd.project` 与 `apps`）时，`blcli init` 会生成 GitOps 配置到 `{workspace}/gitops/{project}/{app_name}/`，包含 Deployment/StatefulSet、Service、ConfigMap、ArgoCD Application（`app.yaml`）等。

### `blcli apply`

对已生成配置执行部署或仓库初始化。子命令：`terraform`、`kubernetes`、`gitops`、`all`、`init-repos`。

#### apply init-repos

对 terraform/kubernetes/gitops 三个目录分别执行：`git init` → 确认后 `gh repo create` → 确认后 `git add` / `commit` / `push`。需已安装并登录 `gh`。

```bash
# 必填：-d 工作目录（包含 terraform、kubernetes、gitops），-o/--org GitHub 组织或用户名
blcli apply init-repos --org myorg -d ./workspace/output

# 自定义各目录路径（可选）
blcli apply init-repos -o myorg -d ./out --terraform-dir ./out/tf --kubernetes-dir ./out/k8s --gitops-dir ./out/gitops
```

**Flags：**
- `-d, --dir`：工作目录根路径（必填）
- `-o, --org`：GitHub 组织或用户名（必填）
- `--terraform-dir`：默认 `{dir}/terraform`
- `--kubernetes-dir`：默认 `{dir}/kubernetes`
- `--gitops-dir`：默认 `{dir}/gitops`

#### apply gitops

对 GitOps 目录下所有 `app.yaml`（ArgoCD Application）执行 `kubectl apply -f`。实际应用部署由 ArgoCD 同步完成。

```bash
blcli apply gitops -d ./workspace/output/gitops --args args.yaml

# 指定 kubeconfig 与 context
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

The canonical sample template repository is **github.com/NFTGalaxy/bl-template**. For the full specification of directory structure, `config.yaml`, `default.yaml`, and `args.yaml` formats (including [parameter types](ARGS_YAML_TYPES.md)), see [Template Repository Protocol](TEMPLATE_REPO_PROTOCOL.md).

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
blcli init-args -r github.com/NFTGalaxy/bl-template -o args.yaml

# 2. Edit args.yaml with your values
vim args.yaml

# 3. Initialize projects
blcli init terraform -r github.com/NFTGalaxy/bl-template -a args.yaml

# 4. Review generated files
ls -la terraform/gcp/prd/
```

### Workflow 2: Local Development

```bash
# 1. Clone template repository locally
git clone github.com/NFTGalaxy/bl-template /path/to/bl-template

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
blcli init-args -r github.com/NFTGalaxy/bl-template -o base.yaml

# 2. Create environment-specific overrides
cp base.yaml prod.yaml
cp base.yaml dev.yaml

# Edit prod.yaml and dev.yaml with environment-specific values

# 3. Initialize production
blcli init terraform -r github.com/NFTGalaxy/bl-template -a base.yaml -a prod.yaml

# 4. Initialize development
blcli init terraform -r github.com/NFTGalaxy/bl-template -a base.yaml -a dev.yaml
```

### Workflow 4: Adding New Components

```bash
# 1. Add component to args.yaml
vim args.yaml
# Add new component to terraform.projects[].components[]

# 2. Re-run init (only new components will be rendered)
blcli init terraform -r github.com/NFTGalaxy/bl-template -a args.yaml
```

### Workflow 5: GitOps 生成与部署

```bash
# 1. init-args 会包含 gitops 段（若模板有 gitops/config.yaml 与 default.yaml）
blcli init-args -r github.com/NFTGalaxy/bl-template -o args.yaml

# 2. 在 args.yaml 中填写 gitops.argocd.project、gitops.apps（ApplicationName、SourcePath、SourceRepoURL 等）
vim args.yaml

# 3. init 生成 gitops 配置到 {workspace}/gitops/{project}/{app_name}/
blcli init -r github.com/NFTGalaxy/bl-template -a args.yaml -w

# 4. 对 ArgoCD Application 执行 kubectl apply
blcli apply gitops -d ./workspace/output/gitops --args args.yaml
```

### Workflow 6: init-repos 创建 GitHub 仓库并推送

```bash
# 1. 先完成 init 生成 terraform/kubernetes/gitops 目录
blcli init -r github.com/NFTGalaxy/bl-template -a args.yaml -w --output ./workspace/output

# 2. 对三个目录分别 git init、创建 GitHub 仓库、提交推送（需交互输入 Y 确认）
blcli apply init-repos -o myorg -d ./workspace/output
# 需已安装并登录 gh：gh auth login
```

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

