# blcli apply E2E 测试计划

## 概述

本计划旨在为 `blcli apply` 命令实现端到端（E2E）测试，覆盖所有子命令的功能验证。

## 测试目标

1. **terraform apply E2E 测试**：验证 Terraform 资源能够成功应用到 GCP（使用 fake-gcs-server）
2. **kubernetes apply E2E 测试**：验证 Kubernetes 资源能够成功部署（使用 client-go fake）
3. **gitops apply E2E 测试**：验证 GitOps 流程能够成功执行（Mock GitHub 和 ArgoCD）
4. **apply all E2E 测试**：验证完整流程（terraform → kubernetes → gitops）

## 测试框架和技术栈

### 已具备的基础设施
- ✅ Ginkgo/Gomega 测试框架
- ✅ fake-gcs-server（用于模拟 GCS）
- ✅ client-go fake（用于模拟 Kubernetes API）
- ✅ 测试环境设置和清理函数

### 需要添加的依赖
- `github.com/gruntwork-io/terratest` - Terraform 测试工具
- `github.com/google/go-github` - GitHub API 客户端（用于 Mock）
- ArgoCD API Mock（可能需要自定义或使用现有库）

## 测试文件结构

```
integration/
├── e2e/
│   ├── apply_suite_test.go           # Apply E2E 测试套件
│   ├── apply_terraform_test.go       # Terraform apply E2E 测试
│   ├── apply_kubernetes_test.go     # Kubernetes apply E2E 测试
│   ├── apply_gitops_test.go          # GitOps apply E2E 测试
│   ├── apply_all_test.go             # Apply all E2E 测试
│   └── apply_helpers.go              # Apply 测试辅助函数
├── helpers.go                        # 现有测试辅助函数（已存在）
└── fixtures/
    └── apply/                        # Apply 测试专用 fixtures
        ├── terraform/                # Terraform 测试模板
        ├── kubernetes/               # Kubernetes 测试模板
        └── gitops/                   # GitOps 测试模板
```

## 详细测试场景

### 1. Terraform Apply E2E 测试

#### 测试文件：`integration/e2e/apply_terraform_test.go`

**测试场景**：

1. **成功场景：应用单个 terraform project**
   - 前置条件：已通过 `blcli init terraform` 生成 terraform 目录
   - 步骤：
     1. 启动 fake-gcs-server
     2. 配置 terraform backend 指向 fake-gcs
     3. 执行 `blcli apply terraform -d {terraform_dir} --use-emulator`
     4. 验证 terraform init 成功
     5. 验证 terraform plan 成功
     6. 验证 terraform apply 成功（使用 terratest）
     7. 验证资源状态在 fake-gcs 中
   - 断言：所有步骤成功，资源状态正确

2. **成功场景：应用多个 terraform projects**
   - 测试多个 projects 的顺序执行
   - 验证依赖关系处理

3. **成功场景：应用 init 目录**
   - 测试 init 目录按数字顺序执行
   - 验证 backend 初始化

4. **错误场景：terraform 语法错误**
   - 测试错误检测和报告

5. **错误场景：backend 连接失败**
   - 测试错误处理

**技术实现**：
- 使用 `github.com/gruntwork-io/terratest/modules/terraform` 执行 terraform 命令
- 使用 `fake-gcs-server` 模拟 GCS backend
- 使用 terratest 的 `terraform.InitAndApply` 和 `terraform.Output` 验证结果

**辅助函数**：
```go
// SetupTerraformApplyTest 设置 terraform apply 测试环境
func SetupTerraformApplyTest(workspace string) (*TerraformApplyTestEnv, error)

// ExecuteTerraformApply 执行 terraform apply 命令
func ExecuteTerraformApply(terraformDir string, opts TerraformApplyOptions) error

// VerifyTerraformResources 验证 terraform 资源状态
func VerifyTerraformResources(env *TerraformApplyTestEnv) error
```

### 2. Kubernetes Apply E2E 测试

#### 测试文件：`integration/e2e/apply_kubernetes_test.go`

**测试场景**：

1. **成功场景：应用 Kubernetes manifests**
   - 前置条件：已通过 `blcli init kubernetes` 生成 kubernetes 目录
   - 步骤：
     1. 创建 fake Kubernetes client
     2. 执行 `blcli apply kubernetes -d {kubernetes_dir}`（使用 fake client）
     3. 验证资源创建顺序（Namespace → ConfigMap → Deployment → Service）
     4. 验证资源状态
   - 断言：所有资源创建成功，状态正确

2. **成功场景：资源依赖顺序**
   - 测试资源按正确顺序创建
   - 验证依赖关系

3. **成功场景：资源更新**
   - 测试已存在资源的更新

4. **错误场景：资源冲突**
   - 测试资源冲突检测

5. **错误场景：集群连接失败**
   - 测试错误处理

**技术实现**：
- 使用 `k8s.io/client-go/kubernetes/fake` 创建 fake client
- 拦截 `kubectl` 命令，使用 fake client 执行操作
- 或者直接使用 client-go API 应用资源

**辅助函数**：
```go
// SetupKubernetesApplyTest 设置 kubernetes apply 测试环境
func SetupKubernetesApplyTest() (*KubernetesApplyTestEnv, error)

// ExecuteKubernetesApply 执行 kubernetes apply 命令
func ExecuteKubernetesApply(kubernetesDir string, opts KubernetesApplyOptions) error

// VerifyKubernetesResources 验证 kubernetes 资源状态
func VerifyKubernetesResources(env *KubernetesApplyTestEnv, expectedResources []string) error
```

