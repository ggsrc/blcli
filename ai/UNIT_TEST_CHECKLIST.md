# 单元测试补充清单

本文档列出了 blcli 项目中需要补充的单元测试。

## 测试状态概览

### ✅ 已完成的测试

1. **CLI 包** (`pkg/cli/`)
   - ✅ `init-args` 命令基础测试（部分）
   - ✅ `init` 命令基础测试（部分）

2. **Bootstrap 包** (`pkg/bootstrap/`)
   - ✅ `ExecuteInit` 基础测试（部分）

3. **Template 包** (`pkg/template/`)
   - ✅ `Loader` 基础测试（部分）

4. **Renderer 包** (`pkg/renderer/`)
   - ✅ `LoadArgs` 和 `MergeArgs` 基础测试（部分）

---

## 📋 需要补充的测试清单

### 1. CLI 包 (`pkg/cli/`) - 高优先级

#### 1.1 `init-args` 命令测试补充
- [ ] **测试场景完善**
  - [ ] 测试从 GitHub 仓库加载模板（需要 mock）
  - [ ] 测试 args.yaml 文件不存在的情况
  - [ ] 测试 config.yaml 解析错误的情况
  - [ ] 测试多个 args.yaml 文件的合并顺序
  - [ ] 测试输出格式验证（yaml/toml）
  - [ ] 测试 force-update 标志
  - [ ] 测试 cache-expiry 标志

#### 1.2 `init` 命令测试补充
- [ ] **测试场景完善**
  - [ ] 测试 terraform 模块初始化完整流程
  - [ ] 测试 kubernetes 模块初始化
  - [ ] 测试 gitops 模块初始化
  - [ ] 测试多个模块同时初始化
  - [ ] 测试 overwrite 标志
  - [ ] 测试 profile 性能分析
  - [ ] 测试错误处理和回滚

#### 1.3 其他 CLI 命令测试
- [ ] **`destroy` 命令** (`destroy.go`)
  - [ ] 测试 terraform 销毁
  - [ ] 测试 kubernetes 销毁
  - [ ] 测试 gitops 销毁
  - [ ] 测试确认提示
  - [ ] 测试错误处理

- [ ] **`plan` 命令** (`plan.go`)
  - [ ] 测试渲染计划生成
  - [ ] 测试计划文件输出
  - [ ] 测试计划差异显示

- [ ] **`check` 命令** (`check.go`)
  - [ ] 测试配置验证
  - [ ] 测试模板验证
  - [ ] 测试参数验证

- [ ] **`version` 命令** (`version.go`)
  - [ ] 测试版本信息输出

---

### 2. Bootstrap 包 (`pkg/bootstrap/`) - 高优先级

#### 2.1 Terraform 子包 (`pkg/bootstrap/terraform/`)
- [ ] **`args.go`**
  - [ ] 测试 PrepareTerraformInitData
  - [ ] 测试 PrepareTerraformProjectData
  - [ ] 测试 PrepareTerraformModuleData

- [ ] **`common.go`**
  - [ ] 测试所有通用辅助函数

- [ ] **`init.go`**
  - [ ] 测试 InitializeInitItems
  - [ ] 测试 init 项渲染
  - [ ] 测试依赖解析

- [ ] **`modules.go`**
  - [ ] 测试 InitializeModules
  - [ ] 测试模块渲染
  - [ ] 测试模块文件列表

- [ ] **`projects.go`**
  - [ ] 测试 InitializeProjects
  - [ ] 测试项目渲染
  - [ ] 测试项目依赖排序

- [ ] **`destroy.go`**
  - [ ] 测试 DestroyTerraform
  - [ ] 测试目录清理

- [ ] **`utils.go`**
  - [ ] 测试所有工具函数

#### 2.2 Kubernetes 包 (`pkg/bootstrap/kubernetes.go`)
- [ ] **BootstrapKubernetes**
  - [ ] 测试 Kubernetes 配置加载
  - [ ] 测试 Kubernetes 资源渲染
  - [ ] 测试文件生成

#### 2.3 GitOps 包 (`pkg/bootstrap/gitops.go`)
- [ ] **BootstrapGitops**
  - [ ] 测试 GitOps 配置加载
  - [ ] 测试 GitOps 模板渲染
  - [ ] 测试文件生成

#### 2.4 Plan 包 (`pkg/bootstrap/plan.go`)
- [ ] **BuildRenderPlan**
  - [ ] 测试渲染计划构建
  - [ ] 测试文件列表生成
  - [ ] 测试依赖解析

#### 2.5 Manager 包补充 (`pkg/bootstrap/manager.go`)
- [ ] **ExecuteInit 完整测试**
  - [ ] 测试所有模块初始化
  - [ ] 测试状态管理
  - [ ] 测试工具检查和安装
  - [ ] 测试性能分析

- [ ] **ExecuteDestroy 完整测试**
  - [ ] 测试所有模块销毁
  - [ ] 测试确认流程

---

### 3. Renderer 包 (`pkg/renderer/`) - 中优先级

#### 3.1 ArgsDef 包 (`pkg/renderer/argsdef.go`)
- [ ] **LoadArgsDefinition**
  - [ ] 测试 YAML 解析
  - [ ] 测试错误处理

- [ ] **ToArgsData**
  - [ ] 测试数据转换

- [ ] **ToConfigValues**
  - [ ] 测试默认值提取
  - [ ] 测试示例值提取
  - [ ] 测试注释生成

#### 3.2 ArgsConfig 包 (`pkg/renderer/argsconfig.go`)
- [ ] **ArgsConfig 结构**
  - [ ] 测试配置解析
  - [ ] 测试配置验证

