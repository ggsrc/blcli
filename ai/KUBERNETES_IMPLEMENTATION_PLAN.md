# Kubernetes 部分实现计划

基于对 bl-template/kubernetes 部分的分析，本文档详细说明 Kubernetes 部分的实现方案。

## 一、结构分析

### 1.1 config.yaml 结构

```yaml
version: 1.0.0
init:
  namespace:          # 特殊项：基础命名空间
    name: namespace
    path: kubernetes/base/namespace.yaml.tmpl
    install: kubectl apply -f namespace.yaml
  components:         # 初始化组件列表
    istio:
      name: istio
      version: 1.20.0
      path: [...]
      install: bash install
      dependencies: [namespace]
    sealed-secret:
      name: sealed-secret
      version: v0.32.2
      path: [...]
      install: kustomize build . | kubectl apply -f -
      dependencies: [namespace]
optional:             # 可选组件列表
  argocd:
    name: argocd
    version: v2.11.7
    path: [...]
    install: bash install
    dependencies: [namespace, sealed-secret]
  cnpg:
    name: cnpg
    path: [...]
    install: helm install cnpg cnpg/cnpg --namespace cnpg --create-namespace
```

### 1.2 default.yaml 结构

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
```

### 1.3 与 Terraform 的相似性

1. **结构相似**：
   - 都有 `version` 字段
   - 都有 `init` 部分（Terraform 是 init items，Kubernetes 是 init components）
   - 都有项目列表（Terraform 是 projects，Kubernetes 是 projects）
   - 都支持依赖关系

2. **差异**：
   - Kubernetes 有 `optional` 组件（Terraform 没有）
   - Kubernetes 有特殊的 `namespace` 项（Terraform 没有）
   - Kubernetes 的组件分为 `init.components` 和 `optional` 两类
   - Kubernetes 需要支持 `installType`（kubectl/helm/custom）

## 二、需要实现的功能

### 2.1 installType 支持

每个组件需要支持 `installType` 字段，有三个值：

1. **kubectl**（默认）：
   - 使用 `kubectl apply -k <component-dir>` 应用组件
   - 适用于使用 kustomize 的组件

2. **helm**：
   - 使用 `helm install <name> <chart> --namespace <namespace> --create-namespace` 安装
   - 适用于 Helm chart 组件

3. **custom**：
   - 使用 config.yaml 中该组件配置好的 `install` 命令
   - 适用于需要自定义安装脚本的组件（如 istio、argocd）

### 2.2 目录结构

生成的目录结构应该是：

```
workspace/
  kubernetes/
    base/
      namespace.yaml          # init.namespace 生成的文件
    components/
      istio/                  # init.components.istio 生成的文件
        namespace.yaml
        operator.yaml
        kustomization.yaml
        install
      sealed-secret/          # init.components.sealed-secret 生成的文件
        kustomization.yaml
      external-secrets/       # init.components.external-secrets 生成的文件
        namespace.yaml
        cluster-secret-store.yaml
        kustomization.yaml
      argocd/                 # optional.argocd 生成的文件（如果配置了）
        namespace.yaml
        kustomization.yaml
        argocd-cm.yaml
        install
        gen_ssh_sealed_secrets.sh
```

## 三、实现步骤

### 步骤 1: 更新 KubernetesConfig 结构

在 `pkg/template/config.go` 中更新 `KubernetesConfig` 结构：

```go
// InstallType represents the installation type for a component
type InstallType string

const (
	InstallTypeKubectl InstallType = "kubectl" // Default
	InstallTypeHelm    InstallType = "helm"
	InstallTypeCustom  InstallType = "custom"
)

// KubernetesConfig represents the kubernetes/config.yaml structure
type KubernetesConfig struct {
	Version string              `yaml:"version"`
	Init    KubernetesInit      `yaml:"init"`
	Optional map[string]KubernetesComponent `yaml:"optional"`
}

// KubernetesInit represents the init section
type KubernetesInit struct {
	Namespace  KubernetesNamespaceItem `yaml:"namespace"`
	Components map[string]KubernetesComponent `yaml:"components"`
}

// KubernetesNamespaceItem represents the namespace init item
type KubernetesNamespaceItem struct {
	Name    string   `yaml:"name"`
	Path    []string `yaml:"path"`
	Install string   `yaml:"install,omitempty"`
}

