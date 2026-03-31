package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	k8s "blcli/pkg/bootstrap/kubernetes"
	"blcli/pkg/config"
	"blcli/pkg/internal"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

// BootstrapKubernetes bootstraps Kubernetes projects based on template repository config
func BootstrapKubernetes(global config.GlobalConfig, project *config.ProjectConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData, overwrite bool) error {
	workspace := config.WorkspacePath(global)
	kubernetesDir := filepath.Join(workspace, "kubernetes")

	// Check if kubernetes directory exists and has blcli marker
	if exists, err := internal.CheckBlcliMarker(kubernetesDir); err == nil && exists {
		if !overwrite {
			return fmt.Errorf("kubernetes directory at %s was created by blcli. Use --overwrite to allow overwriting", kubernetesDir)
		}
	}

	if templateLoader == nil {
		return fmt.Errorf("template repository is required for kubernetes bootstrap")
	}

	// Load kubernetes config from template repository
	kubernetesConfig, err := templateLoader.LoadKubernetesConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubernetes config: %w", err)
	}

	// Create kubernetes directory and marker file
	if err := internal.EnsureDir(kubernetesDir); err != nil {
		return fmt.Errorf("failed to create kubernetes dir: %w", err)
	}

	// Create marker file to indicate this directory was created by blcli
	if err := internal.CreateBlcliMarker(kubernetesDir); err != nil {
		return fmt.Errorf("failed to create blcli marker: %w", err)
	}

	if err := runKubernetesInitScript(templateLoader, templateArgs, workspace, kubernetesDir, overwrite); err != nil {
		return err
	}

	// Initialize components for each project
	projects := k8s.GetKubernetesProjects(templateArgs)
	if len(projects) == 0 {
		fmt.Println("No kubernetes projects found in args, skipping component initialization")
	} else {
		var errors []error
		for _, projectName := range projects {
			projectData := k8s.PrepareKubernetesData(global, projectName)
			if err := k8s.InitializeComponents(kubernetesConfig, templateLoader, templateArgs, workspace, projectData, projectName, overwrite); err != nil {
				errors = append(errors, fmt.Errorf("project %s: %w", projectName, err))
				// Continue with next project instead of returning
				continue
			}
		}
		// If all projects failed, return error
		if len(errors) > 0 && len(errors) == len(projects) {
			return fmt.Errorf("all projects failed to initialize: %v", errors)
		}
		// If some projects failed, log warnings but don't fail
		if len(errors) > 0 {
			fmt.Printf("Warning: %d out of %d projects failed to initialize completely\n", len(errors), len(projects))
			for _, err := range errors {
				fmt.Printf("  - %v\n", err)
			}
		}
	}

	return nil
}

// DestroyKubernetes destroys Kubernetes projects
func DestroyKubernetes(global config.GlobalConfig, project *config.ProjectConfig) error {
	workspace := config.WorkspacePath(global)
	kubernetesDir := filepath.Join(workspace, "kubernetes")

	// If directory does not exist, nothing to do.
	if _, err := os.Stat(kubernetesDir); os.IsNotExist(err) {
		return nil
	}

	// Best-effort cleanup: delete applied manifests by running kubectl/helm
	// based on directory layout: kubernetes/{projectName}/{componentName}
	entries, err := os.ReadDir(kubernetesDir)
	if err != nil {
		return fmt.Errorf("failed to read kubernetes dir %s: %w", kubernetesDir, err)
	}

	ctx := context.Background()

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		projectName := e.Name()
		if strings.EqualFold(projectName, "base") {
			// Base namespace / shared resources: delete via kubectl if kustomization exists.
			baseDir := filepath.Join(kubernetesDir, projectName)
			args := []string{"delete", "-k", baseDir}
			cmd := exec.CommandContext(ctx, "kubectl", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Printf("Warning: failed to delete base kubernetes resources in %s: %v\n", baseDir, err)
			}
			continue
		}

		projectDir := filepath.Join(kubernetesDir, projectName)
		componentEntries, err := os.ReadDir(projectDir)
		if err != nil {
			fmt.Printf("Warning: failed to read kubernetes project dir %s: %v\n", projectDir, err)
			continue
		}

		for _, ce := range componentEntries {
			if !ce.IsDir() {
				continue
			}
			componentDir := filepath.Join(projectDir, ce.Name())
			// Prefer kustomize delete; fallback to delete -f dir.
			kustomizationPath := filepath.Join(componentDir, "kustomization.yaml")
			args := []string{"delete"}
			if _, statErr := os.Stat(kustomizationPath); statErr == nil {
				args = append(args, "-k", componentDir)
			} else {
				args = append(args, "-f", componentDir)
			}
			cmd := exec.CommandContext(ctx, "kubectl", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Printf("Warning: failed to delete kubernetes component %s/%s: %v\n", projectName, ce.Name(), err)
			} else {
				fmt.Printf("Deleted kubernetes component %s/%s\n", projectName, ce.Name())
			}
		}
	}

	// Finally, remove generated files on disk.
	if err := os.RemoveAll(kubernetesDir); err != nil {
		return fmt.Errorf("failed to remove kubernetes dir %s: %w", kubernetesDir, err)
	}

	return nil
}
