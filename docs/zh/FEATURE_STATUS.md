# blcli 功能盘点与版本范围

> 更新日期：2026-06-26  
> 基于 **main**（v1.0 + v2.0 已合入）

## 版本策略（修订）

| 版本 | 含义 | 状态 |
|------|------|------|
| **v1.0** | GCP-first Phase 1 闭环 + Resume + 失败指引 | ✅ |
| **v2.0** | 向导/预览、Agent 工具、CI（原规划 v1.5） | ✅ |
| **v3.0** | workflow、`--env`、第二云、monitor（原规划 v2.0） | 📋 未开始 |
| **v4.0** | 服务端 + Web UI（原 v3.0） | 🔮 |
| **v5.0** | AI 与深度自动化（原 v4.0） | 🔮 |

**Slogan：** 一份配置，走完云平台全链路。 / One config. Full cloud platform lifecycle.

---

## 一、已实现（v1.0 + v2.0）✅

### 核心命令

| 命令 | 版本 | 说明 |
|------|------|------|
| **init** | v1 + v2 | 模板渲染；v2：`--preview` |
| **init-args** | v1 + v2 | 生成 args；v2：`--profile`、`--wizard`、`--preview` |
| **destroy** | v1 | 全链路销毁 |
| **check** | v1 + v2 | plugin / repo / kubernetes；v2：**args** |
| **apply** * | v1 | terraform/kubernetes/gitops/all；`--project`、`--dry-run` |
| **apply init-repos** | v1 | Git 仓库初始化与推送 |
| **status** | v1 | `--format=table|json|yaml` |
| **rollback** | v1 | 按 config 回滚 |
| **explain** | v1 | 组件与参数说明 |
| **contract** | v2 | Agent 工具契约 |
| **diagnose** | v2 | 失败分类与修复建议 |
| **runs** | v2 | progress 运行记录查询 |
| **version** | v1 | 版本信息 |

### 支撑能力

| 能力 | 版本 |
|------|------|
| 依赖排序、执行计划、--dry-run | v1 |
| 进度持久化、Resume（module 级） | v1 |
| 失败 hints + agent 诊断 | v1 + v2 |
| step log、progress 子命令记录 | v2 |
| init 前校验 + `check args` | v1 + v2 |
| GitHub Action + CI 文档 | v2 |
| failure fixtures | v2 |

---

## 二、明确不做（跨版本产品约定）❌

| 项 | 说明 |
|----|------|
| 并行 init | 产品约定 |
| init 后自动 Git 提交 | 用 `apply init-repos` |
| apply 失败自动 rollback | 用 `rollback` |
| 失败步骤自动重试 | 产品约定（workflow 可显式定义重试） |
| 多模板源一次合并 | 产品约定 |
| `blcli bootstrap` 全 session | 弱需求，与 wizard 重叠 |
| Resume 细粒度（project 级） | v2 已明确排除 |

---

## 三、v2.0 交付清单（原 v1.5，已完成）

| 项 | 状态 |
|----|------|
| `init-args --profile` | ✅ |
| `init-args --wizard` / `--preview` | ✅ |
| `init --preview` | ✅ |
| `check args` | ✅ |
| `contract` / `diagnose` / `runs` | ✅ |
| failure fixtures | ✅ |
| GitHub Action + `docs/zh/CI.md` | ✅ |
| CHANGELOG | ✅ |
| 模板 CI 样板（bl-template） | ⚠️ 待单独 PR |

---

## 四、v3.0 缺口预览（原 v2.0）

按 slogan 优先级，详见 `docs/zh/Roadmap.md`。

| 优先级 | 能力 | 状态 |
|--------|------|------|
| **P0** | `--env` 环境抽象 | ❌ |
| **P0** | `blcli workflow` | ❌ |
| **P0** | 第二云官方模板 | ❌ |
| **P0** | `monitor` 观测增强 | ❌ |
| **P0** | 环境状态快照 | ❌ |
| P1 | 环境 diff/promote、workflow 模板、CI 扩展 | ❌ |
| P2 | 模板市场、插件市场、依赖可视化 | ❌ |

---

## 五、发布待办

- [ ] `v1.0.0` / `v2.0.0` Git tag 与 Release
- [ ] 文档与 main 完全同步

---

*功能盘点用于产品与开发对齐；路线图与优先级以 `docs/zh/Roadmap.md` 为准。*
