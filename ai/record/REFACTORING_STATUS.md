# 重构进度状态

## 已完成 ✅

1. **CLI 层 (pkg/cli/)** - ✅ 完成
   - root.go - 根命令
   - init.go - init 命令
   - destroy.go - destroy 命令
   - config.go - config 命令
   - version.go - version 命令
   - check.go - check 命令

2. **生成器层 (pkg/generator/)** - ✅ 完成
   - config.go - 配置文件生成

3. **渲染器层 (pkg/renderer/)** - ✅ 完成
   - args.go - 参数加载和解析

4. **内部工具 (pkg/internal/)** - ✅ 完成
   - fs.go - 文件系统操作
   - tools.go - 工具检查

5. **引导管理器 (pkg/bootstrap/)** - ✅ 部分完成
   - manager.go - 统一的引导管理器

6. **主入口** - ✅ 完成
   - cmd/blcli/main.go - 已更新使用 cli.Execute()

## 待完成 ⏳

1. **模板层重构 (pkg/template/)**
   - 需要将 templates 包重命名为 template
   - 需要更新所有导入引用
   - loader.go 需要从 templates 移动到 template 并更新命名

2. **引导层模块 (pkg/bootstrap/)**
   - terraform.go - 需要从 commands/terraform.go 迁移
   - kubernetes.go - 需要从 commands/kubernetes.go 迁移
   - gitops.go - 需要从 commands/gitops.go 迁移

3. **更新所有导入引用**
   - 将所有 `blcli/pkg/templates` 改为 `blcli/pkg/template`
   - 将所有 `blcli/pkg/commands` 的引用更新到新包
   - 更新 bootstrap 中的函数调用

4. **删除旧文件**
   - pkg/commands/ 目录（保留作为参考，确认无误后删除）
   - pkg/templates/ 目录（迁移完成后删除）

## 下一步操作

由于代码量较大，建议分步进行：

1. 先完成 template 包的重命名和 loader 迁移
2. 然后迁移 bootstrap 的三个模块文件
3. 最后更新所有导入并测试编译
4. 确认无误后删除旧文件

## 关键文件位置

- 旧命令处理: `pkg/commands/init_destroy.go`
- 旧 Terraform: `pkg/commands/terraform.go`
- 旧 Kubernetes: `pkg/commands/kubernetes.go`
- 旧 GitOps: `pkg/commands/gitops.go`
- 旧模板加载器: `pkg/templates/loader.go` (如果存在)

## 命名变更对照

| 旧名称 | 新名称 |
|--------|--------|
| `handleInit` | `ExecuteInit` |
| `handleDestroy` | `ExecuteDestroy` |
| `initTerraform` | `BootstrapTerraform` |
| `initKubernetes` | `BootstrapKubernetes` |
| `initGitops` | `BootstrapGitops` |
| `generateConfig` | `GenerateConfigFile` |
| `TemplateLoader` | `Loader` |
| `NewTemplateLoader` | `NewLoader` |
| `templates` 包 | `template` 包 |

