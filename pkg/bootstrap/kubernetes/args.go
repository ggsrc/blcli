package kubernetes

import (
	"regexp"
	"strings"

	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

var componentOrderingPrefixRegexp = regexp.MustCompile(`^[0-9]+-(.+)$`)

func normalizeComponentLookupName(name string) string {
	if matches := componentOrderingPrefixRegexp.FindStringSubmatch(name); len(matches) == 2 {
		return matches[1]
	}
	return name
}

func componentNamesMatch(actualName, targetName string) bool {
	return normalizeComponentLookupName(actualName) == normalizeComponentLookupName(targetName)
}

// toCamelCase converts kebab-case or snake_case to CamelCase
// Handles common abbreviations like ID, URL, API, etc. by keeping them uppercase
// Example: "project-id" -> "ProjectID", "secret-manager-service-account-email" -> "SecretManagerServiceAccountEmail"
func toCamelCase(s string) string {
	if s == "" {
		return s
	}

	// Common abbreviations that should be kept uppercase
	abbreviations := map[string]string{
		"id":     "ID",
		"url":    "URL",
		"api":    "API",
		"uid":    "UID",
		"gid":    "GID",
		"pid":    "PID",
		"ip":     "IP",
		"dns":    "DNS",
		"tls":    "TLS",
		"ssl":    "SSL",
		"http":   "HTTP",
		"https":  "HTTPS",
		"json":   "JSON",
		"yaml":   "YAML",
		"xml":    "XML",
		"csv":    "CSV",
		"jwt":    "JWT",
		"oauth":  "OAuth",
		"argocd": "ArgoCD",
		"saml":   "SAML",
		"aws":    "AWS",
		"gcp":    "GCP",
		"azure":  "Azure",
		"k8s":    "K8s",
		"gke":    "GKE",
		"eks":    "EKS",
		"aks":    "AKS",
	}

	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_'
	})

	result := ""
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		partLower := strings.ToLower(part)
		if abbrev, ok := abbreviations[partLower]; ok {
			result += abbrev
		} else {
			result += strings.ToUpper(part[:1]) + part[1:]
		}
	}

	return result
}

