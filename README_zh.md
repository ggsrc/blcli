<div align="center">

# blcli

**一份配置，走完云平台全链路。**

*One config. Full cloud platform lifecycle.*

用一份 `args.yaml` 和自描述模板仓，串联 Terraform、Kubernetes 与 GitOps。**当前深度支持 GCP**，架构面向多云。

[![GitHub stars](https://img.shields.io/github/stars/ggsrc/blcli?style=flat-square)](https://github.com/ggsrc/blcli/stargazers)
[![GitHub release](https://img.shields.io/github/v/release/ggsrc/blcli?style=flat-square)](https://github.com/ggsrc/blcli/releases)
[![License](https://img.shields.io/github/license/ggsrc/blcli?style=flat-square)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.21%2B-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![Template](https://img.shields.io/badge/template-bl--template-orange?style=flat-square)](https://github.com/ggsrc/bl-template)

[快速开始](README.md#quick-start-5-minutes) · [完整英文文档](README.md) · [个人版模板](https://github.com/ggsrc/bl-template-personal)

</div>

<!-- ADOPTION:START -->
**采用情况（2026-07-20）：** 11 次 Release 下载 · 4 GitHub Stars · 最新版本 `v0.1.3` · [Releases](https://github.com/ggsrc/blcli/releases)
<!-- ADOPTION:END -->

---

## blcli 是什么？

`blcli` 用**一个 `args.yaml`** 和**可替换的自描述模板仓库**，把 GCP 平台从落地到运维串成一条链路：Terraform 建项目与网络、Kubernetes 装平台组件、GitOps 同步应用。

```
args.yaml  +  bl-template  →  blcli  →  GCP 项目 + GKE 集群 + ArgoCD 应用
```

常见做法是 **Terraform 蓝图 + 安装脚本 + ArgoCD 清单** 自己拼。blcli 用统一 CLI 和模板协议（`config.yaml` + `args.yaml`）替代这层胶水，模板可 fork，CLI 不用改。

---

## 适合谁？

| 用户 | 模板 | 典型场景 |
|------|------|----------|
| 平台 / SRE 团队 | [bl-template](https://github.com/ggsrc/bl-template) | 多项目多环境（corp / stg / prd）、ArgoCD、完整 GitOps |
| 个人开发者 / AI 辅助开发 | [bl-template-personal](https://github.com/ggsrc/bl-template-personal) | 单项目单集群、`minimal` / `full` profile、低配置负担 |

---

## 为什么选择 blcli？

- **全生命周期，不只生成代码** — `init` 出 IaC 与清单；`apply`、`status`、`rollback` 覆盖部署与运维。
- **自描述模板** — 组件、参数、依赖都在模板仓里；`init-args`、`explain` 让人和 Agent 对齐同一套结构。
- **有序、可计划的变更** — Terraform / Kubernetes 按依赖排序；执行前有 plan，支持 `--dry-run`。
- **一份配置、分层继承** — `global → terraform → project → component`，环境差异集中在 `args.yaml`。
- **Agent 友好** — `contract`、`diagnose`、`runs` 提供稳定 schema 与失败修复提示。

| 没有 blcli | 有 blcli |
|---|---|
| 每个环境手工复制 Terraform | 一套模板 → N 个环境 |
| 手工编排 K8s 组件安装顺序 | 按 config 依赖自动排序 |
| GitOps 清单单独维护 | `blcli apply gitops` 一条命令 |
| 没有标准销毁路径 | `destroy` 双重确认 + 全链路回收 |

---

## 和常见方案怎么比？

| 方案 | 擅长 | blcli 多出来的 |
|------|------|----------------|
| [CFT](https://github.com/GoogleCloudPlatform/cloud-foundation-toolkit) / [Fabric FAST](https://github.com/GoogleCloudPlatform/cloud-foundation-fabric) | 企业级 GCP Landing Zone（Terraform） | 同一 CLI 编排 Kubernetes + GitOps |
| [Kubestack](https://www.kubestack.com/) / `kbst` | 以 Terraform 为中心的 K8s 平台 | 外部模板仓 + args 驱动，fork 更轻 |
| [gke-tf](https://github.com/GoogleCloudPlatform/gke-terraform-generator) | YAML 生成 GKE Terraform | 全栈 + `apply` / `status` / `rollback` |
| 自建脚本 | 完全自控 | 约定、文档、依赖顺序、安全护栏开箱即用 |

blcli 的定位是 **「GCP 平台从 0 到可运行」的编排层**，不替代 Terraform、kubectl 或 Argo CD。

---

## 谁在用？

我们正在收集早期用户案例。若你的团队或实验环境在用 blcli，请 [提 PR 编辑 README.md](https://github.com/ggsrc/blcli/edit/main/README.md) 在下方列表补充你的组织（logo 可选）；列表会自动同步到本页。

<!-- ADOPTERS:START -->
<!-- Example:
- [Your Org](https://example.com) — multi-env GCP platform
-->
<!-- ADOPTERS:END -->

---

## 文档导航

| 文档 | 说明 |
|------|------|
| [README.md](README.md) | 完整英文文档（安装、命令、配置、排障） |
| [docs/zh/](docs/zh/) | 中文使用说明与 Roadmap |
| [docs/Roadmap.md](docs/Roadmap.md) | 版本规划 |

**常用命令速览：**

```bash
blcli init-args -r github.com/ggsrc/bl-template -o args.yaml
blcli init -r github.com/ggsrc/bl-template -a args.yaml
blcli apply all -a args.yaml
blcli status all -a args.yaml
```

个人 / 轻量场景将 `-r` 换为 [bl-template-personal](https://github.com/ggsrc/bl-template-personal) 即可。
