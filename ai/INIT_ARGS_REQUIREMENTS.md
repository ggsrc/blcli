# init-args 需求总结

## 核心功能
1. 从 template repo 收集所有 `args.yaml` 文件（从 `terraform/config.yaml` 中的 Init, Modules, Projects）
2. 合并所有 `args.yaml` 定义（后面的覆盖前面的）
3. 将定义转换为实际可用的配置值（使用 default/example）
4. 重新组织为层级化结构，支持继承和覆盖
5. 生成 YAML 或 TOML 格式文件（默认 YAML）

## 输出结构（层级化，支持继承）
```yaml
global:
  # 全局配置，所有子项继承

terraform:
  version: "1.0.0"  # 从 config.yaml 获取
  global:
    # 继承自 global，可覆盖
  projects:
    - name: <project-name>  # 默认列表 (prd, stg, corp, ...)；ID 由占位符与 replaceProjectIDInValue 解析
      global:
        # 继承自 terraform.global，可覆盖
      components:
        - name: <component-name>  # 从 config.yaml 动态获取
          parameters:
            # 组件参数
```

## 关键要求

### 1. 无硬编码
- ❌ 不硬编码 `variables`, `provider`, `backend`
- ❌ 不硬编码 `ssl-cert -> ssl-certificate` 映射
- ❌ 不硬编码项目名称（如 `prd`, `dev`）
- ❌ 不硬编码 `blcli` 配置
- ✅ 所有信息从 `config.yaml` 和 `args.yaml` 动态获取

### 2. 项目名称推断
- 从 default 的 projects 列表与默认项目名；init 段使用占位符 ${project.<name>.id}，由 resolveProjectPlaceholders 解析
- 如果无法推断，生成空列表 `[]`，让用户自定义

### 3. 组件识别
- **项目级别组件**：从 `terraformConfig.Projects` 的路径中提取
  - 例如：`terraform/project/backend.tf.tmpl` -> `backend`
  - 跳过：`main`, `README`, `outputs` 等非组件文件
- **模块组件**：直接从 `terraformConfig.Modules` 获取，使用模块名称（不做任何映射）

### 4. TOML 写入
- 使用 `github.com/pelletier/go-toml/v2` 库，不要手动实现

### 5. 注释处理
- 注释中包含元数据：description, required, example, default
- 注释位置与配置结构对应

### 6. 值提取
- 使用 `ArgsDefinition.ToConfigValues()` 方法
- 优先级：default > example > 空值（根据 type）

## 实现步骤
1. 加载 `terraform/config.yaml`
2. 收集所有 `args.yaml` 路径（Init, Modules, Projects）
3. 加载并解析所有 `args.yaml` 为 `ArgsDefinition`
4. 合并所有定义（后面的覆盖前面的）
5. 转换为实际值（`ToConfigValues()`）
6. 项目名称：默认列表 (prd, stg, corp, ...)；init 内 ID 通过占位符 ${project.<name>.id} 解析
7. 识别项目级别组件（从 `Projects` 路径）
8. 识别模块组件（从 `Modules`）
9. 重新组织为层级结构
10. 写入文件（YAML 或 TOML，使用库）