// KubernetesComponent represents a kubernetes component
type KubernetesComponent struct {
	Name         string       `yaml:"name"`
	Version      string       `yaml:"version,omitempty"`
	Path         []string     `yaml:"path"`
	Args         []string     `yaml:"args,omitempty"`
	Install      string       `yaml:"install,omitempty"`
	Upgrade      string       `yaml:"upgrade,omitempty"`
	InstallType  InstallType  `yaml:"installType,omitempty"` // kubectl, helm, custom (default: kubectl)
	Dependencies []string     `yaml:"dependencies,omitempty"`
}
```

### 步骤 2: 实现 init-args 的 Kubernetes 部分

在 `pkg/cli/init_args.go` 的 `convertToArgsConfig` 函数中实现：

```go
// Extract kubernetes data if exists
if kubernetesData, ok := defaultData["kubernetes"].(map[string]interface{}); ok {
    // Build kubernetes section
    if version, ok := kubernetesData["version"].(string); ok {
        result.Kubernetes.Version = version
    }
    // kubernetes.global
    if global, ok := kubernetesData["global"].(map[string]interface{}); ok {
        result.Kubernetes.Global = global
    }
    // kubernetes.projects
    if projects, ok := kubernetesData["projects"].([]interface{}); ok {
        result.Kubernetes.Projects = convertProjectsList(projects)
    }
}
```

同时需要在 `ArgsConfig` 中添加 `Kubernetes` 字段（在 `pkg/renderer/argsconfig.go` 中）。

### 步骤 3: 创建 Kubernetes 子包

创建 `pkg/bootstrap/kubernetes/` 目录，包含以下文件：

#### 3.1 `init.go` - 处理 init 组件

```go
package kubernetes

import (
    "fmt"
    "path/filepath"
    "blcli/pkg/internal"
    "blcli/pkg/renderer"
    "blcli/pkg/template"
)

// InitializeInitItems initializes all kubernetes init items
func InitializeInitItems(
    kubernetesConfig *template.KubernetesConfig,
    templateLoader *template.Loader,
    templateArgs renderer.ArgsData,
    workspaceRoot string,
    data map[string]interface{},
) error {
    // 1. 处理 namespace
    if err := initializeNamespace(kubernetesConfig.Init.Namespace, templateLoader, templateArgs, workspaceRoot, data); err != nil {
        return err
    }

    // 2. 处理 init components
    availableInitComponents := getAvailableInitComponents(templateArgs)
    
    for compName, component := range kubernetesConfig.Init.Components {
        if !availableInitComponents[compName] {
            continue
        }
        if err := initializeComponent(component, compName, templateLoader, templateArgs, workspaceRoot, data, "init"); err != nil {
            return err
        }
    }

    return nil
}

// initializeNamespace initializes the namespace init item
func initializeNamespace(namespaceItem template.KubernetesNamespaceItem, ...) error {
    baseDir := filepath.Join(workspaceRoot, "kubernetes", "base")
    // 渲染 namespace 模板文件
    // ...
}

// initializeComponent initializes a single component
func initializeComponent(component template.KubernetesComponent, compName string, ...) error {
    componentDir := filepath.Join(workspaceRoot, "kubernetes", "components", compName)
    // 渲染组件模板文件
    // ...
}
```

#### 3.2 `optional.go` - 处理 optional 组件

```go
package kubernetes

// InitializeOptionalComponents initializes all optional components
func InitializeOptionalComponents(
    kubernetesConfig *template.KubernetesConfig,
    templateLoader *template.Loader,
    templateArgs renderer.ArgsData,
    workspaceRoot string,
    data map[string]interface{},
) error {
    // 获取可用的 optional 组件（从 args.yaml 中）
    availableOptionalComponents := getAvailableOptionalComponents(templateArgs)
    
    for compName, component := range kubernetesConfig.Optional {
        if !availableOptionalComponents[compName] {
            continue
        }
        if err := initializeComponent(component, compName, templateLoader, templateArgs, workspaceRoot, data, "optional"); err != nil {
            return err
        }
    }
    
    return nil
}
```

#### 3.3 `args.go` - 参数提取工具函数

```go
package kubernetes

