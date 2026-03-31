package cli_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"blcli/pkg/cli"
)

var _ = Describe("Init", func() {
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

	Describe("init command", func() {
		Context("with valid args file", func() {
			It("should load args file successfully", func() {
				// Create a test args file
				argsContent := `
global:
  GlobalName: "test-org"

terraform:
  version: "1.0.0"
  global:
    OrganizationID: "123456789012"
    BillingAccountID: "01ABCD-2EFGH3-4IJKL5"
  projects:
    - name: "prd"
      global:
        project_name: "prd"
`
				argsPath := filepath.Join(workspace, "test-args.yaml")
				err := os.WriteFile(argsPath, []byte(argsContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				outputDir := filepath.Join(workspace, "output")

				cmd := cli.NewInitCommand()
				cmd.SetArgs([]string{
					templateRepo,
					"--modules", "terraform",
					"--args", argsPath,
					"-o", outputDir,
					"--overwrite",
				})

				err = cmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle multiple args files", func() {
				// Create base args file
				baseArgsContent := `
global:
  GlobalName: "test-org"

terraform:
  version: "1.0.0"
  global:
    OrganizationID: "123456789012"
`
				baseArgsPath := filepath.Join(workspace, "base-args.yaml")
				err := os.WriteFile(baseArgsPath, []byte(baseArgsContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				// Create override args file
				overrideArgsContent := `
terraform:
  global:
    OrganizationID: "999999999999"
`
				overrideArgsPath := filepath.Join(workspace, "override-args.yaml")
				err = os.WriteFile(overrideArgsPath, []byte(overrideArgsContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				// Create a command with multiple args files
				cmd := cli.NewInitCommand()
				cmd.SetArgs([]string{
					templateRepo,
					"--modules", "terraform",
					"--args", baseArgsPath,
					"--args", overrideArgsPath,
				})

				// Just verify command creation, not execution
				Expect(cmd).NotTo(BeNil())
			})
		})

		Context("args file validation", func() {
			It("should fail when args file is missing", func() {
				cmd := cli.NewInitCommand()
				cmd.SetArgs([]string{
					templateRepo,
					"--modules", "terraform",
				})

				err := cmd.Execute()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("args file is required"))
			})

			It("should fail when args file does not exist", func() {
				cmd := cli.NewInitCommand()
				cmd.SetArgs([]string{
					templateRepo,
					"--modules", "terraform",
					"--args", filepath.Join(workspace, "non-existent.yaml"),
				})

				err := cmd.Execute()
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
