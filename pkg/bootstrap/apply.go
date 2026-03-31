package bootstrap

import (
	"time"
)

// ApplyTerraformOptions holds options for apply terraform command
type ApplyTerraformOptions struct {
	TerraformDir    string
	ProjectName     string
	Timeout         time.Duration
	UseEmulator     bool
	EmulatorPort    int
	EmulatorDataDir string
	AutoApprove     bool
	SkipBackend     bool
	TemplateRepo    string        // Optional: template repository URL or local path for dependency resolution
	DryRun          bool          // If true, only show execution plan without executing
	InitDelay       time.Duration // Delay after each init directory (apply init only). 0 = no delay.
}

// ApplyKubernetesOptions holds options for apply kubernetes command
type ApplyKubernetesOptions struct {
	KubernetesDir           string
	ProjectName             string // If set, only apply components under this project (e.g. stg)
	Kubeconfig              string
	Context                 string
	Namespace               string
	Timeout                 time.Duration
	DryRun                  bool
	Wait                    bool
	ComponentWaitAfterApply time.Duration // Wait duration after each component apply before next (e.g. 30s). 0 = no wait.
}

// ApplyGitOpsOptions holds options for apply gitops command
type ApplyGitOpsOptions struct {
	GitOpsDir    string
	Project      string // If set, only apply ArgoCD Applications under this project (e.g. stg)
	ArgsPaths    []string
	Kubeconfig   string
	Context      string
	CreateRepo   bool
	RepoURL      string
	Branch       string
	ArgoCDServer string
	ArgoCDToken  string
	Timeout      time.Duration
	SkipSync     bool
	DryRun       bool // If true, only show execution plan without executing
}

// ApplyAllOptions holds options for apply all command
type ApplyAllOptions struct {
	WorkspaceDir    string
	TerraformDir    string
	KubernetesDir   string
	GitOpsDir       string
	ArgsPaths       []string
	ContinueOnError bool
	SkipModules     []string
	// Terraform options
	TerraformUseEmulator     bool
	TerraformEmulatorPort    int
	TerraformEmulatorDataDir string
	TerraformAutoApprove     bool
	TerraformSkipBackend     bool
	// Kubernetes options
	KubernetesKubeconfig string
	KubernetesContext    string
	KubernetesNamespace  string
	KubernetesDryRun     bool
	KubernetesWait       bool
	// GitOps options
	GitOpsCreateRepo   bool
	GitOpsRepoURL      string
	GitOpsBranch       string
	GitOpsArgoCDServer string
	GitOpsArgoCDToken  string
	GitOpsSkipSync     bool
	GitOpsDryRun       bool
	// Terraform dry-run (for apply all)
	TerraformDryRun bool
}

// ExecuteApplyTerraform is implemented in apply_terraform.go

// ExecuteApplyKubernetes, ExecuteApplyGitOps, and ExecuteApplyAll are implemented in:
// - apply_kubernetes.go
// - apply_gitops.go
// - apply_all.go
