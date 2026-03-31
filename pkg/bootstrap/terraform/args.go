package terraform

import (
	"blcli/pkg/config"
	"blcli/pkg/renderer"
)

// toMapStringInterface converts various map types to map[string]interface{}
// This is a common pattern when dealing with YAML/TOML decoded data which can be
// map[string]interface{}, map[interface{}]interface{}, or renderer.ArgsData
func toMapStringInterface(v interface{}) map[string]interface{} {
	switch m := v.(type) {
	case map[string]interface{}:
		return m
	case renderer.ArgsData:
		return map[string]interface{}(m)
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for k, val := range m {
			if keyStr, ok := k.(string); ok {
				result[keyStr] = val
			}
		}
		return result
	default:
		return nil
	}
}

// PrepareTerraformInitData prepares data map for terraform init templates
// All parameters should come from args.yaml (via templateArgs), which will be automatically
// merged by RenderWithArgs. Returns empty map as all values come from templateArgs.
func PrepareTerraformInitData(global config.GlobalConfig, tf *config.TerraformConfig, templateArgs renderer.ArgsData) map[string]interface{} {
	// All parameters (GlobalName, OrganizationID, BillingAccountID, Version, etc.)
	// should be provided in args.yaml and will be automatically merged by RenderWithArgs
	return map[string]interface{}{}
}

// PrepareTerraformProjectData prepares data map for terraform project templates
// All parameters should come from templateArgs, including ProjectName (from terraform.projects[].global.ProjectName)
func PrepareTerraformProjectData(global config.GlobalConfig, projectName string, tf *config.TerraformConfig) map[string]interface{} {
	// All parameters should come from templateArgs via RenderWithArgs
	// ProjectName should be in terraform.projects[].global.ProjectName
	return map[string]interface{}{}
}

// PrepareTerraformModuleData prepares data map for terraform module templates
// All parameters should come from templateArgs
func PrepareTerraformModuleData(global config.GlobalConfig) map[string]interface{} {
	// All parameters should come from templateArgs via RenderWithArgs
	return map[string]interface{}{}
}

// ExtractProjectComponentArgs extracts component-specific args for a project-level component
// This allows project templates to use {{.GlobalName}} instead of {{.components.backend.parameters.GlobalName}}
func ExtractProjectComponentArgs(projectArgs renderer.ArgsData, componentName string) renderer.ArgsData {
	result := make(renderer.ArgsData)

	// Get components from project args
	if components, ok := projectArgs[renderer.FieldComponents]; ok {
		componentsMap := toMapStringInterface(components)
		if componentsMap == nil {
			return result
		}

		if componentData, ok := componentsMap[componentName]; ok {
			// componentData should be the parameters map directly (from mergeProjectArgsFromNewFormat)
			componentParams := toMapStringInterface(componentData)
			if componentParams != nil {
				// Flatten component parameters to top level
				for k, v := range componentParams {
					result[k] = v
				}
			}
		}
	}

	return result
}

// ExtractComponentArgs extracts component-specific args and flattens them to top level
// This allows templates to use {{project}} instead of {{.components.ssl-certificate.project}}
func ExtractComponentArgs(templateArgs renderer.ArgsData, componentName string) renderer.ArgsData {
	result := make(renderer.ArgsData)

	// Copy global args
	if global, ok := templateArgs[renderer.FieldGlobal]; ok {
		result[renderer.FieldGlobal] = global
		// Also flatten global to top level for easier access
		if globalMap, ok := global.(map[string]interface{}); ok {
			for k, v := range globalMap {
				result[k] = v
			}
		}
	}

	// Extract component-specific args and flatten to top level
	if components, ok := templateArgs[renderer.FieldComponents]; ok {
		if componentsMap, ok := components.(map[string]interface{}); ok {
			if componentData, ok := componentsMap[componentName]; ok {
				if componentMap, ok := componentData.(map[string]interface{}); ok {
					// Flatten component data to top level
					for k, v := range componentMap {
						result[k] = v
					}
				}
			}
		}
	}

	return result
}

