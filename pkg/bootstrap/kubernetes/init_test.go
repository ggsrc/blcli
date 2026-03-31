package kubernetes_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"blcli/pkg/bootstrap/kubernetes"
	"blcli/pkg/config"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

var _ = Describe("Kubernetes Init", func() {
	var (
		workspace string
		loader    *template.Loader
	)

	BeforeEach(func() {
		var err error
		workspace, err = createTestWorkspace()
		Expect(err).NotTo(HaveOccurred())

		// Load local template repository
		_, loader, err = loadTestTemplateRepo()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		cleanupTestWorkspace(workspace)
	})

	Describe("InitializeComponents", func() {
		It("should initialize components for a project", func() {
			// Load kubernetes config
			kubernetesConfig, err := loader.LoadKubernetesConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(kubernetesConfig).NotTo(BeNil())

			// Prepare template args with project components
			templateArgs := renderer.ArgsData{
				"global": map[string]interface{}{
					"GlobalName": "test-org",
				},
				"kubernetes": map[string]interface{}{
					"projects": []interface{}{
						map[string]interface{}{
							"name": "test-project",
							"components": []interface{}{
								map[string]interface{}{
									"name": "sealed-secret",
									"parameters": map[string]interface{}{
										"namespace": "sealed-secret",
									},
								},
							},
						},
					},
				},
			}

			// Prepare data
			global := config.GlobalConfig{
				Workspace: workspace,
			}
			data := kubernetes.PrepareKubernetesData(global, "test-project")

			// Initialize components
			err = kubernetes.InitializeComponents(kubernetesConfig, loader, templateArgs, workspace, data, "test-project", true)
			Expect(err).NotTo(HaveOccurred())

			// Verify component was created (new structure: kubernetes/{projectName}/{componentName})
			componentDir := filepath.Join(workspace, "kubernetes", "test-project", "sealed-secret")
			Expect(componentDir).To(BeADirectory())

			kustomizationPath := filepath.Join(componentDir, "kustomization.yaml")
			content, err := os.ReadFile(kustomizationPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.32.2/controller.yaml"))
			Expect(string(content)).NotTo(ContainSubstring("<no value>"))
		})

		It("should skip components not in project", func() {
			kubernetesConfig, err := loader.LoadKubernetesConfig()
			Expect(err).NotTo(HaveOccurred())

			templateArgs := renderer.ArgsData{
				"kubernetes": map[string]interface{}{
					"projects": []interface{}{
						map[string]interface{}{
							"name":       "test-project",
							"components": []interface{}{}, // No components
						},
					},
				},
			}

			global := config.GlobalConfig{Workspace: workspace}
			data := kubernetes.PrepareKubernetesData(global, "test-project")

			err = kubernetes.InitializeComponents(kubernetesConfig, loader, templateArgs, workspace, data, "test-project", true)
			Expect(err).NotTo(HaveOccurred())

			// Component should not be created (new structure: kubernetes/{projectName}/{componentName})
			componentDir := filepath.Join(workspace, "kubernetes", "test-project", "sealed-secret")
			_, err = os.Stat(componentDir)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})
	})
})

// loadTestTemplateRepo loads a local template repository for testing
func loadTestTemplateRepo() (string, *template.Loader, error) {
	// Try to find bl-template relative to current directory
	cwd, _ := os.Getwd()
	repoPath := filepath.Join(cwd, "..", "..", "..", "..", "bl-template")
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve template repo path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", nil, fmt.Errorf("template repository not found at %s", absPath)
	}

	loader := template.NewLoader(absPath)
	return absPath, loader, nil
}
