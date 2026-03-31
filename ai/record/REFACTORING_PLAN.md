# 项目重构方案

## 项目核心功能分析
- **CLI工具**：用于初始化公有云集群和常规业务
- **模板系统**：从GitHub仓库加载模板
- **参数渲染**：使用本地YAML参数文件渲染模板
- **多模块支持**：Terraform、Kubernetes、GitOps

## 当前问题分析

### 1. 文件命名问题
- `init_destroy.go` - 包含两个不相关的操作，命名不够清晰
- `init_config.go` - 容易误解为"初始化配置"，实际是"生成配置模板"
- `run.go` - 名字太泛，实际是辅助函数集合
- `cobra.go` - 可以，但可以更明确为 `cli.go` 或 `root.go`

### 2. 函数命名问题
- `handleInit` / `handleDestroy` - "handle"前缀不够明确，应该用更具体的动词
- `initTerraform` / `initKubernetes` / `initGitops` - 命名一致，但可以更明确
- `generateConfig` - 应该明确为 `generateConfigTemplate` 或 `createConfigFile`
- `loadConfigFromPath` / `loadState` / `saveState` - 辅助函数，应该放在合适的包中

### 3. 包结构问题
- `commands` 包职责过重：包含命令定义、命令处理、辅助函数
- `templates` 包职责混杂：模板引擎、加载器、参数处理混在一起

## 重构方案

### 方案一：按功能域拆分（推荐）

```
pkg/
├── cli/                    # CLI命令定义（原commands/cobra.go）
│   ├── root.go            # 根命令
│   ├── init.go            # init命令定义
│   ├── destroy.go         # destroy命令定义
│   ├── config.go          # config相关命令
│   └── version.go         # version命令
│
├── bootstrap/              # 核心引导逻辑（原commands/init_destroy.go的核心）
│   ├── terraform.go       # Terraform项目引导
│   ├── kubernetes.go      # Kubernetes项目引导
│   ├── gitops.go          # GitOps项目引导
│   └── manager.go         # 统一的引导管理器
│
├── generator/              # 生成器（原commands/init_config.go）
│   └── config.go          # 配置文件生成器
│
├── template/               # 模板系统（原templates）
│   ├── engine.go         # 模板渲染引擎
│   ├── loader.go         # GitHub模板加载器
│   ├── repository.go     # 模板仓库管理
│   └── cache.go          # 模板缓存
│
├── renderer/               # 渲染器（从template分离）
│   ├── args.go           # 参数加载和解析
│   ├── data.go           # 数据准备
│   └── renderer.go       # 渲染逻辑
│
├── config/                 # 配置管理（保持不变）
│   └── config.go
│
├── state/                  # 状态管理（保持不变）
│   └── state.go
│
└── internal/               # 内部工具函数
    ├── fs.go              # 文件系统操作
    └── tools.go           # 工具检查
```

### 方案二：按层次拆分

```
pkg/
├── cmd/                    # 命令层（用户交互）
│   ├── root.go
│   ├── init.go
│   ├── destroy.go
│   └── config.go
│
├── service/                # 服务层（业务逻辑）
│   ├── bootstrap.go      # 引导服务
│   ├── generator.go      # 生成服务
│   └── manager.go        # 管理服务
│
├── template/               # 模板层
│   ├── engine.go
│   ├── loader.go
│   └── repository.go
│
├── render/                 # 渲染层
│   ├── args.go
│   └── renderer.go
│
├── config/                # 配置层
├── state/                 # 状态层
└── internal/              # 工具层
```

## 推荐方案：方案一（按功能域拆分）

### 理由
1. **职责清晰**：每个包有明确的单一职责
2. **易于理解**：包名直接反映功能
3. **便于扩展**：新增功能模块时结构清晰
4. **符合Go惯例**：internal包用于内部工具

### 详细重构计划

#### 1. CLI命令层 (`pkg/cli/`)
```go
// cli/root.go
func NewRootCommand() *cobra.Command

// cli/init.go  
func NewInitCommand() *cobra.Command

// cli/destroy.go
func NewDestroyCommand() *cobra.Command

// cli/config.go
func NewConfigCommand() *cobra.Command
```

