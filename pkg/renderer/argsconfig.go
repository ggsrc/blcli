package renderer

// ComponentData represents a component with ordered fields for YAML output
type ComponentData struct {
	Name       string                 `yaml:"name"`
	Parameters map[string]interface{} `yaml:"parameters"`
}

// ProjectData represents a project with ordered fields for YAML output
// gopkg.in/yaml.v3 preserves struct field order, so fields are ordered as defined:
// 1. Name
// 2. ID (generated project ID, e.g. org-prd-xxxxxxxx)
// 3. Marshalled (if false, project does not participate in init rendering, only init-args)
// 4. Global
// 5. Components
type ProjectData struct {
	Name       string                 `yaml:"name,omitempty"`
	ID         string                 `yaml:"id,omitempty"`
	Marshalled *bool                  `yaml:"marshalled,omitempty"`
	Global     map[string]interface{} `yaml:"global,omitempty"`
	Components []ComponentData        `yaml:"components,omitempty"`
}

// InitSection represents the init section with components
type InitSection struct {
	Components map[string]interface{} `yaml:"components,omitempty"`
}

// TerraformSection represents the terraform section with ordered fields
type TerraformSection struct {
	Version  string                 `yaml:"version,omitempty"`
	Global   map[string]interface{} `yaml:"global,omitempty"`
	Init     *InitSection           `yaml:"init,omitempty"`
	Projects []ProjectData          `yaml:"projects,omitempty"`
}

// KubernetesSection represents the kubernetes section with ordered fields
type KubernetesSection struct {
	Version  string                 `yaml:"version,omitempty"`
	Global   map[string]interface{} `yaml:"global,omitempty"`
	Init     *InitSection           `yaml:"init,omitempty"`
	Projects []ProjectData          `yaml:"projects,omitempty"`
}

// ArgsConfig represents the top-level args configuration with ordered fields
type ArgsConfig struct {
	Global     map[string]interface{} `yaml:"global,omitempty"`
	Terraform  TerraformSection       `yaml:"terraform,omitempty"`
	Kubernetes KubernetesSection      `yaml:"kubernetes,omitempty"`
	Gitops     map[string]interface{} `yaml:"gitops,omitempty"`
}
