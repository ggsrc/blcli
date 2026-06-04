package agent

import "strconv"

const ToolContractSchemaVersion = "blcli.tool-contract/v1"

// ToolContract describes how an AI agent can call blcli commands safely.
type ToolContract struct {
	SchemaVersion string                `json:"schema_version" yaml:"schema_version"`
	CLI           string                `json:"cli" yaml:"cli"`
	Contract      ContractMetadata      `json:"contract" yaml:"contract"`
	Compatibility ContractCompatibility `json:"compatibility" yaml:"compatibility"`
	Commands      []CommandContract     `json:"commands" yaml:"commands"`
}

type ContractMetadata struct {
	Version       string   `json:"version" yaml:"version"`
	Stability     string   `json:"stability" yaml:"stability"`
	OutputFormats []string `json:"output_formats" yaml:"output_formats"`
	Notes         []string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type ContractCompatibility struct {
	Strategy                 string   `json:"strategy" yaml:"strategy"`
	CompatibleSchemaVersions []string `json:"compatible_schema_versions" yaml:"compatible_schema_versions"`
	AdditiveChanges          string   `json:"additive_changes" yaml:"additive_changes"`
	BreakingChanges          string   `json:"breaking_changes" yaml:"breaking_changes"`
	FieldSemantics           []string `json:"field_semantics" yaml:"field_semantics"`
}

type CommandContract struct {
	Name          string           `json:"name" yaml:"name"`
	Summary       string           `json:"summary" yaml:"summary"`
	Inputs        []InputField     `json:"inputs" yaml:"inputs"`
	Outputs       []OutputField    `json:"outputs" yaml:"outputs"`
	InputSchema   JSONSchema       `json:"input_schema" yaml:"input_schema"`
	OutputSchema  JSONSchema       `json:"output_schema" yaml:"output_schema"`
	ExitCodes     []ExitCode       `json:"exit_codes" yaml:"exit_codes"`
	Examples      []CommandExample `json:"examples" yaml:"examples"`
	AgentGuidance []string         `json:"agent_guidance,omitempty" yaml:"agent_guidance,omitempty"`
}

type InputField struct {
	Name        string   `json:"name" yaml:"name"`
	Type        string   `json:"type" yaml:"type"`
	Required    bool     `json:"required" yaml:"required"`
	Repeatable  bool     `json:"repeatable,omitempty" yaml:"repeatable,omitempty"`
	Default     string   `json:"default,omitempty" yaml:"default,omitempty"`
	Enum        []string `json:"enum,omitempty" yaml:"enum,omitempty"`
	Description string   `json:"description" yaml:"description"`
}

type OutputField struct {
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"`
	Description string `json:"description" yaml:"description"`
}

type ExitCode struct {
	Code        int    `json:"code" yaml:"code"`
	Description string `json:"description" yaml:"description"`
}

type CommandExample struct {
	Description string `json:"description" yaml:"description"`
	Command     string `json:"command" yaml:"command"`
}

// JSONSchema is the subset of JSON Schema used by the blcli tool contract.
type JSONSchema struct {
	Schema               string                `json:"$schema,omitempty" yaml:"$schema,omitempty"`
	Type                 string                `json:"type,omitempty" yaml:"type,omitempty"`
	Description          string                `json:"description,omitempty" yaml:"description,omitempty"`
	Properties           map[string]JSONSchema `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required             []string              `json:"required,omitempty" yaml:"required,omitempty"`
	Items                *JSONSchema           `json:"items,omitempty" yaml:"items,omitempty"`
	Enum                 []string              `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default              interface{}           `json:"default,omitempty" yaml:"default,omitempty"`
	OneOf                []JSONSchema          `json:"oneOf,omitempty" yaml:"oneOf,omitempty"`
	AdditionalProperties *bool                 `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
}

// BuildToolContract returns the v2 agent-facing command contract.
func BuildToolContract(commandFilter string) ToolContract {
	commands := commandContracts()
	if commandFilter != "" {
		filtered := make([]CommandContract, 0, 1)
		for _, command := range commands {
			if command.Name == commandFilter {
				filtered = append(filtered, command)
				break
			}
		}
		commands = filtered
	}

	return ToolContract{
		SchemaVersion: ToolContractSchemaVersion,
		CLI:           "blcli",
		Contract: ContractMetadata{
			Version:       "2.0",
			Stability:     "beta",
			OutputFormats: []string{"table", "json", "yaml"},
			Notes: []string{
				"Prefer json output for automation when a command supports --format.",
				"Run blcli diagnose with captured stderr/stdout when a command fails.",
				"Use --dry-run before apply commands when planning changes.",
			},
		},
		Compatibility: ContractCompatibility{
			Strategy:                 "semver-like schema compatibility",
			CompatibleSchemaVersions: []string{ToolContractSchemaVersion},
			AdditiveChanges:          "Minor v2 contract updates may add commands, optional fields, enum values, examples, or guidance without changing existing field meaning.",
			BreakingChanges:          "Removing fields, renaming fields, changing required inputs, or changing field semantics requires a new schema_version.",
			FieldSemantics: []string{
				"Unknown top-level fields must be ignored by consumers.",
				"Consumers must validate command arguments against input_schema before execution when possible.",
				"Consumers should prefer output_schema for machine-readable command outputs and fall back to text capture when a command only emits text.",
			},
		},
		Commands: commands,
	}
}

func commandContracts() []CommandContract {
	commonExitCodes := []ExitCode{
		{Code: 0, Description: "Command completed successfully."},
		{Code: 1, Description: "Command failed. Inspect stderr and run blcli diagnose for a classified failure."},
	}

	commands := []CommandContract{
		{
			Name:    "init-args",
			Summary: "Generate starter args.yaml or args.toml from a template repository.",
			Inputs: []InputField{
				{Name: "template_repo", Type: "string", Required: false, Default: "github.com/ggsrc/infra-template", Description: "Template repository URL or local path."},
				{Name: "output", Type: "path", Required: false, Default: "args.yaml", Description: "Output args file path."},
				{Name: "format", Type: "string", Required: false, Default: "yaml", Enum: []string{"yaml", "toml"}, Description: "Generated args format."},
				{Name: "force_update", Type: "boolean", Required: false, Default: "false", Description: "Refresh template cache before generation."},
			},
			Outputs: []OutputField{
				{Name: "args_file", Type: "path", Description: "Generated args file."},
				{Name: "env_file", Type: "path", Description: "Generated .env sample when available."},
			},
			ExitCodes: commonExitCodes,
			Examples: []CommandExample{
				{Description: "Generate default YAML args.", Command: "blcli init-args ../bl-template -o args.yaml"},
			},
			AgentGuidance: []string{"Use this before init when no args file exists."},
		},
		{
			Name:    "init",
			Summary: "Render Terraform, Kubernetes, and GitOps files from templates and args.",
			Inputs: []InputField{
				{Name: "template_repo", Type: "string", Required: false, Default: "github.com/ggsrc/infra-template", Description: "Template repository URL or local path."},
				{Name: "args", Type: "path", Required: true, Repeatable: true, Description: "YAML or TOML args file. Earlier files override later files."},
				{Name: "modules", Type: "string", Required: false, Repeatable: true, Enum: []string{"terraform", "kubernetes", "gitops"}, Description: "Modules to render."},
				{Name: "output", Type: "path", Required: false, Description: "Workspace output directory override."},
				{Name: "overwrite", Type: "boolean", Required: false, Default: "false", Description: "Overwrite blcli-managed generated files."},
			},
			Outputs: []OutputField{
				{Name: "workspace", Type: "path", Description: "Rendered infrastructure workspace."},
				{Name: "marker", Type: "path", Description: "blcli marker files used by apply commands."},
			},
			ExitCodes: commonExitCodes,
			Examples: []CommandExample{
				{Description: "Render all modules.", Command: "blcli init ../bl-template -a args.yaml -o ./workspace/output"},
			},
			AgentGuidance: []string{"Run init again with --overwrite after changing args or templates."},
		},
		{
			Name:    "explain",
			Summary: "Inspect template components and their parameters.",
			Inputs: []InputField{
				{Name: "template_repo", Type: "string", Required: false, Default: "github.com/ggsrc/infra-template", Description: "Template repository URL or local path."},
				{Name: "module", Type: "string", Required: false, Enum: []string{"terraform", "kubernetes", "gitops"}, Description: "Module to inspect."},
				{Name: "component", Type: "string", Required: false, Description: "Component name filter."},
				{Name: "list", Type: "boolean", Required: false, Default: "false", Description: "List component names only."},
			},
			Outputs:   []OutputField{{Name: "component_metadata", Type: "text", Description: "Component paths, install commands, and args definitions."}},
			ExitCodes: commonExitCodes,
			Examples:  []CommandExample{{Description: "List Terraform components.", Command: "blcli explain -r ../bl-template -m terraform -l"}},
		},
		{
			Name:    "check",
			Summary: "Run local dependency, repository, and Kubernetes preflight checks.",
			Inputs: []InputField{
				{Name: "scope", Type: "string", Required: false, Enum: []string{"plugin", "repo", "kubernetes"}, Description: "Check scope or subcommand."},
				{Name: "template_repo", Type: "string", Required: false, Description: "Template repository used by repo checks."},
			},
			Outputs:       []OutputField{{Name: "check_results", Type: "text", Description: "Preflight pass/fail details."}},
			ExitCodes:     commonExitCodes,
			Examples:      []CommandExample{{Description: "Check local toolchain.", Command: "blcli check"}},
			AgentGuidance: []string{"Run this before apply when dependency or credential state is unknown."},
		},
		{
			Name:    "apply init",
			Summary: "Apply Terraform init directories such as remote state setup.",
			Inputs:  applyTerraformInputs(false),
			Outputs: []OutputField{
				{Name: "execution_plan", Type: "text", Description: "Ordered Terraform init plan before execution."},
				{Name: "terraform_output", Type: "text", Description: "Terraform command output."},
			},
			ExitCodes:     commonExitCodes,
			Examples:      []CommandExample{{Description: "Apply init directories.", Command: "blcli apply init -d ./workspace/output/terraform --auto-approve"}},
			AgentGuidance: []string{"Run before apply terraform on a fresh workspace."},
		},
		{
			Name:    "apply terraform",
			Summary: "Run terraform init, validate, plan, and apply for generated project directories.",
			Inputs:  applyTerraformInputs(true),
			Outputs: []OutputField{
				{Name: "execution_plan", Type: "text", Description: "Ordered Terraform apply plan."},
				{Name: "terraform_output", Type: "text", Description: "Terraform command output."},
			},
			ExitCodes:     commonExitCodes,
			Examples:      []CommandExample{{Description: "Apply one project.", Command: "blcli apply terraform -d ./workspace/output/terraform --project prd --auto-approve"}},
			AgentGuidance: []string{"Use --dry-run first when making changes."},
		},
		{
			Name:    "apply kubernetes",
			Summary: "Apply generated Kubernetes components in dependency order.",
			Inputs: []InputField{
				{Name: "dir", Type: "path", Required: true, Description: "Generated kubernetes directory."},
				{Name: "project", Type: "string", Required: false, Description: "Project/environment filter when supported by generated layout."},
				{Name: "kubeconfig", Type: "path", Required: false, Description: "Kubeconfig path."},
				{Name: "context", Type: "string", Required: false, Description: "Kubernetes context."},
				{Name: "namespace", Type: "string", Required: false, Description: "Namespace override."},
				{Name: "dry_run", Type: "boolean", Required: false, Default: "false", Description: "Print plan without applying resources."},
				{Name: "wait", Type: "boolean", Required: false, Default: "false", Description: "Wait for resources when supported."},
			},
			Outputs:   []OutputField{{Name: "kubectl_or_helm_output", Type: "text", Description: "Kubernetes install command output."}},
			ExitCodes: commonExitCodes,
			Examples:  []CommandExample{{Description: "Apply Kubernetes components.", Command: "blcli apply kubernetes -d ./workspace/output/kubernetes --context <context>"}},
		},
		{
			Name:    "apply gitops",
			Summary: "Apply generated ArgoCD Application manifests.",
			Inputs: []InputField{
				{Name: "dir", Type: "path", Required: true, Description: "Generated gitops directory."},
				{Name: "args", Type: "path", Required: true, Repeatable: true, Description: "Args file used for repo and app metadata."},
				{Name: "project", Type: "string", Required: false, Description: "Project/environment filter when supported by generated layout."},
				{Name: "kubeconfig", Type: "path", Required: false, Description: "Kubeconfig path."},
				{Name: "context", Type: "string", Required: false, Description: "Kubernetes context."},
				{Name: "dry_run", Type: "boolean", Required: false, Default: "false", Description: "Print plan without applying applications."},
				{Name: "skip_sync", Type: "boolean", Required: false, Default: "false", Description: "Skip ArgoCD sync when supported."},
			},
			Outputs:   []OutputField{{Name: "argocd_application_output", Type: "text", Description: "kubectl or ArgoCD command output."}},
			ExitCodes: commonExitCodes,
			Examples:  []CommandExample{{Description: "Apply GitOps manifests.", Command: "blcli apply gitops -d ./workspace/output/gitops --args args.yaml"}},
		},
		{
			Name:    "apply all",
			Summary: "Apply terraform, kubernetes, and gitops in order.",
			Inputs: []InputField{
				{Name: "dir", Type: "path", Required: false, Description: "Workspace output directory."},
				{Name: "args", Type: "path", Required: false, Repeatable: true, Description: "Args file for GitOps metadata."},
				{Name: "continue_on_error", Type: "boolean", Required: false, Default: "false", Description: "Continue to later modules after a failure."},
				{Name: "skip_modules", Type: "string", Required: false, Repeatable: true, Enum: []string{"terraform", "kubernetes", "gitops"}, Description: "Modules to skip."},
			},
			Outputs: []OutputField{
				{Name: "operation_id", Type: "string", Description: "Progress operation id when tracking starts."},
				{Name: "progress_file", Type: "path", Description: "Progress record under ~/.blcli/progress."},
			},
			ExitCodes:     commonExitCodes,
			Examples:      []CommandExample{{Description: "Apply complete workspace.", Command: "blcli apply all -d ./workspace/output --args args.yaml"}},
			AgentGuidance: []string{"Use skip_modules to resume manually after a failed module is repaired."},
		},
		{
			Name:    "apply init-repos",
			Summary: "Initialize local git repositories, create GitHub repositories, and push generated output.",
			Inputs: []InputField{
				{Name: "dir", Type: "path", Required: true, Description: "Workspace output directory."},
				{Name: "org", Type: "string", Required: true, Description: "GitHub organization or owner."},
			},
			Outputs:       []OutputField{{Name: "git_repositories", Type: "text", Description: "Created or updated repository URLs."}},
			ExitCodes:     commonExitCodes,
			Examples:      []CommandExample{{Description: "Create and push generated repos.", Command: "blcli apply init-repos --org myorg -d ./workspace/output"}},
			AgentGuidance: []string{"Requires gh authentication and interactive confirmation."},
		},
		{
			Name:    "status",
			Summary: "Check Terraform, Kubernetes, and GitOps deployment status.",
			Inputs: []InputField{
				{Name: "type", Type: "string", Required: false, Default: "all", Enum: []string{"terraform", "kubernetes", "gitops", "all"}, Description: "Status scope."},
				{Name: "args", Type: "path", Required: true, Repeatable: true, Description: "Args file."},
				{Name: "workspace", Type: "path", Required: false, Description: "Workspace override."},
				{Name: "format", Type: "string", Required: false, Default: "table", Enum: []string{"table", "json", "yaml"}, Description: "Output format."},
			},
			Outputs: []OutputField{
				{Name: "summary", Type: "object", Description: "Overall status summary when format is json or yaml."},
				{Name: "module_status", Type: "object", Description: "Module-level Terraform, Kubernetes, or GitOps status."},
			},
			ExitCodes:     commonExitCodes,
			Examples:      []CommandExample{{Description: "Get machine-readable status.", Command: "blcli status --args args.yaml --format json"}},
			AgentGuidance: []string{"Prefer --format json for automation."},
		},
		{
			Name:    "rollback",
			Summary: "Run rollback commands configured in the template config.",
			Inputs: []InputField{
				{Name: "args", Type: "path", Required: true, Repeatable: true, Description: "Args file."},
				{Name: "workspace", Type: "path", Required: false, Description: "Workspace override."},
				{Name: "dry_run", Type: "boolean", Required: false, Default: "false", Description: "Print rollback plan without executing."},
			},
			Outputs:   []OutputField{{Name: "rollback_plan", Type: "text", Description: "Rollback commands and execution result."}},
			ExitCodes: commonExitCodes,
			Examples:  []CommandExample{{Description: "Preview rollback.", Command: "blcli rollback --args args.yaml --dry-run"}},
		},
		{
			Name:    "destroy",
			Summary: "Destroy generated infrastructure for a selected module.",
			Inputs: []InputField{
				{Name: "module", Type: "string", Required: true, Enum: []string{"terraform", "kubernetes", "gitops", "all"}, Description: "Module to destroy."},
				{Name: "args", Type: "path", Required: false, Repeatable: true, Description: "Args file."},
				{Name: "dir", Type: "path", Required: false, Description: "Generated output directory."},
			},
			Outputs:       []OutputField{{Name: "destroy_output", Type: "text", Description: "Destroy command output."}},
			ExitCodes:     commonExitCodes,
			Examples:      []CommandExample{{Description: "Destroy terraform output.", Command: "blcli destroy terraform --args args.yaml"}},
			AgentGuidance: []string{"Treat as destructive and require explicit user confirmation."},
		},
		{
			Name:    "contract",
			Summary: "Print the AI-agent tool contract for blcli.",
			Inputs: []InputField{
				{Name: "command", Type: "string", Required: false, Description: "Optional command filter."},
				{Name: "format", Type: "string", Required: false, Default: "json", Enum: []string{"json", "yaml", "table"}, Description: "Output format."},
			},
			Outputs:       []OutputField{{Name: "tool_contract", Type: "object", Description: "Machine-readable command contract."}},
			ExitCodes:     commonExitCodes,
			Examples:      []CommandExample{{Description: "Print full contract.", Command: "blcli contract --format json"}},
			AgentGuidance: []string{"Use this as the first command when an agent needs to plan blcli calls."},
		},
		{
			Name:    "diagnose",
			Summary: "Classify a failure message and return repair guidance.",
			Inputs: []InputField{
				{Name: "message", Type: "string", Required: false, Description: "Failure message to classify."},
				{Name: "file", Type: "path", Required: false, Description: "File containing command output or error text."},
				{Name: "format", Type: "string", Required: false, Default: "table", Enum: []string{"table", "json", "yaml"}, Description: "Output format."},
			},
			Outputs:       []OutputField{{Name: "failure_diagnosis", Type: "object", Description: "Category, confidence, matched keywords, and repair commands."}},
			ExitCodes:     commonExitCodes,
			Examples:      []CommandExample{{Description: "Diagnose captured output.", Command: "blcli diagnose --file execution_stage5.log --format json"}},
			AgentGuidance: []string{"Pass the first failing command output, not the whole successful run log, when possible."},
		},
		{
			Name:    "runs list",
			Summary: "List persisted blcli run records by operation id.",
			Inputs: []InputField{
				{Name: "status", Type: "string", Required: false, Enum: []string{"pending", "in_progress", "completed", "failed", "cancelled"}, Description: "Optional run status filter."},
				{Name: "format", Type: "string", Required: false, Default: "table", Enum: []string{"table", "json", "yaml"}, Description: "Output format."},
			},
			Outputs:       []OutputField{{Name: "runs", Type: "array", Description: "Persisted progress records keyed by operation_id."}},
			ExitCodes:     commonExitCodes,
			Examples:      []CommandExample{{Description: "List failed runs.", Command: "blcli runs list --status failed --format json"}},
			AgentGuidance: []string{"Use this to discover run ids before calling runs show."},
		},
		{
			Name:    "runs show",
			Summary: "Show one persisted blcli run record by operation id.",
			Inputs: []InputField{
				{Name: "operation_id", Type: "string", Required: true, Description: "Run id returned by apply/init progress tracking or runs list."},
				{Name: "format", Type: "string", Required: false, Default: "table", Enum: []string{"table", "json", "yaml"}, Description: "Output format."},
			},
			Outputs:       []OutputField{{Name: "runs", Type: "array", Description: "Single persisted progress record."}},
			ExitCodes:     commonExitCodes,
			Examples:      []CommandExample{{Description: "Show one run.", Command: "blcli runs show op-20260529-103000-app --format json"}},
			AgentGuidance: []string{"Inspect steps and error_message fields before deciding whether to resume or retry."},
		},
		{
			Name:      "version",
			Summary:   "Print blcli version information.",
			Inputs:    []InputField{},
			Outputs:   []OutputField{{Name: "version", Type: "text", Description: "Version string."}},
			ExitCodes: commonExitCodes,
			Examples:  []CommandExample{{Description: "Print version.", Command: "blcli version"}},
		},
	}

	for i := range commands {
		commands[i].InputSchema = buildInputSchema(commands[i].Inputs)
		commands[i].OutputSchema = buildOutputSchema(commands[i])
	}

	return commands
}

func applyTerraformInputs(includeProject bool) []InputField {
	inputs := []InputField{
		{Name: "dir", Type: "path", Required: true, Description: "Generated terraform directory."},
		{Name: "auto_approve", Type: "boolean", Required: false, Default: "false", Description: "Skip Terraform approval prompts."},
		{Name: "timeout", Type: "duration", Required: false, Default: "1h", Description: "Command timeout."},
		{Name: "skip_backend", Type: "boolean", Required: false, Default: "false", Description: "Skip backend init for local testing."},
		{Name: "dry_run", Type: "boolean", Required: false, Default: "false", Description: "Print plan without applying changes."},
	}
	if includeProject {
		inputs = append(inputs, InputField{Name: "project", Type: "string", Required: false, Description: "Apply only one Terraform project."})
	}
	return inputs
}

func buildInputSchema(inputs []InputField) JSONSchema {
	properties := make(map[string]JSONSchema, len(inputs))
	required := make([]string, 0)

	for _, input := range inputs {
		property := schemaForType(input.Type)
		property.Description = input.Description
		property.Enum = input.Enum
		if input.Default != "" {
			property.Default = defaultValueForType(input.Type, input.Default)
		}
		if input.Repeatable {
			item := property
			item.Default = nil
			property = JSONSchema{
				Type:        "array",
				Description: input.Description,
				Items:       &item,
			}
			if len(input.Enum) > 0 {
				property.Items.Enum = input.Enum
			}
		}

		properties[input.Name] = property
		if input.Required {
			required = append(required, input.Name)
		}
	}

	return JSONSchema{
		Schema:               "https://json-schema.org/draft/2020-12/schema",
		Type:                 "object",
		Properties:           properties,
		Required:             required,
		AdditionalProperties: boolPtr(false),
	}
}

func buildOutputSchema(command CommandContract) JSONSchema {
	switch command.Name {
	case "contract":
		return JSONSchema{
			Schema: "https://json-schema.org/draft/2020-12/schema",
			Type:   "object",
			Properties: map[string]JSONSchema{
				"schema_version": {Type: "string", Enum: []string{ToolContractSchemaVersion}},
				"cli":            {Type: "string"},
				"contract":       {Type: "object"},
				"compatibility":  {Type: "object"},
				"commands":       {Type: "array", Items: &JSONSchema{Type: "object"}},
			},
			Required:             []string{"schema_version", "cli", "contract", "compatibility", "commands"},
			AdditionalProperties: boolPtr(true),
		}
	case "runs list", "runs show":
		stepSchema := JSONSchema{
			Type: "object",
			Properties: map[string]JSONSchema{
				"name":           {Type: "string"},
				"status":         {Type: "string", Enum: []string{"pending", "in_progress", "completed", "failed", "skipped"}},
				"command":        {Type: "string"},
				"output_excerpt": {Type: "string"},
				"error_location": {Type: "string"},
				"started_at":     {Type: "string"},
				"completed_at":   {Type: "string"},
				"duration":       {Type: "string"},
				"error_message":  {Type: "string"},
			},
			Required:             []string{"name", "status"},
			AdditionalProperties: boolPtr(false),
		}
		moduleSchema := JSONSchema{
			Type: "object",
			Properties: map[string]JSONSchema{
				"name":            {Type: "string"},
				"status":          {Type: "string", Enum: []string{"pending", "in_progress", "completed", "failed", "skipped"}},
				"progress":        {Type: "integer"},
				"total_steps":     {Type: "integer"},
				"completed_steps": {Type: "integer"},
				"steps":           {Type: "array", Items: &stepSchema},
				"started_at":      {Type: "string"},
				"completed_at":    {Type: "string"},
				"error_message":   {Type: "string"},
			},
			Required:             []string{"name", "status", "progress", "total_steps", "completed_steps", "steps"},
			AdditionalProperties: boolPtr(false),
		}
		runSchema := JSONSchema{
			Type: "object",
			Properties: map[string]JSONSchema{
				"operation_id":    {Type: "string"},
				"type":            {Type: "string"},
				"started_at":      {Type: "string"},
				"updated_at":      {Type: "string"},
				"status":          {Type: "string", Enum: []string{"pending", "in_progress", "completed", "failed", "cancelled"}},
				"total_steps":     {Type: "integer"},
				"completed_steps": {Type: "integer"},
				"current_step":    {Type: "integer"},
				"modules": {
					Type:                 "object",
					AdditionalProperties: boolPtr(true),
					Description:          "Map of module name to module progress; values follow the module progress schema documented in this contract.",
					Properties:           map[string]JSONSchema{"<module_name>": moduleSchema},
				},
			},
			Required:             []string{"operation_id", "type", "started_at", "updated_at", "status", "total_steps", "completed_steps", "current_step", "modules"},
			AdditionalProperties: boolPtr(false),
		}
		return JSONSchema{
			Schema: "https://json-schema.org/draft/2020-12/schema",
			Type:   "object",
			Properties: map[string]JSONSchema{
				"schema_version": {Type: "string", Enum: []string{"blcli.runs/v1"}},
				"runs":           {Type: "array", Items: &runSchema},
			},
			Required:             []string{"schema_version", "runs"},
			AdditionalProperties: boolPtr(false),
		}
	case "diagnose":
		return JSONSchema{
			Schema: "https://json-schema.org/draft/2020-12/schema",
			Type:   "object",
			Properties: map[string]JSONSchema{
				"schema_version":   {Type: "string", Enum: []string{FailureDiagnosisSchemaVersion}},
				"category":         {Type: "string"},
				"confidence":       {Type: "string", Enum: []string{"low", "medium", "high"}},
				"matched_keywords": {Type: "array", Items: &JSONSchema{Type: "string"}},
				"summary":          {Type: "string"},
				"likely_cause":     {Type: "string"},
				"next_steps":       {Type: "array", Items: &JSONSchema{Type: "string"}},
				"repair_commands":  {Type: "array", Items: &JSONSchema{Type: "string"}},
				"references":       {Type: "array", Items: &JSONSchema{Type: "string"}},
			},
			Required: []string{
				"schema_version",
				"category",
				"confidence",
				"matched_keywords",
				"summary",
				"likely_cause",
				"next_steps",
				"repair_commands",
			},
			AdditionalProperties: boolPtr(false),
		}
	default:
		properties := make(map[string]JSONSchema, len(command.Outputs))
		required := make([]string, 0, len(command.Outputs))
		for _, output := range command.Outputs {
			property := schemaForType(output.Type)
			property.Description = output.Description
			properties[output.Name] = property
			required = append(required, output.Name)
		}

		return JSONSchema{
			Schema:               "https://json-schema.org/draft/2020-12/schema",
			Type:                 "object",
			Properties:           properties,
			Required:             required,
			AdditionalProperties: boolPtr(true),
		}
	}
}

func schemaForType(fieldType string) JSONSchema {
	switch fieldType {
	case "boolean":
		return JSONSchema{Type: "boolean"}
	case "integer":
		return JSONSchema{Type: "integer"}
	case "number":
		return JSONSchema{Type: "number"}
	case "object":
		return JSONSchema{Type: "object", AdditionalProperties: boolPtr(true)}
	case "array":
		return JSONSchema{Type: "array"}
	case "path", "duration", "text", "string":
		return JSONSchema{Type: "string"}
	default:
		return JSONSchema{Type: "string"}
	}
}

func defaultValueForType(fieldType, value string) interface{} {
	switch fieldType {
	case "boolean":
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	case "integer":
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return value
}

func boolPtr(value bool) *bool {
	return &value
}
