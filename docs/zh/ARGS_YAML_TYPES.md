# args.yaml 参数类型与字段说明

本文档列出模板仓库中 **args.yaml** 所支持的参数类型及每个参数可用的元数据字段。实现见 `pkg/renderer/argsdef.go`。与 [Template Repository Protocol](TEMPLATE_REPO_PROTOCOL.md) 中的 args.yaml 格式章节配合使用。

---

## 1. 通用字段（每个参数均可使用）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 否 | 参数类型，见下文「支持的类型」。未写时按 `string` 处理。 |
| `description` | string | 否 | 说明文字，用于 `blcli explain` 及生成配置时的注释。 |
| `required` | boolean | 否 | 是否必填。默认 `false`。 |
| `default` | 任意 | 否 | 默认值。生成配置时优先使用 default，其次 example，再按 type 生成空值。 |
| `example` | 任意 | 否 | 示例值。用于 init-args 填充与文档展示。 |
| `pattern` | string | 否 | 正则约束（如 `^[a-z0-9-]+$`）。可与 `validation` 列表一起使用；若参数有 `validation`，则 `pattern` 作为 fallback 或用于 init-args 文档；`blcli init` 会按 args.yaml 中的校验规则在**合并后、写文件前**强制校验，失败则返回错误不写文件。 |

---

## 2. 支持的类型（type）

以下类型名**大小写敏感**，与实现中的 `extractValueFromParam` / `getDefaultValue` 一致。

### 2.1 标量类型

| type 取值 | 说明 | 无 default/example 时的生成值 |
|-----------|------|------------------------------|
| `string` | 字符串（默认） | `""` |
| `boolean` 或 `bool` | 布尔 | `false` |
| `number`、`integer` 或 `int` | 数值 | `0` |

示例：

```yaml
GlobalName:
  type: string
  description: "Global name for resource naming"
  required: true
  example: "my-org"

EnableFeature:
  type: boolean
  default: false
  description: "Whether to enable the feature"

ReplicaCount:
  type: integer
  default: 1
  description: "Number of replicas"
```

### 2.2 数组/列表（array / list）

| type 取值 | 说明 | 无 default/example 时的生成值 |
|-----------|------|------------------------------|
| `array` 或 `list` | 数组 | `[]` |

可选的 **`items`** 用于描述元素结构，用于 init-args 生成更合理的默认/示例：

- 若 `items` 含 **`example`** 或 **`default`**：用其作为数组元素的示例或默认。
- 若 `items.type` 为 **`object`** 且含 **`properties`**：按 properties 递归生成一个示例对象并放入数组。

示例（简单列表）：

```yaml
Tags:
  type: list
  description: "List of tags"
  default:
    - dev
    - staging
  example:
    - prod
    - env:production
```

示例（对象数组，使用 items）：

```yaml
ContainerPorts:
  type: list
  description: "Container ports configuration"
  items:
    type: object
    properties:
      containerPort:
        type: integer
        example: 8080
      name:
        type: string
        example: http
  default:
    - containerPort: 8080
      name: http
    - containerPort: 9090
      name: grpc
```

### 2.3 对象/映射（object / map）

| type 取值 | 说明 | 无 default/example 时的生成值 |
|-----------|------|------------------------------|
| `object` 或 `map` | 键值对/对象 | `{}` |

可选的 **`properties`** 用于描述各字段，用于 init-args 按结构生成示例对象：

- `properties` 的 key 为字段名，value 为同样支持上述字段（type、description、required、default、example）的参数定义。
- 生成示例时会递归处理每个 property；若某 property 有 `example` 或 `default` 或 `required: true`，会纳入生成的对象。

示例：

```yaml
LivenessProbe:
  type: object
  description: "Liveness probe configuration"
  default:
    httpGet:
      path: "/health/alive"
      port: 8080
    initialDelaySeconds: 0
    periodSeconds: 10
    timeoutSeconds: 5
  example:
    httpGet:
      path: "/healthz"
      port: 8080
```

带 properties 的 object（便于生成结构）：

```yaml
ProbeConfig:
  type: object
  description: "Probe configuration"
  properties:
    path:
      type: string
      example: "/health"
    port:
      type: integer
      example: 8080
    initialDelaySeconds:
      type: integer
      default: 0
```

---

## 3. 类型写法约定（含复合写法）

在模板仓库中常见以下写法，语义与上述类型对应关系如下：

