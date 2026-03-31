package cli_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"blcli/pkg/cli"
	"blcli/pkg/template"
)

var _ = Describe("InitArgs", func() {
	var (
		workspace    string
		templateRepo string
	)

	BeforeEach(func() {
		var err error
		workspace, err = setupTestWorkspace()
		Expect(err).NotTo(HaveOccurred())

		// Load local template repository
		templateRepo, _, err = loadTestTemplateRepo()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if workspace != "" {
			cleanupTestWorkspace(workspace)
		}
	})

	Describe("generateArgsFile", func() {
		Context("with local template repository", func() {
			It("should generate YAML args file", func() {
				outputPath := filepath.Join(workspace, "args.yaml")

				// Create a command and execute it
				cmd := cli.NewInitArgsCommand()
				cmd.SetArgs([]string{
					templateRepo,
					"--output", outputPath,
					"--format", "yaml",
				})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())

				// Verify file exists
				_, err = os.Stat(outputPath)
				Expect(err).NotTo(HaveOccurred())

				// Verify file content
				content, err := os.ReadFile(outputPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("blcli args configuration file"))
				Expect(string(content)).To(ContainSubstring("terraform:"))

				envContent, err := os.ReadFile(filepath.Join(workspace, ".env"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(envContent)).To(ContainSubstring("BLCLI_TERRAFORM_ORGANIZATION_ID=123456789012"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_TERRAFORM_BILLING_ACCOUNT_ID=01ABCD-2EFGH3-4IJKL5"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_GITOPS_PRD_HELLO_WORLD_APPLICATION_REPO=https://github.com/your-org/hello-world.git"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_GITOPS_STG_HELLO_WORLD_APPLICATION_REPO=https://github.com/your-org/hello-world.git"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_GITOPS_STG_HELLO_WORLD_2_APPLICATION_REPO=https://github.com/your-org/hello-world-2.git"))
				Expect(string(envContent)).NotTo(ContainSubstring("BLCLI_ARGOCD_URL="))
				Expect(string(envContent)).NotTo(ContainSubstring("BLCLI_ARGOCD_DEX_GITHUB_CLIENT_ID="))
				Expect(string(envContent)).NotTo(ContainSubstring("BLCLI_ARGOCD_DEX_GITHUB_CLIENT_SECRET="))
				Expect(string(envContent)).NotTo(ContainSubstring("BLCLI_ARGOCD_DEX_GITHUB_ORGS="))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_ARGOCD_PRD_URL=https://app.example.com/argocd"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_ARGOCD_STG_URL=https://app.example.com/argocd"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_ARGOCD_CORP_URL=https://app.example.com/argocd"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_ARGOCD_PRD_DEX_GITHUB_CLIENT_ID=XXXXXXX"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_ARGOCD_STG_DEX_GITHUB_CLIENT_ID=XXXXXXX"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_ARGOCD_PRD_RBAC_GROUP_PREFIX=SiriusPlatform"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_ARGOCD_STG_RBAC_GROUP_PREFIX=SiriusPlatform"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_ARGOCD_CORP_DEX_GITHUB_CLIENT_ID=XXXXXXX"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_ARGOCD_CORP_DEX_GITHUB_CLIENT_SECRET=XXXXXXXX"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_ARGOCD_CORP_DEX_GITHUB_ORGS=example-org"))
				Expect(string(envContent)).To(ContainSubstring("BLCLI_ARGOCD_CORP_RBAC_GROUP_PREFIX=SiriusPlatform"))
			})

			It("should generate TOML args file", func() {
				outputPath := filepath.Join(workspace, "args.toml")

				// Create a command and execute it
				cmd := cli.NewInitArgsCommand()
				cmd.SetArgs([]string{
					templateRepo,
					"--output", outputPath,
					"--format", "toml",
				})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())

				// Verify file exists
				_, err = os.Stat(outputPath)
				Expect(err).NotTo(HaveOccurred())

				// Verify file content
				content, err := os.ReadFile(outputPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("blcli args configuration file"))
			})
		})

		Context("template loading", func() {
			It("should load terraform config from template repository", func() {
				loader := template.NewLoader(templateRepo)
				terraformConfig, err := loader.LoadTerraformConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(terraformConfig).NotTo(BeNil())
				Expect(terraformConfig.Version).NotTo(BeEmpty())
			})

			It("should load args.yaml files from template repository", func() {
				loader := template.NewLoader(templateRepo)

				// Try to load terraform/args.yaml
				content, err := loader.LoadTemplate("terraform/args.yaml")
				if err == nil {
					Expect(content).NotTo(BeEmpty())
				}
			})
		})
	})
})

// setupTestWorkspace creates a temporary test workspace directory
func setupTestWorkspace() (string, error) {
	workspace := filepath.Join("workspace", "test")
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return "", fmt.Errorf("failed to create test workspace: %w", err)
	}
	return workspace, nil
}

// cleanupTestWorkspace removes the test workspace directory
func cleanupTestWorkspace(workspace string) error {
	if workspace == "" {
		return nil
	}
	// Only clean up if it's in the workspace/test directory for safety
	if !filepath.IsAbs(workspace) && filepath.Dir(workspace) == "workspace" {
		return os.RemoveAll(workspace)
	}
	return nil
}

// loadTestTemplateRepo loads a local template repository for testing.
// It looks for bl-template relative to the current working directory.
// When tests run from blcli (go test ./pkg/cli/...), cwd may be blcli or blcli/pkg/cli.
func loadTestTemplateRepo() (string, *template.Loader, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get cwd: %w", err)
	}
	candidates := []string{
		filepath.Clean(filepath.Join(cwd, "bl-template")),
		filepath.Clean(filepath.Join(cwd, "..", "bl-template")),
		filepath.Clean(filepath.Join(cwd, "..", "..", "bl-template")),
		filepath.Clean(filepath.Join(cwd, "..", "..", "..", "bl-template")),
	}
	var repoPath string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			repoPath = p
			break
		}
	}
	if repoPath == "" {
		return "", nil, fmt.Errorf("template repository not found (tried: %v)", candidates)
	}
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve template repo path: %w", err)
	}
	loader := template.NewLoader(absPath)
	return absPath, loader, nil
}
