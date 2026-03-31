package terraform

import (
	"fmt"
	"path/filepath"

	"blcli/pkg/config"
	"blcli/pkg/internal"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

// CopyModuleFiles copies and renders all template files from a module directory
func CopyModuleFiles(loader *template.Loader, modulePath string, targetDir string, args renderer.ArgsData, data map[string]interface{}) error {
	// First, list all .tmpl files in the module directory from cache
	// This establishes a "render plan" - we only render files that actually exist
	moduleFiles, err := loader.ListModuleFiles(modulePath)
	if err != nil {
		return fmt.Errorf("failed to list module files: %w", err)
	}

	// If no files found in cache, the module might not be cached yet
	// In this case, we can't establish a render plan, so return an error
	// The user should run with --force-update first to populate the cache
	if len(moduleFiles) == 0 {
		return fmt.Errorf("no template files found in cache for module %s. Run with --force-update to populate cache first", modulePath)
	}

	// Render each file that exists in the module
	for _, baseFileName := range moduleFiles {
		// Construct the full template path
		filePath := filepath.Join(modulePath, baseFileName+".tmpl")

		// Load template (should be in cache since we listed it)
		content, err := loader.LoadTemplate(filePath)
		if err != nil {
			// If loading fails, skip this file but continue with others
			fmt.Printf("Warning: failed to load template %s: %v\n", filePath, err)
			continue
		}

		// Render template
		rendered, err := template.RenderWithArgs(content, data, args)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", filePath, err)
		}

		// Write rendered file (without .tmpl extension)
		outputPath := filepath.Join(targetDir, baseFileName)
		if err := internal.WriteFileIfAbsent(outputPath, rendered); err != nil {
			return fmt.Errorf("failed to write %s: %w", outputPath, err)
		}
	}

	return nil
}

// InitializeModules initializes all terraform modules
func InitializeModules(terraformConfig *template.TerraformConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData, modulesDir string, global config.GlobalConfig, profiler Profiler) error {
	if profiler != nil {
		return profiler.TimeStep("Initialize terraform modules", func() error {
			return initializeModulesInternal(terraformConfig, templateLoader, templateArgs, modulesDir, global)
		})
	}
	return initializeModulesInternal(terraformConfig, templateLoader, templateArgs, modulesDir, global)
}

func initializeModulesInternal(terraformConfig *template.TerraformConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData, modulesDir string, global config.GlobalConfig) error {
	for moduleName, module := range terraformConfig.Modules {
		moduleDir := filepath.Join(modulesDir, moduleName)
		if err := internal.EnsureDir(moduleDir); err != nil {
			return fmt.Errorf("failed to create module dir %s: %w", moduleDir, err)
		}

		// Extract component-specific args for this module
		// Use module name directly as component name
		moduleArgs := ExtractComponentArgs(templateArgs, moduleName)

		// Load all files from module path (use first path if multiple)
		if len(module.Path) == 0 {
			fmt.Printf("Warning: module %s has no path specified, skipping\n", moduleName)
			continue
		}
		modulePath := module.Path[0] // For modules, typically only one path (directory)
		if err := CopyModuleFiles(templateLoader, modulePath, moduleDir, moduleArgs, PrepareTerraformModuleData(global)); err != nil {
			fmt.Printf("Warning: failed to copy module %s: %v (skipping, but continuing with other modules)\n", moduleName, err)
			continue
		}

		fmt.Printf("Initialized terraform module: %s at %s\n", moduleName, moduleDir)
	}
	return nil
}
