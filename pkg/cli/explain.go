package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

// NewExplainCommand creates the explain command
func NewExplainCommand() *cobra.Command {
	var module string
	var component string
	var listOnly bool

	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Explain components and their parameters from config.yaml",
		Long: `Explain components and their parameters from config.yaml files in the template repository.

This command shows:
- Component configuration (install/upgrade commands)
- Parameter definitions from args.yaml files
- Parameter examples and descriptions

You can filter by module (terraform, kubernetes, gitops) and component name.`,
		Example: `  # Explain all components in terraform module
  blcli explain -r github.com/user/repo -m terraform

  # Explain a specific component
  blcli explain -r github.com/user/repo -m terraform -c vpc

  # List all available components in terraform module
  blcli explain -r github.com/user/repo -m terraform -l

  # Explain all components
  blcli explain -r github.com/user/repo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			templateRepoValue := templateRepo
			if templateRepoValue == "" {
				templateRepoValue = "github.com/ggsrc/infra-template"
			}

			return explainComponents(templateRepoValue, module, component, listOnly, forceUpdate, cacheExpiry)
		},
	}

	cmd.Flags().StringVarP(&templateRepo, "template-repo", "r", "github.com/ggsrc/infra-template",
		`Template repository (GitHub repo or local path). Supports public and private repos.
Format: 
  - GitHub: github.com/user/repo or github.com/user/repo@branch
  - Local path: /path/to/template or ./relative/path or $HOME/code/bl-template
For private repos, set GITHUB_TOKEN env var or use 'gh auth login'`)
	cmd.Flags().StringVarP(&module, "module", "m", "",
		"Module name to filter (terraform, kubernetes, gitops). If not specified, shows all modules.")
	cmd.Flags().StringVarP(&component, "component", "c", "",
		"Component name to filter. If not specified, shows all components in the module.")
	cmd.Flags().BoolVarP(&listOnly, "list", "l", false,
		"List all available component names only, without detailed information")
	cmd.Flags().BoolVarP(&forceUpdate, "force-update", "f", false,
		"Force update templates from remote repository, ignoring cache")
	cmd.Flags().DurationVar(&cacheExpiry, "cache-expiry", 24*time.Hour,
		"Cache expiry duration for templates (e.g., 1h, 30m, 0 = no expiry)")

	return cmd
}

// explainComponents explains components from config.yaml files
func explainComponents(repoURL, moduleFilter, componentFilter string, listOnly bool, forceUpdate bool, cacheExpiry time.Duration) error {
	if !listOnly {
		fmt.Printf("Loading templates from: %s\n", repoURL)
	}

	loaderOptions := template.LoaderOptions{
		ForceUpdate: forceUpdate,
		CacheExpiry: cacheExpiry,
	}
	if loaderOptions.CacheExpiry == 0 {
		loaderOptions.CacheExpiry = 24 * time.Hour
	}
	loader := template.NewLoaderWithOptions(repoURL, loaderOptions)

	modules := []string{"terraform", "kubernetes", "gitops"}
	if moduleFilter != "" {
		modules = []string{moduleFilter}
	}

	for _, mod := range modules {
		if err := explainModule(loader, mod, componentFilter, listOnly); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to explain module %s: %v\n", mod, err)
			continue
		}
	}

	return nil
}

// explainModule explains components in a specific module
func explainModule(loader *template.Loader, moduleName, componentFilter string, listOnly bool) error {
	configPath := fmt.Sprintf("%s/config.yaml", moduleName)
	if !loader.CacheExists(configPath) {
		return fmt.Errorf("config.yaml not found for module %s", moduleName)
	}

	content, err := loader.LoadTemplate(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch moduleName {
	case "terraform":
		if listOnly {
			return listTerraformComponents(content, componentFilter)
		}
		return explainTerraformModule(loader, content, componentFilter)
	case "kubernetes":
		if listOnly {
			return listKubernetesComponents(content, componentFilter)
		}
		return explainKubernetesModule(loader, content, componentFilter)
	case "gitops":
		if listOnly {
			return listGitopsComponents(content, componentFilter)
		}
		return explainGitopsModule(loader, content, componentFilter)
	default:
		return fmt.Errorf("unknown module: %s", moduleName)
	}
}

// explainTerraformModule explains terraform components
func explainTerraformModule(loader *template.Loader, configContent string, componentFilter string) error {
	var config template.TerraformConfig
	if err := yaml.Unmarshal([]byte(configContent), &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	fmt.Printf("\n=== Terraform Module ===\n\n")

	// Explain init components
	if len(config.Init) > 0 {
		fmt.Printf("## Init Components\n\n")
		for _, initItem := range config.Init {
			if componentFilter != "" && initItem.Name != componentFilter {
				continue
			}
			explainInitItem(loader, initItem)
		}
	}

	// Explain projects
	if len(config.Projects) > 0 {
		fmt.Printf("## Project Components\n\n")
		for _, project := range config.Projects {
			if componentFilter != "" && project.Name != componentFilter {
				continue
			}
			explainProjectItem(loader, project)
		}
	}

	return nil
}

// explainKubernetesModule explains kubernetes components
func explainKubernetesModule(loader *template.Loader, configContent string, componentFilter string) error {
	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(configContent), &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	fmt.Printf("\n=== Kubernetes Module ===\n\n")

	// Explain components (new unified structure)
	if components, ok := config["components"].([]interface{}); ok {
		fmt.Printf("## Components\n\n")
		for _, compItem := range components {
			if compMap, ok := compItem.(map[string]interface{}); ok {
				name, ok := compMap["name"].(string)
				if !ok {
					continue
				}
				if componentFilter != "" && name != componentFilter {
					continue
				}
				explainKubernetesComponent(loader, name, compMap, "kubernetes")
			}
		}
	}

	return nil
}

// explainGitopsModule explains gitops components
func explainGitopsModule(loader *template.Loader, configContent string, componentFilter string) error {
	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(configContent), &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	fmt.Printf("\n=== GitOps Module ===\n\n")

	if baseTemplates, ok := config["base-templates"].([]interface{}); ok {
		fmt.Printf("## Base Templates\n\n")
		for _, templateData := range baseTemplates {
			if templateMap, ok := templateData.(map[string]interface{}); ok {
				name, _ := templateMap["name"].(string)
				if componentFilter != "" && name != componentFilter {
					continue
				}
				explainGitopsTemplate(loader, templateMap)
			}
		}
	}

	return nil
}

// explainInitItem explains an init item
func explainInitItem(loader *template.Loader, item template.InitItem) {
	fmt.Printf("### %s\n\n", item.Name)
	fmt.Printf("- **Type**: Init Component\n")
	if item.Install != "" {
		fmt.Printf("- **Install**: `%s`\n", item.Install)
	}
	if item.Upgrade != "" {
		fmt.Printf("- **Upgrade**: `%s`\n", item.Upgrade)
	}
	if len(item.Path) > 0 {
		fmt.Printf("- **Path**: %s\n", strings.Join(item.Path, ", "))
	}

	// Load and explain args
	argsPath := findArgsPath(loader, item.Args, item.Path)
	if argsPath != "" {
		explainArgs(loader, argsPath, item.Name)
	}

	fmt.Printf("\n")
}

// explainProjectItem explains a project item
func explainProjectItem(loader *template.Loader, item template.ProjectItem) {
	fmt.Printf("### %s\n\n", item.Name)
	fmt.Printf("- **Type**: Project Component\n")
	if item.Install != "" {
		fmt.Printf("- **Install**: `%s`\n", item.Install)
	}
	if item.Upgrade != "" {
		fmt.Printf("- **Upgrade**: `%s`\n", item.Upgrade)
	}
	if len(item.Path) > 0 {
		fmt.Printf("- **Path**: %s\n", strings.Join(item.Path, ", "))
	}
	if len(item.Dependencies) > 0 {
		fmt.Printf("- **Dependencies**: %s\n", strings.Join(item.Dependencies, ", "))
	}

	// Load and explain args
	argsPath := findArgsPath(loader, item.Args, item.Path)
	if argsPath != "" {
		explainArgs(loader, argsPath, item.Name)
	}

	fmt.Printf("\n")
}

// explainKubernetesComponent explains a kubernetes component
func explainKubernetesComponent(loader *template.Loader, name string, compData interface{}, moduleName string) {
	fmt.Printf("### %s\n\n", name)

	if compMap, ok := compData.(map[string]interface{}); ok {
		if install, ok := compMap["install"].(string); ok && install != "" {
			fmt.Printf("- **Install**: `%s`\n", install)
		}
		if upgrade, ok := compMap["upgrade"].(string); ok && upgrade != "" {
			fmt.Printf("- **Upgrade**: `%s`\n", upgrade)
		}
		if path, ok := compMap["path"].(interface{}); ok {
			var paths []string
			switch v := path.(type) {
			case string:
				paths = []string{v}
			case []interface{}:
				for _, p := range v {
					if s, ok := p.(string); ok {
						paths = append(paths, s)
					}
				}
			}
			if len(paths) > 0 {
				fmt.Printf("- **Path**: %s\n", strings.Join(paths, ", "))
				// Try to find args.yaml
				argsPath := findArgsPathFromPaths(loader, paths)
				if argsPath != "" {
					explainArgs(loader, argsPath, name)
				}
			}
		}
	}

	fmt.Printf("\n")
}

// explainGitopsTemplate explains a gitops template
func explainGitopsTemplate(loader *template.Loader, templateMap map[string]interface{}) {
	name, _ := templateMap["name"].(string)
	path, _ := templateMap["path"].(string)

	fmt.Printf("### %s\n\n", name)
	if path != "" {
		fmt.Printf("- **Path**: %s\n", path)
		// Try to find args.yaml
		argsPath := findArgsPathFromPaths(loader, []string{path})
		if argsPath != "" {
			explainArgs(loader, argsPath, name)
		}
	}
	fmt.Printf("\n")
}

// findArgsPath finds args.yaml path from item args or infers from paths
func findArgsPath(loader *template.Loader, args []string, paths []string) string {
	// First, try explicit args paths
	for _, argsPath := range args {
		if argsPath != "" && loader.CacheExists(argsPath) {
			return argsPath
		}
	}

	// If no explicit args, try to infer from paths
	return findArgsPathFromPaths(loader, paths)
}

// findArgsPathFromPaths infers args.yaml path from template paths
func findArgsPathFromPaths(loader *template.Loader, paths []string) string {
	for _, path := range paths {
		if path == "" {
			continue
		}

		// If path is a directory, look for {path}/args.yaml
		// If path is a file, look for {dirname(path)}/args.yaml
		var argsPath string
		if strings.HasSuffix(path, ".tmpl") || strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".tf") {
			// It's a file, use its directory
			argsPath = filepath.Join(filepath.Dir(path), "args.yaml")
		} else {
			// It's a directory
			argsPath = filepath.Join(path, "args.yaml")
		}

		if loader.CacheExists(argsPath) {
			return argsPath
		}
	}

	return ""
}

// explainArgs explains parameters from args.yaml file
func explainArgs(loader *template.Loader, argsPath, componentName string) {
	content, err := loader.LoadTemplate(argsPath)
	if err != nil {
		return // Silently skip if args file doesn't exist or can't be loaded
	}

	def, err := renderer.LoadArgsDefinition(content)
	if err != nil {
		return // Silently skip if args file can't be parsed
	}

	// Extract parameters for this component
	configValues, comments := def.ToConfigValues()
	if components, ok := configValues[renderer.FieldComponents].(map[string]interface{}); ok {
		if compParams, ok := components[componentName].(map[string]interface{}); ok {
			fmt.Printf("\n#### Parameters\n\n")
			explainParameters(compParams, comments, fmt.Sprintf("components.%s", componentName))
		}
	}

	// Also show global parameters if any
	if global, ok := configValues[renderer.FieldGlobal].(map[string]interface{}); ok && len(global) > 0 {
		fmt.Printf("\n#### Global Parameters\n\n")
		explainParameters(global, comments, "global")
	}
}

// explainParameters explains parameter definitions
func explainParameters(params map[string]interface{}, comments map[string]string, prefix string) {
	for paramName, paramValue := range params {
		fullPath := fmt.Sprintf("%s.%s", prefix, paramName)
		comment := comments[fullPath]

		fmt.Printf("- **%s**", paramName)
		if comment != "" {
			fmt.Printf(": %s", comment)
		}
		fmt.Printf("\n")

		// If paramValue is a map, it might contain type, description, example, etc.
		if paramMap, ok := paramValue.(map[string]interface{}); ok {
			if paramType, ok := paramMap["type"].(string); ok {
				fmt.Printf("  - Type: `%s`\n", paramType)
			}
			if description, ok := paramMap["description"].(string); ok {
				fmt.Printf("  - Description: %s\n", description)
			}
			if example, ok := paramMap["example"]; ok {
				fmt.Printf("  - Example: `%v`\n", example)
			}
			if required, ok := paramMap["required"].(bool); ok && required {
				fmt.Printf("  - Required: Yes\n")
			}
			if defaultVal, ok := paramMap["default"]; ok {
				fmt.Printf("  - Default: `%v`\n", defaultVal)
			}
		} else {
			// It's a direct value, show it as example
			fmt.Printf("  - Example: `%v`\n", paramValue)
		}
		fmt.Printf("\n")
	}
}

// listTerraformComponents lists all terraform component names
func listTerraformComponents(configContent string, componentFilter string) error {
	var config template.TerraformConfig
	if err := yaml.Unmarshal([]byte(configContent), &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	fmt.Printf("\n=== Terraform Module Components ===\n\n")

	// List init components
	if len(config.Init) > 0 {
		fmt.Printf("Init Components:\n")
		for _, initItem := range config.Init {
			if componentFilter == "" || initItem.Name == componentFilter {
				fmt.Printf("  - %s\n", initItem.Name)
			}
		}
		fmt.Printf("\n")
	}

	// List project components
	if len(config.Projects) > 0 {
		fmt.Printf("Project Components:\n")
		for _, project := range config.Projects {
			if componentFilter == "" || project.Name == componentFilter {
				fmt.Printf("  - %s", project.Name)
				if len(project.Dependencies) > 0 {
					fmt.Printf(" (depends on: %s)", strings.Join(project.Dependencies, ", "))
				}
				fmt.Printf("\n")
			}
		}
		fmt.Printf("\n")
	}

	return nil
}

// listKubernetesComponents lists all kubernetes component names
func listKubernetesComponents(configContent string, componentFilter string) error {
	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(configContent), &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	fmt.Printf("\n=== Kubernetes Module Components ===\n\n")

	// List components (new unified structure)
	if components, ok := config["components"].([]interface{}); ok {
		fmt.Printf("Components:\n")
		for _, compItem := range components {
			if compMap, ok := compItem.(map[string]interface{}); ok {
				name, ok := compMap["name"].(string)
				if !ok {
					continue
				}
				if componentFilter == "" || name == componentFilter {
					fmt.Printf("  - %s\n", name)
				}
			}
		}
		fmt.Printf("\n")
	}

	return nil
}

// listGitopsComponents lists all gitops component names
func listGitopsComponents(configContent string, componentFilter string) error {
	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(configContent), &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	fmt.Printf("\n=== GitOps Module Components ===\n\n")

	if baseTemplates, ok := config["base-templates"].([]interface{}); ok {
		fmt.Printf("Base Templates:\n")
		for _, templateData := range baseTemplates {
			if templateMap, ok := templateData.(map[string]interface{}); ok {
				name, _ := templateMap["name"].(string)
				if name != "" && (componentFilter == "" || name == componentFilter) {
					fmt.Printf("  - %s\n", name)
				}
			}
		}
		fmt.Printf("\n")
	}

	return nil
}
