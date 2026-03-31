package gitops_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	integration "blcli/integration"
)

var _ = Describe("GitOps Integration", func() {
	var env *integration.TestEnvironment

	BeforeEach(func() {
		var err error
		env, err = integration.SetupTestEnvironment()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		integration.TeardownTestEnvironment(env)
	})

	Describe("blcli apply gitops", func() {
		Context("successful apply", func() {
			It("should initialize and apply gitops configuration", func() {
				// 1. Create test args file with minimal terraform (required by config loader) and gitops.apps/argocd
				argsPath := filepath.Join(env.Workspace, "test-args.yaml")
				argsContent := `global:
  GlobalName: "test-org"
  Workspace: "` + env.Workspace + `"

terraform:
  version: "1.0.0"

gitops:
  argocd:
    project: ["stg"]
  apps:
    - name: hello-world
      kind: deployment
      project:
        - name: stg
          parameters:
            ApplicationName: hello-world
            SourcePath: stg/hello-world
            SourceRepoURL: https://github.com/example/gitops.git
`
				err := os.WriteFile(argsPath, []byte(argsContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				// 2. Execute init gitops
				err = integration.ExecuteBlcliCommand(
					"init",
					env.TemplateRepo,
					"--modules", "gitops",
					"--args", argsPath,
					"--output", env.Workspace,
					"--overwrite",
				)
				Expect(err).NotTo(HaveOccurred())

				// 3. Verify gitops files are generated
				gitopsDir := filepath.Join(env.Workspace, "gitops")
				Expect(gitopsDir).To(BeADirectory())

				appDir := filepath.Join(gitopsDir, "stg", "hello-world")
				Expect(appDir).To(BeADirectory())

				appYaml := filepath.Join(appDir, "app.yaml")
				Expect(appYaml).To(BeAnExistingFile())
			})
		})
	})
})