### 3. GitOps Apply E2E 测试

#### 测试文件：`integration/e2e/apply_gitops_test.go`

**测试场景**：

1. **成功场景：创建 GitHub repo 并部署 ArgoCD Application**
   - 前置条件：已通过 `blcli init gitops` 生成 gitops 目录
   - 步骤：
     1. Mock GitHub API（创建 repository）
     2. 推送配置到 repository
     3. Mock ArgoCD API（创建 Application）
     4. 验证 Application 状态
   - 断言：Repository 创建成功，Application 部署成功

2. **成功场景：使用已存在的 repository**
   - 测试不创建新 repository 的情况

3. **成功场景：更新已存在的 Application**
   - 测试 Application 更新

4. **错误场景：GitHub token 无效**
   - 测试错误处理

5. **错误场景：ArgoCD 连接失败**
   - 测试错误处理

**技术实现**：
- 使用 `github.com/google/go-github` 的 mock 或自定义 mock
- Mock ArgoCD API（可能需要自定义实现）
- 使用 in-memory git repository 模拟 GitHub

**辅助函数**：
```go
// SetupGitOpsApplyTest 设置 gitops apply 测试环境
func SetupGitOpsApplyTest() (*GitOpsApplyTestEnv, error)

// MockGitHubAPI 创建 GitHub API mock
func MockGitHubAPI() (*GitHubMock, error)

// MockArgoCDAPI 创建 ArgoCD API mock
func MockArgoCDAPI() (*ArgoCDMock, error)

// ExecuteGitOpsApply 执行 gitops apply 命令
func ExecuteGitOpsApply(gitopsDir string, argsPath string, opts GitOpsApplyOptions) error

// VerifyGitOpsResources 验证 gitops 资源状态
func VerifyGitOpsResources(env *GitOpsApplyTestEnv) error
```

### 4. Apply All E2E 测试

#### 测试文件：`integration/e2e/apply_all_test.go`

**测试场景**：

1. **成功场景：完整流程**
   - 步骤：
     1. 执行 `blcli init all` 生成所有配置
     2. 执行 `blcli apply all -d {workspace} --args {args.yaml}`
     3. 验证 terraform 资源
     4. 验证 kubernetes 资源
     5. 验证 gitops 资源
   - 断言：所有模块成功应用

2. **成功场景：模块依赖顺序**
   - 验证执行顺序：terraform → kubernetes → gitops

3. **错误场景：部分模块失败**
   - 测试 `--continue-on-error` 行为
   - 测试错误汇总

4. **错误场景：跳过模块**
   - 测试 `--skip-modules` 功能

**技术实现**：
- 组合使用上述三个模块的测试环境
- 验证模块间的数据传递（terraform output → kubernetes config）

## 实现步骤

### 阶段 1：添加依赖和基础框架

1. **添加 terratest 依赖**
   ```bash
   go get github.com/gruntwork-io/terratest/modules/terraform
   go get github.com/gruntwork-io/terratest/modules/gcp
   ```

2. **添加 GitHub API 依赖**
   ```bash
   go get github.com/google/go-github/v60
   ```

3. **创建测试辅助函数文件**
   - `integration/e2e/apply_helpers.go`

### 阶段 2：实现 Terraform Apply E2E 测试

1. 创建 `apply_terraform_test.go`
2. 实现 terraform apply 测试辅助函数
3. 实现测试场景 1-5
4. 验证测试通过

### 阶段 3：实现 Kubernetes Apply E2E 测试

1. 创建 `apply_kubernetes_test.go`
2. 实现 kubernetes apply 测试辅助函数
3. 实现测试场景 1-5
4. 验证测试通过

### 阶段 4：实现 GitOps Apply E2E 测试

1. 创建 `apply_gitops_test.go`
2. 实现 GitHub 和 ArgoCD mock
3. 实现 gitops apply 测试辅助函数
4. 实现测试场景 1-5
5. 验证测试通过

### 阶段 5：实现 Apply All E2E 测试

1. 创建 `apply_all_test.go`
2. 组合使用上述测试环境
3. 实现测试场景 1-4
4. 验证测试通过

## 测试数据准备

### Terraform 测试数据
- 简单的 GCP 资源（Storage Bucket、Compute Instance 等）
- 测试 backend 配置
- 测试变量和输出

### Kubernetes 测试数据
- Namespace
- ConfigMap/Secret
- Deployment
- Service
- Ingress（可选）

### GitOps 测试数据
- ArgoCD Application manifest
- 简单的业务服务配置
- GitHub repository 配置

## 注意事项

1. **测试隔离**：每个测试使用独立的测试环境，避免相互影响
2. **资源清理**：测试完成后清理所有创建的资源
3. **超时设置**：为长时间运行的测试设置合理的超时时间
4. **错误处理**：测试应该验证错误场景，确保错误被正确处理
5. **Mock 实现**：GitHub 和 ArgoCD 的 mock 需要仔细设计，确保覆盖主要场景

## 运行测试

```bash
# 运行所有 apply E2E 测试
go test ./integration/e2e/... -v -ginkgo.focus="Apply"

# 运行特定模块的测试
go test ./integration/e2e/... -v -ginkgo.focus="Terraform Apply"
go test ./integration/e2e/... -v -ginkgo.focus="Kubernetes Apply"
go test ./integration/e2e/... -v -ginkgo.focus="GitOps Apply"
go test ./integration/e2e/... -v -ginkgo.focus="Apply All"
```

## 后续工作

1. 实现 `blcli apply` 命令本身
2. 根据测试结果调整实现
3. 添加更多边界场景测试
4. 性能测试和优化

