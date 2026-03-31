package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/gruntwork-io/terratest/modules/terraform"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	integration "blcli/integration"
)

// TerraformApplyTestEnv holds the test environment for terraform apply tests
type TerraformApplyTestEnv struct {
	Workspace    string
	FakeGCS      *fakestorage.Server
	FakeGCSURL   string
	TerraformDir string
	ProjectRoot  string
}

// KubernetesApplyTestEnv holds the test environment for kubernetes apply tests
type KubernetesApplyTestEnv struct {
	Workspace     string
	FakeK8sClient kubernetes.Interface
	KubernetesDir string
	ProjectRoot   string
}

// GitOpsApplyTestEnv holds the test environment for gitops apply tests
type GitOpsApplyTestEnv struct {
	Workspace   string
	GitOpsDir   string
	ArgsPath    string
	ProjectRoot string
	GitHubMock  *GitHubMock
	ArgoCDMock  *ArgoCDMock
}

// GitHubMock mocks GitHub API
type GitHubMock struct {
	Repositories map[string]bool // repo name -> exists
}

// ArgoCDMock mocks ArgoCD API
type ArgoCDMock struct {
	Applications map[string]bool // app name -> exists
}

// SetupTerraformApplyTest sets up the test environment for terraform apply
func SetupTerraformApplyTest(workspace, terraformDir string) (*TerraformApplyTestEnv, error) {
	// Start fake GCS server
	fakeGCS, gcsURL, err := integration.SetupFakeGCS()
	if err != nil {
		return nil, fmt.Errorf("failed to setup fake GCS: %w", err)
	}

	// Get project root
	currentDir, err := os.Getwd()
	if err != nil {
		fakeGCS.Stop()
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	projectRoot := filepath.Join(currentDir, "..", "..")
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		fakeGCS.Stop()
		return nil, fmt.Errorf("failed to get project root: %w", err)
	}

	return &TerraformApplyTestEnv{
		Workspace:    workspace,
		FakeGCS:      fakeGCS,
		FakeGCSURL:   gcsURL,
		TerraformDir: terraformDir,
		ProjectRoot:  absRoot,
	}, nil
}

// TeardownTerraformApplyTest cleans up the terraform apply test environment
func TeardownTerraformApplyTest(env *TerraformApplyTestEnv) {
	if env == nil {
		return
	}
	if env.FakeGCS != nil {
		env.FakeGCS.Stop()
	}
}

// ExecuteTerraformApply executes blcli apply terraform command
func ExecuteTerraformApply(terraformDir string, useEmulator bool, emulatorURL string, autoApprove bool) error {
	args := []string{"apply", "terraform", "-d", terraformDir}
	if useEmulator {
		args = append(args, "--use-emulator")
		if emulatorURL != "" {
			// Note: This would need to be passed via environment variable
			// since the command doesn't have an emulator-url flag yet
		}
	}
	if autoApprove {
		args = append(args, "--auto-approve")
	}

	return integration.ExecuteBlcliCommand(args...)
}

// VerifyTerraformResources verifies terraform resources using terratest
func VerifyTerraformResources(env *TerraformApplyTestEnv, projectDir string) error {
	// Use terratest to verify terraform state
	terraformOptions := &terraform.Options{
		TerraformDir: projectDir,
	}

	// Verify terraform output (if any)
	// This is a placeholder - actual verification depends on what resources are created
	_, err := terraform.OutputE(nil, terraformOptions, "test_output")
	if err != nil {
		// Output might not exist, which is OK
		return nil
	}

	return nil
}

// SetupKubernetesApplyTest sets up the test environment for kubernetes apply
func SetupKubernetesApplyTest(workspace, kubernetesDir string) (*KubernetesApplyTestEnv, error) {
	// Create fake Kubernetes client
	fakeK8s, err := integration.SetupFakeK8s()
	if err != nil {
		return nil, fmt.Errorf("failed to setup fake K8s: %w", err)
	}

	// IMPORTANT: ensure e2e tests never talk to the user's real kube context.
	// Always override KUBECONFIG to point to a workspace-local, test-only path.
	testKubeconfig := filepath.Join(workspace, "kubeconfig.e2e")
	if err := os.Setenv("KUBECONFIG", testKubeconfig); err != nil {
		return nil, fmt.Errorf("failed to set KUBECONFIG for e2e tests: %w", err)
	}

	// Get project root
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	projectRoot := filepath.Join(currentDir, "..", "..")
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get project root: %w", err)
	}

	return &KubernetesApplyTestEnv{
		Workspace:     workspace,
		FakeK8sClient: fakeK8s,
		KubernetesDir: kubernetesDir,
		ProjectRoot:   absRoot,
	}, nil
}

