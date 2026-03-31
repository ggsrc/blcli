# Diagnostics

## Missing generated file

Check in this order:

1. Is the component listed in args for the target project?
2. Does the referenced template path exist in `bl-template`?
3. Is the component in the correct config section (`init`, `optional`, or legacy `components`)?
4. Is the file being skipped because it already exists and overwrite is off?
5. Is the file copied as-is or rendered from `.tmpl`?

## Wrong script path at runtime

Check in this order:

1. Where was the script rendered in the workspace?
2. What `cmd.Dir` does the Go code use?
3. Is the subprocess invoked with a relative path that becomes wrong after `cmd.Dir` is applied?
4. Is execute permission added before running?

## Kubernetes apply/install failure

Check in this order:

1. `installType` in template config
2. rendered `install` script or manifest content in workspace
3. dependency order from config and normalized component names
4. required cluster tools such as `kubectl`, `helm`, `kustomize`, `kubeseal`
5. environment variables such as `KUBECONFIG` or script-specific inputs

## Template not found in cache

Check in this order:

1. Does the file exist in the local template repo?
2. Does `config.yaml` still reference an old path?
3. Is the template loader pointed at the expected local path or remote repo?
4. If remote, was cache synced from the intended branch or ref?

## Test failure with local environment constraints

- If Go build cache is blocked, rerun with `GOCACHE=/tmp/gocache`.
- If a test expects an external template repo, state that clearly instead of treating it as a product regression.
- If networking or port binding is blocked by sandboxing, separate environment failure from code failure.
