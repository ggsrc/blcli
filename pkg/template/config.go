package template

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// TerraformConfig represents the terraform/config.yaml structure
type TerraformConfig struct {
	Version       string            `yaml:"version"`
	Init          []InitItem        `yaml:"init"`
	Modules       map[string]Module `yaml:"modules"`
	ProjectGlobal string            `yaml:"project-global,omitempty"` // Project-level global args file
	Projects      []ProjectItem     `yaml:"projects"`
}

// UnmarshalYAML implements custom YAML unmarshaling for TerraformConfig
// to support the new modules format: a list of maps where each map has one key (module name)
func (cfg *TerraformConfig) UnmarshalYAML(value *yaml.Node) error {
	// First, unmarshal into a temporary struct to handle the new modules format
	type tempConfig struct {
		Version       string        `yaml:"version"`
		Init          []InitItem    `yaml:"init"`
		Modules       yaml.Node     `yaml:"modules"`
		ProjectGlobal string        `yaml:"project-global,omitempty"`
		Projects      []ProjectItem `yaml:"projects"`
	}

	var temp tempConfig
	if err := value.Decode(&temp); err != nil {
		return err
	}

	cfg.Version = temp.Version
	cfg.Init = temp.Init
	cfg.ProjectGlobal = temp.ProjectGlobal
	cfg.Projects = temp.Projects

	// Handle modules: can be either map or list of maps
	cfg.Modules = make(map[string]Module)
	if temp.Modules.Kind == yaml.MappingNode {
		// Old format: modules is a map
		var modulesMap map[string]Module
		if err := temp.Modules.Decode(&modulesMap); err != nil {
			return fmt.Errorf("failed to decode modules as map: %w", err)
		}
		cfg.Modules = modulesMap
	} else if temp.Modules.Kind == yaml.SequenceNode {
		// New format: modules is a list of Module items
		// Example: [{name: gke, path: [terraform/modules/gke]}, ...]
		var modulesList []Module
		if err := temp.Modules.Decode(&modulesList); err != nil {
			return fmt.Errorf("failed to decode modules as list: %w", err)
		}
		for _, module := range modulesList {
			if module.Name == "" {
				continue // Skip if no name
			}
			cfg.Modules[module.Name] = module
		}
	}

	return nil
}

// InitItem represents an init item in config.yaml
type InitItem struct {
	Name        string   `yaml:"name"`
	Prepare     bool     `yaml:"prepare,omitempty"`     // If true, run first in apply init (e.g. create remote state bucket)
	Path        []string `yaml:"path"`                 // List of template paths
	Destination string   `yaml:"destination,omitempty"` // Optional: output directory path (relative to workspace root, does NOT include filename, supports template variables like {{.GlobalName}})
	Args        []string `yaml:"args,omitempty"`        // List of args file paths
	Install     string   `yaml:"install,omitempty"`
	Upgrade     string   `yaml:"upgrade,omitempty"`
	Rollback    string   `yaml:"rollback,omitempty"` // Optional: custom rollback command (supports template variables)
}

// UnmarshalYAML implements custom YAML unmarshaling to support both string and list formats
func (i *InitItem) UnmarshalYAML(value *yaml.Node) error {
	type InitItemAlias struct {
		Name        string      `yaml:"name"`
		Prepare     bool        `yaml:"prepare,omitempty"`
		Path        interface{} `yaml:"path"`
		Destination string      `yaml:"destination,omitempty"`
		Args        interface{} `yaml:"args,omitempty"`
		Install     string      `yaml:"install,omitempty"`
		Upgrade     string      `yaml:"upgrade,omitempty"`
		Rollback    string      `yaml:"rollback,omitempty"`
	}

	var aux InitItemAlias
	if err := value.Decode(&aux); err != nil {
		return err
	}

	// Copy all fields
	i.Name = aux.Name
	i.Prepare = aux.Prepare
	i.Destination = aux.Destination
	i.Install = aux.Install
	i.Upgrade = aux.Upgrade
	i.Rollback = aux.Rollback

	// Handle Path: support both string (backward compatible) and list
	i.Path = normalizePath(aux.Path)
	// Handle Args: support both string (backward compatible) and list
	i.Args = normalizeArgs(aux.Args)
	return nil
}

