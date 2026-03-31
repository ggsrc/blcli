# 测试计划文档

本文档描述了 blcli 项目的未来测试计划和改进方向。

## 当前状态

目前已经完成了基础的单元测试框架搭建，使用 ginkgo 和 gomega 框架，覆盖了以下功能：

- `blcli init-args` 命令的测试
- `blcli init` 命令的测试
- 模板加载和配置解析的测试
- Args 文件加载和合并的测试

## 未来计划

### 1. Install 改为 Apply

**目标**: 将命令中的 `install` 部分改为 `apply`，以更好地反映实际操作。

**影响范围**:
- `config.yaml` 中的 `install` 字段改为 `apply`
- 相关代码中的变量名和函数名需要更新
- 文档和注释需要更新

**实施步骤**:
1. 更新 `template/config.go` 中的结构体定义
2. 更新所有使用 `install` 字段的代码
3. 更新模板仓库中的 `config.yaml` 文件
4. 更新相关文档

### 2. Terraform 测试 - 使用 fake-gcs-server

**目标**: 使用 [fake-gcs-server](https://github.com/fsouza/fake-gcs-server) 来模拟完整的 GCS 服务，实现 terraform 相关的单元测试。

**背景**: 
Terraform 的 backend 配置通常使用 GCS (Google Cloud Storage) 来存储状态文件。为了在单元测试中测试 terraform 相关功能，需要一个模拟的 GCS 服务。

**实施步骤**:

1. **添加依赖**:
   ```bash
   go get github.com/fsouza/fake-gcs-server/fakestorage
   ```

2. **创建测试辅助函数**:
   - 在测试中启动 fake-gcs-server
   - 配置 Terraform backend 指向 fake-gcs-server
   - 在测试结束后关闭 fake-gcs-server

3. **测试场景**:
   - Terraform backend 初始化
   - Terraform 状态文件读写
   - Terraform 模块渲染和验证
   - Terraform 项目初始化

4. **示例代码结构**:
   ```go
   func setupFakeGCS() (*fakestorage.Server, error) {
       server, err := fakestorage.NewServer(fakestorage.Options{
           InitialObjects: []fakestorage.Object{},
       })
       return server, err
   }
   ```

**参考资源**:
- [fake-gcs-server GitHub](https://github.com/fsouza/fake-gcs-server)
- [fake-gcs-server 文档](https://github.com/fsouza/fake-gcs-server#usage)

### 3. Kubernetes 测试 - 使用 client-go fake

**目标**: 使用 client-go 的 fake 功能来模拟 Kubernetes API，实现 kubernetes 相关的单元测试。

**背景**:
Kubernetes 相关的操作需要与 Kubernetes API 交互。使用 client-go 的 fake 客户端可以在不连接真实集群的情况下进行测试。

**实施步骤**:

1. **添加依赖**:
   ```bash
   go get k8s.io/client-go/kubernetes/fake
   go get k8s.io/client-go/kubernetes
   ```

2. **创建测试辅助函数**:
   - 创建 fake Kubernetes 客户端
   - 设置测试所需的初始资源
   - 验证 Kubernetes 资源的创建和更新

3. **测试场景**:
   - Kubernetes 配置渲染
   - Namespace 创建
   - Deployment 创建和更新
   - ConfigMap 和 Secret 管理
   - Service 创建

4. **示例代码结构**:
   ```go
   func setupFakeK8sClient() *fake.Clientset {
       return fake.NewSimpleClientset()
   }
   ```

**参考资源**:
- [client-go fake 文档](https://pkg.go.dev/k8s.io/client-go/kubernetes/fake)
- [Kubernetes 测试最佳实践](https://kubernetes.io/docs/concepts/cluster-administration/testing/)

## 测试覆盖目标

### 当前覆盖
- ✅ CLI 命令测试（init-args, init）
- ✅ 模板加载测试
- ✅ Args 文件处理测试
- ✅ 配置解析测试

### 待实现
- ⏳ Terraform backend 操作测试
- ⏳ Terraform 模块渲染测试
- ⏳ Kubernetes 资源创建测试
- ⏳ GitOps 配置测试
- ⏳ 错误处理和边界情况测试
- ⏳ 集成测试

## 测试规范

### 测试文件组织
- 测试文件使用 `_test.go` 后缀
- 测试文件与源文件在同一包中（使用 `package_test` 或相同包名）
- 使用 ginkgo 的 `Describe`、`Context`、`It` 结构组织测试

### 测试数据管理
- 所有测试过程中产生的临时文件统一放在 `workspace/` 目录
- 测试完成后及时清理临时文件
- 使用绝对路径（根据项目规范）

### 测试辅助函数
- 测试辅助函数放在测试文件中或单独的 `test_helpers.go` 文件中
- 辅助函数应该可复用，避免重复代码

## 注意事项

1. **网络依赖**: 测试应该尽可能避免网络依赖，使用本地模板仓库和模拟服务
2. **测试隔离**: 每个测试应该独立，不依赖其他测试的状态
3. **清理资源**: 测试完成后应该清理所有创建的资源
4. **错误处理**: 测试应该覆盖错误情况，不仅仅是成功路径

## 实施优先级

1. **高优先级**: Install 改为 Apply（影响用户体验和一致性）
2. **中优先级**: Terraform 测试（核心功能，需要 fake-gcs-server）
3. **中优先级**: Kubernetes 测试（核心功能，需要 client-go fake）
4. **低优先级**: 集成测试和性能测试

## 时间估算

- Install 改为 Apply: 1-2 天
- Terraform 测试实现: 3-5 天
- Kubernetes 测试实现: 3-5 天
- 集成测试: 5-7 天

## 相关文件

- 测试框架入口: `suite_test.go`
- CLI 测试: `pkg/cli/*_test.go`
- Bootstrap 测试: `pkg/bootstrap/*_test.go`
- Template 测试: `pkg/template/*_test.go`
- Renderer 测试: `pkg/renderer/*_test.go`

