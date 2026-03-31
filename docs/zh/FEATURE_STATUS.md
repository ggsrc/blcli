# blcli 功能盘点与待办

> 更新日期：2026-01-27  
> 基于当前代码与 Roadmap / IMPLEMENTATION_PLAN 对照

## 一、已实现功能 ✅

### 核心命令

| 命令 | 状态 | 说明 |
|------|------|------|
| **init** | ✅ | 支持 terraform/kubernetes/gitops，模板加载、args 合并、--overwrite、--output |
| **init-args** | ✅ | 从模板仓库生成 args.yaml/args.toml |
| **destroy** | ✅ | 销毁已初始化目录；对 terraform 模块可执行 `terraform destroy`（带双重确认，仅建议在测试/非生产环境使用） |
| **check** | ✅ | plugin / repo / kubernetes 检查 |
| **apply terraform** | ✅ | 按 init→gcp 顺序执行，支持 template config 依赖排序、执行计划、--dry-run |
| **apply kubernetes** | ✅ | 按 config 依赖排序，kubectl/helm/custom，执行计划、--dry-run |
| **apply gitops** | ✅ | 收集 app.yaml、执行计划、--dry-run |
| **apply all** | ✅ | terraform→kubernetes→gitops，各模块 --dry-run 传递，进度条 |
| **apply init-repos** | ✅ | 为 terraform/kubernetes/gitops 做 git init、创建 GitHub 仓库、提交推送（需 Y 确认） |
| **status** | ✅ | terraform / kubernetes / gitops 状态检查（ExecuteStatus） |
| **rollback** | ✅ | 独立 `blcli rollback`，按 config 的 Rollback 配置执行，支持 --dry-run |
| **explain** | ✅ | 解释模板组件与参数 |
| **version** | ✅ | 版本与构建时间 |

### 支撑能力

| 能力 | 状态 | 说明 |
|------|------|------|
| **Terraform 依赖排序** | ✅ | ResolveTerraformDependencies + apply 时按序执行 |
| **Kubernetes 依赖排序** | ✅ | ResolveKubernetesDependencies，DAG 拓扑排序 |
| **执行计划输出** | ✅ | ExecutionPlan/PlanItem，各 apply 前 PrintExecutionPlan |
| **--dry-run** | ✅ | apply terraform/kubernetes/gitops 及 apply all 的对应标志 |
| **进度显示与持久化** | ✅ | ProgressTracker，init/apply all 使用，持久化到 ~/.blcli/progress/ |
| **模板加载** | ✅ | GitHub/本地、缓存、ForceUpdate、私有仓库 token |
| **参数系统** | ✅ | 多 args 合并、YAML/TOML、workspace 等 |

---

## 二、部分实现或未实现 ⚠️ / ❌

### 2.1 进度与恢复

| 项 | 状态 | 说明 |
|----|------|------|
| 进度持久化 | ✅ | 有，~/.blcli/progress/{operation-id}.yaml |
| 中断后恢复（Resume） | ⚠️ | LoadProgressTracker 存在，但 **init/apply 入口未提供「发现未完成操作是否恢复」** 的交互与续跑逻辑 |

### 2.2 Apply 增强（Roadmap 中的 install）

| 项 | 状态 | 说明 |
|----|------|------|
| 分批安装 | 不要求 | 产品不要求分批安装 |
| 按 project 执行 | ⚠️ | **Terraform** 已支持 `--project`；**Kubernetes / GitOps** 待支持按 project 过滤 |
| 安装状态持久化 | ⚠️ | 仅有当次 progress，无长期“已安装组件”状态存储（可选） |

### 2.3 错误与可观测性

| 项 | 状态 | 说明 |
|----|------|------|
| 失败自动重试 | 不要求 | 产品不添加重试机制 |
| 错误提示完善 | 待办 | 需完善错误提示（清晰、可操作） |
| 部分失败时的回滚 | ⚠️ | 有 rollback 命令，非“apply 失败自动触发” |
| 操作历史 / 审计 | ❌ | 无独立操作历史或审计日志 |

