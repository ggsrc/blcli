# Kubernetes 部分实现指南

本文档基于 Terraform 部分的实现经验，为 Kubernetes 部分的实现提供指导。

## 一、架构概览

### 1.1 整体流程

Terraform 部分的实现遵循以下流程：

```
blcli init-args
  ↓
  加载模板仓库的 default.yaml 文件
  ↓
  转换为 ArgsConfig 结构
  ↓
  生成 args.yaml 文件

blcli init
  ↓
  加载 args.yaml 文件
  ↓
  解析配置 (config.LoadFromArgs)
  ↓
  加载模板仓库的 config.yaml
  ↓
  初始化模块 (BootstrapTerraform)
    ├─ InitializeInitItems (处理 init 项)
    ├─ InitializeModules (处理模块)
    └─ InitializeProjects (处理项目)

blcli apply
  ↓
  执行 terraform apply 命令
  ↓
  按顺序应用 init 目录和项目目录
```

### 1.2 关键组件

1. **CLI 层** (`pkg/cli/`)
   - `init_args.go`: 处理 `blcli init-args` 命令
   - `init.go`: 处理 `blcli init` 命令
   - `apply_terraform.go`: 处理 `blcli apply terraform` 命令

2. **Bootstrap 层** (`pkg/bootstrap/`)
   - `terraform.go`: Terraform 模块的入口函数
   - `terraform/`: Terraform 专用的子包
     - `init.go`: 处理 init 项
     - `modules.go`: 处理模块
     - `projects.go`: 处理项目
     - `args.go`: 参数提取工具函数
     - `common.go`: 通用工具函数

3. **Template 层** (`pkg/template/`)
   - `loader.go`: 模板加载器
   - `config.go`: 配置结构定义
   - `engine.go`: 模板渲染引擎

4. **Renderer 层** (`pkg/renderer/`)
   - `args.go`: 参数加载和合并
   - `argsconfig.go`: ArgsConfig 结构定义
   - `argsdef.go`: ArgsDefinition 结构定义

5. **Config 层** (`pkg/config/`)
   - `config.go`: 配置加载和解析

## 二、init-args 命令实现

### 2.1 核心逻辑

`blcli init-args` 命令的核心逻辑在 `pkg/cli/init_args.go` 的 `generateArgsFile` 函数中：

1. **加载 default.yaml 文件**
   ```go
   defaultPaths := []string{
       "terraform/default.yaml",
       "kubernetes/default.yaml",
       "gitops/default.yaml",
   }
   ```

2. **转换为 ArgsConfig 结构**
   - 使用 `convertToArgsConfig` 函数将 default.yaml 数据转换为 ArgsConfig
   - 目前只实现了 terraform 部分，kubernetes 和 gitops 部分标记为 TODO

3. **写入文件**
   - 支持 YAML 和 TOML 格式
   - 默认输出到 `workspace/config/args.yaml`

### 2.2 Kubernetes 部分需要实现

在 `convertToArgsConfig` 函数中，需要处理 kubernetes 数据：

```go
// Extract kubernetes data if exists
if kubernetesData, ok := defaultData["kubernetes"].(map[string]interface{}); ok {
    // TODO: Handle kubernetes section when needed
    _ = kubernetesData
}
```

需要实现：
1. 解析 `kubernetes/default.yaml` 的结构
2. 转换为 Kubernetes 配置结构（类似 TerraformSection）
3. 支持 `kubernetes.init` 和 `kubernetes.optional` 组件

### 2.3 Kubernetes default.yaml 结构

根据 `bl-template/kubernetes/default.yaml`，结构如下：

```yaml
version: 1.0.0
global: {}
projects:
  - name: prd
    components:
      - name: external-secrets
        parameters: {...}
      - name: sealed-secret
        parameters: {...}
      - name: istio
        parameters: {...}
      - name: argocd
        parameters: {...}
```

注意：Kubernetes 的 default.yaml 结构与 Terraform 类似，但组件分为 `init` 和 `optional` 两类。

## 三、init 命令实现

### 3.1 核心流程

`blcli init` 命令的核心流程在 `pkg/bootstrap/manager.go` 的 `ExecuteInit` 函数中：

1. **加载 args 文件**
   ```go
   templateArgs, err := renderer.LoadArgs(argsPath)
   ```

2. **解析配置**
   ```go
   cfg, err := config.LoadFromArgs(templateArgs)
   ```

3. **初始化模块**
   ```go
   initializeModule("kubernetes", cfg, templateLoader, templateArgs, overwrite, &st, profiler)
   ```

### 3.2 BootstrapKubernetes 函数

当前实现（`pkg/bootstrap/kubernetes.go`）：

```go
func BootstrapKubernetes(global config.GlobalConfig, project *config.ProjectConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData) error {
    // Load kubernetes config from template repository
    cfg, err := templateLoader.LoadKubernetesConfig()
    if err != nil {
        return fmt.Errorf("failed to load kubernetes config: %w", err)
    }
    
    _ = cfg // TODO: Implement kubernetes bootstrap logic
    return fmt.Errorf("kubernetes bootstrap not yet implemented")
}
```

