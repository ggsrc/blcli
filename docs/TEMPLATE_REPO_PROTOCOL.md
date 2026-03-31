# Template Repository Protocol

本文档约定 blcli 所使用的**模板仓库**的目录结构、配置文件格式与发现规则，供实现或维护模板仓库（如 github.com/NFTGalaxy/bl-template）时遵循。实现对应关系见 `pkg/template/config.go`、`pkg/cli/init_args.go`、`pkg/renderer/argsdef.go`。

---

## 1. 概述

**模板仓库**是 blcli 通过 `--template-repo` 加载的代码库，可以是：

- GitHub 仓库 URL：`github.com/user/repo` 或 `github.com/user/repo@branch`
- 本地路径：绝对路径或相对路径（如克隆后的 `./bl-template`，或 [github.com/NFTGalaxy/bl-template](https://github.com/NFTGalaxy/bl-template) 的本地路径）

blcli 从模板仓库中读取**三大模块**的配置与模板文件：

| 模块        | 说明                         |
|-------------|------------------------------|
| `terraform` | Terraform 初始化、模块、项目 |
| `kubernetes`| Kubernetes 组件（init/optional） |
| `gitops`    | GitOps 应用模板与 ArgoCD 配置 |

每个模块**可选**：仅当仓库中存在对应子目录且包含 `config.yaml` 时，blcli 才会加载该模块。

**约定**：

- 模板文件使用 **`.tmpl`** 扩展名，语法为 **Go `text/template`**。
- 所有 config 中的 **path** 均为**相对仓库根目录**的路径，且指向仓库内实际存在的 `.tmpl` 文件或目录。

---

## 2. 根目录结构

顶层结构约定如下：

- **可选** 根级 **`args.yaml`**：仓库级全局参数定义（`parameters.global`），会被 `blcli init-args` 等合并到生成配置中。
- **固定子目录**（存在则需符合下文各模块格式）：
  - `terraform/`
  - `kubernetes/`
  - `gitops/`

每个模块目录内需包含：

- **`config.yaml`**（必需）：该模块的组件/项目/模板定义。
- **`default.yaml`**（可选但建议）：默认值结构，供 `blcli init-args` 生成用户侧 args 文件；若三个模块都无 `default.yaml`，init-args 会报错。
- 子目录下的 **`args.yaml`** 与 **`.tmpl`** 按 config 中的 `path` / `args` 引用。

最小合法结构示意：

```
template-repo/
├── args.yaml                 # 可选，仓库级 parameters.global
├── terraform/
│   ├── config.yaml           # 必需
│   ├── default.yaml          # 可选，init-args 用
│   ├── args.yaml             # 可选，terraform 级全局
│   ├── init/
│   │   ├── args.yaml
│   │   └── *.tf.tmpl
│   ├── modules/
│   │   └── <module-name>/
│   │       └── *.tf.tmpl
│   └── project/
│       ├── args.yaml
│       └── *.tf.tmpl
├── kubernetes/
│   ├── config.yaml
│   ├── default.yaml
│   └── components/
│       └── <component-name>/
│           ├── args.yaml
│           └── *.yaml.tmpl
└── gitops/
    ├── config.yaml
    ├── default.yaml
    ├── args.yaml
    └── *.yaml.tmpl
```

---

## 3. Terraform 模块

### 3.1 config 路径与加载

- **路径**：`terraform/config.yaml`（Loader 固定读取，见 `config.go` 中 `LoadTerraformConfig`）。

### 3.2 config 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `version` | string | 建议 `1.0.0` |
| `init` | list | 初始化项列表 |
| `modules` | list 或 map | 可复用模块列表/映射 |
| `project-global` | string | 项目级全局 args 文件路径（如 `terraform/project/args.yaml`） |
| `projects` | list | 项目项列表 |

**init 项**（每项）：

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 名称 |
| `path` | string 或 list | 模板路径，相对仓库根 |
| `destination` | string | 输出目录（可含模板变量如 `{{.GlobalName}}`） |
| `args` | string 或 list | args 文件路径，可选 |
| `install` / `upgrade` | string | 安装/升级命令 |
| `rollback` | string | 可选，回滚命令 |

**modules 项**（每项）：`name`, `path`（string 或 list）, 可选 `args`, `install`, `upgrade`, `rollback`。

**projects 项**（每项）：`name`, `path`, `args`, `dependencies`（依赖的其他 project name）, `install`, `upgrade`, `rollback`。

### 3.3 args 发现规则

若 config 中未指定 `args`，blcli 根据 `path` 推断：

- 若 `path` 为**文件**（以 `.tmpl`、`.yaml`、`.tf` 结尾）：查找 **`{path 所在目录}/args.yaml`**。
- 若 `path` 为**目录**：查找 **`{path}/args.yaml`**。

（与 `pkg/cli/explain.go` 中 `findArgsPath` / `findArgsPathFromPaths` 一致。）

### 3.4 default.yaml

结构需与 init-args 期望一致，用于生成用户侧 args，通常包含：

- `version`
- `global`：全局变量默认值
- `init.components`：各 init 项对应的参数默认值（key 为 init 项 name）
- `projects`：项目列表，每项含 `name`、`global`、`components` 等

### 3.5 示例（Minimal Terraform）

**terraform/config.yaml**（1 个 init 项 + 1 个 project）：

```yaml
version: 1.0.0

init:
  - name: backend
    path:
      - terraform/init/main.tf.tmpl
      - terraform/init/variable.tf.tmpl
    destination: terraform/init/0-terraform-statestore/
    args: terraform/init/args.yaml
    install: terraform apply -var-file=terraform/init/0-terraform-statestore/*.tf
    upgrade: terraform apply -var-file=terraform/init/0-terraform-statestore/*.tf

project-global: terraform/project/args.yaml

projects:
  - name: main
    path:
      - terraform/project/main.tf.tmpl
    args: terraform/project/args.yaml
    install: terraform apply -var-file=terraform/project/main.tf.tmpl
    upgrade: terraform apply -var-file=terraform/project/main.tf.tmpl
```

**terraform/default.yaml** 片段：

```yaml
version: 1.0.0
global:
  GlobalName: my-org
  TerraformBackendBucket: my-org-statestore
  # Use "0" to disable org_id in init templates (variable/resource lines omitted)
  OrganizationID: "123456789012"
  BillingAccountID: 01ABCD-2EFGH3-4IJKL5
init:
  components:
    backend: {}
projects:
  - name: prd
    global:
      project_name: prd
    components: []
```

参考实现：示例模板仓库 [github.com/NFTGalaxy/bl-template](https://github.com/NFTGalaxy/bl-template) 下的 `terraform/`。

---

## 4. Kubernetes 模块

### 4.1 config 路径与加载

- **路径**：`kubernetes/config.yaml`（Loader 固定读取，见 `config.go` 中 `LoadKubernetesConfig`）。

### 4.2 config 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `version` | string | 可选 |
| `init` | object | 可选，含 `namespace`、`components`（map） |
| `optional` | map | 可选组件（key 为组件名） |
| `components` | list | 兼容旧版：组件列表（与 optional 二选一或共存，由实现合并） |

**组件项**（每个 component）：

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 组件名 |
| `path` | string 或 list | 模板路径 |
| `args` | string 或 list | 可选 |
| `install` / `upgrade` / `rollback` | string | 可选 |
| `check` | string | 可选，custom 类型时的检查命令 |
| `installType` | string | `kubectl`（默认）、`helm`、`custom` |
| `chart` | string | helm 时可选，如 `bitnami/redis` |
| `namespace` | string | helm 安装的 namespace，默认可用组件名 |
| `dependencies` | list | 依赖的其他组件 name |

### 4.3 default.yaml

通常包含：

- `version`
- `global`：全局默认值
- `projects`：项目列表，每项 `name`、`components`（组件列表，每组件可含 `parameters`）

### 4.4 示例（Minimal Kubernetes）

**kubernetes/config.yaml**（2 个组件：kubectl + custom）：

```yaml
components:
  - name: sealed-secret
    path:
      - kubernetes/components/sealed-secret/kustomization.yaml.tmpl
    installType: kubectl
  - name: my-component
    path:
      - kubernetes/components/my-component/namespace.yaml.tmpl
    install: bash install
    installType: custom
```

**kubernetes/default.yaml** 片段：

```yaml
version: 1.0.0
global: {}
projects:
  - name: stg
    components:
      - name: sealed-secret
        parameters:
          namespace: sealed-secret
      - name: my-component
        parameters:
          namespace: my-ns
```

参考实现：示例模板仓库 [github.com/NFTGalaxy/bl-template](https://github.com/NFTGalaxy/bl-template) 下的 `kubernetes/`。

---

## 5. GitOps 模块

### 5.1 config 路径与加载

- **路径**：`gitops/config.yaml`（Loader 固定读取，见 `config.go` 中 `LoadGitopsConfig`）。

### 5.2 config 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `version` | string | 建议 `1.0.0` |
| `app-templates` | object | **固定两个 key**：`deployment`、`statefulset` |
| `argocd` | object | ArgoCD Application 模板配置 |

**app-templates.deployment / app-templates.statefulset**：均为列表，每项：

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 模板名 |
| `path` | string | 模板文件路径 |
| `args` | string 或 list | 可选 |
| `description` | string | 可选 |

**argocd**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `path` | string | ArgoCD Application 模板路径（如 `gitops/app.yaml.tmpl`） |
| `args` | string 或 list | 可选 |

### 5.3 default.yaml

通常包含：

- `argocd.project`：ArgoCD 项目名列表。
- `apps`：应用列表，每项 `name`、`kind`（deployment/statefulset）、`repo`、`project`（列表，每项可含 per-project `parameters`）。

### 5.4 示例（Minimal GitOps）

**gitops/config.yaml**（deployment 下 2 个模板 + argocd）：

```yaml
version: 1.0.0

app-templates:
  deployment:
    - name: deployment
      path: gitops/base-deployment.yaml.tmpl
      args: gitops/args.yaml
      description: "Base deployment template"
    - name: app
      path: gitops/app.yaml.tmpl
      args: gitops/args.yaml
      description: "ArgoCD Application template"
  statefulset: []

argocd:
  path: gitops/app.yaml.tmpl
  args: gitops/args.yaml
```

**gitops/default.yaml** 片段：

```yaml
argocd:
  project:
    - stg
apps:
  - name: hello-world
    kind: deployment
    repo: https://github.com/your-org/hello-world.git
    project:
      - name: stg
        parameters:
          SourceRepoURL: https://github.com/your-org/gitops.git
          SourcePath: stg/hello-world
```

参考实现：示例模板仓库 [github.com/NFTGalaxy/bl-template](https://github.com/NFTGalaxy/bl-template) 下的 `gitops/`。

---

## 6. args.yaml 格式（参数定义）

所有 **args.yaml**（根级或各模块/子目录下）采用统一结构，与 `pkg/renderer/argsdef.go` 中 **ArgsDefinition** 一致。**参数类型与所有可用字段的完整说明**见 [args.yaml 参数类型说明](ARGS_YAML_TYPES.md)。

### 6.1 顶层结构

```yaml
version: 1.0.0

parameters:
  global:       # 可选，全局参数定义
    ParamName:
      type: string
      description: "..."
      required: true
      example: "value"
      default: ...
  components:   # 可选，按组件或类型组织的嵌套参数
    component-name:
      ParamName:
        type: string
        description: "..."
        required: false
        example: "..."
```

- **parameters.global**：key 为参数名，value 为参数定义（type、description、required、default、example、pattern、**validation** 等，详见 [ARGS_YAML_TYPES.md](ARGS_YAML_TYPES.md)）。
- **parameters.components**：嵌套 map，用于按组件或部署类型（如 deployment/statefulset）组织参数。
- **validation**（顶层，可选）：全局校验规则，如 `validation.unique` 用于约束某路径下取值唯一（见下文 6.3）。

约定：参数定义用于 `blcli init-args` 生成默认值、`blcli explain` 展示、**`blcli init` 前校验**（按 validation 规则在合并后、写文件前执行；失败则返回 `validation failed: ...` 且不写文件）、以及渲染时与用户 args 合并。`type`、`example`、`default` 为可选但建议提供。

### 6.2 示例

**根级 args.yaml**（仅 parameters.global）：

```yaml
version: 1.0.0

parameters:
  global:
    GlobalName:
      type: string
      description: "Global name used for resource naming"
      required: true
      example: "my-org"
```

**terraform/init/args.yaml** 片段：

```yaml
version: 1.0.0

parameters:
  global:
    GlobalName:
      type: string
      description: "Global name for resource naming"
      required: true
      example: "my-org"
    TerraformBackendBucket:
      type: string
      description: "GCS bucket for Terraform state"
      required: false
      example: "my-org-statestore"
```

**gitops/args.yaml** 片段（components 按类型）：

```yaml
version: 1.0.0

parameters:
  components:
    deployment:
      ContainerPorts:
        type: list[object]
        description: "Container ports configuration"
        required: false
        default:
          - containerPort: 8080
            name: http
```

### 6.3 校验规则（validation）

参数可定义 **validation** 列表，blcli 在 init 合并 args 后、写文件前执行校验；失败则返回 `validation failed: ...` 且不写文件。

- **参数级 validation**：每条规则为 map，必含 `kind`（`required`、`stringLength`、`pattern`、`format`、`enum`、`numberRange`）及对应 params。完整说明见 [ARGS_YAML_TYPES.md 第 6 节](ARGS_YAML_TYPES.md#6-校验字段validation)。
- **顶层 validation.unique**：约束某路径取值唯一，例如：

```yaml
validation:
  unique:
    - path: "terraform.projects[].name"
      message: "Project names must be unique"
```

---

## 7. default.yaml 与 init-args 的关系

- **`blcli init-args`** 会从模板仓库读取：
  - 根级 `args.yaml`（若存在）：提取 `parameters.global` 等。
  - 各模块的 **default.yaml**：`terraform/default.yaml`、`kubernetes/default.yaml`、`gitops/default.yaml`（若存在）。
- 将 default 中的值与 args.yaml 中的参数定义结合，生成用户侧的 args 文件（YAML/TOML）。
- **至少需存在一个模块的 default.yaml**；若三个模块下都未找到 default.yaml，init-args 会报错：`no default.yaml files found in template repository (checked: terraform, kubernetes, gitops)`（见 `pkg/cli/init_args.go`）。
- 未提供 default.yaml 的模块，生成结果中该模块可能为空或仅含骨架，取决于当前实现。

---

## 8. 模板文件约定

- **扩展名**：`.tmpl`。
- **语法**：Go 标准库 **`text/template`**。
- **常用写法**：
  - 变量：`{{ .VariableName }}`、`{{ .GlobalName }}`
  - 条件：`{{ if .Condition }}...{{ end }}`
  - 循环：`{{ range .Items }}...{{ end }}`
  - 管道：`{{ .Value | tojson }}`
- **路径**：config 中 `path` 均为**相对仓库根**的路径，且应指向仓库内实际存在的 `.tmpl` 文件或目录；为目录时，blcli 会遍历该目录下模板进行渲染。

---

## 9. 示例汇总（Example 区）

以下为可直接复用的最小示例，与示例模板仓库 [github.com/NFTGalaxy/bl-template](https://github.com/NFTGalaxy/bl-template) 结构兼容。

### 9.1 Minimal Terraform

- **config**：1 个 init 项（backend）+ 1 个 project（main）；见 [3.5 示例](#35-示例minimal-terraform)。
- **default**：含 `global`、`init.components`、`projects` 片段；见 [3.5](#35-示例minimal-terraform)。
- 参考实现：github.com/NFTGalaxy/bl-template 下的 `terraform/`。

### 9.2 Minimal Kubernetes

- **config**：1 个 kubectl 组件 + 1 个 custom 组件；见 [4.4 示例](#44-示例minimal-kubernetes)。
- **default**：含 `projects[].components` 及各组件 `parameters`；见 [4.4](#44-示例minimal-kubernetes)。
- 参考实现：github.com/NFTGalaxy/bl-template 下的 `kubernetes/`。

### 9.3 Minimal GitOps

- **config**：`app-templates.deployment` 下 2 个模板 + `argocd` 段；见 [5.4 示例](#54-示例minimal-gitops)。
- **default**：含 `argocd.project`、`apps` 及 per-project `parameters`；见 [5.4](#54-示例minimal-gitops)。
- 参考实现：github.com/NFTGalaxy/bl-template 下的 `gitops/`。

---

**参考实现**：本协议与示例模板仓库 **github.com/NFTGalaxy/bl-template** 结构对齐，实现或扩展模板仓库时可直接参考该仓库作为完整示例。
