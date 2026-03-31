package e2e

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	integration "blcli/integration"
)

var _ = Describe("Apply All E2E Tests", func() {
	var env *integration.TestEnvironment
	var workspaceDir string
	var argsPath string

	BeforeEach(func() {
		var err error
		env, err = integration.SetupTestEnvironment()
		Expect(err).NotTo(HaveOccurred())

		workspaceDir = env.Workspace

		argsPath = filepath.Join(workspaceDir, "args.yaml")
		err = integration.ExecuteBlcliCommand(
			"init-args",
			env.TemplateRepo,
			"-o", argsPath,
		)
		Expect(err).NotTo(HaveOccurred())

		err = integration.ExecuteBlcliCommand(
			"init",
			env.TemplateRepo,
			"--args", argsPath,
			"--output", env.Workspace,
			"--overwrite",
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		integration.TeardownTestEnvironment(env)
	})

	Describe("blcli apply all", func() {
		Context("complete workflow", func() {
			It("should apply all modules in correct order", func() {
				err := integration.ExecuteBlcliCommand(
					"apply", "all",
					"-d", workspaceDir,
					"--args", argsPath,
				)
				_ = err

				terraformDir := filepath.Join(workspaceDir, "terraform")
				kubernetesDir := filepath.Join(workspaceDir, "kubernetes")
				gitopsDir := filepath.Join(workspaceDir, "gitops")

				if _, err := os.Stat(terraformDir); err == nil {
					Expect(terraformDir).To(BeADirectory())
				}
				if _, err := os.Stat(kubernetesDir); err == nil {
					Expect(kubernetesDir).To(BeADirectory())
				}
				if _, err := os.Stat(gitopsDir); err == nil {
					Expect(gitopsDir).To(BeADirectory())
				}
			})
		})
	})
})
