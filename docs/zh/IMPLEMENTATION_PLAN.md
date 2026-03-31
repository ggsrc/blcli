# blcli v1.0 功能实现计划

> 创建日期：2026-01-27  
> 基于：用户需求与 v1.0_STATUS_ANALYSIS.md

## 目录

1. [Status 命令实现](#1-status-命令实现)
2. [Apply Rollback 功能](#2-apply-rollback-功能)
3. [进度显示与持久化](#3-进度显示与持久化)
4. [Terraform Apply 依赖排序](#4-terraform-apply-依赖排序)
5. [执行计划输出](#5-执行计划输出)

---

## 1. Status 命令实现

### 1.1 功能需求

`blcli status` 命令用于检查各组件安装情况，包括：
- Terraform 状态检查（`terraform show`）
- Kubernetes 资源状态（`kubectl get`）
- GitOps 同步状态（ArgoCD Application 状态）

### 1.2 命令结构

```bash
blcli status [terraform|kubernetes|gitops|all]
  --args <args.yaml>          # 必需：配置文件路径
  --workspace <path>          # 可选：工作目录（默认从 args 读取）
  --format <json|yaml|table> # 可选：输出格式（默认 table）
  --kubeconfig <path>        # 可选：kubeconfig 路径
  --context <name>           # 可选：Kubernetes context
```

### 1.3 实现方案

#### 1.3.1 Status Terraform

**功能**：检查 Terraform 资源状态

**实现步骤**：
1. 读取 args.yaml，获取 terraform 配置
2. 遍历 `terraform/init/` 和 `terraform/gcp/{project}/` 目录
3. 对每个目录执行 `terraform show -json`（如果已初始化）
4. 解析 JSON 输出，提取资源状态
5. 汇总显示：
   - 已创建资源数量
   - 资源类型分布
   - 最近更新时间
   - 状态摘要（正常/异常）

**输出示例**：
```
Terraform Status:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Init: 0-backend
  Resources: 3 created, 0 changed, 0 destroyed
  Last Updated: 2026-01-27 10:30:00
  Status: ✅ Healthy

Project: prd
  Resources: 15 created, 2 changed, 0 destroyed
  Last Updated: 2026-01-27 10:35:00
  Status: ⚠️  Some changes pending
```

#### 1.3.2 Status Kubernetes

**功能**：检查 Kubernetes 资源状态

**实现步骤**：
1. 读取 args.yaml，获取 kubernetes 配置
2. 遍历 `kubernetes/{project}/{component}/` 目录
3. 对每个组件执行 `kubectl get` 检查资源状态
4. 汇总显示：
   - Namespace 状态
   - Deployment/StatefulSet 就绪状态
   - Service 状态
   - 组件健康状态

**输出示例**：
```
Kubernetes Status:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Project: stg
  Component: istio
    Namespace: ✅ Active
    Deployments: 3/3 Ready
    Services: 5 Active
    Status: ✅ Healthy
  
  Component: argocd
    Namespace: ✅ Active
    Deployments: 5/5 Ready
    Status: ✅ Healthy
```

#### 1.3.3 Status GitOps

**功能**：检查 GitOps/ArgoCD 同步状态

**实现步骤**：
1. 读取 args.yaml，获取 gitops 配置
2. 遍历 `gitops/{project}/{app}/` 目录，查找 `app.yaml`
3. 对每个 ArgoCD Application 执行 `kubectl get application`
4. 检查同步状态：
   - Sync Status（Synced/OutOfSync）
   - Health Status（Healthy/Degraded）
   - 最后同步时间

**输出示例**：
```
GitOps Status:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Project: stg
  Application: hello-world
    Sync Status: ✅ Synced
    Health Status: ✅ Healthy
    Last Synced: 2026-01-27 10:40:00
    Revision: main@abc123
  
  Application: hello-world-2
    Sync Status: ⚠️  OutOfSync
    Health Status: ✅ Healthy
    Last Synced: 2026-01-27 10:35:00
    Revision: main@def456
```

### 1.4 代码结构

```
pkg/
├── cli/
│   └── status.go              # Status 命令定义
├── bootstrap/
│   └── status.go               # Status 执行逻辑
│       ├── ExecuteStatus()
│       ├── StatusTerraform()
│       ├── StatusKubernetes()
│       └── StatusGitOps()
└── status/
    └── status.go               # Status 数据结构
        ├── StatusResult
        ├── TerraformStatus
        ├── KubernetesStatus
        └── GitOpsStatus
```

---

## 2. Apply Rollback 功能

### 2.1 功能需求

当 `blcli apply` 执行失败时，能够回滚已成功部署的资源。

### 2.2 设计原则

1. **操作记录**：每次 apply 操作前记录操作计划
2. **检查点机制**：每个组件部署成功后创建检查点
3. **回滚策略**：按依赖顺序逆序回滚
4. **状态持久化**：操作记录和检查点持久化到文件

### 2.3 实现方案

#### 2.3.1 操作记录结构

**文件位置**：`~/.blcli/operations/{operation-id}.json`

**数据结构**：
```json
{
  "id": "op-20260127-103000-abc123",
  "type": "apply",
  "started_at": "2026-01-27T10:30:00Z",
  "completed_at": null,
  "status": "in_progress",
  "modules": ["terraform", "kubernetes", "gitops"],
  "checkpoints": [
    {
      "module": "terraform",
      "component": "init/0-backend",
      "status": "completed",
      "timestamp": "2026-01-27T10:30:15Z",
      "rollback_command": "terraform destroy -auto-approve",
      "rollback_dir": "/path/to/terraform/init/0-backend"
    },
    {
      "module": "terraform",
      "component": "gcp/prd",
      "status": "completed",
      "timestamp": "2026-01-27T10:35:00Z",
      "rollback_command": "terraform destroy -auto-approve",
      "rollback_dir": "/path/to/terraform/gcp/prd"
    },
    {
      "module": "kubernetes",
      "component": "stg/istio",
      "status": "failed",
      "timestamp": "2026-01-27T10:40:00Z",
      "error": "kubectl apply failed: ..."
    }
  ],
  "rollback_plan": [
    {
      "step": 1,
      "module": "kubernetes",
      "component": "stg/istio",
      "command": "kubectl delete -k /path/to/kubernetes/stg/istio",
      "status": "pending"
    },
    {
      "step": 2,
      "module": "terraform",
      "component": "gcp/prd",
      "command": "terraform destroy -auto-approve",
      "status": "pending"
    }
  ]
}
```

#### 2.3.2 检查点创建时机

**Terraform Apply**：
- 每个目录的 `terraform apply` 成功后创建检查点
- 记录：目录路径、terraform state 路径、回滚命令

**Kubernetes Apply**：
- 每个组件的 `kubectl apply` 成功后创建检查点
- 记录：组件路径、应用的资源清单、回滚命令（kubectl delete）

**GitOps Apply**：
- 每个 `app.yaml` 的 `kubectl apply` 成功后创建检查点
- 记录：Application 名称、Namespace、回滚命令

#### 2.3.3 回滚命令实现

**命令结构**：
```bash
blcli apply rollback [operation-id]
  --operation-id <id>    # 可选：指定操作 ID（默认最新失败操作）
  --dry-run              # 可选：仅显示回滚计划，不执行
  --auto-approve         # 可选：自动确认回滚
```

**回滚流程**：
1. 加载操作记录（如果未指定 ID，查找最新失败的操作）
2. 构建回滚计划（按依赖顺序逆序）
3. 显示回滚计划，等待用户确认（除非 `--auto-approve`）
4. 执行回滚：
   - Terraform：`terraform destroy -auto-approve`
   - Kubernetes：`kubectl delete -k <component-dir>` 或 `kubectl delete -f <app.yaml>`
   - GitOps：`kubectl delete application <name> -n <namespace>`
5. 更新操作记录状态为 `rolled_back`

#### 2.3.4 错误处理与回滚触发

**自动回滚触发条件**：
- `--auto-rollback` 标志启用
- 组件部署失败
- 用户确认回滚

**回滚策略**：
- **部分回滚**：只回滚失败的模块及其依赖
- **完全回滚**：回滚整个操作的所有已部署资源

### 2.4 代码结构

```
pkg/
├── cli/
│   └── apply_rollback.go     # Rollback 命令定义
├── bootstrap/
│   ├── apply.go              # 修改：添加检查点创建
│   ├── rollback.go           # Rollback 执行逻辑
│   │   ├── ExecuteRollback()
│   │   ├── RollbackTerraform()
│   │   ├── RollbackKubernetes()
│   │   └── RollbackGitOps()
│   └── checkpoint.go          # 检查点管理
│       ├── CreateCheckpoint()
│       ├── LoadOperation()
│       └── SaveOperation()
└── state/
    └── operation.go           # 操作记录数据结构
        ├── Operation
        ├── Checkpoint
        └── RollbackPlan
```

---

## 3. 进度显示与持久化

### 3.1 功能需求

- 实时显示 init/apply 进度
- 进度持久化到文件（支持中断后恢复）
- 显示已完成/进行中/待执行的步骤

### 3.2 进度数据结构

**文件位置**：`~/.blcli/progress/{operation-id}.yaml`

**数据结构**：
```yaml
operation_id: "op-20260127-103000-abc123"
type: "init"  # 或 "apply"
started_at: "2026-01-27T10:30:00Z"
updated_at: "2026-01-27T10:35:00Z"
status: "in_progress"  # pending, in_progress, completed, failed, cancelled
total_steps: 10
completed_steps: 3
current_step: 4

modules:
  terraform:
    status: "completed"
    progress: 100
    steps:
      - name: "init/0-backend"
        status: "completed"
        started_at: "2026-01-27T10:30:05Z"
        completed_at: "2026-01-27T10:30:15Z"
        duration: "10s"
      - name: "gcp/prd"
        status: "completed"
        started_at: "2026-01-27T10:30:20Z"
        completed_at: "2026-01-27T10:35:00Z"
        duration: "4m40s"
  
  kubernetes:
    status: "in_progress"
    progress: 50
    steps:
      - name: "stg/istio"
        status: "completed"
        started_at: "2026-01-27T10:35:05Z"
        completed_at: "2026-01-27T10:36:00Z"
        duration: "55s"
      - name: "stg/argocd"
        status: "in_progress"
        started_at: "2026-01-27T10:36:05Z"
        completed_at: null
        duration: null
      - name: "prd/istio"
        status: "pending"
        started_at: null
        completed_at: null
        duration: null
  
  gitops:
    status: "pending"
    progress: 0
    steps: []
```

### 3.3 进度显示实现

#### 3.3.1 控制台输出

**格式**：
```
🚀 Initializing infrastructure...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Progress: [████████░░░░░░░░░░] 40% (4/10 steps)

✅ Terraform (100%)
  ✓ init/0-backend (10s)
  ✓ gcp/prd (4m40s)

🔄 Kubernetes (50%)
  ✓ stg/istio (55s)
  ⏳ stg/argocd (in progress...)
  ⏸  prd/istio (pending)

⏸  GitOps (0%)
  (pending)

Current: Applying kubernetes component stg/argocd...
```

#### 3.3.2 进度更新时机

**Init 操作**：
- 每个模块开始：更新 `status: in_progress`
- 每个组件完成：更新 `status: completed`，记录时间
- 操作完成：更新总体状态

**Apply 操作**：
- 每个模块开始：更新 `status: in_progress`
- 每个组件开始：创建步骤记录
- 每个组件完成：更新步骤状态，记录时间
- 操作完成/失败：更新总体状态

### 3.4 进度持久化

#### 3.4.1 持久化时机

- **操作开始**：创建进度文件
- **步骤完成**：更新进度文件（每次步骤完成后立即保存）
- **操作完成/失败**：最终保存

#### 3.4.2 恢复机制

**场景**：操作中断后恢复

**实现**：
1. 检查 `~/.blcli/progress/` 目录，查找 `status: in_progress` 的进度文件
2. 提示用户是否恢复：
   ```
   Found incomplete operation: op-20260127-103000-abc123
   Status: in_progress (4/10 steps completed)
   Do you want to resume? [y/N]
   ```
3. 如果恢复：
   - 加载进度文件
   - 跳过已完成的步骤
   - 从失败/中断的步骤继续

### 3.5 代码结构

```
pkg/
├── bootstrap/
│   └── progress.go            # 进度管理
│       ├── ProgressTracker
│       │   ├── StartOperation()
│       │   ├── StartStep()
│       │   ├── CompleteStep()
│       │   ├── FailStep()
│       │   ├── UpdateDisplay()
│       │   └── Save()
│       └── LoadProgress()
└── state/
    └── progress.go             # 进度数据结构
        ├── Progress
        ├── ModuleProgress
        └── StepProgress
```

---

## 4. Terraform Apply 依赖排序

### 4.1 功能需求

Terraform apply 应按 config.yaml 中的 dependencies 构建 DAG 并拓扑排序执行。

### 4.2 实现方案

#### 4.2.1 添加 ResolveTerraformDependencies

**位置**：`pkg/template/config.go`

**实现**：参考 `ResolveKubernetesDependencies`，添加：
```go
func (cfg *TerraformConfig) ResolveTerraformDependencies(componentNames []string) ([]string, error) {
    // 构建组件映射
    componentMap := make(map[string]ProjectItem)
    for _, item := range cfg.Projects {
        componentMap[item.Name] = item
    }
    
    // DFS 拓扑排序（与 Kubernetes 相同逻辑）
    // ...
}
```

#### 4.2.2 修改 Apply Terraform

**位置**：`pkg/bootstrap/apply_terraform.go`

**修改点**：
1. 加载 terraform config.yaml（如果 template repo 可用）
2. 收集项目组件名称
3. 调用 `ResolveTerraformDependencies` 获取排序后的组件列表
4. 按依赖顺序执行 apply

### 4.3 代码修改

```go
// 在 ExecuteApplyTerraform 中
if templateLoader != nil {
    terraformConfig, err := templateLoader.LoadTerraformConfig()
    if err == nil {
        // 收集组件名称
        componentNames := []string{"backend", "variables", "gke", ...}
        
        // 解析依赖
        orderedComponents, err := terraformConfig.ResolveTerraformDependencies(componentNames)
        if err == nil {
            // 按依赖顺序执行
        }
    }
}
```

---

## 5. 执行计划输出

### 5.1 功能需求

执行前向用户输出具体执行命令及参数，支持 `--dry-run` 模式。

### 5.2 实现方案

#### 5.2.1 执行计划结构

```go
type ExecutionPlan struct {
    OperationID string
    Type        string  // "init" 或 "apply"
    Modules     []ModulePlan
}

type ModulePlan struct {
    Name        string
    Steps       []StepPlan
    Dependencies []string
}

type StepPlan struct {
    Number      int
    Module      string
    Component   string
    Command     string
    Args        []string
    WorkingDir  string
    Dependencies []string
    EstimatedDuration time.Duration
}
```

#### 5.2.2 计划输出格式

```
📋 Execution Plan
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Operation ID: op-20260127-103000-abc123
Type: apply
Estimated Duration: ~15 minutes

Step 1: Terraform - init/0-backend
  Command: terraform apply -auto-approve
  Directory: /path/to/terraform/init/0-backend
  Estimated: 30s

Step 2: Terraform - gcp/prd/backend
  Command: terraform apply -var-file=backend.tf.tmpl -auto-approve
  Directory: /path/to/terraform/gcp/prd
  Depends on: Step 1
  Estimated: 2m

Step 3: Terraform - gcp/prd/gke
  Command: terraform apply -var-file=gke.tf.tmpl -auto-approve
  Directory: /path/to/terraform/gcp/prd
  Depends on: Step 2
  Estimated: 5m

Step 4: Kubernetes - stg/istio
  Command: kubectl apply -k /path/to/kubernetes/stg/istio
  Context: my-cluster
  Estimated: 1m

Step 5: Kubernetes - stg/argocd
  Command: kubectl apply -k /path/to/kubernetes/stg/argocd
  Context: my-cluster
  Depends on: Step 4
  Estimated: 2m

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Total Steps: 5
Total Estimated Duration: ~10m30s

Proceed? [y/N]
```

#### 5.2.3 实现位置

```
pkg/
├── bootstrap/
│   └── plan.go                # 执行计划（扩展现有文件）
│       ├── BuildExecutionPlan()
│       ├── PrintExecutionPlan()
│       └── ExecutePlan()
```

---

## 实施优先级

### P0（v1.0 必需）
1. ✅ Status 命令实现
2. ✅ Apply Rollback 功能
3. ✅ 进度显示与持久化

### P1（v1.0 重要）
4. ✅ Terraform Apply 依赖排序
5. ✅ 执行计划输出

### P2（v1.0 可选）
6. 并行初始化（后续版本）

### 待办（已规划未实现）
7. **进度恢复（Resume）**：init/apply all 入口检测未完成进度，提示「是否恢复」并从断点继续（`LoadProgressTracker` 已存在）
8. **apply 按 project**：Kubernetes、GitOps 支持 `--project` 只处理指定 project（Terraform 已支持）
9. **完善错误提示**：错误信息清晰、可操作（不添加自动重试）
10. **安装状态长期持久化**（可选）：将本次 apply 成功组件写入状态，供 status/增量部署使用

### 产品约定（不实现）
- 不要求：并行初始化、初始化后自动提交 Git、失败自动重试、多模板源合并、模板版本管理
- Git 提交保持手动调用 `blcli apply init-repos`

---

## 文件清单（与当前实现一致）

### 已存在文件
- `pkg/cli/status.go` - Status 命令
- `pkg/cli/rollback.go` - 独立 Rollback 命令（非 apply 子命令）
- `pkg/bootstrap/status.go` - ExecuteStatus、StatusTerraform/Kubernetes/GitOps
- `pkg/bootstrap/rollback.go` - ExecuteRollback、RollbackTerraform/Kubernetes/GitOps
- `pkg/bootstrap/progress.go` - ProgressTracker、LoadProgressTracker
- `pkg/bootstrap/execution_plan.go` - ExecutionPlan、PlanItem、PrintExecutionPlan
- `pkg/state/progress.go` - Progress 数据结构、LoadProgress

### 未实现（原计划）
- `pkg/bootstrap/checkpoint.go` - 检查点管理（当前回滚采用 config 驱动，无检查点）
- `pkg/state/operation.go` - 操作记录（未采用）

### 已修改文件
- `pkg/cli/root.go` - 注册 status、rollback
- `pkg/bootstrap/apply_terraform.go` - 依赖排序、执行计划、--dry-run
- `pkg/bootstrap/apply_kubernetes.go` - 执行计划、--dry-run
- `pkg/bootstrap/apply_gitops.go` - 执行计划、--dry-run
- `pkg/bootstrap/apply_all.go` - 进度显示、各模块 --dry-run 传递
- `pkg/bootstrap/manager.go` - 进度显示
- `pkg/template/config.go` - ResolveTerraformDependencies

---

*最后更新：2026-01-27*