// ExtractComponentArgs extracts component-specific args
func ExtractComponentArgs(templateArgs renderer.ArgsData, componentName string) renderer.ArgsData {
    // 类似 terraform 的参数提取逻辑
    // ...
}

// GetAvailableInitComponents gets available init components from args
func GetAvailableInitComponents(templateArgs renderer.ArgsData) map[string]bool {
    // 从 kubernetes.init.components 中提取
    // ...
}

// GetAvailableOptionalComponents gets available optional components from args
func GetAvailableOptionalComponents(templateArgs renderer.ArgsData) map[string]bool {
    // 从 kubernetes.projects[].components 中提取 optional 组件
    // ...
}
```

#### 3.4 `common.go` - 通用工具函数

```go
package kubernetes

// PrepareKubernetesData prepares data for template rendering
func PrepareKubernetesData(global config.GlobalConfig, projectName string) map[string]interface{} {
    // 类似 terraform 的数据准备逻辑
    // ...
}
```

### 步骤 4: 实现 BootstrapKubernetes

在 `pkg/bootstrap/kubernetes.go` 中实现：

```go
func BootstrapKubernetes(
    global config.GlobalConfig,
    project *config.ProjectConfig,
    templateLoader *template.Loader,
    templateArgs renderer.ArgsData,
    overwrite bool,
) error {
    workspace := config.WorkspacePath(global)
    kubernetesDir := filepath.Join(workspace, "kubernetes")

    // 检查目录和 marker
    if exists, err := internal.CheckBlcliMarker(kubernetesDir); err == nil && exists {
        if !overwrite {
            return fmt.Errorf("kubernetes directory at %s was created by blcli. Use --overwrite to allow overwriting", kubernetesDir)
        }
    }

    // 加载 kubernetes config
    kubernetesConfig, err := templateLoader.LoadKubernetesConfig()
    if err != nil {
        return fmt.Errorf("failed to load kubernetes config: %w", err)
    }

    // 创建目录
    if err := internal.EnsureDir(kubernetesDir); err != nil {
        return fmt.Errorf("failed to create kubernetes dir: %w", err)
    }
    if err := internal.CreateBlcliMarker(kubernetesDir); err != nil {
        return fmt.Errorf("failed to create blcli marker: %w", err)
    }

    // 准备数据
    data := k8s.PrepareKubernetesData(global, "")

    // 1. 初始化 init 组件
    if err := k8s.InitializeInitItems(kubernetesConfig, templateLoader, templateArgs, workspace, data); err != nil {
        return err
    }

    // 2. 初始化 optional 组件（根据项目配置）
    // 需要从 templateArgs 中获取项目列表
    projects := getKubernetesProjects(templateArgs)
    for _, projectName := range projects {
        projectData := k8s.PrepareKubernetesData(global, projectName)
        if err := k8s.InitializeOptionalComponentsForProject(kubernetesConfig, templateLoader, templateArgs, workspace, projectData, projectName); err != nil {
            return err
        }
    }

    return nil
}
```

### 步骤 5: 实现 ExecuteApplyKubernetes

在 `pkg/bootstrap/apply_kubernetes.go` 中实现，支持 installType：

```go
func ExecuteApplyKubernetes(opts ApplyKubernetesOptions) error {
    // 验证目录存在
    if _, err := os.Stat(opts.KubernetesDir); os.IsNotExist(err) {
        return fmt.Errorf("kubernetes directory not found: %s", opts.KubernetesDir)
    }

    ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
    defer cancel()

    // 加载 kubernetes config（从模板仓库）
    // 需要知道模板仓库路径，可能需要从 state 或 args 中获取
    kubernetesConfig, err := loadKubernetesConfig(opts)
    if err != nil {
        return err
    }

    // 1. 应用 init 组件（按依赖顺序）
    if err := applyInitComponents(ctx, opts, kubernetesConfig); err != nil {
        return err
    }

    // 2. 应用 optional 组件（按依赖顺序）
    if err := applyOptionalComponents(ctx, opts, kubernetesConfig); err != nil {
        return err
    }

    return nil
}

