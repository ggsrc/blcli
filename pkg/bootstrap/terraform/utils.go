package terraform

import (
	"fmt"
	"sort"
	"strings"

	"blcli/pkg/config"
	"blcli/pkg/renderer"
)

// GetTerraformProjectNames extracts project names from terraform config
func GetTerraformProjectNames(tf *config.TerraformConfig) ([]string, error) {
	if tf == nil {
		return nil, fmt.Errorf("terraform config missing")
	}

	var projects []string
	if len(tf.Projects) > 0 {
		projects = append(projects, tf.Projects...)
	} else if tf.Name != "" {
		projects = append(projects, tf.Name)
	} else {
		return nil, fmt.Errorf("no terraform.projects or terraform.name configured; nothing to initialize")
	}

	sort.Strings(projects)
	projects = DedupeStrings(projects)
	return projects, nil
}

// DedupeStrings removes duplicate strings from a slice
func DedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var result []string
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}

// GenerateDefaultProjectServices generates default GCP services for projects based on project name patterns
// This provides a reasonable default set of services that most projects need
func GenerateDefaultProjectServices(projectIDs map[string]string) map[string][]string {
	services := make(map[string][]string)

	// Common services that all projects typically need
	commonServices := []string{
		"cloudresourcemanager.googleapis.com",
	}

	// Services for production/staging/corp environments
	envServices := []string{
		"compute.googleapis.com",
		"container.googleapis.com",
		"iamcredentials.googleapis.com",
		"dns.googleapis.com",
		"secretmanager.googleapis.com",
		"iap.googleapis.com",
		"logging.googleapis.com",
	}

	// Services for static resources
	staticResourcesServices := []string{
		"compute.googleapis.com",
		"container.googleapis.com",
		"iamcredentials.googleapis.com",
		"dns.googleapis.com",
	}

	// Services for internal artifacts (artifact registry)
	internalArtifactsServices := []string{
		"iamcredentials.googleapis.com",
		"artifactregistry.googleapis.com",
	}

	// Services for proxies
	proxyServices := []string{
		"compute.googleapis.com",
		"iamcredentials.googleapis.com",
		"dns.googleapis.com",
	}

	for projectName := range projectIDs {
		var projectServices []string

		// Determine services based on project name patterns
		switch {
		case strings.Contains(projectName, "internal-artifacts") || strings.Contains(projectName, "artifacts"):
			// Internal artifacts projects
			projectServices = append(projectServices, commonServices...)
			projectServices = append(projectServices, internalArtifactsServices...)
		case strings.Contains(projectName, "static-resources") || strings.Contains(projectName, "static"):
			// Static resources projects
			projectServices = append(projectServices, commonServices...)
			projectServices = append(projectServices, staticResourcesServices...)
		case strings.Contains(projectName, "proxy") || strings.Contains(projectName, "proxies"):
			// Proxy projects
			projectServices = append(projectServices, commonServices...)
			projectServices = append(projectServices, proxyServices...)
		case strings.Contains(projectName, "corp") || strings.Contains(projectName, "stg") || strings.Contains(projectName, "prd") || strings.Contains(projectName, "prod") || strings.Contains(projectName, "staging"):
			// Production/staging/corp environments
			projectServices = append(projectServices, commonServices...)
			projectServices = append(projectServices, envServices...)
			// Add additional services for staging/production
			if strings.Contains(projectName, "stg") || strings.Contains(projectName, "staging") {
				projectServices = append(projectServices,
					"alloydb.googleapis.com",
					"servicenetworking.googleapis.com",
					"redis.googleapis.com",
					"certificatemanager.googleapis.com",
					"cloudidentity.googleapis.com",
					"serviceusage.googleapis.com",
					"networkmanagement.googleapis.com",
				)
			} else if strings.Contains(projectName, "prd") || strings.Contains(projectName, "prod") {
				projectServices = append(projectServices,
					"alloydb.googleapis.com",
					"servicenetworking.googleapis.com",
					"certificatemanager.googleapis.com",
					"redis.googleapis.com",
				)
			} else if strings.Contains(projectName, "corp") {
				projectServices = append(projectServices,
					"certificatemanager.googleapis.com",
				)
			}
		default:
			// Default: provide basic services for any other project
			projectServices = append(projectServices, commonServices...)
			projectServices = append(projectServices,
				"compute.googleapis.com",
				"container.googleapis.com",
				"iamcredentials.googleapis.com",
			)
		}

		// Deduplicate services
		seen := make(map[string]struct{})
		var deduplicated []string
		for _, svc := range projectServices {
			if _, ok := seen[svc]; !ok {
				seen[svc] = struct{}{}
				deduplicated = append(deduplicated, svc)
			}
		}

		services[projectName] = deduplicated
	}

	return services
}

// RemoveTmplExtension removes .tmpl extension from a path
func RemoveTmplExtension(path string) string {
	if strings.HasSuffix(path, ".tmpl") {
		return strings.TrimSuffix(path, ".tmpl")
	}
	return path
}

// IsStandardTerraformFileName checks if the name is a standard terraform file name
func IsStandardTerraformFileName(name string) bool {
	standardNames := []string{
		"backend", "variables", "outputs", "provider", "main", "versions",
		"terraform", "providers", "locals", "data", "resources",
	}
	for _, std := range standardNames {
		if name == std {
			return true
		}
	}
	return false
}

// GetAvailableComponents extracts available component names from project args
// Returns a set (map[string]bool) of component names that are specified in the args
func GetAvailableComponents(projectArgs renderer.ArgsData) map[string]bool {
	available := make(map[string]bool)

	// Get components from project args
	if components, ok := projectArgs[renderer.FieldComponents]; ok {
		// Handle map[string]interface{} format
		if componentsMap, ok := components.(map[string]interface{}); ok {
			// Components is a map, keys are component names
			for compName := range componentsMap {
				available[compName] = true
			}
		} else if componentsMap, ok := components.(map[interface{}]interface{}); ok {
			// Handle map[interface{}]interface{} format (from YAML parsing)
			for compName := range componentsMap {
				if compNameStr, ok := compName.(string); ok {
					available[compNameStr] = true
				}
			}
		} else if componentsList, ok := components.([]interface{}); ok {
			// Components is a list of ComponentData
			for _, compItem := range componentsList {
				if compMap, ok := compItem.(map[string]interface{}); ok {
					if compName, ok := compMap[renderer.FieldName]; ok {
						if compNameStr, ok := compName.(string); ok {
							available[compNameStr] = true
						}
					}
				}
			}
		}
	}

	return available
}

// IsComponentAvailable checks if a component name is in the available components set
func IsComponentAvailable(componentName string, availableComponents map[string]bool) bool {
	return availableComponents[componentName]
}
