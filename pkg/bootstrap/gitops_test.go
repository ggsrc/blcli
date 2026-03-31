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

var _ = Describe("BootstrapGitops", func() {
	var (
		workspace string
		loader    *template.Loader
	)

	BeforeEach(func() {
		var err error
		workspace, err = setupGitopsTestWorkspace()
		Expect(err).NotTo(HaveOccurred())

		_, loader, err = loadGitopsTestTemplateRepo()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		cleanupGitopsTestWorkspace(workspace)
	})

	Describe("BootstrapGitops", func() {
		It("should bootstrap gitops and create workspace/gitops/{project}/{app}/ files", func() {
			global := config.GlobalConfig{
				Workspace: workspace,
			}

			templateArgs := renderer.ArgsData{
				"gitops": map[string]interface{}{
					"argocd": map[string]interface{}{
						"project": []interface{}{"stg"},
					},
					"apps": []interface{}{
						map[string]interface{}{
							"name":  "hello-world",
							"kind":  "deployment",
							"image": "hello:1.0",
							"project": []interface{}{
								map[string]interface{}{
									"name": "stg",
									"parameters": map[string]interface{}{
										"ApplicationName":       "hello-world",
										"SourcePath":            "stg/hello-world",
										"SourceRepoURL":         "https://github.com/example/gitops.git",
										"ApplicationSyncPolicy": "Automatic",
										"ContainerArgs": []interface{}{
											"-listen=:8080",
											"-text=Hello from hello-world",
										},
									},
								},
							},
						},
					},
				},
			}

			err := bootstrap.BootstrapGitops(global, nil, loader, templateArgs)
			Expect(err).NotTo(HaveOccurred())

			gitopsDir := filepath.Join(workspace, "gitops")
			Expect(gitopsDir).To(BeADirectory())

			appDir := filepath.Join(gitopsDir, "stg", "hello-world")
			Expect(appDir).To(BeADirectory())

			appYaml := filepath.Join(appDir, "app.yaml")
			Expect(appYaml).To(BeAnExistingFile())
			appContent, err := os.ReadFile(appYaml)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(appContent)).To(ContainSubstring("server: https://kubernetes.default.svc"))
			Expect(string(appContent)).To(ContainSubstring("syncPolicy:"))
			Expect(string(appContent)).To(ContainSubstring("automated:"))
			Expect(string(appContent)).To(ContainSubstring("prune: true"))
			Expect(string(appContent)).To(ContainSubstring("selfHeal: true"))

			deploymentYaml := filepath.Join(appDir, "deployment.yaml")
			content, err := os.ReadFile(deploymentYaml)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("image: hello:1.0"))
			Expect(string(content)).To(ContainSubstring("args:"))
			Expect(string(content)).To(ContainSubstring(`"-listen=:8080"`))
			Expect(string(content)).To(ContainSubstring(`"-text=Hello from hello-world"`))
		})

		It("should return error when template loader is nil", func() {
			global := config.GlobalConfig{Workspace: workspace}
			templateArgs := renderer.ArgsData{
				"gitops": map[string]interface{}{
					"argocd": map[string]interface{}{"project": []interface{}{"stg"}},
					"apps":   []interface{}{},
				},
			}

			err := bootstrap.BootstrapGitops(global, nil, nil, templateArgs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("template repository is required"))
		})

		It("should return error when gitops section is missing", func() {
			global := config.GlobalConfig{Workspace: workspace}
			templateArgs := renderer.ArgsData{}

			err := bootstrap.BootstrapGitops(global, nil, loader, templateArgs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("gitops section required"))
		})

		It("should return error when gitops.argocd.project is empty", func() {
			global := config.GlobalConfig{Workspace: workspace}
			templateArgs := renderer.ArgsData{
				"gitops": map[string]interface{}{
					"argocd": map[string]interface{}{"project": []interface{}{}},
					"apps":   []interface{}{map[string]interface{}{"name": "x", "project": []interface{}{}}},
				},
			}

			err := bootstrap.BootstrapGitops(global, nil, loader, templateArgs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("argocd.project"))
		})
	})
})

// setupGitopsTestWorkspace creates a temporary test workspace directory under workspace/
func setupGitopsTestWorkspace() (string, error) {
	workspace := filepath.Join("workspace", "test-gitops-bootstrap")
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return "", fmt.Errorf("failed to create test workspace: %w", err)
	}
	absPath, err := filepath.Abs(workspace)
	if err != nil {
		return workspace, nil
	}
	return absPath, nil
}

// cleanupGitopsTestWorkspace removes the test workspace directory
func cleanupGitopsTestWorkspace(workspace string) error {
	if workspace == "" {
		return nil
	}
	if filepath.Base(filepath.Dir(workspace)) == "workspace" {
		return os.RemoveAll(workspace)
	}
	return nil
}

// loadGitopsTestTemplateRepo loads bl-template for gitops tests.
// Tries ../bl-template (when cwd is blcli-go) and ../../../bl-template (when cwd is pkg/bootstrap).
func loadGitopsTestTemplateRepo() (string, *template.Loader, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get cwd: %w", err)
	}
	for _, rel := range []string{filepath.Join("..", "bl-template"), filepath.Join("..", "..", "..", "bl-template")} {
		repoPath := filepath.Join(cwd, rel)
		absPath, err := filepath.Abs(repoPath)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err == nil {
			return absPath, template.NewLoader(absPath), nil
		}
	}
	return "", nil, fmt.Errorf("bl-template not found (tried from cwd %s)", cwd)
}
