# blcli 功能盘点与 v1 范围

> 更新日期：2026-06-25  
> 基于当前 **main 分支代码** 与修订后的 v1 发布标准

## v1 发布标准（修订后）

**v1 = Phase 1 核心闭环（GCP-first）+ 文档与代码一致 + 两项可靠性体验（Resume、失败指引）**

- **GCP-first today**：首个完整实现为 GCP；品牌 slogan 面向云平台，多云在 v2。
- **不包含 Phase 2 体验项**：交互式 bootstrap 向导、配置模板市场、操作时间估算、独立审计系统等。
- **`install` 已迁移为 `apply`**：不再作为缺口跟踪。

---

## 一、已实现功能 ✅

### 核心命令

| 命令 | 状态 | 说明 |
|------|------|------|
| **init** | ✅ | terraform/kubernetes/gitops；模板加载、args 合并、校验、进度 |
| **init-args** | ✅ | 从模板仓生成 args；支持 `--profile` overlay（personal 模板） |
| **destroy** | ✅ | 销毁已初始化目录；terraform destroy 带双重确认 |
| **check** | ✅ | plugin / repo / kubernetes |
| **apply terraform/kubernetes/gitops/all** | ✅ | 依赖排序、执行计划、--dry-run、`--project`（三模块均支持） |
| **apply init-repos** | ✅ | git init + GitHub 仓库创建与推送 |
| **status** | ✅ | terraform/kubernetes/gitops；`--format=table|json|yaml` |
| **rollback** | ✅ | 按 config Rollback 配置；支持 `--project` |
| **explain** | ✅ | 解释模板组件与参数 |
| **version** | ✅ | 版本与构建时间 |

### 支撑能力

| 能力 | 状态 | 说明 |
|------|------|------|
| Terraform/K8s 依赖排序 | ✅ | DFS 拓扑排序 |
| 执行计划与 --dry-run | ✅ | 各 apply 子命令 |
| 进度持久化 | ✅ | `~/.blcli/progress/{operation-id}.yaml` |
| **中断续跑 Resume** | ✅ | `init` / `apply all` 检测未完成操作并提示恢复；`--no-resume` 跳过 |
| **失败修复指引** | ✅ | 常见错误模式输出 next steps（`PrintFailureHints`） |
| init 参数校验 | ✅ | `validator.Run` 在 init 写文件前执行 |
| 模板加载 | ✅ | GitHub/本地、缓存、私有仓库 |

---

## 二、v1 明确不做 ❌

| 项 | 说明 |
|----|------|
| 并行初始化 | 产品约定 |
| init 后自动 Git 提交 | 使用 `apply init-repos` |
| 失败自动重试 | 产品约定 |
| 多模板源合并 | 产品约定 |
| 模板版本锁定 | 留给 v2 依赖管理 |
| apply 失败自动 rollback | 使用独立 `blcli rollback` |
| **`blcli bootstrap` 交互会话** | Roadmap 标注 **暂不实现**（v1.5+） |
| 配置模板库 / 模板市场 | Phase 2.3 / v2 生态 |
| 操作时间估算 | 非 v1 |
| 独立审计日志系统 | v3 范畴；progress 文件已够用 |
| **`contract` / `diagnose` / `runs` 命令** | **未在 main 实现**；若需要单独立项 v1.5 Agent 专项 |
| 多云模板与引擎 | v2（当前 GCP-first） |

---

## 三、可选增强（v1.0.x，不挡发布）

| 项 | 说明 |
|----|------|
| 安装状态长期持久化 | 与 v2 环境快照一并设计 |
| 更丰富的 diagnose 规则库 | 在现有 `PrintFailureHints` 上增量扩展 |
| 官方 GitHub Action 示例 | 推广向，非核心 |
| progress 查询子命令 | 可读 `~/.blcli/progress/` 文档说明代替 |

---

## 四、文档纠错记录（2026-06-25）

以下条目曾误标为「欠账」，已与代码对齐：

| 原表述 | 更正 |
|--------|------|
| k8s/gitops 缺 `--project` | ✅ 已实现 |
| status 缺 JSON/YAML | ✅ 已实现 |
| contract/diagnose/runs 已完成 | ❌ main 无对应命令 |
| v1 完成度 ~75% | 修订为 **~95%**（GCP 闭环 + Resume + 失败指引） |
| Phase 2 项算 v1 缺口 | 已移出 v1 范围 |

---

## 五、v1.5 / v2 预览

| 版本 | 重点 |
|------|------|
| **v1.5** | 轻量 `init --wizard`；Agent 工具（contract/diagnose）；CI Action |
| **v2** | `workflow`；`--env` 环境抽象；第二云模板；`monitor`；插件 |

---

*本文档用于产品与开发对齐 v1 范围与下一阶段顺序。*