// GetProjectArgs extracts project-specific args from templateArgs
// Supports both old format (projects.<projectName>) and new format (terraform.projects[])
// New format: terraform.projects[] with name, global, components
// Old format: projects.<projectName> with global, components
func GetProjectArgs(templateArgs renderer.ArgsData, projectName string) renderer.ArgsData {
	// Try new format first: terraform.projects[]
	if terraform, ok := templateArgs[renderer.FieldTerraform]; ok {
		terraformMap := toMapStringInterface(terraform)
		if terraformMap == nil {
			return getProjectArgsFallback(templateArgs, projectName)
		}

		if projects, ok := terraformMap[renderer.FieldProjects]; ok {
			if projectsList, ok := projects.([]interface{}); ok {
				// Find project by name
				for _, projectItem := range projectsList {
					projectMap := toMapStringInterface(projectItem)
					if projectMap == nil {
						continue
					}

					// Check if this is the project we're looking for
					if name, ok := projectMap[renderer.FieldName]; ok {
						if nameStr, ok := name.(string); ok && nameStr == projectName {
							// Found the project, merge its args
							return mergeProjectArgsFromNewFormat(templateArgs, projectMap)
						}
					}
				}
			}
		}
	}

	return getProjectArgsFallback(templateArgs, projectName)
}

// getProjectArgsFallback handles fallback to old format or returns base args
func getProjectArgsFallback(templateArgs renderer.ArgsData, projectName string) renderer.ArgsData {
	// Fall back to old format: projects.<projectName>
	if projects, ok := templateArgs[renderer.FieldProjects]; ok {
		projectsMap := toMapStringInterface(projects)
		if projectsMap != nil {
			if projectData, ok := projectsMap[projectName]; ok {
				projectMap := toMapStringInterface(projectData)
				if projectMap != nil {
					return mergeProjectArgsFromOldFormat(templateArgs, projectMap)
				}
			}
		}
	}

	// If no project-specific args, return original but keep global and terraform sections
	// We need global and terraform.global for parameter access
	result := make(renderer.ArgsData)
	for k, v := range templateArgs {
		if k != renderer.FieldProjects {
			// Keep everything except projects, including terraform and global
			result[k] = v
		}
	}
	return result
}

