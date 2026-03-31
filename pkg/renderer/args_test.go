package renderer_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"blcli/pkg/renderer"
)

var _ = Describe("Args", func() {
	Describe("LoadArgs", func() {
		It("should load YAML args file", func() {
			argsContent := `
global:
  GlobalName: "test-org"

terraform:
  version: "1.0.0"
  global:
    OrganizationID: "123456789012"
`
			// Create a temporary file
			tmpFile := createTempFile("test-args.yaml", argsContent)
			defer os.Remove(tmpFile)

			args, err := renderer.LoadArgs(tmpFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(args).NotTo(BeNil())

			// Check that global section exists
			global := args.GetMap("global")
			Expect(global).NotTo(BeNil())
			Expect(global["GlobalName"]).To(Equal("test-org"))
		})

		It("should load TOML args file", func() {
			argsContent := `
[global]
GlobalName = "test-org"

[terraform]
version = "1.0.0"

[terraform.global]
OrganizationID = "123456789012"
`
			// Create a temporary file
			tmpFile := createTempFile("test-args.toml", argsContent)
			defer os.Remove(tmpFile)

			args, err := renderer.LoadArgs(tmpFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(args).NotTo(BeNil())
		})
	})

	Describe("MergeArgs", func() {
		It("should merge multiple args maps", func() {
			args1 := renderer.ArgsData{
				"global": map[string]interface{}{
					"GlobalName": "test-org",
				},
			}

			args2 := renderer.ArgsData{
				"global": map[string]interface{}{
					"GlobalName": "override-org",
				},
			}

			merged := renderer.MergeArgs(args1, args2)
			Expect(merged).NotTo(BeNil())

			global := merged.GetMap("global")
			Expect(global["GlobalName"]).To(Equal("override-org"))
		})
	})
})

// Helper function to create temporary files
func createTempFile(name, content string) string {
	tmpFile, err := os.CreateTemp("", name)
	if err != nil {
		Fail(fmt.Sprintf("Failed to create temp file: %v", err))
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		Fail(fmt.Sprintf("Failed to write temp file: %v", err))
	}

	if err := tmpFile.Close(); err != nil {
		Fail(fmt.Sprintf("Failed to close temp file: %v", err))
	}

	return tmpFile.Name()
}
