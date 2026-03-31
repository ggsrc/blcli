package integration_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"blcli/pkg/template"
)

// TestEnvironment holds the test environment setup
type TestEnvironment struct {
	Workspace      string
	FakeGCS        *fakestorage.Server
	FakeGCSURL     string
	FakeK8sClient  kubernetes.Interface
	TemplateRepo   string
	TemplateLoader *template.Loader
}

// SetupFakeGCS starts a fake-gcs-server instance
func SetupFakeGCS() (*fakestorage.Server, string, error) {
	server := fakestorage.NewServer([]fakestorage.Object{})

	// Create a test bucket
	server.CreateBucket("test-codestore")

	return server, server.URL(), nil
}

// SetupFakeK8s creates a fake Kubernetes client
func SetupFakeK8s() (kubernetes.Interface, error) {
	// Create a fake client with no initial objects
	client := fake.NewSimpleClientset()
	return client, nil
}

// SetupTestWorkspace creates a temporary test workspace directory
func SetupTestWorkspace() (string, error) {
	// Use absolute path to ensure local path detection works correctly
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	workspace := filepath.Join(currentDir, "workspace", "integration-test", fmt.Sprintf("test-%d", time.Now().Unix()))
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return "", fmt.Errorf("failed to create test workspace: %w", err)
	}

	// Return absolute path
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return workspace, nil // Fallback to relative path
	}
	return absWorkspace, nil
}

// CleanupTestWorkspace removes the test workspace directory
func CleanupTestWorkspace(workspace string) error {
	if workspace == "" {
		return nil
	}
	// Only clean up if it's in the workspace/integration-test directory for safety
	if filepath.Base(filepath.Dir(workspace)) == "integration-test" {
		return os.RemoveAll(workspace)
	}
	return nil
}