// mergeProjectArgsFromNewFormat merges project args from new format (terraform.projects[])
func mergeProjectArgsFromNewFormat(templateArgs renderer.ArgsData, projectMap map[string]interface{}) renderer.ArgsData {
	projectArgs := make(renderer.ArgsData)

	// Start with base args (global, terraform.global, components from top level)
	baseArgs := make(renderer.ArgsData)
	for k, v := range templateArgs {
		if k != renderer.FieldTerraform {
			baseArgs[k] = v
		}
	}

	// IMPORTANT: Keep terraform section in projectArgs so RenderWithArgs can access terraform.global
	// This is needed for flattening terraform.global parameters to top level
	if terraform, ok := templateArgs[renderer.FieldTerraform]; ok {
		projectArgs[renderer.FieldTerraform] = terraform
	}

	// Merge top-level global and terraform.global as base global
	// Priority: top-level global (contains GlobalName) + terraform.global (contains terraform-specific params)
	var baseGlobal map[string]interface{}

	// Start with top-level global (contains GlobalName from bl-template/args.yaml)
	if topLevelGlobal, ok := templateArgs[renderer.FieldGlobal]; ok {
		topLevelGlobalMap := toMapStringInterface(topLevelGlobal)
		if topLevelGlobalMap != nil {
			baseGlobal = make(map[string]interface{})
			for k, v := range topLevelGlobalMap {
				baseGlobal[k] = v
			}
		}
	}

	// Merge terraform.global on top (terraform-specific params override top-level if same key)
	if terraform, ok := templateArgs[renderer.FieldTerraform]; ok {
		terraformMap := toMapStringInterface(terraform)
		if terraformMap != nil {
			if tfGlobal, ok := terraformMap[renderer.FieldGlobal]; ok {
				tfGlobalMap := toMapStringInterface(tfGlobal)
				if tfGlobalMap != nil {
					if baseGlobal == nil {
						baseGlobal = make(map[string]interface{})
					}
					// Merge terraform.global into baseGlobal (terraform params override top-level)
					for k, v := range tfGlobalMap {
						baseGlobal[k] = v
					}
				}
			}
		}
	}

	// Merge project global with base global
	// baseGlobal contains GlobalName and terraform params, projGlobal contains project_name and project_id
	// project_name and project_id are always set from project's name and id for template rendering
	projectName, _ := projectMap[renderer.FieldName].(string)
	projectID, _ := projectMap["id"].(string)
	if projectID == "" {
		projectID = projectName
	}
	mergedGlobal := make(map[string]interface{})
	if baseGlobal != nil {
		for k, v := range baseGlobal {
			mergedGlobal[k] = v
		}
	}
	if projGlobal, ok := projectMap[renderer.FieldGlobal]; ok {
		projGlobalMap := toMapStringInterface(projGlobal)
		if projGlobalMap != nil {
			for k, v := range projGlobalMap {
				mergedGlobal[k] = v
			}
		}
	}
	// Always set project_name and project_id for template rendering
	mergedGlobal["project_name"] = projectName
	mergedGlobal["project_id"] = projectID
	if len(mergedGlobal) > 0 {
		projectArgs[renderer.FieldGlobal] = mergedGlobal
	}

	// Merge components: start with base components, then override with project-specific
	allComponents := make(map[string]interface{})
	// Get base components from top level
	if baseComponents, ok := baseArgs[renderer.FieldComponents]; ok {
		if baseComponentsMap, ok := baseComponents.(map[string]interface{}); ok {
			for k, v := range baseComponentsMap {
				allComponents[k] = v
			}
		}
	}
	// Get project components (new format: components is a list of ComponentData)
	if projComponents, ok := projectMap[renderer.FieldComponents]; ok {
		// Handle list format (from YAML parsing as []interface{})
		if projComponentsList, ok := projComponents.([]interface{}); ok {
			// Convert list format to map format
			for _, compItem := range projComponentsList {
				// Handle map[string]interface{} (from yaml.v3)
				if compMap, ok := compItem.(map[string]interface{}); ok {
					if compName, ok := compMap[renderer.FieldName]; ok {
						if compNameStr, ok := compName.(string); ok {
							if compParams, ok := compMap["parameters"]; ok {
								allComponents[compNameStr] = compParams
							}
						}
					}
				} else if compMap, ok := compItem.(map[interface{}]interface{}); ok {
					// Handle map[interface{}]interface{} (from some YAML parsers)
					var compNameStr string
					var compParams interface{}
					for k, v := range compMap {
						if keyStr, ok := k.(string); ok {
							if keyStr == renderer.FieldName {
								if nameStr, ok := v.(string); ok {
									compNameStr = nameStr
								}
							} else if keyStr == "parameters" {
								compParams = v
							}
						}
					}
					if compNameStr != "" && compParams != nil {
						allComponents[compNameStr] = compParams
					}
				} else if compMap, ok := compItem.(renderer.ArgsData); ok {
					// Handle renderer.ArgsData type
					if compName, ok := compMap[renderer.FieldName]; ok {
						if compNameStr, ok := compName.(string); ok {
							if compParams, ok := compMap["parameters"]; ok {
								allComponents[compNameStr] = compParams
							}
						}
					}
				}
			}
		} else if projComponentsMap, ok := projComponents.(map[string]interface{}); ok {
			// Also support map format for backward compatibility
			for k, v := range projComponentsMap {
				allComponents[k] = v
			}
		} else if projComponentsMap, ok := projComponents.(map[interface{}]interface{}); ok {
			// Handle map[interface{}]interface{} format
			for k, v := range projComponentsMap {
				if keyStr, ok := k.(string); ok {
					allComponents[keyStr] = v
				}
			}
		}
	}
	if len(allComponents) > 0 {
		projectArgs[renderer.FieldComponents] = allComponents
	}

	return projectArgs
}

