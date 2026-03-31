package renderer

// Common field names used in args configuration
const (
	// Top-level sections
	FieldGlobal    = "global"
	FieldTerraform = "terraform"

	// Terraform sub-sections
	FieldInit       = "init"
	FieldProjects   = "projects"
	FieldModules    = "modules"
	FieldComponents = "components"

	// Common fields
	FieldName = "name"
	FieldArgs = "args"
)