### 2.4 Init 增强

| 项 | 状态 | 说明 |
|----|------|------|
| 并行初始化多项目 | 不要求 | 产品不要求并行初始化 |
| 初始化后提交 Git | ✅ 按设计 | 保持手动调用 `blcli apply init-repos`，不计划自动 |

### 2.5 模板系统

| 项 | 状态 | 说明 |
|----|------|------|
| 多模板源合并 | 不要求 | 不支持、也不计划：一次操作只从一个模板仓库加载，合并多源无需求 |
| 模板版本管理 | 不要求 | 不增加版本锁定/更新检测；当前支持 @branch 即可 |
| 单次操作单仓库 | ✅ | 每次 init/apply 通过 `--template-repo` 指定一个仓库；不同命令/不同时机可用不同仓库 |

### 2.6 交互与配置

| 项 | 状态 | 说明 |
|----|------|------|
| 交互式配置向导 | ❌ | 无 |
| 配置预览与确认 | ❌ | 无 |
| 操作时间估算 | ❌ | 无 |

---

## 三、建议的下一步（按优先级）

### P0：补齐 v1.0 体验

1. **进度恢复（Resume）**  
   - 在 init / apply all 开始时：检查 ~/.blcli/progress/ 是否存在 `in_progress` 的操作。  
   - 若有，询问「发现未完成操作 xxx，是否恢复？[y/N]」，若选恢复则 LoadProgressTracker，跳过已完成步骤继续执行。  
   - 工作量：小，主要改 `manager.go` 与 `apply_all.go` 的入口。

2. **（可选）安装状态持久化**  
   - 在 apply 成功后，将「本次 apply 成功的模块/组件」写入 ~/.blcli/state/ 或扩展现有 state。  
   - 供后续 status / rollback 或“仅部署未部署部分”使用。  
   - 工作量：中。

### P1：体验与可靠性

3. **完善错误提示**  
   - 错误信息清晰、可操作（含建议命令或链接）；不添加自动重试。  
   - 工作量：小。

4. **Apply 按 project**  
   - **Kubernetes**：支持 `--project` 只 apply 指定 project 下的组件。  
   - **GitOps**：支持按 project 过滤（如 `--project stg`）。  
   - Terraform 已支持 `--project`。

6. **Status 输出格式**  
   - 实现 status 的 --format=json|yaml|table（若尚未完全实现）。  
   - 工作量：小。

### P2：中长期

7. **交互式向导**  
   - 如 `blcli init --wizard` 引导生成/编辑 args。  
   - 工作量：中到大。

---

## 四、产品约定（已确认）

- **不要求**：并行初始化、初始化后自动提交 Git、失败自动重试、多模板源合并、模板版本管理。
- **保持**：Git 提交手动调用 `apply init-repos`。
- **需要**：按 project 做 apply（terraform 已有；kubernetes/gitops 待支持）、完善错误提示。
- **模板**：当前为单次操作单仓库（`--template-repo`）；不同时机用不同仓库已支持。不支持“一次加载合并多个模板仓库”。

---

## 五、文档与代码同步建议

- **V1.0_STATUS_ANALYSIS.md**：建议更新为“Status 已实现”“Terraform 依赖排序与执行计划已实现”“执行计划与 --dry-run 已实现”，避免与当前实现不一致。
- **IMPLEMENTATION_PLAN.md**：P0/P1 的 Status、Rollback、进度、依赖排序、执行计划可标为已完成；补充“进度恢复”“安装状态持久化”为待办。
- **Roadmap.md**：Phase 1 中与 init/install/status 已实现部分可打勾；未实现项保留为 [ ]。

---

*本文档用于产品与开发对齐当前能力与下一阶段实现顺序。*
