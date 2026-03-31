package kubernetes_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"blcli/pkg/bootstrap/kubernetes"
	"blcli/pkg/config"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

var _ = Describe("Kubernetes Optional", func() {
	var (
		workspace string
		loader    *template.Loader
	)

	BeforeEach(func() {
		var err error
		workspace, err = createTestWorkspace()
		Expect(err).NotTo(HaveOccurred())

		_, loader, err = loadTestTemplateRepo()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		cleanupTestWorkspace(workspace)
	})

	Describe("InitializeOptionalComponents", func() {
		It("should initialize optional components for a project", func() {
			// Load kubernetes config
			kubernetesConfig, err := loader.LoadKubernetesConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(kubernetesConfig).NotTo(BeNil())

			// Prepare template args with optional component (use actual component from config: argocd)
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
									"name": "argocd",
									"parameters": map[string]interface{}{
										"namespace":           "test-namespace",
										"argocd-url":          "http://einsli.com:28080/argocd",
										"argocd-manifest-url": "https://raw.githubusercontent.com/argoproj/argo-cd/v2.11.7/manifests/install.yaml",
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

			// Initialize optional components
			err = kubernetes.InitializeOptionalComponents(kubernetesConfig, loader, templateArgs, workspace, data, "test-project")
			Expect(err).NotTo(HaveOccurred())

			// Verify optional component was created (new structure: kubernetes/{projectName}/{componentName})
			componentDir := filepath.Join(workspace, "kubernetes", "test-project", "argocd")
			Expect(componentDir).To(BeADirectory())

			kustomizationPath := filepath.Join(componentDir, "kustomization.yaml")
			kustomizationContent, err := os.ReadFile(kustomizationPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(kustomizationContent)).To(ContainSubstring("https://raw.githubusercontent.com/argoproj/argo-cd/v2.11.7/manifests/install.yaml"))
			Expect(string(kustomizationContent)).NotTo(ContainSubstring(".tmpl"))
			Expect(string(kustomizationContent)).To(ContainSubstring("argocd-cmd-params-cm.yaml"))
			Expect(string(kustomizationContent)).To(ContainSubstring("app-namespace.yaml"))

			appNamespacePath := filepath.Join(componentDir, "app-namespace.yaml")
			appNamespaceContent, err := os.ReadFile(appNamespacePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(appNamespaceContent)).To(ContainSubstring("name: app"))

			argocdCMPath := filepath.Join(componentDir, "argocd-cm.yaml")
			argocdCMContent, err := os.ReadFile(argocdCMPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(argocdCMContent)).To(ContainSubstring("url: http://einsli.com:28080/argocd"))
		})

		It("should skip optional components not in project", func() {
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

			err = kubernetes.InitializeOptionalComponents(kubernetesConfig, loader, templateArgs, workspace, data, "test-project")
			Expect(err).NotTo(HaveOccurred())

			// Optional component should not be created (argocd not in project)
			// New structure: kubernetes/{projectName}/{componentName}
			componentDir := filepath.Join(workspace, "kubernetes", "test-project", "argocd")
			_, err = os.Stat(componentDir)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})
	})
})
