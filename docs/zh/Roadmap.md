# blcli Roadmap

## 项目愿景

`blcli` 旨在成为**云平台基础设施**的一站式 CLI：用一份 `args.yaml` 和自描述模板仓，串联 Terraform、Kubernetes 与 GitOps 的生成、部署、观测与回收。

**品牌 slogan：** 一份配置，走完云平台全链路。 / One config. Full cloud platform lifecycle.

**版本策略：**
- **v1（GCP-first）**：Phase 1 核心闭环 + Resume + 失败指引；首个完整实现为 GCP。
- **v1.5**：轻量向导、Agent 工具、CI 示例。
- **v2**：workflow、环境抽象、第二云模板、monitor、插件。

详见 `docs/zh/FEATURE_STATUS.md`、`docs/zh/V1.0_STATUS_ANALYSIS.md`。

## 当前状态（v1 候选）

### 已实现（Phase 1）
- ✅ **init / init-args / apply / status / rollback / check / destroy / explain**
- ✅ **apply**：terraform、kubernetes、gitops、all；依赖排序、执行计划、--dry-run、**三模块 `--project`**
- ✅ **status**：`--format=table|json|yaml`
- ✅ **进度持久化与 Resume**：`init`、`apply all` 可续跑未完成操作（`--no-resume` 跳过）
- ✅ **失败修复指引**：常见错误输出 next steps
- ✅ **模板系统**：GitHub/本地、缓存、单次单仓库、init 前 args 校验

### v1 明确不做
并行 init、自动 Git 提交、失败重试、多模板合并、模板版本锁、apply 失败自动 rollback、`blcli bootstrap` 会话（见 Phase 2）、多云实现（见 v2）。

## 短期路线图 (v1.0)

### Phase 1: 核心功能完善

#### 1.1 增强 `init` 命令
- [x] **一键初始化所有 repo**（已实现 `blcli init`）
  - 不要求：并行初始化多个项目
  - [x] 智能依赖检测和顺序执行（Terraform/Kubernetes 按 config 依赖排序）
  - [x] 初始化进度显示（ProgressTracker，持久化到 ~/.blcli/progress/）
  - [x] **中断续跑 Resume**（检测未完成 init，跳过已完成 module；`--no-resume`）
  - [x] 初始化后提交到 Git 仓库（手动调用 `blcli apply init-repos`，不计划自动）

#### 1.2 部署命令（Roadmap 原 `install`，已实现为 `apply`）
- [x] **一键部署所有组件**（`blcli apply terraform/kubernetes/gitops/all`）
  - [x] Terraform 模块部署（含依赖排序、执行计划、--dry-run）
  - [x] Kubernetes 资源部署（含依赖排序、执行计划、--dry-run）
  - [x] GitOps 配置同步（含执行计划、--dry-run）
  - 不要求：分批安装；回滚已实现为 `blcli rollback`
  - [x] **按 project 执行**：terraform、kubernetes、gitops 均已支持 `--project`
  - [x] 安装前依赖检查（按 config 依赖排序执行）
  - [x] **apply all 中断续跑**（按 module 跳过已完成项）
  - [ ] 安装状态长期持久化（**v1 可选 / v2 环境快照**；当次 progress 已有）

#### 1.3 `status` 命令
- [x] **检查各组件安装情况**（已实现 `blcli status [terraform|kubernetes|gitops|all]`）
  - [x] Terraform 状态检查（`terraform show`）
  - [x] Kubernetes 资源状态（`kubectl get`）
  - [x] GitOps 同步状态
  - [x] 健康检查汇总报告
  - [x] **JSON/YAML 输出**（`--format=table|json|yaml`）

#### 1.4 增强模板系统
- 不要求：模板版本管理、多模板源合并
- [x] **单次操作单仓库**：每次通过 `--template-repo` 指定一个仓库；不同命令/不同时机可用不同仓库
  - 不支持一次加载合并多个模板仓库（无此需求）

### Phase 2: 用户体验优化（**不属于 v1 发布门槛**，见 v1.5）

#### 2.1 交互式 CLI
- [ ] **交互式配置向导**
  - `blcli init` 交互式引导
  - 参数验证和提示
  - 配置预览和确认
- [ ] **一站式 Bootstrap 交互会话（`blcli bootstrap`）**（规划中，暂不实现）
  - **形态**：键入 `blcli bootstrap` 后进入持久化交互终端 session，状态文件保存在本地磁盘（如 `~/.blcli/bootstrap/` 或 workspace 内），一次完成 init-args → init → apply 的连贯流程。
  - **目标**：用户关注的核心参数尽量少，通过向导式交互与智能默认值降低心智负担，获得流畅的「开箱即用」体验。
  - **设计要点**：
    - **状态持久化**：当前进度、已填参数、已执行步骤等可序列化到本地，支持中断后恢复、多次进入同一 session 继续。
    - **步骤编排**：按 init-args（生成/合并 args）→ init（拉模板、渲染）→ apply（terraform/k8s/gitops）顺序引导，每步可确认或跳过（若状态显示已完成）。
    - **参数渐进披露**：仅必填项在首屏询问，其余用模板默认或后续步骤按需提示；与现有 `init-args` 生成的 args 结构兼容，便于与非交互流程共用同一套配置。
    - **与现有命令关系**：bootstrap 为「向导式入口」，底层仍可调用现有 init-args/init/apply；非交互场景继续使用现有子命令与 args 文件。
  - **可选扩展**：session 与 workspace 绑定、多环境选择（dev/stg/prd）、一键回滚入口等。
