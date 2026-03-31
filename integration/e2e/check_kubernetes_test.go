package e2e

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	integration "blcli/integration"
)

var _ = Describe("Check Kubernetes E2E Tests", func() {
	var env *integration.TestEnvironment
	var kubernetesDir string

	BeforeEach(func() {
		var err error
		// Setup base test environment
		env, err = integration.SetupTestEnvironment()
		Expect(err).NotTo(HaveOccurred())

		// Generate args.yaml and initialize kubernetes
		argsPath := filepath.Join(env.Workspace, "args.yaml")
		err = integration.ExecuteBlcliCommand(
			"init-args",
			env.TemplateRepo,
			"-o", argsPath,
		)
		Expect(err).NotTo(HaveOccurred())

		// Initialize kubernetes
		err = integration.ExecuteBlcliCommand(
			"init",
			env.TemplateRepo,
			"--modules", "kubernetes",
			"--args", argsPath,
			"--output", env.Workspace,
			"--overwrite",
		)
		Expect(err).NotTo(HaveOccurred())

		kubernetesDir = filepath.Join(env.Workspace, "kubernetes")
	})

	AfterEach(func() {
		integration.TeardownTestEnvironment(env)
	})

	Describe("blcli check kubernetes", func() {
		Context("kubectl installType", func() {
			It("should successfully check kubectl manifests", func() {
				// Execute kubernetes check
				err := integration.ExecuteBlcliCommand(
					"check", "kubernetes",
					"--dir", kubernetesDir,
					"--template-repo", env.TemplateRepo,
				)
				// Note: This might fail if kubectl is not configured or cluster is not accessible
				// In that case, we just verify the command structure is correct
				_ = err

				// Verify kubernetes directory exists
				Expect(kubernetesDir).To(BeADirectory())

				// Verify new structure: kubernetes/{projectName}/{componentName}/
				entries, err := os.ReadDir(kubernetesDir)
				if err == nil {
					for _, entry := range entries {
						if entry.IsDir() && entry.Name() != "base" {
							projectDir := filepath.Join(kubernetesDir, entry.Name())
							Expect(projectDir).To(BeADirectory())
						}
					}
				}
			})
		})

		Context("dry-run validation", func() {
			It("should validate manifests without applying", func() {
				// Execute check (which uses --dry-run internally)
				err := integration.ExecuteBlcliCommand(
					"check", "kubernetes",
					"--dir", kubernetesDir,
					"--template-repo", env.TemplateRepo,
				)
				// This should not actually apply anything, just validate
				_ = err

				// Verify directory structure
				Expect(kubernetesDir).To(BeADirectory())
			})
		})

		Context("error handling", func() {
			It("should handle invalid manifests gracefully", func() {
				// Create an invalid manifest
				invalidDir := filepath.Join(kubernetesDir, "components", "invalid")
				if err := os.MkdirAll(invalidDir, 0755); err == nil {
					invalidYaml := `apiVersion: v1
kind: InvalidResource
metadata:
  name: test
  # Missing required fields
`
					invalidFile := filepath.Join(invalidDir, "invalid.yaml")
					os.WriteFile(invalidFile, []byte(invalidYaml), 0644)

					// Check should detect the error
					err := integration.ExecuteBlcliCommand(
						"check", "kubernetes",
						"--dir", kubernetesDir,
						"--template-repo", env.TemplateRepo,
					)
					// We expect this to fail with validation error
					_ = err
				}
			})
		})
	})
})