### 3.3 需要实现的功能

参考 `BootstrapTerraform` 的实现，需要：

1. **创建目录结构**
   ```go
   kubernetesDir := filepath.Join(workspace, "kubernetes")
   ```

2. **处理 init 组件**
   - 根据 `kubernetes/config.yaml` 中的 `init` 配置
   - 渲染模板文件到 `kubernetes/base/` 目录

3. **处理 optional 组件**
   - 根据 `kubernetes/config.yaml` 中的 `optional` 配置
   - 根据 args.yaml 中配置的组件，选择性渲染

4. **处理依赖关系**
   - 根据 `dependencies` 字段解析依赖
   - 按依赖顺序渲染组件

### 3.4 Kubernetes Config 结构

根据 `bl-template/kubernetes/config.yaml`，结构如下：

```yaml
version: 1.0.0
init:
  namespace:
    name: namespace
    path: kubernetes/base/namespace.yaml.tmpl
    install: kubectl apply -f namespace.yaml
  components:
    istio:
      name: istio
      version: 1.20.0
      path:
        - kubernetes/components/istio/namespace.yaml.tmpl
        - kubernetes/components/istio/operator.yaml.tmpl
      install: bash install
      installType: custom  # kubectl (default), helm, custom
      dependencies:
        - namespace
optional:
  argocd:
    name: argocd
    version: v2.11.7
    path:
      - kubernetes/components/argocd/namespace.yaml.tmpl
    install: bash install
    installType: custom
    dependencies:
      - namespace
      - sealed-secret
  cnpg:
    name: cnpg
    path:
      - kubernetes/optional/cnpg.yaml.tmpl
    install: helm install cnpg cnpg/cnpg --namespace cnpg --create-namespace
    installType: helm
```

**installType 字段说明**：
- **kubectl**（默认）：使用 `kubectl apply -k <component-dir>` 应用组件目录
- **helm**：使用 `helm install <name> <chart> --namespace <namespace> --create-namespace` 安装
- **custom**：使用 config.yaml 中该组件配置好的 `install` 命令执行

需要在 `pkg/template/config.go` 中定义 `KubernetesConfig` 结构（目前已有基础结构，需要完善，并添加 `installType` 字段支持）。

## 四、apply 命令实现

### 4.1 核心流程

`blcli apply terraform` 命令的核心流程在 `pkg/bootstrap/apply_terraform.go` 的 `ExecuteApplyTerraform` 函数中：

1. **验证目录存在**
   ```go
   if _, err := os.Stat(opts.TerraformDir); os.IsNotExist(err) {
       return fmt.Errorf("terraform directory not found: %s", opts.TerraformDir)
   }
   ```

2. **按顺序应用**
   - Step 1: 应用 init 目录（按数字顺序）
   - Step 2: 应用项目目录

3. **执行 terraform 命令**
   - `terraform init`
   - `terraform validate`
   - `terraform plan`
   - `terraform apply`

### 4.2 Kubernetes Apply 需要实现

参考 `ExecuteApplyTerraform`，需要实现 `ExecuteApplyKubernetes`：

1. **验证目录存在**
   ```go
   if _, err := os.Stat(opts.KubernetesDir); os.IsNotExist(err) {
       return fmt.Errorf("kubernetes directory not found: %s", opts.KubernetesDir)
   }
   ```

2. **按顺序应用**
   - Step 1: 应用 init 组件（按依赖顺序）
   - Step 2: 应用 optional 组件（按依赖顺序）

3. **执行安装命令**（根据 `installType` 字段）
   - **kubectl**（默认）：执行 `kubectl apply -k <component-dir>`
   - **helm**：执行 `helm install <name> <chart> --namespace <namespace> --create-namespace`
   - **custom**：执行 config.yaml 中配置的 `install` 命令

### 4.3 依赖解析

Kubernetes 组件有依赖关系，需要实现依赖解析：

```go
// 解析依赖关系
orderedComponents, err := resolveDependencies(kubernetesConfig)
if err != nil {
    return fmt.Errorf("failed to resolve dependencies: %w", err)
}

// 按顺序应用
for _, component := range orderedComponents {
    if err := applyComponent(component); err != nil {
        return err
    }
}
```

## 五、关键设计模式

### 5.1 参数提取模式

Terraform 使用了多层次的参数提取：

1. **全局参数**: `templateArgs["global"]`
2. **模块级参数**: `templateArgs["terraform"]["global"]`
3. **项目级参数**: `templateArgs["terraform"]["projects"][projectName]["global"]`
4. **组件级参数**: `templateArgs["terraform"]["projects"][projectName]["components"][componentName]["parameters"]`

Kubernetes 应该采用类似的结构：
- `templateArgs["kubernetes"]["global"]`
- `templateArgs["kubernetes"]["projects"][projectName]["components"][componentName]["parameters"]`

### 5.2 模板渲染模式

Terraform 使用 `template.RenderWithArgs` 进行模板渲染：

```go
content, err := template.RenderWithArgs(tmplContent, data, args)
```

