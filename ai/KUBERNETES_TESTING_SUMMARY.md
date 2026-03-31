# Kubernetes 测试实现总结

本文档总结了为 Kubernetes 部分实现的单元测试和 e2e 测试。

## 一、单元测试（Unit Tests）

### 1.1 Kubernetes 子包测试

#### `pkg/bootstrap/kubernetes/args_test.go`
测试参数提取功能：
- `ExtractComponentArgs`: 从 `kubernetes.projects[].components[]` 中提取组件参数
- `GetAvailableInitComponents`: 获取可用的 init 组件列表
- `GetAvailableOptionalComponents`: 获取可用的 optional 组件列表
- `GetKubernetesProjects`: 获取项目名称列表
- `GetProjectArgs`: 提取项目特定的参数

#### `pkg/bootstrap/kubernetes/init_test.go`
测试 init 组件初始化：
- `InitializeInitItems`: 初始化 namespace 和 init 组件
- 验证组件过滤（只初始化在 args.yaml 中配置的组件）
- 验证文件生成

#### `pkg/bootstrap/kubernetes/optional_test.go`
测试 optional 组件初始化：
- `InitializeOptionalComponents`: 为项目初始化 optional 组件
- 验证组件过滤（只初始化在项目配置中的组件）

#### `pkg/bootstrap/kubernetes/kubernetes_suite_test.go`
Ginkgo 测试套件入口文件

### 1.2 Bootstrap 层测试

#### `pkg/bootstrap/kubernetes_bootstrap_test.go`
测试 `BootstrapKubernetes` 函数：
- 成功初始化 Kubernetes 项目
- 验证目录结构和 marker 文件
- 处理 overwrite 标志
- 错误处理（缺少 template loader）

#### `pkg/bootstrap/check_kubernetes_test.go`
测试 `ExecuteCheckKubernetes` 函数：
- kubectl installType 组件检查
- 处理缺失的 kubernetes 目录
- 验证检查流程

## 二、E2E 测试（End-to-End Tests）

### 2.1 Apply Kubernetes E2E 测试

#### `integration/e2e/apply_kubernetes_test.go`
更新了现有的 e2e 测试：
- 成功应用 Kubernetes 资源
- Dry-run 模式
- 资源依赖顺序
- installType 支持

### 2.2 Check Kubernetes E2E 测试

#### `integration/e2e/check_kubernetes_test.go`
新增的 check kubernetes e2e 测试：
- kubectl installType 检查
- Dry-run 验证
- 错误处理（无效的 manifests）

### 2.3 Kubernetes Integration 测试

#### `integration/kubernetes/kubernetes_integration_test.go`
更新了集成测试：
- 成功初始化
- 带 init 组件的初始化
- 错误处理

## 三、测试辅助函数

### 3.1 测试模板仓库

更新了 `integration/helpers.go` 中的 `CreateTestTemplateRepo` 函数：
- 创建符合新结构的 `kubernetes/config.yaml`
- 支持 init/optional 组件
- 支持 installType 字段
- 创建测试模板文件

### 3.2 测试环境设置

- `setupTestWorkspace`: 创建临时测试工作空间
- `cleanupTestWorkspace`: 清理测试工作空间
- `loadTestTemplateRepo`: 加载本地模板仓库

## 四、测试覆盖范围

### 4.1 已覆盖的功能

1. **参数提取**：
   - ✅ 从 kubernetes.projects 提取组件参数
   - ✅ 获取可用组件列表
   - ✅ 提取项目参数

2. **初始化**：
   - ✅ Namespace 初始化
   - ✅ Init 组件初始化
   - ✅ Optional 组件初始化
   - ✅ 组件过滤（只初始化配置的组件）

3. **检查**：
   - ✅ kubectl installType 检查
   - ✅ 错误处理

4. **Bootstrap**：
   - ✅ 完整的初始化流程
   - ✅ Overwrite 处理
   - ✅ 错误处理

### 4.2 待完善的功能

1. **Helm installType 测试**：
   - 需要真实的 helm chart 或 mock helm
   - 测试 helm lint 和 helm template

2. **Custom installType 测试**：
   - 测试自定义 check 命令
   - 测试 check 命令为空的情况

3. **依赖解析测试**：
   - 测试复杂的依赖关系
   - 测试循环依赖检测

4. **Apply 测试**：
   - 需要真实的 Kubernetes 集群或更好的 mock
   - 测试不同 installType 的应用

## 五、运行测试

### 5.1 运行单元测试

```bash
# 运行所有 Kubernetes 单元测试
go test ./pkg/bootstrap/kubernetes/... -v

# 运行 Bootstrap 测试
go test ./pkg/bootstrap/... -v -run Kubernetes
```

### 5.2 运行 E2E 测试

```bash
# 运行 Kubernetes e2e 测试
ginkgo integration/e2e/apply_kubernetes_test.go
ginkgo integration/e2e/check_kubernetes_test.go

# 运行集成测试
ginkgo integration/kubernetes/kubernetes_integration_test.go
```

### 5.3 运行所有测试

```bash
# 运行所有测试
go test ./...

# 使用 ginkgo 运行
ginkgo -r
```

## 六、测试注意事项

### 6.1 测试环境要求

1. **kubectl**: 某些测试需要 kubectl 可用（即使只是验证命令结构）
2. **模板仓库**: 测试需要访问 bl-template 仓库（本地路径）
3. **工作空间**: 测试文件统一在 `workspace/` 目录下创建

### 6.2 测试隔离

- 每个测试使用独立的临时工作空间
- 测试完成后自动清理
- 使用绝对路径避免路径问题

### 6.3 Mock 和 Fake

- 使用 fake Kubernetes client（在 integration 包中）
- 对于需要真实集群的测试，会跳过或标记为可选
- 使用本地模板仓库而不是远程仓库

## 七、测试文件结构

```
blcli-go/
├── pkg/bootstrap/kubernetes/
│   ├── args_test.go              # 参数提取测试
│   ├── init_test.go              # Init 组件测试
│   ├── optional_test.go          # Optional 组件测试
│   └── kubernetes_suite_test.go  # 测试套件
├── pkg/bootstrap/
│   ├── kubernetes_bootstrap_test.go  # Bootstrap 测试
│   └── check_kubernetes_test.go     # Check 测试
└── integration/
    ├── e2e/
    │   ├── apply_kubernetes_test.go  # Apply e2e 测试
    │   └── check_kubernetes_test.go  # Check e2e 测试
    └── kubernetes/
        └── kubernetes_integration_test.go  # 集成测试
```

## 八、后续改进建议

1. **增加更多测试用例**：
   - 测试 helm installType
   - 测试 custom installType 的 check 命令
   - 测试复杂的依赖关系

2. **改进 Mock**：
   - 更好的 Kubernetes API mock
   - Helm chart mock

3. **性能测试**：
   - 测试大量组件的初始化性能
   - 测试依赖解析的性能

4. **错误场景测试**：
   - 测试各种错误情况
   - 测试部分失败的处理