// CreateTestTemplateRepo creates a minimal test template repository
func CreateTestTemplateRepo(workspace string) (string, error) {
	templateDir := filepath.Join(workspace, "test-template-repo")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create template directory: %w", err)
	}

	// Return absolute path to ensure it's recognized as local path
	absTemplateDir, err := filepath.Abs(templateDir)
	if err != nil {
		// Fallback to relative path if abs fails
		absTemplateDir = templateDir
	}
	templateDir = absTemplateDir

	// Create terraform config.yaml
	terraformConfig := `version: "1.0.0"

init:
  - name: backend
    path:
      - terraform/init/backend.tf.tmpl
    args:
      - terraform/init/args.yaml

modules:
  - name: test-module
    path:
      - terraform/modules/test-module/main.tf.tmpl
    args:
      - terraform/modules/test-module/args.yaml

projects:
  - name: test-project
    path:
      - terraform/project/main.tf.tmpl
    args:
      - terraform/project/args.yaml
`

	terraformConfigPath := filepath.Join(templateDir, "terraform", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(terraformConfigPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(terraformConfigPath, []byte(terraformConfig), 0644); err != nil {
		return "", err
	}

	// Create terraform args.yaml (root level)
	terraformArgs := `version: "1.0.0"

parameters:
  global:
    GlobalName:
      type: string
      description: "Global name used for resource naming"
      example: "test-org"
    OrganizationID:
      type: string
      description: "GCP Organization ID"
      example: "123456789012"
    BillingAccountID:
      type: string
      description: "GCP Billing Account ID"
      example: "01ABCD-2EFGH3-4IJKL5"
`

	terraformArgsPath := filepath.Join(templateDir, "terraform", "args.yaml")
	if err := os.WriteFile(terraformArgsPath, []byte(terraformArgs), 0644); err != nil {
		return "", err
	}

	// Create terraform default.yaml (required for init-args command)
	// Init uses placeholders ${project.<name>.id}; resolveProjectPlaceholders replaces them after ID generation.
	terraformDefault := `version: 1.0.0
global:
    BillingAccountID: 01ABCD-2EFGH3-4IJKL5
    OrganizationID: "123456789012"
    Region: us-west1
    TerraformBackendBucket: test-project
    TerraformVersion: 1.5.0
init:
    components:
        backend: {}
        projects:
            ProjectServices:
                ${project.test-project.id}:
                    - compute.googleapis.com
                    - container.googleapis.com
projects:
    - name: test-project
      global:
        project_name: test-project
      components:
        - name: config-gcs-tfbackend
          parameters: {}
        - name: provider
          parameters:
            project_id: test-project
            region: us-west1
        - name: variables
          parameters:
            project_id: test-project
            region: us-west1
            zone: us-west1-a
`

	terraformDefaultPath := filepath.Join(templateDir, "terraform", "default.yaml")
	if err := os.WriteFile(terraformDefaultPath, []byte(terraformDefault), 0644); err != nil {
		return "", err
	}

	// Create a simple terraform template
	terraformInitDir := filepath.Join(templateDir, "terraform", "init")
	if err := os.MkdirAll(terraformInitDir, 0755); err != nil {
		return "", err
	}

	backendTemplate := `# Backend configuration (optional - can be skipped with -backend=false)
# terraform {
#   backend "gcs" {
#     bucket = "{{ .TerraformBackendBucket }}"
#     prefix = "terraform/state"
#   }
# }
`
	backendTemplatePath := filepath.Join(terraformInitDir, "backend.tf.tmpl")
	if err := os.WriteFile(backendTemplatePath, []byte(backendTemplate), 0644); err != nil {
		return "", err
	}

	// Create init args.yaml
	initArgs := `version: "1.0.0"

parameters:
  global:
    TerraformBackendBucket:
      type: string
      description: "Terraform backend bucket name"
      example: "test-codestore"
`
	initArgsPath := filepath.Join(terraformInitDir, "args.yaml")
	if err := os.WriteFile(initArgsPath, []byte(initArgs), 0644); err != nil {
		return "", err
	}

	// Create a simple project template
	projectDir := filepath.Join(templateDir, "terraform", "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return "", err
	}

	projectTemplate := `# Test Terraform configuration
variable "project_id" {
  description = "GCP Project ID"
  type        = string
  default     = "{{ .project_id }}"
}

variable "region" {
  description = "GCP Region"
  type        = string
  default     = "us-west1"
}

variable "zone" {
  description = "GCP Zone"
  type        = string
  default     = "us-west1-a"
}

resource "google_storage_bucket" "test" {
  name     = "{{ .project_name }}-test-bucket"
  location = "US"
  project  = var.project_id
}
`
	projectTemplatePath := filepath.Join(projectDir, "main.tf.tmpl")
	if err := os.WriteFile(projectTemplatePath, []byte(projectTemplate), 0644); err != nil {
		return "", err
	}

	// Create project args.yaml
	projectArgs := `version: "1.0.0"

parameters:
  global:
    ProjectName:
      type: string
      description: "Project name"
      example: "test-project"
`
	projectArgsPath := filepath.Join(projectDir, "args.yaml")
	if err := os.WriteFile(projectArgsPath, []byte(projectArgs), 0644); err != nil {
		return "", err
	}

	// Create kubernetes config.yaml
	kubernetesConfig := `components:
  - name: namespace
    path:
      - kubernetes/base/namespace.yaml.tmpl
    install: kubectl apply -f namespace.yaml
    installType: kubectl
  - name: test-component
    path:
      - kubernetes/components/test-component/kustomization.yaml.tmpl
    install: kustomize build . | kubectl apply -f -
    installType: kubectl
    dependencies:
      - namespace
  - name: test-optional
    path:
      - kubernetes/components/test-optional/kustomization.yaml.tmpl
    install: kustomize build . | kubectl apply -f -
    installType: kubectl
    dependencies:
      - namespace
`
	kubernetesConfigPath := filepath.Join(templateDir, "kubernetes", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(kubernetesConfigPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(kubernetesConfigPath, []byte(kubernetesConfig), 0644); err != nil {
		return "", err
	}

	// Create kubernetes template
	kubernetesBaseDir := filepath.Join(templateDir, "kubernetes", "base")
	if err := os.MkdirAll(kubernetesBaseDir, 0755); err != nil {
		return "", err
	}

	namespaceTemplate := `apiVersion: v1
kind: Namespace
metadata:
  name: {{ .NamespaceName }}
`
	namespaceTemplatePath := filepath.Join(kubernetesBaseDir, "namespace.yaml.tmpl")
	if err := os.WriteFile(namespaceTemplatePath, []byte(namespaceTemplate), 0644); err != nil {
		return "", err
	}

	// Create kubernetes default.yaml (required for init-args command)
	kubernetesDefault := `version: 1.0.0
global: {}
projects:
  - name: test-project
    components:
      - name: test-component
        parameters:
          namespace: test-namespace
      - name: test-optional
        parameters:
          namespace: test-namespace
`
	kubernetesDefaultPath := filepath.Join(templateDir, "kubernetes", "default.yaml")
	if err := os.WriteFile(kubernetesDefaultPath, []byte(kubernetesDefault), 0644); err != nil {
		return "", err
	}

	// Create kubernetes component templates
	componentDir := filepath.Join(templateDir, "kubernetes", "components", "test-component")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		return "", err
	}
	kustomizationTemplate := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: {{ .Namespace }}
resources:
  - namespace.yaml
`
	kustomizationPath := filepath.Join(componentDir, "kustomization.yaml.tmpl")
	if err := os.WriteFile(kustomizationPath, []byte(kustomizationTemplate), 0644); err != nil {
		return "", err
	}

	// Create optional component template
	optionalDir := filepath.Join(templateDir, "kubernetes", "components", "test-optional")
	if err := os.MkdirAll(optionalDir, 0755); err != nil {
		return "", err
	}
	optionalKustomizationPath := filepath.Join(optionalDir, "kustomization.yaml.tmpl")
	if err := os.WriteFile(optionalKustomizationPath, []byte(kustomizationTemplate), 0644); err != nil {
		return "", err
	}

	// Create gitops config.yaml (app-templates format for BootstrapGitops / LoadGitopsConfig)
	gitopsConfig := `version: "1.0.0"

app-templates:
  deployment:
    - name: app
      path: gitops/app.yaml.tmpl
      description: "ArgoCD Application"
  statefulset: []

argocd:
  path: gitops/app.yaml.tmpl
  args: gitops/args.yaml
`
	gitopsConfigPath := filepath.Join(templateDir, "gitops", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(gitopsConfigPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(gitopsConfigPath, []byte(gitopsConfig), 0644); err != nil {
		return "", err
	}

	// Create gitops/app.yaml.tmpl (minimal) so init gitops can render
	appTmpl := `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: {{ .ApplicationName | default "app" }}
spec:
  source:
    path: {{ .SourcePath }}
    repoURL: {{ .SourceRepoURL }}
`
	appTmplPath := filepath.Join(templateDir, "gitops", "app.yaml.tmpl")
	if err := os.WriteFile(appTmplPath, []byte(appTmpl), 0644); err != nil {
		return "", err
	}

	// Create gitops default.yaml so init-args produces gitops.argocd and gitops.apps
	gitopsDefault := `argocd:
  project: [stg]
apps:
  - name: hello-world
    kind: deployment
    project:
      - name: stg
        parameters:
          ApplicationName: hello-world
          SourcePath: stg/hello-world
          SourceRepoURL: https://github.com/example/gitops.git
`
	gitopsDefaultPath := filepath.Join(templateDir, "gitops", "default.yaml")
	if err := os.WriteFile(gitopsDefaultPath, []byte(gitopsDefault), 0644); err != nil {
		return "", err
	}

	return templateDir, nil
}

// ConfigureTerraformBackend configures terraform backend to use fake-gcs
func ConfigureTerraformBackend(workspace, gcsURL string) error {
	// Create backend configuration file
	backendConfig := fmt.Sprintf(`terraform {
  backend "gcs" {
    bucket = "test-codestore"
    prefix = "terraform/state"
  }
}

# Override provider to use fake GCS
provider "google" {
  # Use fake GCS endpoint
  storage_custom_endpoint = "%s"
}
`, gcsURL)

	terraformDir := filepath.Join(workspace, "terraform")
	if err := os.MkdirAll(terraformDir, 0755); err != nil {
		return err
	}

	backendPath := filepath.Join(terraformDir, "backend.tf")
	return os.WriteFile(backendPath, []byte(backendConfig), 0644)
}

// SetupTestEnvironment sets up a complete test environment
func SetupTestEnvironment() (*TestEnvironment, error) {
	workspace, err := SetupTestWorkspace()
	if err != nil {
		return nil, fmt.Errorf("failed to setup workspace: %w", err)
	}

	// IMPORTANT: Ensure that all tests run with an isolated, workspace-local kubeconfig.
	// This prevents any accidental interaction with the user's real kube context or cluster.
	testKubeconfig := filepath.Join(workspace, "kubeconfig.e2e")
	if err := os.Setenv("KUBECONFIG", testKubeconfig); err != nil {
		return nil, fmt.Errorf("failed to set KUBECONFIG for test environment: %w", err)
	}

	fakeGCS, gcsURL, err := SetupFakeGCS()
	if err != nil {
		CleanupTestWorkspace(workspace)
		return nil, fmt.Errorf("failed to setup fake GCS: %w", err)
	}

	fakeK8s, err := SetupFakeK8s()
	if err != nil {
		fakeGCS.Stop()
		CleanupTestWorkspace(workspace)
		return nil, fmt.Errorf("failed to setup fake K8s: %w", err)
	}

	// Use GitHub repository for e2e tests
	// For private repos, authentication is required via GITHUB_TOKEN env or gh cli
	templateRepo := "github.com/ggsrc/bl-template"

	// Check if we should use local test template (for unit tests)
	// This can be overridden by environment variable for e2e tests
	if useLocalTemplate := os.Getenv("BLCLI_USE_LOCAL_TEMPLATE"); useLocalTemplate == "true" {
		localTemplateRepo, err := CreateTestTemplateRepo(workspace)
		if err != nil {
			fakeGCS.Stop()
			CleanupTestWorkspace(workspace)
			return nil, fmt.Errorf("failed to create test template repo: %w", err)
		}
		templateRepo = localTemplateRepo
	}

	loader := template.NewLoader(templateRepo)

	return &TestEnvironment{
		Workspace:      workspace,
		FakeGCS:        fakeGCS,
		FakeGCSURL:     gcsURL,
		FakeK8sClient:  fakeK8s,
		TemplateRepo:   templateRepo,
		TemplateLoader: loader,
	}, nil
}

// TeardownTestEnvironment cleans up the test environment
func TeardownTestEnvironment(env *TestEnvironment) {
	if env == nil {
		return
	}

	if env.FakeGCS != nil {
		env.FakeGCS.Stop()
	}

	if env.Workspace != "" {
		CleanupTestWorkspace(env.Workspace)
	}
}

// ExecuteBlcliCommand executes a blcli command
func ExecuteBlcliCommand(args ...string) error {
	// Get the project root directory
	// integration/helpers.go -> blcli/
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Find project root (blcli directory)
	// Current dir might be integration/ or integration/terraform/ etc.
	// We need to go up to blcli/
	projectRoot := filepath.Join(currentDir, "..", "..")
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	cmd := exec.Command("go", append([]string{"run", "./cmd/blcli"}, args...)...)
	cmd.Dir = absRoot
	output, err := cmd.CombinedOutput()
	// Always print output for debugging in test mode
	if os.Getenv("BLCLI_DEBUG") == "true" {
		fmt.Printf("blcli command: %v\nOutput:\n%s\n", args, string(output))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
	if err != nil {
		return fmt.Errorf("blcli command failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// VerifyTerraformState verifies terraform state exists in fake-gcs
func VerifyTerraformState(env *TestEnvironment) error {
	// List objects in the bucket
	objects, _, err := env.FakeGCS.ListObjects("test-codestore", "terraform/state", "", false)
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	if len(objects) == 0 {
		return fmt.Errorf("no terraform state found in bucket")
	}

	return nil
}

// VerifyKubernetesResources verifies kubernetes resources exist
func VerifyKubernetesResources(env *TestEnvironment, namespace string) error {
	ctx := context.Background()
	_, err := env.FakeK8sClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("namespace %s not found: %w", namespace, err)
	}
	return nil
}
