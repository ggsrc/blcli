# blcli 功能盘点与 v1 / v1.5 范围

> 更新日期：2026-06-25  
> 基于 **feat/v1.5-ruipeng**（main + dev/v2.0-feature 合并后）

## v1 发布标准（修订后）

**v1 = Phase 1 核心闭环（GCP-first）+ 文档与代码一致 + 两项可靠性体验（Resume、失败指引）**

- **GCP-first today**：首个完整实现为 GCP；品牌 slogan 面向云平台，多云在 v2。
- **不包含 Phase 2 体验项**：完整 bootstrap 会话、配置模板市场、操作时间估算、独立审计系统等。
- **`install` 已迁移为 `apply`**：不再作为缺口跟踪。

---

## 一、已实现功能 ✅

### 核心命令

| 命令 | 状态 | 说明 |
|------|------|------|
| **init** | ✅ | terraform/kubernetes/gitops；模板加载、args 合并、校验、进度、**`--preview`** |
| **init-args** | ✅ | 从模板仓生成 args；`--profile` overlay；**`--wizard` / `--preview`** |
| **destroy** | ✅ | 销毁已初始化目录；terraform destroy 带双重确认 |
| **check** | ✅ | plugin / repo / kubernetes / **args** |
| **apply terraform/kubernetes/gitops/all** | ✅ | 依赖排序、执行计划、--dry-run、`--project`（三模块均支持） |
| **apply init-repos** | ✅ | git init + GitHub 仓库创建与推送 |
| **status** | ✅ | terraform/kubernetes/gitops；`--format=table|json|yaml` |
| **rollback** | ✅ | 按 config Rollback 配置；支持 `--project` |
| **explain** | ✅ | 解释模板组件与参数 |
| **contract** | ✅ | 输出 Agent 工具契约，支持 json/yaml/table |
| **diagnose** | ✅ | 失败分类与 next steps / repair commands |
| **runs** | ✅ | 查询 `~/.blcli/progress` 运行记录 |
| **version** | ✅ | 版本与构建时间 |

### 支撑能力

| 能力 | 状态 | 说明 |
|------|------|------|
| Terraform/K8s 依赖排序 | ✅ | DFS 拓扑排序 |
| 执行计划与 --dry-run | ✅ | 各 apply 子命令 |
| 进度持久化 | ✅ | `~/.blcli/progress/{operation-id}.yaml` |
| **中断续跑 Resume** | ✅ | `init` / `apply all` 检测未完成操作并提示恢复；`--no-resume` 跳过 |
| **失败修复指引** | ✅ | `PrintFailureHints` + `agent.DiagnoseFailure` 双层提示 |
| **机器可读 step log** | ✅ | ProgressTracker 记录子命令与输出摘要 |
| init 参数校验 | ✅ | `validator.Run` 在 init 写文件前执行；`check args` 可单独校验 |
| 模板加载 | ✅ | GitHub/本地、缓存、私有仓库 |
| **Agent 工具契约** | ✅ | `blcli contract` |
| **失败注入场景** | ✅ | `integration/fixtures/failures` 离线样本 |
| **CI 集成** | ✅ | `.github/actions/blcli` composite action + `docs/zh/CI.md` |

---

## 二、v1 / v1.5 明确不做 ❌

| 项 | 说明 |
|----|------|
| 并行初始化 | 产品约定 |
| init 后自动 Git 提交 | 使用 `apply init-repos` |
| 失败自动重试 | 产品约定 |
| 多模板源合并 | 产品约定 |
| 模板版本锁定 | 留给 v2 依赖管理 |
| apply 失败自动 rollback | 使用独立 `blcli rollback` |
| **`blcli bootstrap` 交互会话** | Roadmap 标注 **暂不实现** |
| **Resume 细粒度（terraform project 级）** | v1.5 明确排除（D3） |
| 配置模板库 / 模板市场 | Phase 2.3 / v2 生态 |
| 操作时间估算 | 非 v1 |
| 独立审计日志系统 | v3 范畴 |
| 多云模板与引擎 | v2（当前 GCP-first） |

---

## 三、v1.5 交付清单（除 D3 外）

| 编号 | 项 | 状态 |
|------|-----|------|
| A1 | `init-args --profile` | ✅ |
| A2 | `init-args --wizard` | ✅ |
| A3 | 配置预览 `--preview` | ✅ |
| B1/B2/B3 | contract / diagnose / fixtures | ✅ |
| C1/C2 | GitHub Action + CI 文档 | ✅ |
| D1 | `blcli runs` | ✅ |
| D2 | 扩展失败 hints（agent 整合） | ✅ |
| D4 | `check args` | ✅ |
| D5 | CHANGELOG | ✅ |
| **D3** | Resume 细粒度 | ❌ 不做 |
| T1/T2 | 模板文档 + CI 样板 | ✅（bl-template workflow） |

---

## 四、v2 预览

| 版本 | 重点 |
|------|------|
| **v2** | `workflow`；`--env` 环境抽象；第二云模板；`monitor`；插件 |

---

*本文档用于产品与开发对齐 v1 / v1.5 范围与下一阶段顺序。*