// ExtractComponentArgs extracts component-specific args and flattens them to top level
// This allows templates to use {{.Namespace}} instead of {{.components.istio.parameters.Namespace}}
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

	// Prefer already-scoped project components when present.
	// GetProjectArgs(project) populates result["components"] with that project's component list,
	// so we should not fall back to scanning kubernetes.projects across all projects first.
	if components, ok := templateArgs[renderer.FieldComponents]; ok {
		if componentsList, ok := components.([]interface{}); ok {
			for _, compItem := range componentsList {
				compMap := toMapStringInterface(compItem)
				if compMap == nil {
					continue
				}
				if compName, ok := compMap[renderer.FieldName]; ok {
					if compNameStr, ok := compName.(string); ok && componentNamesMatch(compNameStr, componentName) {
						if compParams, ok := compMap["parameters"]; ok {
							if paramsMap := toMapStringInterface(compParams); paramsMap != nil {
								for k, v := range paramsMap {
									result[k] = v
									camelKey := toCamelCase(k)
									if camelKey != k {
										if _, exists := paramsMap[camelKey]; exists {
											continue
										}
										result[camelKey] = v
									}
								}
							}
						}
						return result
					}
				}
			}
		}
	}

	// Extract component-specific args and flatten to top level
	// Look in kubernetes.projects[].components[] for the component
	if kubernetes, ok := templateArgs["kubernetes"]; ok {
		kubernetesMap := toMapStringInterface(kubernetes)
		if kubernetesMap != nil {
			if projects, ok := kubernetesMap[renderer.FieldProjects]; ok {
				if projectsList, ok := projects.([]interface{}); ok {
					// Search through all projects for the component
					for _, projectItem := range projectsList {
						projectMap := toMapStringInterface(projectItem)
						if projectMap == nil {
							continue
						}
						if components, ok := projectMap[renderer.FieldComponents]; ok {
							if componentsList, ok := components.([]interface{}); ok {
								for _, compItem := range componentsList {
									compMap := toMapStringInterface(compItem)
									if compMap == nil {
										continue
									}
									if compName, ok := compMap[renderer.FieldName]; ok {
										if compNameStr, ok := compName.(string); ok && componentNamesMatch(compNameStr, componentName) {
											// Found the component, extract parameters
											if compParams, ok := compMap["parameters"]; ok {
												if paramsMap := toMapStringInterface(compParams); paramsMap != nil {
													// Flatten component parameters to top level
													// Convert kebab-case keys to CamelCase for template compatibility
													for k, v := range paramsMap {
														// Add both original key and camelCase key for compatibility
														result[k] = v
														camelKey := toCamelCase(k)
														if camelKey != k {
															if _, exists := paramsMap[camelKey]; exists {
																continue
															}
															result[camelKey] = v
														}
													}
												}
											}
											return result
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

	// Fallback: try to get from components directly (old format)
	if components, ok := templateArgs[renderer.FieldComponents]; ok {
		if componentsMap, ok := components.(map[string]interface{}); ok {
			if componentData, ok := componentsMap[componentName]; ok {
				if componentMap, ok := componentData.(map[string]interface{}); ok {
					// Flatten component data to top level
					// Convert kebab-case keys to CamelCase for template compatibility
					for k, v := range componentMap {
						result[k] = v
						camelKey := toCamelCase(k)
						if camelKey != k {
							if _, exists := componentMap[camelKey]; exists {
								continue
							}
							result[camelKey] = v
						}
					}
				}
			}
		}
	}

	return result
}

// LoadComponentParameterDefaults loads default/example parameter values for a kubernetes component
// from the component's referenced args.yaml files.
func LoadComponentParameterDefaults(templateLoader *template.Loader, component template.KubernetesComponent, componentName string) (map[string]interface{}, error) {
	defaults := make(map[string]interface{})

	if templateLoader == nil || len(component.Args) == 0 {
		return defaults, nil
	}

	for _, argsPath := range component.Args {
		content, err := templateLoader.LoadTemplate(argsPath)
		if err != nil {
			return nil, err
		}

		def, err := renderer.LoadArgsDefinition(content)
		if err != nil {
			return nil, err
		}

		configValues, _ := def.ToConfigValues()
		componentsMap := toMapStringInterface(configValues[renderer.FieldComponents])
		if componentsMap == nil {
			continue
		}

		componentDefaults := toMapStringInterface(componentsMap[componentName])
		if componentDefaults == nil {
			continue
		}

		for k, v := range componentDefaults {
			defaults[k] = v
		}
	}

	return defaults, nil
}

// MergeComponentArgsWithDefaults overlays explicit component args on top of component defaults
// and ensures templates can access both original and CamelCase aliases.
func MergeComponentArgsWithDefaults(componentArgs renderer.ArgsData, defaults map[string]interface{}) renderer.ArgsData {
	merged := make(renderer.ArgsData)

	for k, v := range componentArgs {
		merged[k] = v
	}

	for k, v := range defaults {
		if _, exists := merged[k]; !exists {
			merged[k] = v
		}

		camelKey := toCamelCase(k)
		if _, exists := merged[camelKey]; !exists {
			merged[camelKey] = v
		}
	}

	return merged
}

// GetAvailableInitComponents gets available components from args
// Deprecated: Use GetAvailableOptionalComponents instead (new structure doesn't distinguish init/optional)
// Kept for backward compatibility
func GetAvailableInitComponents(templateArgs renderer.ArgsData) map[string]bool {
	// New structure: all components are in projects[].components[]
	// So we just use GetAvailableOptionalComponents
	return GetAvailableOptionalComponents(templateArgs)
}

// GetAvailableOptionalComponents gets available optional components from args
// Returns a map of component name to bool indicating if it's available
// Components are available if they are listed in kubernetes.projects[].components[]
func GetAvailableOptionalComponents(templateArgs renderer.ArgsData) map[string]bool {
	available := make(map[string]bool)

	// Check kubernetes.projects[].components[]
	if kubernetes, ok := templateArgs["kubernetes"]; ok {
		kubernetesMap := toMapStringInterface(kubernetes)
		if kubernetesMap != nil {
			if projects, ok := kubernetesMap[renderer.FieldProjects]; ok {
				if projectsList, ok := projects.([]interface{}); ok {
					// Search through all projects for components
					for _, projectItem := range projectsList {
						projectMap := toMapStringInterface(projectItem)
						if projectMap == nil {
							continue
						}
						if components, ok := projectMap[renderer.FieldComponents]; ok {
							if componentsList, ok := components.([]interface{}); ok {
								for _, compItem := range componentsList {
									compMap := toMapStringInterface(compItem)
									if compMap == nil {
										continue
									}
									if compName, ok := compMap[renderer.FieldName]; ok {
										if compNameStr, ok := compName.(string); ok {
											available[normalizeComponentLookupName(compNameStr)] = true
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

	return available
}

// GetKubernetesProjects gets the list of project names from templateArgs
func GetKubernetesProjects(templateArgs renderer.ArgsData) []string {
	projects := make([]string, 0)

	if kubernetes, ok := templateArgs["kubernetes"]; ok {
		kubernetesMap := toMapStringInterface(kubernetes)
		if kubernetesMap != nil {
			if projectsList, ok := kubernetesMap[renderer.FieldProjects]; ok {
				if projectsListArray, ok := projectsList.([]interface{}); ok {
					for _, projectItem := range projectsListArray {
						projectMap := toMapStringInterface(projectItem)
						if projectMap == nil {
							continue
						}
						if name, ok := projectMap[renderer.FieldName]; ok {
							if nameStr, ok := name.(string); ok {
								projects = append(projects, nameStr)
							}
						}
					}
				}
			}
		}
	}

	return projects
}

// GetProjectArgs extracts project-specific args from templateArgs
// Supports new format: kubernetes.projects[] with name, global, components
func GetProjectArgs(templateArgs renderer.ArgsData, projectName string) renderer.ArgsData {
	result := make(renderer.ArgsData)

	// Try new format: kubernetes.projects[]
	if kubernetes, ok := templateArgs["kubernetes"]; ok {
		kubernetesMap := toMapStringInterface(kubernetes)
		if kubernetesMap != nil {
			// Keep kubernetes section for parameter access
			result["kubernetes"] = kubernetes

			if projects, ok := kubernetesMap[renderer.FieldProjects]; ok {
				if projectsList, ok := projects.([]interface{}); ok {
					// Find project by name
					for _, projectItem := range projectsList {
						projectMap := toMapStringInterface(projectItem)
						if projectMap == nil {
							continue
						}

						// Check if this is the project we're looking for
						if name, ok := projectMap[renderer.FieldName]; ok {
							if nameStr, ok := name.(string); ok {
								if nameStr == projectName {
									// Found the project, merge its args
									// Merge global
									if global, ok := templateArgs[renderer.FieldGlobal]; ok {
										result[renderer.FieldGlobal] = global
									}
									// Merge kubernetes.global
									if k8sGlobal, ok := kubernetesMap[renderer.FieldGlobal]; ok {
										// Merge into result global
										if resultGlobal, ok := result[renderer.FieldGlobal]; ok {
											resultGlobalMap := toMapStringInterface(resultGlobal)
											k8sGlobalMap := toMapStringInterface(k8sGlobal)
											if resultGlobalMap != nil && k8sGlobalMap != nil {
												mergedGlobal := make(map[string]interface{})
												for k, v := range resultGlobalMap {
													mergedGlobal[k] = v
												}
												for k, v := range k8sGlobalMap {
													mergedGlobal[k] = v
												}
												result[renderer.FieldGlobal] = mergedGlobal
											}
										} else {
											result[renderer.FieldGlobal] = k8sGlobal
										}
									}
									// Merge project global
									if projGlobal, ok := projectMap[renderer.FieldGlobal]; ok {
										projGlobalMap := toMapStringInterface(projGlobal)
										if projGlobalMap != nil {
											if resultGlobal, ok := result[renderer.FieldGlobal]; ok {
												resultGlobalMap := toMapStringInterface(resultGlobal)
												if resultGlobalMap != nil {
													mergedGlobal := make(map[string]interface{})
													for k, v := range resultGlobalMap {
														mergedGlobal[k] = v
													}
													for k, v := range projGlobalMap {
														mergedGlobal[k] = v
													}
													result[renderer.FieldGlobal] = mergedGlobal
												}
											} else {
												result[renderer.FieldGlobal] = projGlobalMap
											}
										}
									}
									// Merge components
									if projComponents, ok := projectMap[renderer.FieldComponents]; ok {
										result[renderer.FieldComponents] = projComponents
									}
									return result
								}
							}
						}
					}
				}
			}
		}
	}

	// If no project-specific args, return base args
	if global, ok := templateArgs[renderer.FieldGlobal]; ok {
		result[renderer.FieldGlobal] = global
	}
	if kubernetes, ok := templateArgs["kubernetes"]; ok {
		result["kubernetes"] = kubernetes
	}

	return result
}