// Module represents a module in config.yaml
type Module struct {
	Name     string   `yaml:"name"`
	Path     []string `yaml:"path"`           // List of template paths (for modules, typically just one directory)
	Args     []string `yaml:"args,omitempty"` // List of args file paths
	Install  string   `yaml:"install,omitempty"`
	Upgrade  string   `yaml:"upgrade,omitempty"`
	Rollback string   `yaml:"rollback,omitempty"` // Optional: custom rollback command (supports template variables)
}

// UnmarshalYAML implements custom YAML unmarshaling to support both string and list formats
func (m *Module) UnmarshalYAML(value *yaml.Node) error {
	type Alias struct {
		Name     string      `yaml:"name"`
		Path     interface{} `yaml:"path"`
		Args     interface{} `yaml:"args,omitempty"`
		Install  string      `yaml:"install,omitempty"`
		Upgrade  string      `yaml:"upgrade,omitempty"`
		Rollback string      `yaml:"rollback,omitempty"`
	}

	var aux Alias
	if err := value.Decode(&aux); err != nil {
		return err
	}

	// Copy all fields
	m.Name = aux.Name
	m.Install = aux.Install
	m.Upgrade = aux.Upgrade
	m.Rollback = aux.Rollback

	// Handle Path: support both string (backward compatible) and list
	m.Path = normalizePath(aux.Path)
	// Handle Args: support both string (backward compatible) and list
	m.Args = normalizeArgs(aux.Args)
	return nil
}

// ProjectItem represents a project item in config.yaml
type ProjectItem struct {
	Name         string   `yaml:"name"`
	Path         []string `yaml:"path"`           // List of template paths
	Args         []string `yaml:"args,omitempty"` // List of args file paths
	Install      string   `yaml:"install,omitempty"`
	Upgrade      string   `yaml:"upgrade,omitempty"`
	Rollback     string   `yaml:"rollback,omitempty"` // Optional: custom rollback command (supports template variables)
	Dependencies []string `yaml:"dependencies,omitempty"`
}

// UnmarshalYAML implements custom YAML unmarshaling to support both string and list formats
func (p *ProjectItem) UnmarshalYAML(value *yaml.Node) error {
	type Alias struct {
		Name         string      `yaml:"name"`
		Path         interface{} `yaml:"path"`
		Args         interface{} `yaml:"args,omitempty"`
		Install      string      `yaml:"install,omitempty"`
		Upgrade      string      `yaml:"upgrade,omitempty"`
		Rollback     string      `yaml:"rollback,omitempty"`
		Dependencies []string    `yaml:"dependencies,omitempty"`
	}

	var aux Alias
	if err := value.Decode(&aux); err != nil {
		return err
	}

	// Copy all fields
	p.Name = aux.Name
	p.Install = aux.Install
	p.Upgrade = aux.Upgrade
	p.Rollback = aux.Rollback
	p.Dependencies = aux.Dependencies

	// Handle Path: support both string (backward compatible) and list
	p.Path = normalizePath(aux.Path)
	// Handle Args: support both string (backward compatible) and list
	p.Args = normalizeArgs(aux.Args)
	return nil
}

// normalizePath converts path from string or []string to []string
// Supports backward compatibility: if path is a string, convert to single-item list
func normalizePath(path interface{}) []string {
	if path == nil {
		return nil
	}

	switch v := path.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok && str != "" {
				result = append(result, str)
			}
		}
		return result
	case []string:
		return v
	default:
		return nil
	}
}

// normalizeArgs converts args from string or []string to []string
// Supports backward compatibility: if args is a string, convert to single-item list
func normalizeArgs(args interface{}) []string {
	if args == nil {
		return nil
	}

	switch v := args.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok && str != "" {
				result = append(result, str)
			}
		}
		return result
	case []string:
		return v
	default:
		return nil
	}
}

// LoadTerraformConfig loads terraform/config.yaml from the template repository
func (l *Loader) LoadTerraformConfig() (*TerraformConfig, error) {
	configPath := "terraform/config.yaml"
	content, err := l.LoadTemplate(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load terraform config: %w", err)
	}

	var cfg TerraformConfig
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse terraform config: %w", err)
	}

	return &cfg, nil
}

