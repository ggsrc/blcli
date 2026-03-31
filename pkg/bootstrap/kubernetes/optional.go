package kubernetes

import (
	"fmt"
	"path/filepath"

	"blcli/pkg/internal"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

// InitializeOptionalComponents initializes all optional components for a specific project
// Optional components are defined in the "optional" section of kubernetes/config.yaml
func InitializeOptionalComponents(
	kubernetesConfig *template.KubernetesConfig,
	templateLoader *template.Loader,
	templateArgs renderer.ArgsData,
	workspaceRoot string,
	data map[string]interface{},
	projectName string,
) error {
	// Get available optional components from templateArgs
	var componentNames []string
	if projectName == "" {
		// Get all components from all projects
		availableComponents := GetAvailableOptionalComponents(templateArgs)
		for compName := range availableComponents {
			componentNames = append(componentNames, compName)
		}
	} else {
		// Get components for specific project
		projectArgs := GetProjectArgs(templateArgs, projectName)
		if components, ok := projectArgs[renderer.FieldComponents]; ok {
			if componentsList, ok := components.([]interface{}); ok {
				for _, compItem := range componentsList {
					var compMap map[string]interface{}
					switch v := compItem.(type) {
					case renderer.ArgsData:
						compMap = map[string]interface{}(v)
					case map[string]interface{}:
						compMap = v
					case map[interface{}]interface{}:
						compMap = make(map[string]interface{})
						for k, val := range v {
							if keyStr, ok := k.(string); ok {
								compMap[keyStr] = val
							}
						}
					}
					if compMap != nil {
						if name, ok := compMap[renderer.FieldName]; ok {
							if nameStr, ok := name.(string); ok {
								componentNames = append(componentNames, nameStr)
							}
						}
					}
				}
			}
		}
	}

	// Filter to only include optional components (from optional section)
	optionalComponents := make(map[string]template.KubernetesComponent)
	if kubernetesConfig.Optional != nil {
		optionalComponents = kubernetesConfig.Optional
	}

	// Create filtered component map with only optional components
	componentMap := make(map[string]template.KubernetesComponent)
	for _, compName := range componentNames {
		if comp, exists := optionalComponents[compName]; exists {
			componentMap[compName] = comp
		}
	}

	// Resolve dependencies using all components (init + optional) for dependency resolution
	allComponents := kubernetesConfig.GetAllComponents()
	orderedComponents, err := kubernetesConfig.ResolveKubernetesDependencies(componentNames)
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Initialize components in dependency order
	for _, compName := range orderedComponents {
		component, exists := componentMap[compName]
		if !exists {
			// Check if it's in allComponents (might be a dependency from init section)
			if comp, ok := allComponents[compName]; ok {
				component = comp
			} else {
				continue // Skip if not found
			}
		}

		if len(component.Path) == 0 {
			continue // Skip if no paths specified
		}

		// Extract component-specific args
		var componentArgs renderer.ArgsData
		if projectName != "" {
			projectArgs := GetProjectArgs(templateArgs, projectName)
			componentArgs = ExtractComponentArgs(projectArgs, compName)
		} else {
			componentArgs = ExtractComponentArgs(templateArgs, compName)
		}

		componentDefaults, err := LoadComponentParameterDefaults(templateLoader, component, compName)
		if err != nil {
			return fmt.Errorf("failed to load args defaults for component %s: %w", compName, err)
		}
		componentArgs = MergeComponentArgsWithDefaults(componentArgs, componentDefaults)

		// Merge component args into data
		mergedData := make(map[string]interface{})
		for k, v := range data {
			mergedData[k] = v
		}
		for k, v := range componentArgs {
			mergedData[k] = v
		}

		// Determine output directory (new structure: kubernetes/{projectName}/{componentName})
		var componentDir string
		if projectName != "" {
			componentDir = filepath.Join(workspaceRoot, "kubernetes", projectName, compName)
		} else {
			componentDir = filepath.Join(workspaceRoot, "kubernetes", compName)
		}

		// Initialize component (reuse logic from init.go)
		if err := internal.EnsureDir(componentDir); err != nil {
			return fmt.Errorf("failed to create component dir %s: %w", componentDir, err)
		}

		basePath := GetComponentBasePath(component.Path)

		// Process each path in the Path list
		for _, path := range component.Path {
			// Load file content
			tmplContent, err := templateLoader.LoadTemplate(path)
			if err != nil {
				return fmt.Errorf("failed to load template %s: %w", path, err)
			}

			var rendered string
			if IsTemplatePath(path) {
				rendered, err = template.RenderWithArgs(tmplContent, mergedData, componentArgs)
				if err != nil {
					return fmt.Errorf("failed to render template %s: %w", path, err)
				}
			} else {
				rendered = tmplContent
			}

			var relPath string
			if basePath != "" && basePath != "." {
				relPath, err = filepath.Rel(basePath, path)
				if err != nil {
					relPath = filepath.Base(path)
				}
			} else {
				relPath = filepath.Base(path)
			}
			if IsTemplatePath(path) {
				relPath = filepath.Join(filepath.Dir(relPath), RemoveTmplExtension(filepath.Base(relPath)))
			}

			outputPath := filepath.Join(componentDir, relPath)
			if err := internal.EnsureDir(filepath.Dir(outputPath)); err != nil {
				return fmt.Errorf("failed to create component output dir for %s: %w", outputPath, err)
			}
			if err := internal.WriteFileIfAbsent(outputPath, rendered); err != nil {
				return fmt.Errorf("failed to write component file %s: %w", outputPath, err)
			}
		}

		if projectName != "" {
			fmt.Printf("Initialized kubernetes component: %s (project: %s) -> %s\n", compName, projectName, componentDir)
		} else {
			fmt.Printf("Initialized kubernetes component: %s -> %s\n", compName, componentDir)
		}
	}

	return nil
}
