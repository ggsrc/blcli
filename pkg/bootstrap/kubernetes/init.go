package kubernetes

import (
	"fmt"
	"path/filepath"
	"regexp"

	"blcli/pkg/internal"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// NormalizeComponentName removes numeric ordering prefixes like "0-" from component names.
// For example, "0-external-secrets-operator" becomes "external-secrets-operator".
// Exported for use by apply kubernetes (same directory names vs config.yaml logical names).
var componentNamePrefixRegexp = regexp.MustCompile(`^[0-9]+-(.+)$`)

func NormalizeComponentName(name string) string {
	if matches := componentNamePrefixRegexp.FindStringSubmatch(name); len(matches) == 2 {
		return matches[1]
	}
	return name
}

func normalizeComponentName(name string) string { return NormalizeComponentName(name) }

// InitializeComponents initializes all kubernetes components for a specific project
// workspaceRoot is the root directory of the workspace (where files will be generated)
// projectName is the name of the project (e.g., "prd", "stg"), empty string means initialize all components
// When overwrite is true, component files are overwritten with current template output; when false, existing files are left unchanged.
func InitializeComponents(
	kubernetesConfig *template.KubernetesConfig,
	templateLoader *template.Loader,
	templateArgs renderer.ArgsData,
	workspaceRoot string,
	data map[string]interface{},
	projectName string,
	overwrite bool,
) error {
	// Get available components from templateArgs
	// If projectName is empty, get all components from all projects
	// Otherwise, get components for the specific project
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
					// Handle both renderer.ArgsData and map[string]interface{}
					var compMap map[string]interface{}
					switch v := compItem.(type) {
					case renderer.ArgsData:
						compMap = map[string]interface{}(v)
					case map[string]interface{}:
						compMap = v
					default:
						// Try to convert using toMapStringInterface
						compMap = toMapStringInterface(compItem)
					}
					if compMap != nil {
						if name, ok := compMap["name"]; ok {
							if nameStr, ok := name.(string); ok {
								componentNames = append(componentNames, nameStr)
							}
						}
					}
				}
			}
		}
	}

	// Create a map for quick lookup (supports both new and old format)
	// For init components, prefer init.components over optional
	componentMap := make(map[string]template.KubernetesComponent)

	// First, add components from init.components (new format)
	if kubernetesConfig.Init != nil && kubernetesConfig.Init.Components != nil {
		for key, comp := range kubernetesConfig.Init.Components {
			if comp.Name == "" {
				comp.Name = key
			}
			componentMap[comp.Name] = comp
		}
	}

	// Then add from old format (backward compatibility)
	for _, comp := range kubernetesConfig.Components {
		// Only add if not already in componentMap (init.components takes precedence)
		if _, exists := componentMap[comp.Name]; !exists {
			componentMap[comp.Name] = comp
		}
	}

	// Normalize component names for dependency resolution.
	// Args 中的组件名带有排序前缀（例如 "0-external-secrets-operator"），而 config.yaml 中的组件名不带前缀
	//（例如 "external-secrets-operator"）。这里将用于依赖解析的名字统一为去前缀后的形式，
	// 同时保留从规范名到原始名的映射，后续在读取参数和输出目录时仍然使用原始名。
	normalizedNames := make([]string, 0, len(componentNames))
	logicalToOriginal := make(map[string]string, len(componentNames))
	for _, name := range componentNames {
		logical := normalizeComponentName(name)
		normalizedNames = append(normalizedNames, logical)
		if _, exists := logicalToOriginal[logical]; !exists {
			logicalToOriginal[logical] = name
		}
	}

	// Resolve dependencies
	orderedComponents, err := kubernetesConfig.ResolveKubernetesDependencies(normalizedNames)
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Initialize components in dependency order
	for _, logicalName := range orderedComponents {
		component, exists := componentMap[logicalName]
		if !exists {
			continue // Skip if component not found in config
		}

		originalName := logicalToOriginal[logicalName]
		if originalName == "" {
			originalName = logicalName
		}

		if len(component.Path) == 0 {
			continue // Skip if no paths specified
		}

		// Extract component-specific args
		var componentArgs renderer.ArgsData
		if projectName != "" {
			projectArgs := GetProjectArgs(templateArgs, projectName)
			componentArgs = ExtractComponentArgs(projectArgs, originalName)
		} else {
			componentArgs = ExtractComponentArgs(templateArgs, originalName)
		}

		componentDefaults, err := LoadComponentParameterDefaults(templateLoader, component, logicalName)
		if err != nil {
			return fmt.Errorf("failed to load args defaults for component %s: %w", logicalName, err)
		}
		componentArgs = MergeComponentArgsWithDefaults(componentArgs, componentDefaults)

		// Determine component directory
		// If projectName is specified, organize by project: kubernetes/{projectName}/{componentName}
		// Otherwise, use old structure: kubernetes/components/{componentName}
		var componentDir string
		if projectName != "" {
			componentDir = filepath.Join(workspaceRoot, "kubernetes", projectName, originalName)
		} else {
			componentDir = filepath.Join(workspaceRoot, "kubernetes", "components", originalName)
		}

		if err := internal.EnsureDir(componentDir); err != nil {
			return fmt.Errorf("failed to create component dir %s: %w", componentDir, err)
		}

		basePath := GetComponentBasePath(component.Path)

		// Process each path in the Path list
		for _, path := range component.Path {
			// Load file content (from template repo or cache)
			tmplContent, err := templateLoader.LoadTemplate(path)
			if err != nil {
				return fmt.Errorf("failed to load template %s: %w", path, err)
			}

			var rendered string
			if IsTemplatePath(path) {
				// Only .tmpl files are rendered as Go templates; others are copied as-is
				// (e.g. alert-rules/*.yaml contain Prometheus {{ $labels }} syntax for VM, not Go template)
				rendered, err = template.RenderWithArgs(tmplContent, data, componentArgs)
				if err != nil {
					return fmt.Errorf("failed to render template %s: %w", path, err)
				}
			} else {
				rendered = tmplContent
			}

			// Output path: preserve subdir structure (e.g. alert-rules/vmoperator.yaml) so kustomize can find them
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
			if overwrite {
				if err := internal.WriteFile(outputPath, rendered); err != nil {
					return fmt.Errorf("failed to write component file %s: %w", outputPath, err)
				}
			} else {
				if err := internal.WriteFileIfAbsent(outputPath, rendered); err != nil {
					return fmt.Errorf("failed to write component file %s: %w", outputPath, err)
				}
			}
		}

		if projectName != "" {
			fmt.Printf("Initialized kubernetes component: %s (project: %s) -> %s\n", originalName, projectName, componentDir)
		} else {
			fmt.Printf("Initialized kubernetes component: %s -> %s\n", originalName, componentDir)
		}
	}

	return nil
}