// LoadKubernetesConfig loads kubernetes/config.yaml from the template repository
func (l *Loader) LoadKubernetesConfig() (*KubernetesConfig, error) {
	configPath := "kubernetes/config.yaml"
	content, err := l.LoadTemplate(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubernetes config: %w", err)
	}

	var cfg KubernetesConfig
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse kubernetes config: %w", err)
	}

	return &cfg, nil
}

// LoadGitopsConfig loads gitops/config.yaml from the template repository
func (l *Loader) LoadGitopsConfig() (*GitopsConfig, error) {
	configPath := "gitops/config.yaml"
	content, err := l.LoadTemplate(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load gitops config: %w", err)
	}

	var cfg GitopsConfig
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse gitops config: %w", err)
	}

	return &cfg, nil
}

// InstallType represents the installation type for a component
type InstallType string

const (
	InstallTypeKubectl InstallType = "kubectl" // Default
	InstallTypeHelm    InstallType = "helm"
	InstallTypeCustom  InstallType = "custom"
)

// KubernetesConfig represents the kubernetes/config.yaml structure
// New structure with init and optional sections
type KubernetesConfig struct {
	Version  string                         `yaml:"version,omitempty"`
	Init     *KubernetesInitSection         `yaml:"init,omitempty"`
	Optional map[string]KubernetesComponent `yaml:"optional,omitempty"`
	// Backward compatibility: support old format with components list
	Components []KubernetesComponent `yaml:"components,omitempty"`
}

// KubernetesInitSection represents the init section in kubernetes config
type KubernetesInitSection struct {
	Namespace  *KubernetesComponent           `yaml:"namespace,omitempty"`
	Components map[string]KubernetesComponent `yaml:"components,omitempty"`
}

// KubernetesComponent represents a kubernetes component
type KubernetesComponent struct {
	Name         string      `yaml:"name"`
	Path         []string    `yaml:"path"`
	Args         []string    `yaml:"args,omitempty"`
	Install      string      `yaml:"install,omitempty"`
	Upgrade      string      `yaml:"upgrade,omitempty"`
	Rollback     string      `yaml:"rollback,omitempty"`    // Optional: custom rollback command (supports template variables)
	Check        string      `yaml:"check,omitempty"`       // Custom check command (for installType: custom)
	InstallType  InstallType `yaml:"installType,omitempty"` // kubectl, helm, custom (default: kubectl)
	Chart        string      `yaml:"chart,omitempty"`       // Helm chart (remote repo/chart or local path)
	Namespace    string      `yaml:"namespace,omitempty"`   // Namespace for helm install (default: component name)
	Dependencies []string    `yaml:"dependencies,omitempty"`
}

// UnmarshalYAML implements custom YAML unmarshaling for KubernetesComponent
// to support both string and list formats for Path and Args
func (c *KubernetesComponent) UnmarshalYAML(value *yaml.Node) error {
	type Alias struct {
		Name         string      `yaml:"name"`
		Path         interface{} `yaml:"path"`
		Args         interface{} `yaml:"args,omitempty"`
		Install      string      `yaml:"install,omitempty"`
		Upgrade      string      `yaml:"upgrade,omitempty"`
		Rollback     string      `yaml:"rollback,omitempty"`
		Check        string      `yaml:"check,omitempty"`
		InstallType  string      `yaml:"installType,omitempty"`
		Chart        string      `yaml:"chart,omitempty"`
		Namespace    string      `yaml:"namespace,omitempty"`
		Dependencies []string    `yaml:"dependencies,omitempty"`
	}

	var aux Alias
	if err := value.Decode(&aux); err != nil {
		return err
	}

	c.Name = aux.Name
	c.Install = aux.Install
	c.Upgrade = aux.Upgrade
	c.Rollback = aux.Rollback
	c.Check = aux.Check
	c.Chart = aux.Chart
	c.Namespace = aux.Namespace
	c.Dependencies = aux.Dependencies

	// Handle Path: support both string (backward compatible) and list
	c.Path = normalizePath(aux.Path)
	// Handle Args: support both string (backward compatible) and list
	c.Args = normalizeArgs(aux.Args)

	// Handle InstallType: convert string to InstallType, default to kubectl
	if aux.InstallType == "" {
		c.InstallType = InstallTypeKubectl
	} else {
		c.InstallType = InstallType(aux.InstallType)
		// Validate installType
		if c.InstallType != InstallTypeKubectl && c.InstallType != InstallTypeHelm && c.InstallType != InstallTypeCustom {
			return fmt.Errorf("invalid installType: %s (must be kubectl, helm, or custom)", aux.InstallType)
		}
	}

	return nil
}