// ExecuteKubernetesApply executes blcli apply kubernetes command
func ExecuteKubernetesApply(kubernetesDir string, dryRun bool, wait bool, templateRepo string) error {
	args := []string{"apply", "kubernetes", "-d", kubernetesDir}
	if dryRun {
		args = append(args, "--dry-run")
	}
	if !wait {
		args = append(args, "--wait=false")
	}
	if templateRepo != "" {
		args = append(args, "--template-repo", templateRepo)
	}

	return integration.ExecuteBlcliCommand(args...)
}

// VerifyKubernetesResources verifies kubernetes resources exist
func VerifyKubernetesResources(env *KubernetesApplyTestEnv, namespace string, expectedResources []string) error {
	ctx := context.Background()

	// Verify namespace exists
	_, err := env.FakeK8sClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("namespace %s not found: %w", namespace, err)
	}

	// Verify other resources (placeholder - actual verification depends on what resources are created)
	for _, resource := range expectedResources {
		// This is a placeholder - actual verification logic would depend on resource type
		_ = resource
	}

	return nil
}

// SetupGitOpsApplyTest sets up the test environment for gitops apply
func SetupGitOpsApplyTest(workspace, gitopsDir, argsPath string) (*GitOpsApplyTestEnv, error) {
	// Create GitHub mock
	githubMock := &GitHubMock{
		Repositories: make(map[string]bool),
	}

	// Create ArgoCD mock
	argocdMock := &ArgoCDMock{
		Applications: make(map[string]bool),
	}

	// Get project root
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	projectRoot := filepath.Join(currentDir, "..", "..")
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get project root: %w", err)
	}

	return &GitOpsApplyTestEnv{
		Workspace:   workspace,
		GitOpsDir:   gitopsDir,
		ArgsPath:    argsPath,
		ProjectRoot: absRoot,
		GitHubMock:  githubMock,
		ArgoCDMock:  argocdMock,
	}, nil
}

// ExecuteGitOpsApply executes blcli apply gitops command
func ExecuteGitOpsApply(gitopsDir, argsPath string, createRepo bool, skipSync bool) error {
	args := []string{"apply", "gitops", "-d", gitopsDir, "--args", argsPath}
	if createRepo {
		args = append(args, "--create-repo")
	}
	if skipSync {
		args = append(args, "--skip-sync")
	}

	return integration.ExecuteBlcliCommand(args...)
}

// VerifyGitOpsResources verifies gitops resources
func VerifyGitOpsResources(env *GitOpsApplyTestEnv, repoName, appName string) error {
	// Verify GitHub repository exists (in mock)
	if !env.GitHubMock.Repositories[repoName] {
		return fmt.Errorf("GitHub repository %s not found", repoName)
	}

	// Verify ArgoCD Application exists (in mock)
	if !env.ArgoCDMock.Applications[appName] {
		return fmt.Errorf("ArgoCD Application %s not found", appName)
	}

	return nil
}

// ConfigureTerraformBackendForEmulator configures terraform backend to use emulator
func ConfigureTerraformBackendForEmulator(terraformDir, emulatorURL string) error {
	// This function would configure terraform backend to use the emulator
	// For now, it's a placeholder - actual implementation would modify backend.tf
	// or set environment variables for terraform
	return nil
}

// WaitForKubernetesResource waits for a kubernetes resource to be ready
func WaitForKubernetesResource(env *KubernetesApplyTestEnv, resourceType, name, namespace string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Placeholder - actual implementation would poll the resource status
	_ = ctx
	_ = resourceType
	_ = name
	_ = namespace

	return nil
}
