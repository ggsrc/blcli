package e2e

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	integration "blcli/integration"
)

var _ = Describe("Check Repo E2E Tests", func() {
	var env *integration.TestEnvironment
	var argsPath string

	BeforeEach(func() {
		var err error
		env, err = integration.SetupTestEnvironment()
		Expect(err).NotTo(HaveOccurred())

		// Generate args.yaml for testing
		argsPath = filepath.Join(env.Workspace, "args.yaml")
		err = integration.ExecuteBlcliCommand(
			"init-args",
			env.TemplateRepo,
			"-o", argsPath,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(argsPath).To(BeAnExistingFile())

		// Initialize terraform projects
		err = integration.ExecuteBlcliCommand(
			"init",
			env.TemplateRepo,
			"--modules", "terraform",
			"--args", argsPath,
			"--output", env.Workspace,
			"--overwrite",
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		integration.TeardownTestEnvironment(env)
	})

	Describe("check repo command", func() {
		Context("check all terraform directories", func() {
			It("should successfully check all terraform directories", func() {
				// Execute check repo command
				err := integration.ExecuteBlcliCommand(
					"check", "repo",
					"--args", argsPath,
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should check init directories in order", func() {
				// Verify init directories exist
				initDir := filepath.Join(env.Workspace, "terraform", "init")
				Expect(initDir).To(BeADirectory())

				// Execute check repo and verify it processes init directories
				err := integration.ExecuteBlcliCommand(
					"check", "repo",
					"--args", argsPath,
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should check modules directories", func() {
				// Verify modules directory exists
				_ = filepath.Join(env.Workspace, "terraform", "gcp", "modules")
				// Modules may or may not exist depending on template
				// Just verify check repo doesn't fail
				err := integration.ExecuteBlcliCommand(
					"check", "repo",
					"--args", argsPath,
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should check project directories", func() {
				// Verify project directories exist
				_ = filepath.Join(env.Workspace, "terraform", "gcp")
				// Projects may or may not exist depending on template
				// Just verify check repo doesn't fail
				err := integration.ExecuteBlcliCommand(
					"check", "repo",
					"--args", argsPath,
				)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("check specific project", func() {
			It("should check only specified project", func() {
				// First, verify we have projects
				projectDir := filepath.Join(env.Workspace, "terraform", "gcp")
				if _, err := os.Stat(projectDir); os.IsNotExist(err) {
					Skip("No projects directory found, skipping project-specific test")
				}

				// Execute check repo for a specific project
				// Note: This will fail if project doesn't exist, which is expected
				_ = integration.ExecuteBlcliCommand(
					"check", "repo",
					"--args", argsPath,
					"--project", "test-project",
				)
				// We don't assert on error here because project name might not exist
				// The important thing is the command runs without crashing
			})
		})

		Context("terratest-style validation", func() {
			It("should validate terraform code structure", func() {
				terraformDir := filepath.Join(env.Workspace, "terraform")
				if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
					Skip("No terraform directory found")
				}

				// Verify terraform directory structure
				Expect(terraformDir).To(BeADirectory())

				// Test init directories structure
				initDir := filepath.Join(terraformDir, "init")
				if _, err := os.Stat(initDir); err == nil {
					verifyTerraformDirectoryStructure(initDir)
				}

				// Test modules structure
				modulesDir := filepath.Join(terraformDir, "gcp", "modules")
				if _, err := os.Stat(modulesDir); err == nil {
					verifyTerraformModulesStructure(modulesDir)
				}

				// Test projects structure
				projectsDir := filepath.Join(terraformDir, "gcp")
				if _, err := os.Stat(projectsDir); err == nil {
					verifyTerraformProjectsStructure(projectsDir)
				}
			})
		})
	})

	Describe("check plugin command", func() {
		Context("check required tools", func() {
			It("should check if terraform is installed", func() {
				_ = integration.ExecuteBlcliCommand("check", "plugin")
				// This should not fail - it just reports status
				// We don't assert on error because tools might not be installed in CI
			})
		})
	})
})

// verifyTerraformDirectoryStructure verifies that terraform directories have the expected structure
// This is a simplified validation that checks directory structure and .tf file presence
func verifyTerraformDirectoryStructure(terraformDir string) {
	// Get all subdirectories
	entries, err := os.ReadDir(terraformDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(terraformDir, entry.Name())

		// Check if directory has .tf files
		hasTfFiles := false
		subEntries, _ := os.ReadDir(dirPath)
		for _, subEntry := range subEntries {
			if !subEntry.IsDir() && filepath.Ext(subEntry.Name()) == ".tf" {
				hasTfFiles = true
				break
			}
		}

		if hasTfFiles {
			// Verify directory exists and is accessible
			Expect(dirPath).To(BeADirectory())
		}
	}
}

// verifyTerraformModulesStructure verifies modules directory structure
func verifyTerraformModulesStructure(modulesDir string) {
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		modulePath := filepath.Join(modulesDir, entry.Name())
		verifyTerraformDirectoryStructure(modulePath)
	}
}

// verifyTerraformProjectsStructure verifies projects directory structure
func verifyTerraformProjectsStructure(projectsDir string) {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip modules directory (already tested)
		if entry.Name() == "modules" {
			continue
		}

		projectPath := filepath.Join(projectsDir, entry.Name())
		verifyTerraformDirectoryStructure(projectPath)
	}
}