// mergeProjectArgsFromOldFormat merges project args from old format (projects.<projectName>)
func mergeProjectArgsFromOldFormat(templateArgs renderer.ArgsData, projectMap map[string]interface{}) renderer.ArgsData {
	projectArgs := make(renderer.ArgsData)

	// Start with a copy of the original templateArgs
	baseArgs := make(renderer.ArgsData)
	for k, v := range templateArgs {
		if k != renderer.FieldProjects {
			baseArgs[k] = v
		}
	}

	// Merge project-specific global (if exists) into base global
	if projGlobal, ok := projectMap[renderer.FieldGlobal]; ok {
		if baseGlobal, ok := baseArgs[renderer.FieldGlobal]; ok {
			// Merge project global into base global
			mergedGlobal := renderer.MergeArgs(
				renderer.ArgsData{renderer.FieldGlobal: baseGlobal},
				renderer.ArgsData{renderer.FieldGlobal: projGlobal},
			)
			projectArgs[renderer.FieldGlobal] = mergedGlobal[renderer.FieldGlobal]
		} else {
			projectArgs[renderer.FieldGlobal] = projGlobal
		}
	} else {
		// Use base global if no project-specific global
		if baseGlobal, ok := baseArgs[renderer.FieldGlobal]; ok {
			projectArgs[renderer.FieldGlobal] = baseGlobal
		}
	}

	// Merge components: start with base components, then override with project-specific
	allComponents := make(map[string]interface{})
	if baseComponents, ok := baseArgs[renderer.FieldComponents]; ok {
		if baseComponentsMap, ok := baseComponents.(map[string]interface{}); ok {
			for k, v := range baseComponentsMap {
				allComponents[k] = v
			}
		}
	}
	if projComponents, ok := projectMap[renderer.FieldComponents]; ok {
		if projComponentsMap, ok := projComponents.(map[string]interface{}); ok {
			// Merge project components (they override base components)
			for k, v := range projComponentsMap {
				allComponents[k] = v
			}
		}
	}
	if len(allComponents) > 0 {
		projectArgs[renderer.FieldComponents] = allComponents
	}

	return projectArgs
}

// ExtractInitItemArgs extracts args for init items from templateArgs
// It looks for args in global, terraform.global, and terraform.init.components.<componentName>
// and flattens them to top level
// This allows templates to use {{.AtlantisName}} instead of {{.terraform.init.components.atlantis.AtlantisName}}
// Supports multiple args files: if argsPaths is a list, loads and merges all files
func ExtractInitItemArgs(templateArgs renderer.ArgsData, argsPaths []string) renderer.ArgsData {
	return ExtractInitItemArgsForComponent(templateArgs, argsPaths, "")
}

// ExtractInitItemArgsForComponent extracts args for a specific init component
// componentName is the name of the init component (e.g., "backend", "projects", "atlantis")
func ExtractInitItemArgsForComponent(templateArgs renderer.ArgsData, argsPaths []string, componentName string) renderer.ArgsData {
	result := make(renderer.ArgsData)

	// Start with base templateArgs
	for k, v := range templateArgs {
		result[k] = v
	}

	// Extract from top-level global (flatten to top level)
	if global, ok := templateArgs[renderer.FieldGlobal]; ok {
		if globalMap := toMapStringInterface(global); globalMap != nil {
			for k, v := range globalMap {
				result[k] = v
			}
		}
	}

	// Extract from terraform.global (flatten to top level)
	if terraform, ok := templateArgs[renderer.FieldTerraform]; ok {
		if terraformMap := toMapStringInterface(terraform); terraformMap != nil {
			if tfGlobal, ok := terraformMap[renderer.FieldGlobal]; ok {
				if globalMap := toMapStringInterface(tfGlobal); globalMap != nil {
					// Flatten terraform.global to top level for easier template access
					for k, v := range globalMap {
						result[k] = v
					}
				}
			}

			// Extract from terraform.init.components.<componentName> (flatten to top level)
			if componentName != "" {
				if init, ok := terraformMap[renderer.FieldInit]; ok {
					if initMap := toMapStringInterface(init); initMap != nil {
						if components, ok := initMap[renderer.FieldComponents]; ok {
							if componentsMap := toMapStringInterface(components); componentsMap != nil {
								if componentData, ok := componentsMap[componentName]; ok {
									// Flatten component parameters to top level
									if componentParams := toMapStringInterface(componentData); componentParams != nil {
										for k, v := range componentParams {
											result[k] = v
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return result
}