// applyComponent applies a component based on its installType
func applyComponent(ctx context.Context, opts ApplyKubernetesOptions, component template.KubernetesComponent, componentDir string) error {
    installType := component.InstallType
    if installType == "" {
        installType = template.InstallTypeKubectl // 默认值
    }

    switch installType {
    case template.InstallTypeKubectl:
        // kubectl apply -k <component-dir>
        return applyWithKubectl(ctx, opts, componentDir)
    case template.InstallTypeHelm:
        // helm install <name> <chart> --namespace <namespace> --create-namespace
        return applyWithHelm(ctx, opts, component, componentDir)
    case template.InstallTypeCustom:
        // 使用 config.yaml 中的 install 命令
        return applyWithCustom(ctx, opts, component, componentDir)
    default:
        return fmt.Errorf("unknown installType: %s", installType)
    }
}

// applyWithKubectl applies using kubectl apply -k
func applyWithKubectl(ctx context.Context, opts ApplyKubernetesOptions, componentDir string) error {
    cmd := exec.CommandContext(ctx, "kubectl", "apply", "-k", componentDir)
    // 添加 kubeconfig, context, namespace 等选项
    // ...
    return cmd.Run()
}

// applyWithHelm applies using helm install
func applyWithHelm(ctx context.Context, opts ApplyKubernetesOptions, component template.KubernetesComponent, componentDir string) error {
    // 解析 helm install 命令
    // 从 component.Install 中提取 chart 信息，或从 componentDir 中读取
    // ...
    cmd := exec.CommandContext(ctx, "helm", "install", ...)
    return cmd.Run()
}

// applyWithCustom applies using custom install command
func applyWithCustom(ctx context.Context, opts ApplyKubernetesOptions, component template.KubernetesComponent, componentDir string) error {
    // 执行 component.Install 命令
    // 需要在 componentDir 中执行
    // ...
    return nil
}
```

### 步骤 6: 实现依赖解析

在 `pkg/template/config.go` 中添加：

```go
// ResolveKubernetesDependencies resolves component dependencies and returns ordered list
func (cfg *KubernetesConfig) ResolveDependencies(componentType string) ([]string, error) {
    // componentType: "init" or "optional"
    // 返回按依赖顺序排列的组件名称列表
    // 使用拓扑排序算法
    // ...
}
```

## 四、关键实现细节

### 4.1 installType 的默认值处理

- 如果 `installType` 未指定，默认为 `kubectl`
- 如果组件有 `kustomization.yaml`，通常使用 `kubectl`
- 如果组件有 `Chart.yaml` 或使用 helm，使用 `helm`
- 如果组件有自定义 `install` 脚本，使用 `custom`

### 4.2 依赖解析

需要处理以下依赖关系：
- `init.components` 之间的依赖
- `optional` 组件之间的依赖
- `optional` 组件对 `init.components` 的依赖

### 4.3 项目级别的组件

Kubernetes 的组件是按项目配置的（在 default.yaml 的 projects 中），需要：
- 为每个项目单独渲染组件
- 支持项目级别的参数覆盖

### 4.4 文件渲染

- 模板文件使用 `.tmpl` 扩展名
- 渲染后去掉 `.tmpl` 扩展名
- 支持多个模板文件（path 是列表）

## 五、测试计划

1. **单元测试**：
   - 测试参数提取函数
   - 测试依赖解析
   - 测试 installType 判断

2. **集成测试**：
   - 测试完整的 init 流程
   - 测试完整的 apply 流程
   - 测试不同 installType 的应用

3. **端到端测试**：
   - 使用真实的模板仓库测试
   - 验证生成的文件结构
   - 验证 apply 命令执行

## 六、注意事项

1. **向后兼容**：如果 config.yaml 中没有 `installType`，应该默认为 `kubectl`
2. **错误处理**：提供详细的错误信息，帮助用户快速定位问题
3. **性能**：考虑并发处理多个组件（如果依赖关系允许）
4. **状态管理**：记录已初始化的组件，支持增量更新

## 七、参考实现

参考 Terraform 部分的实现：
- `pkg/bootstrap/terraform/init.go` - Init 项处理
- `pkg/bootstrap/terraform/projects.go` - 项目处理
- `pkg/bootstrap/terraform/args.go` - 参数提取
- `pkg/bootstrap/apply_terraform.go` - Apply 实现