| 写法 | 对应实现类型 | 说明 |
|------|--------------|------|
| `string` | string | 字符串 |
| `boolean` / `bool` | boolean | 布尔 |
| `number` / `integer` / `int` | number | 数值 |
| `array` / `list` | array | 数组；可配 `items` |
| `object` / `map` | object | 对象；可配 `properties` |
| `list[object]` | 视为 list | 表示「对象数组」时，建议用 `type: list` + `items.type: object`（及可选 `items.properties`），或直接提供 `default` / `example`。 |
| `map[string]string` | 视为 object/map | 表示「字符串到字符串的映射」时，用 `type: object` 或 `type: map`，并可用 `default`/`example` 给出示例。 |
| `map[list[string]]` | 视为 object | 表示「键为字符串、值为字符串数组」时，用 `type: object` 并配 `default`/`example`。 |

说明：实现中按 **`type` 的精确字符串** 做分支（如 `array`、`list`、`object`、`map`、`boolean`、`bool`、`number`、`integer`、`int`）。写为 `list[object]` 或 `map[string]string` 时，不会走「list/object」分支，但只要有 **`default`** 或 **`example`**，init-args 仍会使用这些值生成配置；仅在没有 default/example 时才会按 type 生成空值（此时会落到 `string` 默认，得到 `""`）。因此复杂结构建议同时提供 `default` 或 `example`。

---

## 4. 完整参数定义示例

以下示例覆盖常用类型与字段，可直接在 args.yaml 的 `parameters.global` 或 `parameters.components.*` 下使用。

```yaml
version: 1.0.0

parameters:
  global:
    # 标量
    GlobalName:
      type: string
      description: "Global name used for resource naming"
      required: true
      pattern: "^[a-z0-9-]+$"
      example: "my-org"

    EnableTLS:
      type: boolean
      default: false
      description: "Enable TLS"

    ReplicaCount:
      type: integer
      default: 1
      description: "Number of replicas"

    # 数组（带 items 描述）
    ServicePorts:
      type: list
      description: "Service ports"
      items:
        type: object
        properties:
          name:
            type: string
            example: http
          port:
            type: integer
            example: 8080
          targetPort:
            type: integer
            example: 8080
      default:
        - name: http
          port: 8080
          targetPort: 8080

    # 对象
    LivenessProbe:
      type: object
      description: "Liveness probe"
      default:
        httpGet:
          path: "/health"
          port: 8080
        initialDelaySeconds: 0
        periodSeconds: 10
```

---

## 6. 校验字段（validation）

args.yaml 支持 **validation** 规则，blcli 在 **init 合并 args 后、写文件前** 执行校验；失败则返回 `validation failed: ...` 且不写文件。

### 6.1 参数级 validation（列表）

每个参数可定义 `validation` 列表，每条规则为 map，必含 `kind` 及 kind 对应的 params：

| kind | 参数 | 说明 |
|------|------|------|
| `required` | `message`（可选） | 值非空 |
| `stringLength` | `min`, `max`, `message` | 字符串长度 |
| `pattern` | `value` 或 `pattern`, `message` | 正则 |
| `format` | `value`（如 `email`, `numeric`）, `message` | 格式校验 |
| `enum` | `values`（列表）, `message` | 枚举 |
| `numberRange` | `min`, `max`, `message` | 数值范围 |

示例：

```yaml
ProjectName:
  type: string
  required: true
  validation:
    - kind: required
    - kind: stringLength
      min: 6
      max: 30
    - kind: pattern
      value: "^[a-z][a-z0-9-]{4,28}[a-z0-9]$"
      message: "GCP project ID format"
```

若参数有 `validation` 列表，则以列表为准；否则将顶层 `required`、`pattern` 视为规则（向后兼容）。

### 6.2 顶层 validation.unique

用于约束某路径下取值集合唯一，例如项目名不重复：

```yaml
validation:
  unique:
    - path: "terraform.projects[].name"
      message: "Project names must be unique"
```

path 使用点号路径，`[]` 表示数组段。

---

## 7. 参考实现

完整 args.yaml 示例见示例模板仓库：

- **github.com/ggsrc/bl-template**（根级 `args.yaml`、`terraform/init/args.yaml`、`terraform/project/args.yaml`、`gitops/args.yaml` 等）。

与 [Template Repository Protocol](TEMPLATE_REPO_PROTOCOL.md) 中的「args.yaml 格式」章节一起使用即可覆盖类型与结构约定。
