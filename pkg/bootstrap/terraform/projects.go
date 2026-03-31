package terraform

import (
	"fmt"
	"path/filepath"

	"blcli/pkg/config"
	"blcli/pkg/internal"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

// InitializeProjects initializes all terraform projects.
// subdirComponents: project -> components that should be rendered to project/component/ subdir (for cross-project deps).
func InitializeProjects(terraformConfig *template.TerraformConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData, gcpDir string, projects []string, global config.GlobalConfig, tf *config.TerraformConfig, subdirComponents map[string][]string, profiler Profiler) ([]string, error) {
	// Resolve dependencies
	orderedProjects, err := terraformConfig.ResolveDependencies()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project dependencies: %w", err)
	}

	var initialized []string

	if profiler != nil {
		err := profiler.TimeStep("Initialize terraform projects", func() error {
			var err error
			initialized, err = initializeProjectsInternal(terraformConfig, templateLoader, templateArgs, gcpDir, projects, global, tf, orderedProjects, subdirComponents)
			return err
		})
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		initialized, err = initializeProjectsInternal(terraformConfig, templateLoader, templateArgs, gcpDir, projects, global, tf, orderedProjects, subdirComponents)
		if err != nil {
			return nil, err
		}
	}

	return initialized, nil
}

func initializeProjectsInternal(terraformConfig *template.TerraformConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData, gcpDir string, projects []string, global config.GlobalConfig, tf *config.TerraformConfig, orderedProjects []template.ProjectItem, subdirComponents map[string][]string) ([]string, error) {
	var initialized []string

	// For each configured project name, create project directory
	for _, projectName := range projects {
		projectDir := filepath.Join(gcpDir, projectName)
		if err := internal.EnsureDir(projectDir); err != nil {
			return nil, fmt.Errorf("failed to create project dir %s: %w", projectDir, err)
		}

		// Get project-specific args for this project
		projectArgs := GetProjectArgs(templateArgs, projectName)

		// Ensure projectArgs has global and terraform sections for proper parameter access
		// Add global from projectArgs or templateArgs if missing
		if _, ok := projectArgs[renderer.FieldGlobal]; !ok {
			// Try to get from terraform.global first, then top-level global
			if terraform, ok := templateArgs[renderer.FieldTerraform]; ok {
				if terraformMap, ok := terraform.(map[string]interface{}); ok {
					if tfGlobal, ok := terraformMap[renderer.FieldGlobal]; ok {
						projectArgs[renderer.FieldGlobal] = tfGlobal
					}
				}
			}
			// Fall back to top-level global
			if _, ok := projectArgs[renderer.FieldGlobal]; !ok {
				if global, ok := templateArgs[renderer.FieldGlobal]; ok {
					projectArgs[renderer.FieldGlobal] = global
				}
			}
		}
		// Add terraform section if missing (needed for terraform.global flattening)
		if _, ok := projectArgs[renderer.FieldTerraform]; !ok {
			if terraform, ok := templateArgs[renderer.FieldTerraform]; ok {
				projectArgs[renderer.FieldTerraform] = terraform
			}
		}

		// Get available components for this project (only render components that are specified)
		availableComponents := GetAvailableComponents(projectArgs)

		if len(availableComponents) == 0 {
			fmt.Printf("Warning: No components found for project %s\n", projectName)
		} else {
			compNames := make([]string, 0, len(availableComponents))
			for name := range availableComponents {
				compNames = append(compNames, name)
			}
			fmt.Printf("Available components for project %s: %v\n", projectName, compNames)
		}

		// Process each project item from config in dependency order
		for _, projectItem := range orderedProjects {
			// Skip if this project item is not in the components list
			if !IsComponentAvailable(projectItem.Name, availableComponents) {
				fmt.Printf("Skipping project item %s (not in available components)\n", projectItem.Name)
				continue
			}

			fmt.Printf("Rendering project item: %s\n", projectItem.Name)

			// Handle multiple paths in Path list
			if len(projectItem.Path) == 0 {
				continue // Skip if no paths specified
			}

			// Process each path in the Path list
			for _, path := range projectItem.Path {
				// Load template
				tmplContent, err := templateLoader.LoadTemplate(path)
				if err != nil {
					return nil, fmt.Errorf("failed to load template %s: %w", path, err)
				}

				// Prepare data for rendering
				data := PrepareTerraformProjectData(global, projectName, tf)

				// Extract component-specific args for this project item
				componentArgs := ExtractProjectComponentArgs(projectArgs, projectItem.Name)

				// Merge component args into project args for rendering
				mergedArgs := make(renderer.ArgsData)
				for k, v := range projectArgs {
					mergedArgs[k] = v
				}
				// Flatten component-specific parameters to top level
				for k, v := range componentArgs {
					mergedArgs[k] = v
				}

				// Render template with project-specific args
				content, err := template.RenderWithArgs(tmplContent, data, mergedArgs)
				if err != nil {
					return nil, fmt.Errorf("failed to render template %s: %w", path, err)
				}

				// Write rendered content: to subdir project/component/ if this component has cross-project deps
				outputDir := projectDir
				if IsComponentInSubdir(subdirComponents, projectName, projectItem.Name) {
					outputDir = filepath.Join(projectDir, projectItem.Name)
					if err := internal.EnsureDir(outputDir); err != nil {
						return nil, fmt.Errorf("failed to create component subdir %s: %w", outputDir, err)
					}
				}
				outputPath := filepath.Join(outputDir, filepath.Base(path))
				outputPath = RemoveTmplExtension(outputPath)
				if err := internal.WriteFileIfAbsent(outputPath, content); err != nil {
					return nil, fmt.Errorf("failed to write project file %s: %w", outputPath, err)
				}
			}
		}

		fmt.Printf("Initialized terraform project: %s at %s\n", projectName, projectDir)
		initialized = append(initialized, projectName)
	}

	return initialized, nil
}