// UnmarshalYAML implements custom YAML unmarshaling for KubernetesConfig
// Supports both new format (init/optional) and old format (components list)
func (cfg *KubernetesConfig) UnmarshalYAML(value *yaml.Node) error {
	type tempConfig struct {
		Version    string                 `yaml:"version,omitempty"`
		Init       *KubernetesInitSection `yaml:"init,omitempty"`
		Optional   map[string]interface{} `yaml:"optional,omitempty"`
		Components []KubernetesComponent  `yaml:"components,omitempty"`
	}

	var temp tempConfig
	if err := value.Decode(&temp); err != nil {
		return err
	}

	cfg.Version = temp.Version
	cfg.Init = temp.Init

	// Parse optional components
	if temp.Optional != nil {
		cfg.Optional = make(map[string]KubernetesComponent)
		for key, val := range temp.Optional {
			// Convert interface{} to KubernetesComponent using yaml.Node
			var comp KubernetesComponent
			compBytes, err := yaml.Marshal(val)
			if err != nil {
				return fmt.Errorf("failed to marshal optional component %s: %w", key, err)
			}
			var node yaml.Node
			if err := yaml.Unmarshal(compBytes, &node); err != nil {
				return fmt.Errorf("failed to unmarshal optional component %s: %w", key, err)
			}
			if len(node.Content) > 0 {
				if err := comp.UnmarshalYAML(node.Content[0]); err != nil {
					return fmt.Errorf("failed to parse optional component %s: %w", key, err)
				}
			}
			// Set name if not already set
			if comp.Name == "" {
				comp.Name = key
			}
			cfg.Optional[key] = comp
		}
	}

	// Backward compatibility: support old format
	cfg.Components = temp.Components

	return nil
}

// KubernetesItem represents an item in kubernetes config (deprecated, kept for backward compatibility)
type KubernetesItem struct {
	Name    string `yaml:"name"`
	Path    string `yaml:"path"`
	Install string `yaml:"install,omitempty"`
	Upgrade string `yaml:"upgrade,omitempty"`
}

// GitopsConfig is the root struct for gitops/config.yaml.
// Layout is fixed: version, app-templates (deployment, statefulset), argocd.
type GitopsConfig struct {
	Version      string              `yaml:"version"`
	AppTemplates GitopsAppTemplates  `yaml:"app-templates"`
	Argocd       GitopsArgocdSection `yaml:"argocd"`
}

// GitopsAppTemplates holds the two fixed app kinds under app-templates.
type GitopsAppTemplates struct {
	Deployment  []GitopsTemplateItem `yaml:"deployment"`
	Statefulset []GitopsTemplateItem `yaml:"statefulset"`
}

// GitopsTemplateItem is one entry under app-templates.deployment or app-templates.statefulset.
type GitopsTemplateItem struct {
	Name        string   `yaml:"name"`
	Path        string   `yaml:"path"`
	Args        []string `yaml:"args,omitempty"`
	Description string   `yaml:"description,omitempty"`
}

// UnmarshalYAML supports args as string or list.
func (g *GitopsTemplateItem) UnmarshalYAML(value *yaml.Node) error {
	type aux struct {
		Name        string      `yaml:"name"`
		Path        string      `yaml:"path"`
		Args        interface{} `yaml:"args,omitempty"`
		Description string      `yaml:"description,omitempty"`
	}
	var a aux
	if err := value.Decode(&a); err != nil {
		return err
	}
	g.Name = a.Name
	g.Path = a.Path
	g.Description = a.Description
	g.Args = normalizeArgs(a.Args)
	return nil
}

// GitopsArgocdSection is the fixed argocd section (path, args).
type GitopsArgocdSection struct {
	Path string   `yaml:"path"`
	Args []string `yaml:"args,omitempty"`
}

// UnmarshalYAML supports args as string or list.
func (g *GitopsArgocdSection) UnmarshalYAML(value *yaml.Node) error {
	type aux struct {
		Path string      `yaml:"path"`
		Args interface{} `yaml:"args,omitempty"`
	}
	var a aux
	if err := value.Decode(&a); err != nil {
		return err
	}
	g.Path = a.Path
	g.Args = normalizeArgs(a.Args)
	return nil
}