#### 2. 引导层 (`pkg/bootstrap/`)
```go
// bootstrap/manager.go
type BootstrapManager struct {
    templateLoader *template.Loader
    argsData       renderer.ArgsData
}

func (m *BootstrapManager) Bootstrap(modules []string) error
func (m *BootstrapManager) Destroy(modules []string) error

// bootstrap/terraform.go
func BootstrapTerraform(cfg config.GlobalConfig, tf *config.TerraformConfig, 
    loader *template.Loader, args renderer.ArgsData) ([]string, error)

// bootstrap/kubernetes.go
func BootstrapKubernetes(cfg config.GlobalConfig, k8s *config.ProjectConfig,
    loader *template.Loader, args renderer.ArgsData) error

// bootstrap/gitops.go
func BootstrapGitops(cfg config.GlobalConfig, gitops *config.ProjectConfig,
    loader *template.Loader, args renderer.ArgsData) error
```

#### 3. 生成器层 (`pkg/generator/`)
```go
// generator/config.go
func GenerateConfigFile(outputPath string) error
```

#### 4. 模板层 (`pkg/template/`)
```go
// template/engine.go - 模板渲染引擎
func Render(tmpl string, data interface{}) (string, error)
func RenderWithArgs(tmpl string, data interface{}, args ArgsData) (string, error)

// template/loader.go - GitHub模板加载器
type Loader struct {
    repoURL string
    cacheDir string
}
func (l *Loader) LoadTemplate(path string) (string, error)

// template/repository.go - 模板仓库管理
type Repository struct {
    URL string
    Branch string
}
func NewRepository(repoURL string) *Repository
```

#### 5. 渲染器层 (`pkg/renderer/`)
```go
// renderer/args.go - 参数加载
func LoadArgs(path string) (ArgsData, error)

// renderer/data.go - 数据准备
func PrepareTerraformData(cfg config.GlobalConfig, ...) TerraformData
func PrepareKubernetesData(cfg config.GlobalConfig, ...) KubernetesData

// renderer/renderer.go - 渲染逻辑
type Renderer struct {
    loader *template.Loader
    args   ArgsData
}
func (r *Renderer) Render(templatePath string, data interface{}) (string, error)
```

## 函数命名规范

### 命令处理函数
- ❌ `handleInit` → ✅ `ExecuteInit` 或 `RunInit`
- ❌ `handleDestroy` → ✅ `ExecuteDestroy` 或 `RunDestroy`

### 引导函数
- ✅ `BootstrapTerraform` - 引导Terraform项目
- ✅ `BootstrapKubernetes` - 引导Kubernetes项目
- ✅ `BootstrapGitops` - 引导GitOps项目

### 生成函数
- ❌ `generateConfig` → ✅ `GenerateConfigFile` 或 `CreateConfigFile`

### 辅助函数
- 移动到 `internal` 包或对应的服务包中
- `loadConfigFromPath` → `config.Load(path)`
- `loadState` → `state.Load()`
- `saveState` → `state.Save(st)`

## 迁移步骤

1. **第一阶段**：创建新包结构，保持旧代码运行
2. **第二阶段**：逐步迁移功能到新包
3. **第三阶段**：更新引用，删除旧代码
4. **第四阶段**：优化和测试

## 命名约定

### 包名
- 使用单数形式：`template` 而非 `templates`
- 使用小写：`bootstrap` 而非 `Bootstrap`
- 简洁明确：`cli` 而非 `command`

### 函数名
- 公开函数使用动词开头：`Bootstrap`, `Generate`, `Render`
- 私有函数使用小写：`prepareData`, `loadTemplate`
- 布尔返回使用 `Is`, `Has`, `Can` 前缀

### 类型名
- 使用名词：`BootstrapManager`, `TemplateLoader`
- 接口使用 `-er` 后缀：`Renderer`, `Loader`

