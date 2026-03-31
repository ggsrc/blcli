package terraform

import (
	"fmt"
	"os"
	"path/filepath"

	"blcli/pkg/config"
)

// DestroyTerraform destroys Terraform projects
func DestroyTerraform(global config.GlobalConfig, tf *config.TerraformConfig) error {
	projects, err := GetTerraformProjectNames(tf)
	if err != nil {
		return err
	}

	workspace := config.WorkspacePath(global)
	// gcpDir is terraform/gcp/ (GlobalName is only used in Terraform backend prefix, not file system path)
	gcpDir := filepath.Join(workspace, "terraform", "gcp")

	for _, name := range projects {
		baseDir := filepath.Join(gcpDir, name)
		if _, err := os.Stat(baseDir); err != nil {
			fmt.Printf("Terraform project directory %s does not exist; skipping.\n", baseDir)
			continue
		}
		if err := os.RemoveAll(baseDir); err != nil {
			return fmt.Errorf("failed to remove terraform dir %s: %w", baseDir, err)
		}
		fmt.Printf("Destroyed terraform project at %s\n", baseDir)
	}

	initDir := filepath.Join(workspace, "terraform", "init")
	if _, err := os.Stat(initDir); err == nil {
		if err := os.RemoveAll(initDir); err != nil {
			return fmt.Errorf("failed to remove terraform init dir %s: %w", initDir, err)
		}
		fmt.Printf("Destroyed terraform init directory at %s\n", initDir)
	}

	modulesDir := filepath.Join(workspace, "terraform", "gcp", "modules")
	if _, err := os.Stat(modulesDir); err == nil {
		if err := os.RemoveAll(modulesDir); err != nil {
			return fmt.Errorf("failed to remove terraform modules dir %s: %w", modulesDir, err)
		}
		fmt.Printf("Destroyed terraform modules at %s\n", modulesDir)
	}

	// Cleanup empty directories
	cleanupEmpty := []string{
		filepath.Join(workspace, "terraform", "gcp"),
		filepath.Join(workspace, "terraform"),
	}
	for _, dir := range cleanupEmpty {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		if len(entries) == 0 {
			if err := os.Remove(dir); err != nil {
				fmt.Printf("Warning: failed to remove empty dir %s: %v\n", dir, err)
			}
		}
	}

	return nil
}