// App kind constants for app-templates keys (fixed in config.yaml).
const (
	AppKindDeployment  = "deployment"
	AppKindStatefulset = "statefulset"
)

// GetTemplatesByKind returns the template list for the given app kind (deployment or statefulset).
func (c *GitopsConfig) GetTemplatesByKind(kind string) []GitopsTemplateItem {
	switch kind {
	case AppKindDeployment:
		return c.AppTemplates.Deployment
	case AppKindStatefulset:
		return c.AppTemplates.Statefulset
	default:
		return nil
	}
}

// LoadConfigFromFile loads a config.yaml from local file system
func LoadConfigFromFile(path string) (*TerraformConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg TerraformConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// ResolveDependencies resolves project dependencies and returns ordered list
func (cfg *TerraformConfig) ResolveDependencies() ([]ProjectItem, error) {
	// Create a map for quick lookup
	projectMap := make(map[string]ProjectItem)
	for _, p := range cfg.Projects {
		projectMap[p.Name] = p
	}

	// Check for circular dependencies
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	var result []ProjectItem

	var visit func(name string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("circular dependency detected: %s", name)
		}
		if visited[name] {
			return nil
		}

		project, exists := projectMap[name]
		if !exists {
			return fmt.Errorf("project %s not found", name)
		}

		visiting[name] = true
		for _, dep := range project.Dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[name] = false
		visited[name] = true
		result = append(result, project)
		return nil
	}

	for _, project := range cfg.Projects {
		if !visited[project.Name] {
			if err := visit(project.Name); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

// ResolveTerraformDependencies resolves project dependencies and returns ordered component names
// componentNames: list of component names to resolve dependencies for
func (cfg *TerraformConfig) ResolveTerraformDependencies(componentNames []string) ([]string, error) {
	// Create a map for quick lookup
	componentMap := make(map[string]ProjectItem)
	for _, item := range cfg.Projects {
		componentMap[item.Name] = item
	}

	// Check for circular dependencies using DFS topological sort
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	var result []string

	var visit func(name string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("circular dependency detected: %s", name)
		}
		if visited[name] {
			return nil
		}

		component, exists := componentMap[name]
		if !exists {
			// If component not found in config, skip it (backward compatibility)
			visited[name] = true
			return nil
		}

		visiting[name] = true
		for _, dep := range component.Dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[name] = false
		visited[name] = true
		result = append(result, name)
		return nil
	}

	for _, name := range componentNames {
		if !visited[name] {
			if err := visit(name); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

// GetAllComponents returns all components from both init and optional sections
// This provides a unified view of all components for dependency resolution
func (cfg *KubernetesConfig) GetAllComponents() map[string]KubernetesComponent {
	componentMap := make(map[string]KubernetesComponent)

	// Add components from init.components (new format)
	if cfg.Init != nil && cfg.Init.Components != nil {
		for key, comp := range cfg.Init.Components {
			if comp.Name == "" {
				comp.Name = key
			}
			componentMap[comp.Name] = comp
		}
	}

	// Add components from optional (new format)
	if cfg.Optional != nil {
		for key, comp := range cfg.Optional {
			if comp.Name == "" {
				comp.Name = key
			}
			componentMap[comp.Name] = comp
		}
	}

	// Backward compatibility: add components from old format
	for _, comp := range cfg.Components {
		componentMap[comp.Name] = comp
	}

	return componentMap
}

// ResolveKubernetesDependencies resolves component dependencies and returns ordered list
// componentNames: list of component names to resolve dependencies for
func (cfg *KubernetesConfig) ResolveKubernetesDependencies(componentNames []string) ([]string, error) {
	// Create a map for quick lookup from component name to component
	componentMap := cfg.GetAllComponents()

	// Check for circular dependencies
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	var result []string

	var visit func(name string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("circular dependency detected: %s", name)
		}
		if visited[name] {
			return nil
		}

		component, exists := componentMap[name]
		if !exists {
			// If component not found, skip it and continue
			visited[name] = true
			return nil
		}

		visiting[name] = true
		for _, dep := range component.Dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[name] = false
		visited[name] = true
		result = append(result, name)
		return nil
	}

	for _, name := range componentNames {
		if !visited[name] {
			if err := visit(name); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}
