# Conventions

## `blcli-go` and `bl-template` move together

- A path listed in `bl-template/*/config.yaml` must exist in the template repo.
- A template-only behavior change is often really a system change because `blcli-go` may depend on file names, layout, or script names.
- When editing Kubernetes component install behavior, inspect:
  - `bl-template/kubernetes/config.yaml`
  - the component directory under `bl-template/kubernetes/components/...`
  - `pkg/bootstrap/kubernetes/*`
  - `pkg/bootstrap/apply_kubernetes.go`

## Kubernetes-specific rules

- Files ending in `.tmpl` are rendered as Go templates.
- Non-`.tmpl` files are copied as-is; do not rename them to `.yaml` if they contain other templating syntax that should remain untouched.
- Component names in args may contain ordering prefixes such as `0-foo`; config names often do not. Check normalization logic before assuming a mismatch is a user error.
- Custom install components depend on the rendered workspace scripts, not only on template source scripts.
- `--overwrite` matters. Missing updates may be caused by write-if-absent behavior rather than bad rendering.

## Operational safety

- Do not change install ordering casually. Check dependency resolution and whether the order is encoded in args names, config dependencies, or both.
- Do not treat generated workspace content as canonical source. Fix the template or generator, then re-render.
- When introducing global scripts such as `kubernetes/init.sh`, reason about both render location and execution working directory.