Kubernetes 应该使用相同的渲染引擎。

### 5.3 文件写入模式

Terraform 使用 `internal.WriteFileIfAbsent` 写入文件：

```go
if err := internal.WriteFileIfAbsent(outputPath, content); err != nil {
    return fmt.Errorf("failed to write file: %w", err)
}
```

Kubernetes 应该使用相同的工具函数。

### 5.4 目录结构模式

Terraform 的目录结构：
```
workspace/
  terraform/
    init/
      0-terraform-statestore/
      1-{GlobalName}-projects/
    gcp/
      modules/
        gke/
        ...
      prd/
      stg/
      ...
```

Kubernetes 的目录结构应该是：
```
workspace/
  kubernetes/
    base/
      namespace.yaml
      ...
    components/
      istio/
        namespace.yaml
        operator.yaml
        ...
      sealed-secret/
        ...
```

## 六、实现步骤建议

### 6.1 第一步：完善 KubernetesConfig 结构

在 `pkg/template/config.go` 中完善 `KubernetesConfig` 结构，支持：
- `init` 部分（包含 `namespace` 和 `components`）
- `optional` 部分

### 6.2 第二步：实现 init-args 的 Kubernetes 部分

在 `pkg/cli/init_args.go` 的 `convertToArgsConfig` 函数中实现 Kubernetes 数据转换。

### 6.3 第三步：创建 Kubernetes 子包

创建 `pkg/bootstrap/kubernetes/` 子包，包含：
- `init.go`: 处理 init 组件
- `optional.go`: 处理 optional 组件
- `args.go`: 参数提取工具函数
- `common.go`: 通用工具函数

### 6.4 第四步：实现 BootstrapKubernetes

在 `pkg/bootstrap/kubernetes.go` 中实现完整的初始化逻辑。

### 6.5 第五步：实现 ApplyKubernetes

在 `pkg/bootstrap/apply_kubernetes.go` 中实现应用逻辑。

### 6.6 第六步：实现依赖解析

实现依赖解析算法，确保组件按正确顺序应用。

## 七、注意事项

### 7.1 与 Terraform 的差异

1. **组件分类**: Kubernetes 有 `init` 和 `optional` 两类组件，而 Terraform 只有 `init` 和 `projects`
2. **应用方式**: Kubernetes 使用 `kubectl`、`kustomize`、`helm` 等工具，而 Terraform 使用 `terraform` 命令
3. **依赖关系**: Kubernetes 的依赖关系更复杂，需要处理跨组件的依赖
4. **安装类型**: Kubernetes 支持 `installType` 字段（kubectl/helm/custom），而 Terraform 统一使用 `terraform apply`
5. **特殊项**: Kubernetes 有特殊的 `namespace` 项（在 init 下），而 Terraform 没有

### 7.2 错误处理

参考 Terraform 的错误处理模式：
- 提供详细的错误信息
- 包含可能的解决方案提示
- 支持部分失败继续执行（可选）

### 7.3 性能优化

参考 Terraform 的性能优化：
- 使用 profiler 进行性能分析
- 支持并发处理（如果适用）
- 缓存模板文件

### 7.4 测试

参考 Terraform 的测试模式：
- 单元测试覆盖关键函数
- 集成测试覆盖完整流程
- 测试文件放在 `workspace/` 目录下

## 八、参考文件

### 8.1 Terraform 实现文件

- `pkg/cli/init_args.go`: init-args 命令实现
- `pkg/cli/init.go`: init 命令实现
- `pkg/cli/apply_terraform.go`: apply terraform 命令实现
- `pkg/bootstrap/terraform.go`: Terraform 模块入口
- `pkg/bootstrap/terraform/init.go`: Init 项处理
- `pkg/bootstrap/terraform/modules.go`: 模块处理
- `pkg/bootstrap/terraform/projects.go`: 项目处理
- `pkg/bootstrap/terraform/args.go`: 参数提取
- `pkg/bootstrap/apply_terraform.go`: Apply 实现

### 8.2 模板文件

- `bl-template/terraform/config.yaml`: Terraform 配置
- `bl-template/terraform/default.yaml`: Terraform 默认值
- `bl-template/kubernetes/config.yaml`: Kubernetes 配置
- `bl-template/kubernetes/default.yaml`: Kubernetes 默认值

### 8.3 配置结构

- `pkg/template/config.go`: 配置结构定义
- `pkg/renderer/argsconfig.go`: ArgsConfig 结构
- `pkg/config/config.go`: 配置加载

## 九、总结

实现 Kubernetes 部分时，应该：

1. **遵循 Terraform 的设计模式**: 保持代码风格和架构一致性
2. **复用现有工具函数**: 使用 `internal`、`renderer`、`template` 包中的工具函数
3. **处理 Kubernetes 的特殊性**: 考虑 init/optional 分类、依赖关系等
4. **提供详细的错误信息**: 帮助用户快速定位问题
5. **编写完整的测试**: 确保功能正确性

通过参考 Terraform 的实现，可以快速实现 Kubernetes 部分，同时保持代码质量和一致性。
