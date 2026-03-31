# Makefile 使用指南

本文档介绍如何使用 blcli-go 项目的 Makefile 进行开发、测试和构建。

## 快速开始

### 查看所有可用命令

```bash
make help
```

### 开发环境设置

```bash
# 安装开发工具（kind, golangci-lint 等）
make install-tools

# 或一次性设置开发环境
make dev-setup
```

## 构建命令

### 基本构建

```bash
# 构建二进制文件（输出到 ../workspace/bin/blcli）
make build

# 开发模式构建（包含 race detector 和调试符号）
make build-dev

# 安装到 $GOPATH/bin
make install
```

## 测试命令

### 单元测试

```bash
# 运行所有单元测试
make test-unit

# 运行单元测试（详细输出）
make test-unit-verbose

# 运行单元测试并生成覆盖率报告
make test-unit-coverage
# 覆盖率报告会生成 coverage.html，可以在浏览器中打开查看
```

### 集成测试

```bash
# 运行集成测试
make test-integration
```

### E2E 测试

```bash
# 运行 E2E 测试（需要 Kubernetes 集群）
make test-e2e

# E2E 测试（详细输出）
make test-e2e-verbose
```

### 运行所有测试

```bash
# 运行所有测试（单元测试 + 集成测试，不包括 E2E）
make test-all

# 运行所有测试（默认只运行单元测试）
make test
```

### 手动测试

```bash
# 查看可用的手动测试场景
make test-manual

# 手动测试：初始化 Kubernetes
make test-manual-init-k8s

# 手动测试：应用 Kubernetes（需要 k8s 集群）
make test-manual-apply-k8s

# 手动测试：初始化 Terraform
make test-manual-init-tf

# 手动测试：初始化 args
make test-manual-init-args
```

## Kubernetes 集群管理

### 创建本地 Kubernetes 集群（Mac）

```bash
# 创建单节点 Kubernetes 集群（使用 kind）
make k8s-create

# 查看集群状态
make k8s-status

# 获取 kubeconfig 导出命令
make k8s-kubeconfig

# 使用集群（执行上面的命令后，会显示 export 命令）
export KUBECONFIG=$(kind get kubeconfig-path --name blcli-test)
```

### 删除集群

```bash
# 删除集群
make k8s-delete

# 重置集群（删除后重新创建）
make k8s-reset
```

### 使用集群进行测试

```bash
# 1. 创建集群
make k8s-create

# 2. 设置 kubeconfig
export KUBECONFIG=$(kind get kubeconfig-path --name blcli-test)

# 3. 验证集群
kubectl get nodes

# 4. 运行 E2E 测试
make test-e2e

# 5. 或手动测试 apply kubernetes
make test-manual-apply-k8s
```

## 代码质量工具

### 格式化

```bash
# 格式化所有 Go 代码
make fmt
```

### 静态分析

```bash
# 运行 go vet
make vet

# 运行 golangci-lint（如果已安装）
make lint
```

### 清理

```bash
# 清理构建产物和测试文件
make clean
```

## CI/CD 命令

```bash
# CI 测试套件（格式化 + vet + 单元测试）
make ci-test

# CI 构建
make ci-build
```

## 开发工作流示例

### 日常开发

```bash
# 1. 设置开发环境（首次）
make dev-setup

# 2. 创建本地 k8s 集群（用于测试）
make k8s-create
export KUBECONFIG=$(kind get kubeconfig-path --name blcli-test)

# 3. 编写代码后，运行测试
make test-unit

# 4. 格式化代码
make fmt

# 5. 构建并测试
make build
make test-manual-init-k8s
```

### 提交前检查

```bash
# 运行完整的 CI 检查
make ci-test

# 或手动运行
make fmt
make vet
make lint
make test-unit
```

### 调试

```bash
# 使用开发模式构建（包含 race detector）
make build-dev

# 运行详细测试
make test-unit-verbose
```

## 常见问题

### kind 未安装

```bash
# Mac 上使用 Homebrew 安装
brew install kind

# 或使用 make 命令自动安装
make install-tools
```

### golangci-lint 未安装

```bash
# Mac 上使用 Homebrew 安装
brew install golangci-lint

# 或使用 make 命令自动安装
make install-tools
```

### Docker 未运行

kind 需要 Docker 运行。确保 Docker Desktop（Mac）已启动。

### 集群创建失败

```bash
# 检查 Docker 是否运行
docker ps

# 删除旧集群后重新创建
make k8s-delete
make k8s-create
```

## 环境变量

可以通过环境变量自定义一些行为：

```bash
# 自定义版本号
VERSION=v1.0.0 make build

# 自定义测试超时时间（在 Makefile 中修改 TEST_TIMEOUT）
```

## 目录结构

- `../workspace/bin/blcli` - 构建的二进制文件
- `coverage.out` - 测试覆盖率数据
- `coverage.html` - 测试覆盖率 HTML 报告

## 注意事项

1. **E2E 测试需要 Kubernetes 集群**：运行 E2E 测试前，请先创建本地集群或配置 kubeconfig
2. **手动测试是交互式的**：某些手动测试可能需要用户输入或验证
3. **kind 集群是临时的**：重启电脑后，kind 集群仍然存在，但需要重新设置 kubeconfig
4. **测试超时**：默认测试超时为 30 分钟，可以在 Makefile 中修改 `TEST_TIMEOUT` 变量
