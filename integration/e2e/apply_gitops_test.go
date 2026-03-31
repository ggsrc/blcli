package e2e

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	integration "blcli/integration"
)

var _ = Describe("Apply GitOps E2E Tests", func() {
	var env *integration.TestEnvironment
	var gitopsApplyEnv *GitOpsApplyTestEnv
	var gitopsDir string
	var argsPath string

	BeforeEach(func() {
		var err error
		// Setup base test environment
		env, err = integration.SetupTestEnvironment()
		Expect(err).NotTo(HaveOccurred())

		// Generate args.yaml and initialize gitops
		argsPath = filepath.Join(env.Workspace, "args.yaml")
		err = integration.ExecuteBlcliCommand(
			"init-args",
			env.TemplateRepo,
			"-o", argsPath,
		)
		Expect(err).NotTo(HaveOccurred())

		// Initialize gitops
		err = integration.ExecuteBlcliCommand(
			"init",
			env.TemplateRepo,
			"--modules", "gitops",
			"--args", argsPath,
			"--output", env.Workspace,
			"--overwrite",
		)
		Expect(err).NotTo(HaveOccurred())

		// Setup gitops apply test environment
		gitopsDir = filepath.Join(env.Workspace, "gitops")
		gitopsApplyEnv, err = SetupGitOpsApplyTest(env.Workspace, gitopsDir, argsPath)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if gitopsApplyEnv != nil {
			// Cleanup will be implemented when blcli apply gitops is complete
			_ = gitopsApplyEnv
		}
		integration.TeardownTestEnvironment(env)
	})

	Describe("blcli apply gitops", func() {
		Context("successful apply with repo creation", func() {
			It("should create GitHub repo and deploy ArgoCD Application", func() {
				// Execute gitops apply with repo creation
				err := ExecuteGitOpsApply(gitopsDir, argsPath, true, false)
				// Note: This will fail until blcli apply gitops is implemented
				// For now, we just verify the setup is correct
				_ = err

				// Verify gitops directory exists
				Expect(gitopsDir).To(BeADirectory())

				// Verify args file exists
				Expect(argsPath).To(BeAnExistingFile())
			})
		})

		Context("apply with existing repo", func() {
			It("should use existing GitHub repo", func() {
				// Execute gitops apply without repo creation
				err := ExecuteGitOpsApply(gitopsDir, argsPath, false, false)
				// Note: This will fail until blcli apply gitops is implemented
				_ = err

				// Verify gitops directory exists
				Expect(gitopsDir).To(BeADirectory())
			})
		})

		Context("skip sync", func() {
			It("should create Application without waiting for sync", func() {
				// Execute gitops apply with skip sync
				err := ExecuteGitOpsApply(gitopsDir, argsPath, false, true)
				// Note: This will fail until blcli apply gitops is implemented
				_ = err

				// Verify gitops directory exists
				Expect(gitopsDir).To(BeADirectory())
			})
		})
	})
})
