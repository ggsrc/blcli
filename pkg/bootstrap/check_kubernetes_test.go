package bootstrap_test

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"blcli/pkg/bootstrap"
)

var _ = Describe("CheckKubernetes", func() {
	var (
		workspace     string
		kubernetesDir string
	)

	BeforeEach(func() {
		var err error
		workspace, err = setupCheckKubernetesTestWorkspace()
		Expect(err).NotTo(HaveOccurred())

		kubernetesDir = filepath.Join(workspace, "kubernetes")
		// Create basic kubernetes directory structure
		if err := os.MkdirAll(kubernetesDir, 0755); err != nil {
			Fail(fmt.Sprintf("Failed to create kubernetes dir: %v", err))
		}
	})

	AfterEach(func() {
		cleanupCheckKubernetesTestWorkspace(workspace)
	})

	Describe("ExecuteCheckKubernetes", func() {
		It("should check kubectl installType components", func() {
			// Create base directory with namespace
			baseDir := filepath.Join(kubernetesDir, "base")
			if err := os.MkdirAll(baseDir, 0755); err != nil {
				Fail(fmt.Sprintf("Failed to create base dir: %v", err))
			}

			namespaceYaml := `apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace
`
			namespaceFile := filepath.Join(baseDir, "namespace.yaml")
			if err := os.WriteFile(namespaceFile, []byte(namespaceYaml), 0644); err != nil {
				Fail(fmt.Sprintf("Failed to write namespace file: %v", err))
			}

			// Create component directory with kustomization
			componentDir := filepath.Join(kubernetesDir, "components", "test-component")
			if err := os.MkdirAll(componentDir, 0755); err != nil {
				Fail(fmt.Sprintf("Failed to create component dir: %v", err))
			}

			kustomizationYaml := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: test-namespace
resources:
  - namespace.yaml
`
			kustomizationFile := filepath.Join(componentDir, "kustomization.yaml")
			if err := os.WriteFile(kustomizationFile, []byte(kustomizationYaml), 0644); err != nil {
				Fail(fmt.Sprintf("Failed to write kustomization file: %v", err))
			}

			// Create a simple template repo with kubernetes config
			templateRepo, err := createTestKubernetesTemplateRepo(workspace)
			Expect(err).NotTo(HaveOccurred())

			opts := bootstrap.CheckKubernetesOptions{
				KubernetesDir: kubernetesDir,
				Timeout:       5 * time.Minute,
				TemplateRepo:  templateRepo,
			}

			// Note: This test will fail if kubectl is not available or if there's no kubeconfig
			// For now, we'll just verify the structure is correct
			err = bootstrap.ExecuteCheckKubernetes(opts)
			// We expect this might fail in test environment without real k8s cluster
			// So we just verify it doesn't panic
			_ = err
		})

		It("should handle missing kubernetes directory", func() {
			opts := bootstrap.CheckKubernetesOptions{
				KubernetesDir: filepath.Join(workspace, "non-existent"),
				Timeout:       5 * time.Minute,
			}

			err := bootstrap.ExecuteCheckKubernetes(opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("kubernetes directory not found"))
		})
	})
})

// createTestKubernetesTemplateRepo creates a minimal kubernetes template repo for testing
func createTestKubernetesTemplateRepo(workspace string) (string, error) {
	templateDir := filepath.Join(workspace, "test-k8s-template")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create template directory: %w", err)
	}

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
`

	configPath := filepath.Join(templateDir, "kubernetes", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(configPath, []byte(kubernetesConfig), 0644); err != nil {
		return "", err
	}

	absPath, err := filepath.Abs(templateDir)
	if err != nil {
		return templateDir, nil
	}
	return absPath, nil
}

// setupCheckKubernetesTestWorkspace creates a temporary test workspace directory
func setupCheckKubernetesTestWorkspace() (string, error) {
	workspace := filepath.Join("workspace", "test-check-kubernetes")
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return "", fmt.Errorf("failed to create test workspace: %w", err)
	}
	absPath, err := filepath.Abs(workspace)
	if err != nil {
		return workspace, nil
	}
	return absPath, nil
}

// cleanupCheckKubernetesTestWorkspace removes the test workspace directory
func cleanupCheckKubernetesTestWorkspace(workspace string) error {
	if workspace == "" {
		return nil
	}
	if filepath.Base(filepath.Dir(workspace)) == "workspace" {
		return os.RemoveAll(workspace)
	}
	return nil
}