- [x] **进度显示**（已实现）
  - [x] 实时进度条（init / apply all 使用 ProgressTracker）
  - [x] 详细日志输出
  - [ ] 操作时间估算

#### 2.2 错误处理
- [x] **回滚机制**（独立 `blcli rollback`，按 config 执行；非 apply 失败自动触发）
- [x] **失败修复指引（v1 粒度）**：常见错误输出 next steps（`PrintFailureHints`）
- 不要求：失败步骤自动重试
- [ ] **完整 diagnose / 审计子系统**（v1.5+ Agent 专项；非 v1 阻塞项）
- [ ] **操作日志 / 审计追踪**（v3 平台化；v1 用 progress 文件）

#### 2.3 配置管理增强（v2 生态）
- [x] **init 前参数校验**（`validator.Run` + args 自描述）
- [ ] 配置最佳实践建议、配置模板库、模板市场

## 中期路线图 (v2.0)

### Phase 3: 高级功能

#### 3.1 工作流管理
- [ ] **`blcli workflow` 命令**
  - 定义和执行复杂工作流
  - 工作流模板和复用
  - 条件执行和分支逻辑
  - 工作流编排（类似 GitHub Actions）

#### 3.2 多环境管理
- [ ] **环境抽象**
  - 环境配置管理（dev/staging/prod）
  - 环境间配置差异管理
  - 环境同步和迁移
- [ ] **环境隔离**
  - 环境级别的权限控制
  - 环境状态快照
  - 环境回滚

#### 3.3 依赖管理
- [ ] **智能依赖解析**
  - 自动检测组件依赖关系
  - 依赖图可视化
  - 依赖更新影响分析
- [ ] **依赖锁定**
  - 依赖版本锁定文件
  - 依赖更新策略配置

#### 3.4 监控和告警
- [ ] **`blcli monitor` 命令**
  - 基础设施健康监控
  - 资源使用情况追踪
  - 成本监控和优化建议
  - 告警规则配置

### Phase 4: 集成和扩展

#### 4.1 CI/CD 集成
- [ ] **CI/CD 插件**
  - GitHub Actions 集成
  - GitLab CI 集成
  - Jenkins 插件
  - 通用 webhook 支持

#### 4.2 多云支持
- [ ] **多云抽象层**
  - AWS 支持
  - Azure 支持
  - 阿里云支持
  - 多云资源统一管理

#### 4.3 插件系统
- [ ] **插件架构**
  - 插件开发 SDK
  - 插件市场和分发
  - 社区插件支持
  - 自定义命令扩展

## 长期路线图 (v3.0+)

### Phase 5: C-S 架构和 Web UI

#### 5.1 服务端架构
- [ ] **blcli-server**
  - RESTful API 服务
  - gRPC 接口
  - WebSocket 实时通信
  - 多租户支持
  - 权限和认证系统

#### 5.2 Web 控制台
- [ ] **可视化操作界面**
  - 项目仪表板
  - 基础设施拓扑图
  - 实时状态监控
  - 操作历史查看
  - 配置编辑器
- [ ] **协作功能**
  - 团队管理
  - 操作审批流程
  - 变更通知
  - 审计日志

#### 5.3 移动端支持
- [ ] **移动应用**
  - iOS/Android 应用
  - 推送通知
  - 快速操作（紧急回滚等）
  - 移动端状态查看

### Phase 6: AI 和自动化

#### 6.1 智能推荐
- [ ] **AI 辅助**
  - 配置优化建议
  - 成本优化推荐
  - 安全最佳实践建议
  - 异常检测和预警

#### 6.2 自动化运维
- [ ] **自愈能力**
  - 自动故障恢复
  - 自动扩缩容
  - 自动备份和恢复
  - 智能资源调度

## 技术债务和优化

### 代码质量
- [ ] 提高测试覆盖率（目标 >80%）
- [ ] 性能优化（并行处理、缓存）
- [ ] 代码重构和模块化
- [ ] 文档完善（API 文档、用户指南）

### 可维护性
- [ ] 日志系统标准化
- [ ] 错误处理统一化
- [ ] 配置管理规范化
- [ ] 向后兼容性保证

## 社区和生态

### 社区建设
- [ ] 开源社区运营
- [ ] 用户案例收集
- [ ] 最佳实践文档
- [ ] 视频教程和培训

### 生态扩展
- [ ] 模板市场
- [ ] 插件市场
- [ ] 集成合作伙伴
- [ ] 认证和培训计划

## 优先级建议（修订）

### v1 发布（已完成或收尾）
1. Phase 1 命令闭环 ✅
2. Resume + 失败指引 ✅
3. 文档与代码一致 ✅

### v1.5
1. 轻量 `init --wizard` / 配置预览
2. Agent 工具（`contract`、`diagnose`）最小集
3. 官方 GitHub Action 示例

### v2（见中期路线图）
1. `blcli workflow`
2. 环境抽象 `--env`
3. 第二云官方模板 + 引擎抽象
4. `blcli monitor`、插件

## 里程碑

- **v1.0** (2026): GCP-first 核心闭环（init/apply/status/rollback）+ Resume + 失败指引
- **v1.5** (2026): 向导与 Agent/CI 增强
- **v2.0** (2026): workflow、多环境、第二云、monitor
- **v3.0** (2026+): C-S 架构和 Web UI
- **v4.0** (2026+): AI 和自动化运维

## 反馈和贡献

欢迎提出建议和贡献代码！请通过以下方式参与：
- GitHub Issues: 报告问题和建议
- GitHub Discussions: 讨论功能和设计
- Pull Requests: 贡献代码

---

*最后更新：2026-06-25（修订 v1 范围、Resume、失败指引、文档与代码对齐）*
