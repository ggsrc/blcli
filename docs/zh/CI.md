# blcli CI 集成指南

本文说明如何在 GitHub Actions 或其他 CI 中使用 blcli 校验配置、生成代码并做合规检查。

## 快速开始（GitHub Actions）

仓库内提供了 composite action：`.github/actions/blcli`。

```yaml
name: blcli validate

on:
  pull_request:
  push:
    branches: [main]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup blcli
        uses: ./.github/actions/blcli
        with:
          version: latest

      - name: Generate args (optional)
        run: |
          blcli init-args ./bl-template --profile minimal -o workspace/config/args.yaml

      - name: Validate args
        run: |
          blcli check args --args workspace/config/args.yaml -r ./bl-template

      - name: Init preview
        run: |
          blcli init ./bl-template -a workspace/config/args.yaml --preview

      - name: Generate infrastructure code
        run: |
          blcli init ./bl-template -a workspace/config/args.yaml -o workspace/output

      - name: Check generated terraform
        run: |
          blcli check repo --args workspace/config/args.yaml --timeout=30m
```

## Action 输入

| 输入 | 说明 | 默认 |
|------|------|------|
| `version` | blcli 版本（`latest` 或 `v1.5.0`） | `latest` |
| `token` | GitHub token（私有模板仓） | `${{ github.token }}` |

## 推荐流水线阶段

1. **`blcli check plugin`** — 确认 terraform / kubectl 等工具可用（自托管 runner 或需要 apply 时）。
2. **`blcli init-args`** — 从模板生成或更新 args（personal 模板可用 `--profile full`）。
3. **`blcli check args`** — 在 init 前校验参数，失败时 CI 直接红。
4. **`blcli init --preview`** — PR 中可选，展示将生成的模块与路径。
5. **`blcli init`** — 生成 terraform / kubernetes / gitops 到 `workspace/output`。
6. **`blcli check repo` / `check kubernetes`** — 对生成物做静态校验。

## 私有模板仓库

设置 `GITHUB_TOKEN` 或 `GH_TOKEN`，action 会自动注入：

```yaml
- uses: ./.github/actions/blcli
  with:
    token: ${{ secrets.GH_PAT }}
```

本地调试时也可使用 `gh auth token` 或 `export GITHUB_TOKEN=...`。

## 安全注意事项

- CI 中 **不要** 对真实 GCP 项目执行 `blcli apply`，除非使用专用测试账号并通过 workflow 环境开关显式启用。
- 默认示例仅做 `init` + `check`，不涉及云资源变更。
- Kubernetes `check kubernetes` 使用 `kubectl --dry-run=client`，不应对生产集群 apply。

## 模板仓库 CI 样板

官方模板 `bl-template` 提供 `.github/workflows/blcli-validate.yml`，可在 fork 后按需修改 args 路径与模板引用。

## 相关命令

| 命令 | 用途 |
|------|------|
| `blcli check args` | 校验 args.yaml |
| `blcli check repo` | terratest 风格 terraform 校验 |
| `blcli check kubernetes` | manifest dry-run 校验 |
| `blcli diagnose --file error.log` | 对 CI 失败日志做离线诊断 |
| `blcli contract --format json` | Agent / 自动化集成契约 |

---

*最后更新：2026-06-25*
