package config

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"

	"blcli/pkg/renderer"
)

type GlobalConfig struct {
	Name             string `toml:"name"`
	Domain           string `toml:"domain"`
	Version          string `toml:"version"`
	Workspace        string `toml:"workspace"`
	BillingAccountID string `toml:"billing_account_id"`
	OrganizationID   string `toml:"organization_id"`
	TerraformVersion string `toml:"terraform_version"`
	KubectlVersion   string `toml:"kubectl_version"`
}

type ProjectConfig struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
}

type TerraformConfig struct {
	Projects []string `toml:"projects"`
	Name     string   `toml:"name"`
	Version  string   `toml:"version"`
}

type BlcliConfig struct {
	Global     GlobalConfig     `toml:"global"`
	Terraform  *TerraformConfig `toml:"terraform"`
	Kubernetes *ProjectConfig   `toml:"kubernetes"`
	Gitops     *ProjectConfig   `toml:"gitops"`
}

// Load loads configuration from a TOML file (legacy support)
func Load(path string) (BlcliConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return BlcliConfig{}, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg BlcliConfig
	if err := toml.Unmarshal(raw, &cfg); err != nil {
		return BlcliConfig{}, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	return applyDefaults(cfg), nil
}

// LoadFromArgs loads configuration from args file (new approach)
// Supports both new format (global, terraform.projects[]) and old format (blcli section)
func LoadFromArgs(argsData renderer.ArgsData) (BlcliConfig, error) {
	var cfg BlcliConfig

	// Try new format first: global and terraform at top level
	if globalSection, ok := argsData[renderer.FieldGlobal]; ok {
		if globalMap, ok := globalSection.(map[string]interface{}); ok {
			cfg.Global = extractGlobalConfig(globalMap)
		}
	}

	// Extract terraform config from new format
	if tfSection, ok := argsData[renderer.FieldTerraform]; ok {
		// Convert to map[string]interface{} - handle both ArgsData and map types
		var tfMap map[string]interface{}
		switch v := tfSection.(type) {
		case renderer.ArgsData:
			tfMap = map[string]interface{}(v)
		case map[string]interface{}:
			tfMap = v
		case map[interface{}]interface{}:
			// Convert map[interface{}]interface{} to map[string]interface{}
			tfMap = make(map[string]interface{})
			for k, val := range v {
				if keyStr, ok := k.(string); ok {
					tfMap[keyStr] = val
				}
			}
		}
		if tfMap != nil {
			// Try terraform.global for global config (overrides top-level global)
			if tfGlobal, ok := tfMap[renderer.FieldGlobal]; ok {
				if tfGlobalMap, ok := tfGlobal.(map[string]interface{}); ok {
					// Merge terraform.global into global config
					mergedGlobal := renderer.MergeArgs(
						renderer.ArgsData{renderer.FieldGlobal: cfg.Global},
						renderer.ArgsData{renderer.FieldGlobal: tfGlobalMap},
					)
					if mergedGlobalMap, ok := mergedGlobal[renderer.FieldGlobal].(map[string]interface{}); ok {
						cfg.Global = extractGlobalConfig(mergedGlobalMap)
					}
				}
			}

			// Extract terraform projects list
			cfg.Terraform = extractTerraformConfigFromNewFormat(tfMap)
		}
	}

	// Extract kubernetes config from new format
	if k8sSection, ok := argsData["kubernetes"]; ok {
		var k8sMap map[string]interface{}
		switch v := k8sSection.(type) {
		case renderer.ArgsData:
			k8sMap = map[string]interface{}(v)
		case map[string]interface{}:
			k8sMap = v
		case map[interface{}]interface{}:
			// Convert map[interface{}]interface{} to map[string]interface{}
			k8sMap = make(map[string]interface{})
			for k, val := range v {
				if keyStr, ok := k.(string); ok {
					k8sMap[keyStr] = val
				}
			}
		}
		if k8sMap != nil {
			// Extract kubernetes config (if it has projects, we consider it enabled)
			if projects, ok := k8sMap[renderer.FieldProjects]; ok {
				if projectsList, ok := projects.([]interface{}); ok {
					if len(projectsList) > 0 {
						// Create a minimal ProjectConfig to indicate kubernetes is enabled
						cfg.Kubernetes = &ProjectConfig{
							Name: "kubernetes",
						}
						// Extract version if available
						if v, ok := k8sMap["version"]; ok {
							if s, ok := v.(string); ok {
								cfg.Kubernetes.Version = s
							}
						}
					}
				}
			}
		}
	}
	// Extract gitops config from new format (top-level gitops with argocd.apps, etc.)
	if gSection, ok := argsData["gitops"]; ok {
		var gMap map[string]interface{}
		switch v := gSection.(type) {
		case renderer.ArgsData:
			gMap = map[string]interface{}(v)
		case map[string]interface{}:
			gMap = v
		case map[interface{}]interface{}:
			gMap = make(map[string]interface{})
			for k, val := range v {
				if keyStr, ok := k.(string); ok {
					gMap[keyStr] = val
				}
			}
		}
		if gMap != nil && (gMap["argocd"] != nil || gMap["apps"] != nil) {
			cfg.Gitops = extractProjectConfig(gMap)
			if cfg.Gitops == nil {
				cfg.Gitops = &ProjectConfig{Name: "gitops"}
			}
		}
	}

	// If no terraform config found, try old format: blcli section
	if cfg.Terraform == nil {
		if blcliSection, ok := argsData["blcli"]; ok {
			if blcliMap, ok := blcliSection.(map[string]interface{}); ok {
				// Extract global config from blcli.global
				if globalSection, ok := blcliMap[renderer.FieldGlobal]; ok {
					if globalMap, ok := globalSection.(map[string]interface{}); ok {
						cfg.Global = extractGlobalConfig(globalMap)
					}
				}

				// Extract terraform config from blcli.terraform
				if tfSection, ok := blcliMap[renderer.FieldTerraform]; ok {
					if tfMap, ok := tfSection.(map[string]interface{}); ok {
						cfg.Terraform = extractTerraformConfig(tfMap)
					}
				}

				// Extract kubernetes config
				if k8sSection, ok := blcliMap["kubernetes"]; ok {
					if k8sMap, ok := k8sSection.(map[string]interface{}); ok {
						cfg.Kubernetes = extractProjectConfig(k8sMap)
					}
				}

				// Extract gitops config
				if gitopsSection, ok := blcliMap["gitops"]; ok {
					if gitopsMap, ok := gitopsSection.(map[string]interface{}); ok {
						cfg.Gitops = extractProjectConfig(gitopsMap)
					}
				}
			}
		}
	}

	// If still no config found, return error
	if cfg.Terraform == nil {
		return BlcliConfig{}, fmt.Errorf("missing 'terraform' section in args file (neither new format 'terraform' nor old format 'blcli.terraform' found)")
	}

	return applyDefaults(cfg), nil
}

// extractTerraformConfigFromNewFormat extracts TerraformConfig from new format (terraform.projects[])
func extractTerraformConfigFromNewFormat(tfMap map[string]interface{}) *TerraformConfig {
	cfg := &TerraformConfig{}

	// Extract version
	if v, ok := tfMap["version"]; ok {
		if s, ok := v.(string); ok {
			cfg.Version = s
		}
	}

	// Extract projects from terraform.projects[] array
	// Exclude marshalled: false projects (they participate in init-args but not init rendering)
	if projects, ok := tfMap[renderer.FieldProjects]; ok {
		if projectsList, ok := projects.([]interface{}); ok {
			cfg.Projects = make([]string, 0, len(projectsList))
			for _, p := range projectsList {
				// Handle different types: renderer.ArgsData, map[string]interface{}, or string
				var projectMap map[string]interface{}
				switch v := p.(type) {
				case renderer.ArgsData:
					projectMap = map[string]interface{}(v)
				case map[string]interface{}:
					projectMap = v
				case map[interface{}]interface{}:
					// Convert map[interface{}]interface{} to map[string]interface{}
					projectMap = make(map[string]interface{})
					for k, val := range v {
						if keyStr, ok := k.(string); ok {
							projectMap[keyStr] = val
						}
					}
				}

				if projectMap != nil {
					// Skip marshalled: false projects (they do not participate in init rendering)
					if marshalled, ok := projectMap["marshalled"].(bool); ok && !marshalled {
						continue
					}
					// New format: project has "name" field
					if name, ok := projectMap[renderer.FieldName]; ok {
						if nameStr, ok := name.(string); ok {
							cfg.Projects = append(cfg.Projects, nameStr)
						}
					}
				} else if s, ok := p.(string); ok {
					// Also support string format for backward compatibility
					cfg.Projects = append(cfg.Projects, s)
				}
			}
		}
	}

	return cfg
}

// extractGlobalConfig extracts GlobalConfig from a map
func extractGlobalConfig(m map[string]interface{}) GlobalConfig {
	var cfg GlobalConfig
	// Extract "name" field (GlobalName is a template variable, not config.Name)
	if v, ok := m[renderer.FieldName]; ok {
		if s, ok := v.(string); ok {
			cfg.Name = s
		}
	}
	if v, ok := m["domain"]; ok {
		if s, ok := v.(string); ok {
			cfg.Domain = s
		}
	}
	if v, ok := m["version"]; ok {
		if s, ok := v.(string); ok {
			cfg.Version = s
		}
	}
	if v, ok := m["workspace"]; ok {
		if s, ok := v.(string); ok {
			cfg.Workspace = s
		}
	}
	// Support both snake_case and CamelCase for billing_account_id
	if v, ok := m["billing_account_id"]; ok {
		if s, ok := v.(string); ok {
			cfg.BillingAccountID = s
		}
	} else if v, ok := m["BillingAccountID"]; ok {
		if s, ok := v.(string); ok {
			cfg.BillingAccountID = s
		}
	}
	// Support both snake_case and CamelCase for organization_id
	if v, ok := m["organization_id"]; ok {
		if s, ok := v.(string); ok {
			cfg.OrganizationID = s
		}
	} else if v, ok := m["OrganizationID"]; ok {
		if s, ok := v.(string); ok {
			cfg.OrganizationID = s
		}
	}
	if v, ok := m["terraform_version"]; ok {
		if s, ok := v.(string); ok {
			cfg.TerraformVersion = s
		}
	}
	if v, ok := m["kubectl_version"]; ok {
		if s, ok := v.(string); ok {
			cfg.KubectlVersion = s
		}
	}
	return cfg
}

// extractTerraformConfig extracts TerraformConfig from a map
func extractTerraformConfig(m map[string]interface{}) *TerraformConfig {
	cfg := &TerraformConfig{}
	if v, ok := m[renderer.FieldProjects]; ok {
		if projects, ok := v.([]interface{}); ok {
			cfg.Projects = make([]string, 0, len(projects))
			for _, p := range projects {
				if s, ok := p.(string); ok {
					cfg.Projects = append(cfg.Projects, s)
				}
			}
		}
	}
	if v, ok := m[renderer.FieldName]; ok {
		if s, ok := v.(string); ok {
			cfg.Name = s
		}
	}
	if v, ok := m["version"]; ok {
		if s, ok := v.(string); ok {
			cfg.Version = s
		}
	}
	return cfg
}

// extractProjectConfig extracts ProjectConfig from a map
func extractProjectConfig(m map[string]interface{}) *ProjectConfig {
	cfg := &ProjectConfig{}
	if v, ok := m[renderer.FieldName]; ok {
		if s, ok := v.(string); ok {
			cfg.Name = s
		}
	}
	if v, ok := m["version"]; ok {
		if s, ok := v.(string); ok {
			cfg.Version = s
		}
	}
	return cfg
}

// applyDefaults applies default values to config
func applyDefaults(cfg BlcliConfig) BlcliConfig {
	if cfg.Global.Workspace == "" {
		cfg.Global.Workspace = "."
	}

	// Set default versions if not specified
	if cfg.Global.TerraformVersion == "" {
		cfg.Global.TerraformVersion = "latest"
	}
	if cfg.Global.KubectlVersion == "" {
		cfg.Global.KubectlVersion = "latest"
	}

	return cfg
}

func WorkspacePath(global GlobalConfig) string {
	// Relative paths are resolved from current working directory, matching the Rust tool behavior.
	return filepath.Clean(global.Workspace)
}
