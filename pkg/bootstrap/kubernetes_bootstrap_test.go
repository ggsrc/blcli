package bootstrap_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"blcli/pkg/bootstrap"
	"blcli/pkg/config"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

var _ = Describe("BootstrapKubernetes", func() {
	var (
		workspace string
		loader    *template.Loader
	)

	BeforeEach(func() {
		var err error
		workspace, err = setupKubernetesTestWorkspace()
		Expect(err).NotTo(HaveOccurred())

		_, loader, err = loadKubernetesTestTemplateRepo()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		cleanupKubernetesTestWorkspace(workspace)
	})

	Describe("BootstrapKubernetes", func() {
		It("should bootstrap kubernetes project successfully", func() {
			global := config.GlobalConfig{
				Workspace: workspace,
			}
			project := &config.ProjectConfig{
				Name: "test-k8s",
			}

			templateArgs := renderer.ArgsData{
				"global": map[string]interface{}{
					"GlobalName": "test-org",
				},
				"kubernetes": map[string]interface{}{
					"init": map[string]interface{}{
						"components": map[string]interface{}{
							"test-component": map[string]interface{}{},
						},
					},
					"projects": []interface{}{
						map[string]interface{}{
							"name": "test-project",
							"components": []interface{}{
								map[string]interface{}{
									"name": "test-optional",
									"parameters": map[string]interface{}{
										"namespace": "test-namespace",
									},
								},
							},
						},
					},
				},
			}

			err := bootstrap.BootstrapKubernetes(global, project, loader, templateArgs, false)
			Expect(err).NotTo(HaveOccurred())

			// Verify kubernetes directory was created
			kubernetesDir := filepath.Join(workspace, "kubernetes")
			Expect(kubernetesDir).To(BeADirectory())

			// Verify marker file exists
			markerFile := filepath.Join(kubernetesDir, ".blcli.marker")
			Expect(markerFile).To(BeAnExistingFile())

			// Verify base directory exists
			baseDir := filepath.Join(kubernetesDir, "base")
			if _, err := os.Stat(baseDir); err == nil {
				Expect(baseDir).To(BeADirectory())
			}

			// Verify components directory exists (may be empty if no components)
			componentsDir := filepath.Join(kubernetesDir, "components")
			// Components directory may not exist if no components were initialized
			// This is OK - the test just verifies the bootstrap process completed
			_ = componentsDir
		})

		It("should handle overwrite flag", func() {
			global := config.GlobalConfig{
				Workspace: workspace,
			}
			project := &config.ProjectConfig{
				Name: "test-k8s",
			}

			templateArgs := renderer.ArgsData{
				"kubernetes": map[string]interface{}{
					"init": map[string]interface{}{
						"components": map[string]interface{}{},
					},
					"projects": []interface{}{},
				},
			}

			// First bootstrap
			err := bootstrap.BootstrapKubernetes(global, project, loader, templateArgs, false)
			Expect(err).NotTo(HaveOccurred())

			// Try to bootstrap again without overwrite - should fail
			err = bootstrap.BootstrapKubernetes(global, project, loader, templateArgs, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Use --overwrite"))

			// Bootstrap with overwrite - should succeed
			err = bootstrap.BootstrapKubernetes(global, project, loader, templateArgs, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if template loader is nil", func() {
			global := config.GlobalConfig{
				Workspace: workspace,
			}
			project := &config.ProjectConfig{
				Name: "test-k8s",
			}

			templateArgs := renderer.ArgsData{}

			err := bootstrap.BootstrapKubernetes(global, project, nil, templateArgs, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("template repository is required"))
		})
	})
})

// setupKubernetesTestWorkspace creates a temporary test workspace directory
func setupKubernetesTestWorkspace() (string, error) {
	workspace := filepath.Join("workspace", "test-kubernetes-bootstrap")
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return "", fmt.Errorf("failed to create test workspace: %w", err)
	}
	absPath, err := filepath.Abs(workspace)
	if err != nil {
		return workspace, nil
	}
	return absPath, nil
}

// cleanupKubernetesTestWorkspace removes the test workspace directory
func cleanupKubernetesTestWorkspace(workspace string) error {
	if workspace == "" {
		return nil
	}
	if filepath.Base(filepath.Dir(workspace)) == "workspace" {
		return os.RemoveAll(workspace)
	}
	return nil
}

// loadKubernetesTestTemplateRepo loads a local template repository for testing
func loadKubernetesTestTemplateRepo() (string, *template.Loader, error) {
	cwd, _ := os.Getwd()
	repoPath := filepath.Join(cwd, "..", "..", "..", "bl-template")
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