#### 3.3 Args 包补充 (`pkg/renderer/args.go`)
- [ ] **GetString, GetMap, GetSlice**
  - [ ] 测试各种数据类型获取
  - [ ] 测试默认值处理
  - [ ] 测试类型转换

- [ ] **MergeArgs**
  - [ ] 测试深度合并
  - [ ] 测试数组合并
  - [ ] 测试覆盖顺序

---

### 4. Template 包 (`pkg/template/`) - 中优先级

#### 4.1 Engine 包 (`pkg/template/engine.go`)
- [ ] **RenderTemplate**
  - [ ] 测试模板渲染
  - [ ] 测试变量替换
  - [ ] 测试条件判断
  - [ ] 测试循环处理
  - [ ] 测试函数调用
  - [ ] 测试错误处理

#### 4.2 Cache 包 (`pkg/template/cache.go`)
- [ ] **Cache 管理**
  - [ ] 测试缓存同步
  - [ ] 测试缓存过期
  - [ ] 测试 ETag 处理
  - [ ] 测试本地路径处理
  - [ ] 测试 GitHub API 调用（需要 mock）

#### 4.3 Config 包补充 (`pkg/template/config.go`)
- [ ] **配置解析**
  - [ ] 测试 TerraformConfig 解析
  - [ ] 测试 KubernetesConfig 解析
  - [ ] 测试 GitopsConfig 解析
  - [ ] 测试依赖解析
  - [ ] 测试循环依赖检测

#### 4.4 Loader 包补充 (`pkg/template/loader.go`)
- [ ] **模板加载**
  - [ ] 测试 GitHub 仓库加载（需要 mock）
  - [ ] 测试本地路径加载
  - [ ] 测试分支/标签支持
  - [ ] 测试私有仓库认证
  - [ ] 测试缓存机制

---

### 5. Config 包 (`pkg/config/`) - 中优先级

- [ ] **Load**
  - [ ] 测试 TOML 配置加载
  - [ ] 测试配置验证

- [ ] **LoadFromArgs**
  - [ ] 测试从 args 加载配置
  - [ ] 测试配置合并
  - [ ] 测试默认值设置

- [ ] **WorkspacePath**
  - [ ] 测试工作空间路径计算

---

### 6. State 包 (`pkg/state/`) - 低优先级

- [ ] **Load**
  - [ ] 测试状态加载
  - [ ] 测试状态文件不存在的情况

- [ ] **Save**
  - [ ] 测试状态保存

- [ ] **RecordTerraformProject**
  - [ ] 测试项目记录

- [ ] **RecordKubernetesProject**
  - [ ] 测试 Kubernetes 项目记录

- [ ] **RecordGitopsProject**
  - [ ] 测试 GitOps 项目记录

---

### 7. Internal 包 (`pkg/internal/`) - 低优先级

#### 7.1 FS 包 (`pkg/internal/fs.go`)
- [ ] **文件系统操作**
  - [ ] 测试目录创建
  - [ ] 测试文件写入
  - [ ] 测试 blcli marker 管理

#### 7.2 Tools 包 (`pkg/internal/tools.go`)
- [ ] **工具管理**
  - [ ] 测试工具检查
  - [ ] 测试工具安装（需要 mock）

#### 7.3 Perf 包 (`pkg/internal/perf.go`)
- [ ] **性能分析**
  - [ ] 测试性能分析器
  - [ ] 测试时间统计

#### 7.4 ToolConfig 包 (`pkg/internal/toolconfig.go`)
- [ ] **工具配置**
  - [ ] 测试配置加载

---

## 测试优先级说明

### 高优先级（核心功能）
- CLI 命令测试（init, init-args, destroy）
- Bootstrap terraform 包测试
- Template engine 测试

### 中优先级（重要功能）
- Renderer 包完整测试
- Template cache 和 loader 测试
- Config 包测试

### 低优先级（辅助功能）
- State 包测试
- Internal 工具函数测试

---

## 测试实施建议

### 1. Mock 和测试工具
- 使用 ginkgo/gomega 进行测试
- 对于外部依赖（GitHub API、GCS、Kubernetes），使用 mock 或 fake 服务
- 参考 `TESTING_PLAN.md` 中的 fake-gcs-server 和 client-go fake 计划

### 2. 测试数据
- 所有测试数据放在 `workspace/` 目录
- 使用本地模板仓库避免网络依赖
- 测试完成后清理临时文件

### 3. 测试覆盖率目标
- 核心功能（CLI、Bootstrap）：80%+
- 重要功能（Renderer、Template）：70%+
- 辅助功能（State、Internal）：60%+

### 4. 测试组织
- 使用 ginkgo 的 Describe/Context/It 结构
- 每个测试文件对应一个源文件
- 测试文件命名：`*_test.go`

---

## 当前测试运行状态

运行 `go test ./...` 应该能够：
- ✅ 编译所有测试文件
- ✅ 运行所有已实现的测试
- ⚠️ 部分测试可能因为路径或依赖问题失败（需要修复）

---

## 下一步行动

1. **修复现有测试问题**
   - 修复路径问题
   - 修复依赖问题
   - 确保所有测试能够运行

2. **补充高优先级测试**
   - 完善 CLI 命令测试
   - 实现 Bootstrap terraform 测试
   - 实现 Template engine 测试

3. **补充中优先级测试**
   - 完善 Renderer 包测试
   - 完善 Template 包测试
   - 实现 Config 包测试

4. **补充低优先级测试**
   - 实现 State 包测试
   - 实现 Internal 包测试

---

## 相关文档

- [TESTING_PLAN.md](TESTING_PLAN.md) - 未来测试计划（fake-gcs-server、client-go fake）
- [README.md](../README.md) - 项目文档
- [USAGE.md](../docs/USAGE.md) - 使用文档

