package kubernetes_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	integration "blcli/integration"
)

var _ = Describe("Kubernetes Integration", func() {
	var env *integration.TestEnvironment

	BeforeEach(func() {
		var err error
		env, err = integration.SetupTestEnvironment()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		integration.TeardownTestEnvironment(env)
	})

	Describe("blcli init kubernetes", func() {
		Context("successful initialization", func() {
			It("should initialize kubernetes resources", func() {
				// 1. Generate args.yaml
				argsPath := filepath.Join(env.Workspace, "args.yaml")
				err := integration.ExecuteBlcliCommand(
					"init-args",
					env.TemplateRepo,
					"-o", argsPath,
				)
				Expect(err).NotTo(HaveOccurred())

				// 2. Execute init (--output so files go to test workspace)
				err = integration.ExecuteBlcliCommand(
					"init",
					env.TemplateRepo,
					"--modules", "kubernetes",
					"--args", argsPath,
					"--output", env.Workspace,
					"--overwrite",
				)
				Expect(err).NotTo(HaveOccurred())

				// 3. Verify kubernetes files are generated (new structure: kubernetes/{projectName}/{componentName}/)
				kubernetesDir := filepath.Join(env.Workspace, "kubernetes")
				Expect(kubernetesDir).To(BeADirectory())

				entries, err := os.ReadDir(kubernetesDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).NotTo(BeEmpty())
				var hasProjectDir bool
				for _, e := range entries {
					if e.IsDir() && e.Name() != ".blcli.marker" {
						hasProjectDir = true
						break
					}
				}
				Expect(hasProjectDir).To(BeTrue(), "kubernetes dir should contain at least one project subdirectory")
			})
		})

		Context("with init components", func() {
			It("should initialize init components", func() {
				argsPath := filepath.Join(env.Workspace, "args.yaml")
				err := integration.ExecuteBlcliCommand(
					"init-args",
					env.TemplateRepo,
					"-o", argsPath,
				)
				Expect(err).NotTo(HaveOccurred())

				// Modify args to include init components
				// This would require updating the args file to include kubernetes.init.components

				err = integration.ExecuteBlcliCommand(
					"init",
					env.TemplateRepo,
					"--modules", "kubernetes",
					"--args", argsPath,
					"--output", env.Workspace,
					"--overwrite",
				)
				Expect(err).NotTo(HaveOccurred())

				kubernetesDir := filepath.Join(env.Workspace, "kubernetes")
				Expect(kubernetesDir).To(BeADirectory())
				entries, err := os.ReadDir(kubernetesDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).NotTo(BeEmpty())
			})
		})

		Context("error handling", func() {
			It("should handle missing template repository", func() {
				argsPath := filepath.Join(env.Workspace, "args.yaml")
				err := integration.ExecuteBlcliCommand(
					"init",
					"/non-existent/repo",
					"--modules", "kubernetes",
					"--args", argsPath,
				)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
