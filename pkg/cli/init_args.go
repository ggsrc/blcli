package cli

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

// Use shared types from renderer package
type (
	ComponentData    = renderer.ComponentData
	ProjectData      = renderer.ProjectData
	TerraformSection = renderer.TerraformSection
	ArgsConfig       = renderer.ArgsConfig
)

// NewInitArgsCommand creates the init-args command
func NewInitArgsCommand() *cobra.Command {
	var outputPath string
	var outputFormat = "yaml"
	var org string

	cmd := &cobra.Command{
		Use:   "init-args [template-repo]",
		Short: "Generate args configuration file from template repository",
		Long: `Generate a merged args configuration file from all args.yaml files in the template repository.

This command scans the template repository for all args.yaml files referenced in config.yaml
and generates a single merged configuration file that can be used with --args flag.

The generated file will contain template parameter definitions from:
- init items (terraform/init/args.yaml)
- modules (terraform/modules/*/args.yaml)
- projects (terraform/projects/args.yaml, terraform/project/args.yaml)

By default, the output format is YAML and the output file is args.yaml in the current directory.
You can specify TOML format using --format=toml or by using .toml extension.

You can then customize the generated file and use it with blcli init --args.`,
		Example: `  # Generate args.yaml in current directory (default)
  blcli init-args
  blcli init-args github.com/user/repo

  # Specify output path
  blcli init-args github.com/user/repo -o args.yaml

  # Use local template repository
  blcli init-args /Users/username/code/bl-template
  blcli init-args ./bl-template

  # Generate args.toml (format detected from extension)
  blcli init-args github.com/user/repo -o args.toml

  # Generate TOML format using --format flag
  blcli init-args github.com/user/repo --format=toml -o config.toml

  # Force update templates
  blcli init-args github.com/user/repo -f`,
		RunE: func(cmd *cobra.Command, args []string) error {
			templateRepoValue := "github.com/ggsrc/infra-template"
			if len(args) > 0 {
				templateRepoValue = args[0]
			}

			// Normalize format: yml -> yaml
			finalFormat := strings.ToLower(outputFormat)
			if finalFormat == "yml" {
				finalFormat = "yaml"
			}
			if finalFormat != "yaml" && finalFormat != "toml" {
				return fmt.Errorf("invalid format: %s. Supported formats: yaml, yml, toml", outputFormat)
			}

			// Set default output path if not specified
			if outputPath == "" {
				if finalFormat == "toml" {
					outputPath = "args.toml"
				} else {
					outputPath = "args.yaml"
				}
			}

			return generateArgsFile(templateRepoValue, outputPath, finalFormat, org, forceUpdate, cacheExpiry)
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "args.yaml",
		"Output file path for generated args configuration (default: args.yaml in current directory)")
	cmd.Flags().StringVar(&org, "org", "",
		"Organization name for project ID generation (e.g., my-org); overrides GlobalName in config")
	cmd.Flags().StringVar(&outputFormat, "format", "yaml",
		"Output format: yaml, yml, or toml (default: yaml)")
	cmd.Flags().BoolVarP(&forceUpdate, "force-update", "f", false,
		"Force update templates from remote repository, ignoring cache")
	cmd.Flags().DurationVar(&cacheExpiry, "cache-expiry", 24*time.Hour,
		"Cache expiry duration for templates (e.g., 1h, 30m, 0 = no expiry)")

	return cmd
}

// generateArgsFile generates an args configuration file from default.yaml files in template repository
func generateArgsFile(repoURL, outputPath, format, org string, forceUpdate bool, cacheExpiry time.Duration) error {
	fmt.Printf("Loading templates from: %s\n", repoURL)

	loaderOptions := template.LoaderOptions{
		ForceUpdate: forceUpdate,
		CacheExpiry: cacheExpiry,
	}
	if loaderOptions.CacheExpiry == 0 {
		loaderOptions.CacheExpiry = 24 * time.Hour
	}
	loader := template.NewLoaderWithOptions(repoURL, loaderOptions)

	// Sync remote repository to cache before checking for files
	// For local paths, this is a no-op
	if !loader.IsLocalPath() {
		fmt.Printf("Syncing remote template repository...\n")
	}
	if err := loader.SyncCache(); err != nil {
		return fmt.Errorf("failed to sync template repository: %w", err)
	}

	// For remote repositories, verify that sync was successful by checking if we can access default.yaml files
	// If no default.yaml files are found, the cache might be incomplete, so force an update
	if !loader.IsLocalPath() {
		defaultPaths := []string{"terraform/default.yaml", "kubernetes/default.yaml", "gitops/default.yaml"}
		hasAnyDefault := false
		for _, defaultPath := range defaultPaths {
			if loader.CacheExists(defaultPath) {
				hasAnyDefault = true
				break
			}
		}
		// If no default.yaml files found, force update to ensure we have all necessary files
		if !hasAnyDefault {
			fmt.Printf("No default.yaml files found in cache, forcing update...\n")
			loaderOptions.ForceUpdate = true
			loader = template.NewLoaderWithOptions(repoURL, loaderOptions)
			if err := loader.SyncCache(); err != nil {
				return fmt.Errorf("failed to sync template repository (force update): %w", err)
			}
		}
	}

	// Load root-level args.yaml for global parameters
	var rootGlobal map[string]interface{}
	rootArgsPath := "args.yaml"
	if loader.CacheExists(rootArgsPath) {
		fmt.Printf("  Loading: %s\n", rootArgsPath)
		content, err := loader.LoadTemplate(rootArgsPath)
		if err == nil {
			// Parse args.yaml to extract global parameters
			def, err := renderer.LoadArgsDefinition(content)
			if err == nil {
				configValues, _ := def.ToConfigValues()
				// ToConfigValues already extracts example/default values
				if global, ok := configValues[renderer.FieldGlobal].(map[string]interface{}); ok {
					rootGlobal = global
				}
			} else {
				fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", rootArgsPath, err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Warning: failed to load %s: %v\n", rootArgsPath, err)
		}
	}

	// Load default.yaml files from each module (terraform, kubernetes, gitops)
	defaultPaths := []string{
		"terraform/default.yaml",
		"kubernetes/default.yaml",
		"gitops/default.yaml",
	}

	var allDefaultData map[string]interface{}
	foundAny := false

	for _, defaultPath := range defaultPaths {
		if !loader.CacheExists(defaultPath) {
			fmt.Printf("  Skipping: %s (not found)\n", defaultPath)
			continue
		}

		if !foundAny {
			allDefaultData = make(map[string]interface{})
			foundAny = true
		}

		fmt.Printf("  Loading: %s\n", defaultPath)
		content, err := loader.LoadTemplate(defaultPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load %s: %v\n", defaultPath, err)
			continue
		}

		var moduleData map[string]interface{}
		if err := yaml.Unmarshal([]byte(content), &moduleData); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", defaultPath, err)
			continue
		}

		// Determine module name from path
		// For "terraform/default.yaml", filepath.Dir returns "terraform"
		moduleName := filepath.Dir(defaultPath) // terraform, kubernetes, or gitops

		// Store the entire moduleData under the module name
		// This means allDefaultData["terraform"] = moduleData (which contains global, init, projects, etc.)
		allDefaultData[moduleName] = moduleData
	}

	if !foundAny {
		return fmt.Errorf("no default.yaml files found in template repository (checked: terraform, kubernetes, gitops)")
	}

	// Convert to ArgsConfig structure
	argsConfig := convertToArgsConfig(allDefaultData, rootGlobal)

	// Auto-generate project IDs (org-env-uuid, max 30 chars) and inject into config
	if err := injectGeneratedProjectIDs(&argsConfig, org); err != nil {
		return fmt.Errorf("inject generated project IDs: %w", err)
	}

	// Resolve ${project.<name>.id} placeholders to actual project IDs
	resolveProjectPlaceholders(&argsConfig)

	// Debug: Print what we're about to write
	fmt.Printf("  Converting default.yaml data:\n")
	fmt.Printf("    - Terraform projects: %d\n", len(argsConfig.Terraform.Projects))
	fmt.Printf("    - Terraform global keys: %d\n", len(argsConfig.Terraform.Global))
	if argsConfig.Terraform.Init != nil {
		fmt.Printf("    - Terraform init components: %d\n", len(argsConfig.Terraform.Init.Components))
	}
	fmt.Printf("    - Kubernetes projects: %d\n", len(argsConfig.Kubernetes.Projects))
	fmt.Printf("    - Kubernetes global keys: %d\n", len(argsConfig.Kubernetes.Global))
	if len(argsConfig.Gitops) > 0 {
		if apps, _ := argsConfig.Gitops["apps"].([]interface{}); apps != nil {
			fmt.Printf("    - Gitops apps: %d\n", len(apps))
		}
		if argocd, _ := argsConfig.Gitops["argocd"].(map[string]interface{}); argocd != nil {
			if projects, _ := argocd["project"].([]interface{}); projects != nil {
				fmt.Printf("    - Gitops argocd projects: %d\n", len(projects))
			}
		}
	}

	// Write args file first, then generate a compact .env alongside it for common overrides.
	if err := writeConfigFile(outputPath, format, &argsConfig, nil); err != nil {
		return err
	}

	envPath := filepath.Join(filepath.Dir(outputPath), ".env")
	if err := writeInitArgsEnvFile(envPath, &argsConfig); err != nil {
		return fmt.Errorf("failed to write env file: %w", err)
	}

	return nil
}

func writeInitArgsEnvFile(path string, data *ArgsConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create env directory: %w", err)
	}

	envValues := extractInitArgsEnvValues(data)

	var lines []string
	lines = append(lines,
		"# blcli init overrides",
		"# Generated by \"blcli init-args\"",
		"# Modify these values, then use: blcli init ... -a args.yaml --env-file .env",
		"",
		"# Terraform organization",
		fmt.Sprintf("BLCLI_TERRAFORM_ORGANIZATION_ID=%s", envQuote(envValues["BLCLI_TERRAFORM_ORGANIZATION_ID"])),
		fmt.Sprintf("BLCLI_TERRAFORM_BILLING_ACCOUNT_ID=%s", envQuote(envValues["BLCLI_TERRAFORM_BILLING_ACCOUNT_ID"])),
		"",
		"# GitOps",
		fmt.Sprintf("BLCLI_GITOPS_SOURCE_REPO_URL=%s", envQuote(envValues["BLCLI_GITOPS_SOURCE_REPO_URL"])),
		"",
	)

	for _, appEnv := range extractGitopsAppEnvEntries(data) {
		lines = append(lines,
			fmt.Sprintf("# GitOps app %s/%s", appEnv.ProjectName, appEnv.AppName),
			fmt.Sprintf("BLCLI_GITOPS_%s_%s_APPLICATION_REPO=%s",
				strings.ToUpper(appEnv.ProjectKey),
				strings.ToUpper(appEnv.AppKey),
				envQuote(appEnv.ApplicationRepo),
			),
			"",
		)
	}

	lines = append(lines,
		"# ArgoCD",
		fmt.Sprintf("BLCLI_ARGOCD_GIT_REPOSITORY_URL=%s", envQuote(envValues["BLCLI_ARGOCD_GIT_REPOSITORY_URL"])),
		"",
	)

	for _, projectName := range extractKubernetesProjectNames(data) {
		projectKey := strings.ToUpper(projectName)
		lines = append(lines,
			fmt.Sprintf("# ArgoCD %s", projectName),
			fmt.Sprintf("BLCLI_ARGOCD_%s_URL=%s", projectKey, envQuote(envValues[fmt.Sprintf("BLCLI_ARGOCD_%s_URL", projectKey)])),
			fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_CLIENT_ID=%s", projectKey, envQuote(envValues[fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_CLIENT_ID", projectKey)])),
			fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_CLIENT_SECRET=%s", projectKey, envQuote(envValues[fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_CLIENT_SECRET", projectKey)])),
			fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_ORGS=%s", projectKey, envQuote(envValues[fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_ORGS", projectKey)])),
			fmt.Sprintf("BLCLI_ARGOCD_%s_RBAC_GROUP_PREFIX=%s", projectKey, envQuote(envValues[fmt.Sprintf("BLCLI_ARGOCD_%s_RBAC_GROUP_PREFIX", projectKey)])),
			"",
		)
	}

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("failed to write env file: %w", err)
	}

	fmt.Printf("✅ Generated init override env: %s\n", path)
	return nil
}

func extractInitArgsEnvValues(data *ArgsConfig) map[string]string {
	values := map[string]string{
		"BLCLI_TERRAFORM_ORGANIZATION_ID":       "",
		"BLCLI_TERRAFORM_BILLING_ACCOUNT_ID":    "",
		"BLCLI_GITOPS_SOURCE_REPO_URL":          "",
		"BLCLI_ARGOCD_GIT_REPOSITORY_URL":       "",
		"BLCLI_ARGOCD_URL":                      "",
		"BLCLI_ARGOCD_DEX_GITHUB_CLIENT_ID":     "",
		"BLCLI_ARGOCD_DEX_GITHUB_CLIENT_SECRET": "",
		"BLCLI_ARGOCD_DEX_GITHUB_ORGS":          "",
	}

	if data != nil && data.Terraform.Global != nil {
		values["BLCLI_TERRAFORM_ORGANIZATION_ID"] = stringifyValue(data.Terraform.Global["OrganizationID"])
		values["BLCLI_TERRAFORM_BILLING_ACCOUNT_ID"] = stringifyValue(data.Terraform.Global["BillingAccountID"])
	}

	if data != nil && len(data.Gitops) > 0 {
		if apps, ok := data.Gitops["apps"].([]interface{}); ok {
			values["BLCLI_GITOPS_SOURCE_REPO_URL"] = firstGitopsSourceRepoURL(apps)
		}
	}

	projectArgocdValues := make(map[string]map[string]string)
	for _, project := range data.Kubernetes.Projects {
		for _, component := range project.Components {
			if normalizeOrderedComponentName(component.Name) != "argocd" {
				continue
			}

			params := component.Parameters
			projectValues := map[string]string{}
			projectValues["BLCLI_ARGOCD_GIT_REPOSITORY_URL"] = firstNonEmptyString(
				stringifyValue(params["GitRepositoryURL"]),
				firstRepositoryURL(params["GitRepositories"]),
			)
			projectValues["BLCLI_ARGOCD_URL"] = firstNonEmptyString(
				stringifyValue(params["ArgoCDURL"]),
				stringifyValue(params["argocd-url"]),
			)
			projectValues["BLCLI_ARGOCD_DEX_GITHUB_CLIENT_ID"] = firstNonEmptyString(
				stringifyValue(params["DexGitHubClientID"]),
				extractDexGithubString(params["DexConfig"], "clientID"),
				extractDexGithubString(params["dex-config"], "clientID"),
			)
			projectValues["BLCLI_ARGOCD_DEX_GITHUB_CLIENT_SECRET"] = firstNonEmptyString(
				stringifyValue(params["DexGitHubClientSecret"]),
				extractDexGithubString(params["DexConfig"], "clientSecret"),
				extractDexGithubString(params["dex-config"], "clientSecret"),
			)
			projectValues["BLCLI_ARGOCD_DEX_GITHUB_ORGS"] = firstNonEmptyString(
				joinStringSlice(params["DexGitHubOrgs"]),
				extractDexGithubOrgs(params["DexConfig"]),
				extractDexGithubOrgs(params["dex-config"]),
			)
			projectValues["BLCLI_ARGOCD_RBAC_GROUP_PREFIX"] = extractRBACGroupPrefix(params["RBACRoleBindings"])

			projectArgocdValues[project.Name] = projectValues

			projectKey := strings.ToUpper(project.Name)
			values[fmt.Sprintf("BLCLI_ARGOCD_%s_URL", projectKey)] = projectValues["BLCLI_ARGOCD_URL"]
			values[fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_CLIENT_ID", projectKey)] = projectValues["BLCLI_ARGOCD_DEX_GITHUB_CLIENT_ID"]
			values[fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_CLIENT_SECRET", projectKey)] = projectValues["BLCLI_ARGOCD_DEX_GITHUB_CLIENT_SECRET"]
			values[fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_ORGS", projectKey)] = projectValues["BLCLI_ARGOCD_DEX_GITHUB_ORGS"]
			values[fmt.Sprintf("BLCLI_ARGOCD_%s_RBAC_GROUP_PREFIX", projectKey)] = projectValues["BLCLI_ARGOCD_RBAC_GROUP_PREFIX"]
			break
		}
	}

	values["BLCLI_ARGOCD_GIT_REPOSITORY_URL"] = firstNonEmptyProjectValue(projectArgocdValues, "BLCLI_ARGOCD_GIT_REPOSITORY_URL")
	values["BLCLI_ARGOCD_URL"] = firstNonEmptyProjectValue(projectArgocdValues, "BLCLI_ARGOCD_URL")
	values["BLCLI_ARGOCD_DEX_GITHUB_CLIENT_ID"] = firstNonEmptyProjectValue(projectArgocdValues, "BLCLI_ARGOCD_DEX_GITHUB_CLIENT_ID")
	values["BLCLI_ARGOCD_DEX_GITHUB_CLIENT_SECRET"] = firstNonEmptyProjectValue(projectArgocdValues, "BLCLI_ARGOCD_DEX_GITHUB_CLIENT_SECRET")
	values["BLCLI_ARGOCD_DEX_GITHUB_ORGS"] = firstNonEmptyProjectValue(projectArgocdValues, "BLCLI_ARGOCD_DEX_GITHUB_ORGS")

	for _, projectName := range extractKubernetesProjectNames(data) {
		projectKey := strings.ToUpper(projectName)
		values[fmt.Sprintf("BLCLI_ARGOCD_%s_URL", projectKey)] = firstNonEmptyString(
			values[fmt.Sprintf("BLCLI_ARGOCD_%s_URL", projectKey)],
			values["BLCLI_ARGOCD_URL"],
		)
		values[fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_CLIENT_ID", projectKey)] = firstNonEmptyString(
			values[fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_CLIENT_ID", projectKey)],
			values["BLCLI_ARGOCD_DEX_GITHUB_CLIENT_ID"],
		)
		values[fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_CLIENT_SECRET", projectKey)] = firstNonEmptyString(
			values[fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_CLIENT_SECRET", projectKey)],
			values["BLCLI_ARGOCD_DEX_GITHUB_CLIENT_SECRET"],
		)
		values[fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_ORGS", projectKey)] = firstNonEmptyString(
			values[fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_ORGS", projectKey)],
			values["BLCLI_ARGOCD_DEX_GITHUB_ORGS"],
		)
		values[fmt.Sprintf("BLCLI_ARGOCD_%s_RBAC_GROUP_PREFIX", projectKey)] = firstNonEmptyString(
			values[fmt.Sprintf("BLCLI_ARGOCD_%s_RBAC_GROUP_PREFIX", projectKey)],
			firstNonEmptyProjectValue(projectArgocdValues, "BLCLI_ARGOCD_RBAC_GROUP_PREFIX"),
		)
	}

	return values
}

func firstNonEmptyProjectValue(projectValues map[string]map[string]string, key string) string {
	for _, values := range projectValues {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return ""
}

func extractKubernetesProjectNames(data *ArgsConfig) []string {
	result := make([]string, 0, len(data.Kubernetes.Projects))
	for _, project := range data.Kubernetes.Projects {
		if project.Name != "" {
			result = append(result, project.Name)
		}
	}
	return result
}

type gitopsAppEnvEntry struct {
	ProjectName     string
	AppName         string
	ProjectKey      string
	AppKey          string
	ApplicationRepo string
}

func extractGitopsAppEnvEntries(data *ArgsConfig) []gitopsAppEnvEntry {
	if data == nil || len(data.Gitops) == 0 {
		return nil
	}

	apps, ok := data.Gitops["apps"].([]interface{})
	if !ok || len(apps) == 0 {
		return nil
	}

	var entries []gitopsAppEnvEntry
	for _, appItem := range apps {
		appMap, ok := appItem.(map[string]interface{})
		if !ok {
			continue
		}
		appName := stringifyValue(appMap["name"])
		if appName == "" {
			continue
		}

		projects, ok := appMap["project"].([]interface{})
		if !ok || len(projects) == 0 {
			continue
		}

		for _, projectItem := range projects {
			projectMap, ok := projectItem.(map[string]interface{})
			if !ok {
				continue
			}
			projectName := stringifyValue(projectMap["name"])
			if projectName == "" {
				continue
			}

			params, _ := projectMap["parameters"].(map[string]interface{})
			applicationRepo := stringifyValue(params["ApplicationRepo"])
			if applicationRepo == "" {
				continue
			}

			entries = append(entries, gitopsAppEnvEntry{
				ProjectName:     projectName,
				AppName:         appName,
				ProjectKey:      envKeyPart(projectName),
				AppKey:          envKeyPart(appName),
				ApplicationRepo: applicationRepo,
			})
		}
	}

	return entries
}

func firstGitopsSourceRepoURL(apps []interface{}) string {
	for _, appItem := range apps {
		appMap, ok := appItem.(map[string]interface{})
		if !ok {
			continue
		}
		projects, ok := appMap["project"].([]interface{})
		if !ok {
			continue
		}
		for _, projectItem := range projects {
			projectMap, ok := projectItem.(map[string]interface{})
			if !ok {
				continue
			}
			params, ok := projectMap["parameters"].(map[string]interface{})
			if !ok {
				continue
			}
			if repo := stringifyValue(params["SourceRepoURL"]); repo != "" {
				return repo
			}
		}
	}
	return ""
}

func envKeyPart(value string) string {
	value = strings.ToUpper(value)
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore && b.Len() > 0 {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func extractRBACGroupPrefix(v interface{}) string {
	list, ok := v.([]interface{})
	if !ok || len(list) == 0 {
		return ""
	}
	first, ok := list[0].(map[string]interface{})
	if !ok {
		return ""
	}
	group := stringifyValue(first["group"])
	if idx := strings.Index(group, ":"); idx >= 0 {
		return group[:idx]
	}
	return group
}

func normalizeOrderedComponentName(name string) string {
	for i, r := range name {
		if r < '0' || r > '9' {
			if r == '-' && i > 0 {
				return name[i+1:]
			}
			return name
		}
	}
	return name
}

func firstRepositoryURL(v interface{}) string {
	list, ok := v.([]interface{})
	if !ok || len(list) == 0 {
		return ""
	}
	first, ok := list[0].(map[string]interface{})
	if !ok {
		return ""
	}
	return stringifyValue(first["url"])
}

func extractDexGithubString(v interface{}, key string) string {
	configMap, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	connectors, ok := configMap["connectors"].([]interface{})
	if !ok {
		return ""
	}
	for _, connector := range connectors {
		connectorMap, ok := connector.(map[string]interface{})
		if !ok {
			continue
		}
		if stringifyValue(connectorMap["type"]) != "github" {
			continue
		}
		cfg, ok := connectorMap["config"].(map[string]interface{})
		if !ok {
			continue
		}
		return stringifyValue(cfg[key])
	}
	return ""
}

func extractDexGithubOrgs(v interface{}) string {
	configMap, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	connectors, ok := configMap["connectors"].([]interface{})
	if !ok {
		return ""
	}
	for _, connector := range connectors {
		connectorMap, ok := connector.(map[string]interface{})
		if !ok {
			continue
		}
		if stringifyValue(connectorMap["type"]) != "github" {
			continue
		}
		cfg, ok := connectorMap["config"].(map[string]interface{})
		if !ok {
			continue
		}
		orgs, ok := cfg["orgs"].([]interface{})
		if !ok {
			return ""
		}
		names := make([]string, 0, len(orgs))
		for _, org := range orgs {
			orgMap, ok := org.(map[string]interface{})
			if !ok {
				continue
			}
			name := stringifyValue(orgMap["name"])
			if name != "" {
				names = append(names, name)
			}
		}
		return strings.Join(names, ",")
	}
	return ""
}

func joinStringSlice(v interface{}) string {
	list, ok := v.([]interface{})
	if ok {
		parts := make([]string, 0, len(list))
		for _, item := range list {
			s := stringifyValue(item)
			if s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ",")
	}
	stringsList, ok := v.([]string)
	if ok {
		return strings.Join(stringsList, ",")
	}
	return ""
}

func stringifyValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch value := v.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	default:
		return fmt.Sprintf("%v", value)
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func envQuote(value string) string {
	if value == "" {
		return "\"\""
	}
	if strings.ContainsAny(value, " #\"\n\t,") {
		return fmt.Sprintf("%q", value)
	}
	return value
}

// extractGlobalValues extracts example/default values from global parameter definitions
func extractGlobalValues(globalParams map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for paramName, paramDef := range globalParams {
		if paramMap, ok := paramDef.(map[string]interface{}); ok {
			// Try to get example value first, then default, then use the example from args.yaml
			if example, ok := paramMap["example"]; ok {
				result[paramName] = example
			} else if defaultVal, ok := paramMap["default"]; ok {
				result[paramName] = defaultVal
			}
		}
	}
	return result
}

// gcpProjectIDMaxLen is the maximum length for a GCP project ID.
const gcpProjectIDMaxLen = 30

// generateProjectIDs returns a map of project name -> generated project ID (org-<name>-<8hex>, max 30 chars).
// Project name is sanitized for the middle segment: lowercase, hyphens, max 14 chars to fit GCP limit.
func generateProjectIDs(projectNames []string, orgPrefix string) (map[string]string, error) {
	out := make(map[string]string)
	shortLen := 8 // 8 hex chars
	b := make([]byte, (shortLen+1)/2)
	for _, name := range projectNames {
		env := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		if len(env) > 14 {
			env = env[:14]
		}
		prefix := orgPrefix + "-" + env + "-"
		if len(prefix) > gcpProjectIDMaxLen-shortLen {
			prefix = prefix[:gcpProjectIDMaxLen-shortLen]
		}
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		id := prefix + hex.EncodeToString(b)[:shortLen]
		if len(id) > gcpProjectIDMaxLen {
			id = id[:gcpProjectIDMaxLen]
		}
		out[name] = id
	}
	return out, nil
}

// injectGeneratedProjectIDs generates org-env-uuid style project IDs and injects them into argsConfig,
// replacing default.yaml placeholder values so rendered output uses unique IDs (max 30 chars).
// orgOverride: if non-empty, used as org prefix; otherwise falls back to GlobalName in config.
// All projects (including marshalled: false) participate in ID generation for placeholder resolution.
func injectGeneratedProjectIDs(cfg *ArgsConfig, orgOverride string) error {
	var projectNames []string
	for _, p := range cfg.Terraform.Projects {
		if p.Name != "" {
			projectNames = append(projectNames, p.Name)
		}
	}
	if len(projectNames) == 0 {
		return nil
	}
	orgPrefix := "org"
	if orgOverride != "" {
		orgPrefix = orgOverride
		// Write org to config so init uses it
		if cfg.Terraform.Global == nil {
			cfg.Terraform.Global = make(map[string]interface{})
		}
		cfg.Terraform.Global["GlobalName"] = orgOverride
		if cfg.Global == nil {
			cfg.Global = make(map[string]interface{})
		}
		cfg.Global["GlobalName"] = orgOverride
	} else {
		// Prefer terraform.global.GlobalName; if not set, fall back to top-level global.GlobalName
		if cfg.Terraform.Global != nil {
			if v, _ := cfg.Terraform.Global["GlobalName"].(string); v != "" {
				orgPrefix = v
			}
		}
		if cfg.Global != nil {
			if v, _ := cfg.Global["GlobalName"].(string); v != "" {
				orgPrefix = v
			}
		}
	}
	nameToID, err := generateProjectIDs(projectNames, orgPrefix)
	if err != nil {
		return err
	}

	// TerraformBackendBucket: if value is a project name (e.g. tfstore), replace with generated ID; no separate naming.
	if cfg.Terraform.Global != nil {
		if v, ok := cfg.Terraform.Global["TerraformBackendBucket"].(string); ok && v != "" {
			if id, ok := nameToID[v]; ok {
				cfg.Terraform.Global["TerraformBackendBucket"] = id
			}
		}
	}

	// Terraform projects: write id to each project, inject project_id into global and components
	for i := range cfg.Terraform.Projects {
		projectName := cfg.Terraform.Projects[i].Name
		projectID := nameToID[projectName]
		if projectID == "" {
			continue
		}
		cfg.Terraform.Projects[i].ID = projectID

		// Ensure project.global has project_id and project_name
		if cfg.Terraform.Projects[i].Global == nil {
			cfg.Terraform.Projects[i].Global = make(map[string]interface{})
		}
		cfg.Terraform.Projects[i].Global["project_id"] = projectID
		cfg.Terraform.Projects[i].Global["project_name"] = projectName

		for j := range cfg.Terraform.Projects[i].Components {
			params := cfg.Terraform.Projects[i].Components[j].Parameters
			if params == nil {
				continue
			}
			for key, val := range params {
				switch key {
				case "project_id", "projectId":
					params[key] = projectID
				default:
					params[key] = replaceProjectIDInValue(val, nameToID)
				}
			}
		}
	}

	// Kubernetes projects: component parameters (project_id / project-id, and deep replace)
	for i := range cfg.Kubernetes.Projects {
		projectName := cfg.Kubernetes.Projects[i].Name
		projectID := nameToID[projectName]
		if projectID == "" {
			continue
		}
		for j := range cfg.Kubernetes.Projects[i].Components {
			params := cfg.Kubernetes.Projects[i].Components[j].Parameters
			if params == nil {
				continue
			}
			for key, val := range params {
				if key == "project-id" || key == "project_id" {
					params[key] = projectID
				} else {
					params[key] = replaceProjectIDInValue(val, nameToID)
				}
			}
			// Ensure project_id is set for templates that use .project_id (no PascalCase)
			if params["project_id"] == nil {
				params["project_id"] = projectID
			}
		}
	}

	return nil
}

// projectPlaceholderRegex matches ${project.<name>.id} placeholders
var projectPlaceholderRegex = regexp.MustCompile(`\$\{project\.([^}]+)\.id\}`)

// resolveProjectPlaceholders recursively replaces ${project.<name>.id} placeholders with actual project IDs.
func resolveProjectPlaceholders(cfg *ArgsConfig) {
	nameToID := make(map[string]string)
	for _, p := range cfg.Terraform.Projects {
		if p.Name != "" && p.ID != "" {
			nameToID[p.Name] = p.ID
		}
	}
	if len(nameToID) == 0 {
		return
	}
	resolveProjectPlaceholdersInConfig(cfg, nameToID)
}

func resolvePlaceholderInString(s string, nameToID map[string]string) string {
	return projectPlaceholderRegex.ReplaceAllStringFunc(s, func(match string) string {
		sub := projectPlaceholderRegex.FindStringSubmatch(match)
		if len(sub) >= 2 {
			if id, ok := nameToID[sub[1]]; ok {
				return id
			}
		}
		return match
	})
}

// resolveProjectPlaceholdersInConfig walks cfg and replaces placeholders.
func resolveProjectPlaceholdersInConfig(cfg *ArgsConfig, nameToID map[string]string) {
	// Walk Terraform.Init.Components and other nested structures
	if cfg.Terraform.Init != nil && cfg.Terraform.Init.Components != nil {
		resolveInMap(cfg.Terraform.Init.Components, nameToID)
	}
	// Also resolve in projects
	for i := range cfg.Terraform.Projects {
		if cfg.Terraform.Projects[i].Global != nil {
			resolveInMap(cfg.Terraform.Projects[i].Global, nameToID)
		}
		for j := range cfg.Terraform.Projects[i].Components {
			if cfg.Terraform.Projects[i].Components[j].Parameters != nil {
				resolveInMap(cfg.Terraform.Projects[i].Components[j].Parameters, nameToID)
			}
		}
	}
	for i := range cfg.Kubernetes.Projects {
		if cfg.Kubernetes.Projects[i].Global != nil {
			resolveInMap(cfg.Kubernetes.Projects[i].Global, nameToID)
		}
		for j := range cfg.Kubernetes.Projects[i].Components {
			if cfg.Kubernetes.Projects[i].Components[j].Parameters != nil {
				resolveInMap(cfg.Kubernetes.Projects[i].Components[j].Parameters, nameToID)
			}
		}
	}
	if cfg.Global != nil {
		resolveInMap(cfg.Global, nameToID)
	}
	if cfg.Terraform.Global != nil {
		resolveInMap(cfg.Terraform.Global, nameToID)
	}
}

func resolveInMap(m map[string]interface{}, nameToID map[string]string) {
	replaceKeys := make(map[string]string)
	for k, v := range m {
		resolveInValue(&v, nameToID)
		m[k] = v
		if newK := resolvePlaceholderInString(k, nameToID); newK != k {
			replaceKeys[k] = newK
		}
	}
	for oldK, newK := range replaceKeys {
		m[newK] = m[oldK]
		delete(m, oldK)
	}
}

func resolveInValue(v *interface{}, nameToID map[string]string) {
	switch x := (*v).(type) {
	case string:
		*v = resolvePlaceholderInString(x, nameToID)
	case map[string]interface{}:
		resolveInMap(x, nameToID)
	case map[interface{}]interface{}:
		for k, val := range x {
			resolveInValue(&val, nameToID)
			x[k] = val
		}
		// Replace keys that are placeholders (collect first to avoid modifying while iterating)
		type kv struct {
			oldK, newK interface{}
			val        interface{}
		}
		var toReplace []kv
		for k, val := range x {
			if kStr, ok := k.(string); ok {
				if newK := resolvePlaceholderInString(kStr, nameToID); newK != kStr {
					toReplace = append(toReplace, kv{k, newK, val})
				}
			}
		}
		for _, r := range toReplace {
			delete(x, r.oldK)
			x[r.newK] = r.val
		}
	case []interface{}:
		for i := range x {
			resolveInValue(&x[i], nameToID)
		}
	}
}

// replaceProjectIDInValue recursively replaces string values that match a project name (old ID) with the generated project ID.
func replaceProjectIDInValue(v interface{}, nameToID map[string]string) interface{} {
	switch x := v.(type) {
	case string:
		if id, ok := nameToID[x]; ok {
			return id
		}
		// Replace substrings like @prd. or /projects/prd/ so URLs and emails get new ID
		for name, id := range nameToID {
			x = strings.ReplaceAll(x, "@"+name+".", "@"+id+".")
			x = strings.ReplaceAll(x, "/projects/"+name+"/", "/projects/"+id+"/")
		}
		return x
	case map[string]interface{}:
		out := make(map[string]interface{})
		for k, val := range x {
			out[k] = replaceProjectIDInValue(val, nameToID)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, val := range x {
			out[i] = replaceProjectIDInValue(val, nameToID)
		}
		return out
	default:
		return v
	}
}

// convertToArgsConfig converts default.yaml data to ArgsConfig structure
func convertToArgsConfig(defaultData map[string]interface{}, rootGlobal map[string]interface{}) ArgsConfig {
	var result ArgsConfig

	// Top-level global from bl-template/args.yaml
	result.Global = rootGlobal

	// Extract terraform data from default.yaml if exists
	// defaultData structure: map["terraform"] = map[string]interface{...}
	if terraformData, ok := defaultData["terraform"].(map[string]interface{}); ok {
		// Build terraform section
		if version, ok := terraformData["version"].(string); ok {
			result.Terraform.Version = version
		}
		// terraform.global (same as top-level global for terraform)
		if global, ok := terraformData["global"].(map[string]interface{}); ok {
			result.Terraform.Global = global
		}
		// terraform.init.components
		if init, ok := terraformData["init"].(map[string]interface{}); ok {
			if components, ok := init["components"].(map[string]interface{}); ok {
				result.Terraform.Init = &renderer.InitSection{
					Components: components,
				}
			}
		}
		// terraform.projects
		if projects, ok := terraformData["projects"].([]interface{}); ok {
			result.Terraform.Projects = convertProjectsList(projects)
		}
	}

	// Extract kubernetes data if exists
	if kubernetesData, ok := defaultData["kubernetes"].(map[string]interface{}); ok {
		// Build kubernetes section
		if version, ok := kubernetesData["version"].(string); ok {
			result.Kubernetes.Version = version
		}
		// kubernetes.global (same as top-level global for kubernetes)
		if global, ok := kubernetesData["global"].(map[string]interface{}); ok {
			result.Kubernetes.Global = global
		}
		// kubernetes.projects
		if projects, ok := kubernetesData["projects"].([]interface{}); ok {
			result.Kubernetes.Projects = convertProjectsList(projects)
		}
		// Note: Kubernetes init components are defined in config.yaml, not default.yaml
		// They will be handled during bootstrap phase
	}

	// Extract gitops data if exists (argocd.project, apps from default.yaml)
	if gitopsData, ok := defaultData["gitops"].(map[string]interface{}); ok {
		result.Gitops = gitopsData
	}

	return result
}

// convertProjectsList converts projects list from default.yaml format to ProjectData format
func convertProjectsList(projects []interface{}) []ProjectData {
	result := make([]ProjectData, 0, len(projects))

	for _, p := range projects {
		projectMap, ok := p.(map[string]interface{})
		if !ok {
			if projectMapAny, ok := p.(map[interface{}]interface{}); ok {
				projectMap = make(map[string]interface{})
				for k, v := range projectMapAny {
					if kStr, ok := k.(string); ok {
						projectMap[kStr] = v
					}
				}
			} else {
				continue
			}
		}

		projectData := ProjectData{}

		if name, ok := projectMap["name"].(string); ok {
			projectData.Name = name
		}

		if id, ok := projectMap["id"].(string); ok {
			projectData.ID = id
		}

		if marshalled, ok := projectMap["marshalled"].(bool); ok {
			projectData.Marshalled = &marshalled
		}

		if global, ok := projectMap["global"].(map[string]interface{}); ok {
			projectData.Global = global
		} else if globalAny, ok := projectMap["global"].(map[interface{}]interface{}); ok {
			projectData.Global = make(map[string]interface{})
			for k, v := range globalAny {
				if kStr, ok := k.(string); ok {
					projectData.Global[kStr] = v
				}
			}
		}

		if components, ok := projectMap["components"].([]interface{}); ok {
			projectData.Components = convertComponentsList(components)
		}

		result = append(result, projectData)
	}

	return result
}

// convertComponentsList converts components list from default.yaml format to ComponentData format
func convertComponentsList(components []interface{}) []ComponentData {
	result := make([]ComponentData, 0, len(components))

	for _, c := range components {
		if compMap, ok := c.(map[string]interface{}); ok {
			compData := ComponentData{}

			if name, ok := compMap["name"].(string); ok {
				compData.Name = name
			}

			if parameters, ok := compMap["parameters"].(map[string]interface{}); ok {
				compData.Parameters = parameters
			}

			result = append(result, compData)
		}
	}

	return result
}

// createLoaderAndLoadConfig creates a template loader and loads terraform config
func createLoaderAndLoadConfig(repoURL string, forceUpdate bool, cacheExpiry time.Duration) (*template.Loader, *template.TerraformConfig, error) {
	loaderOptions := template.LoaderOptions{
		ForceUpdate: forceUpdate,
		CacheExpiry: cacheExpiry,
	}
	if loaderOptions.CacheExpiry == 0 {
		loaderOptions.CacheExpiry = 24 * time.Hour
	}
	loader := template.NewLoaderWithOptions(repoURL, loaderOptions)

	terraformConfig, err := loader.LoadTerraformConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load terraform config: %w", err)
	}

	return loader, terraformConfig, nil
}

// collectArgsPaths collects all unique args.yaml paths from terraform config
func collectArgsPaths(terraformConfig *template.TerraformConfig, loader *template.Loader) map[string]bool {
	argsPaths := make(map[string]bool)

	// 1. Collect repo-level global args (bl-template/args.yaml)
	repoArgsPath := "args.yaml"
	if loader.CacheExists(repoArgsPath) {
		argsPaths[repoArgsPath] = true
	}

	// 2. Collect terraform-level global args (terraform/args.yaml)
	terraformArgsPath := "terraform/args.yaml"
	if loader.CacheExists(terraformArgsPath) {
		argsPaths[terraformArgsPath] = true
	}

	// 3. Collect project-level global args (project-global field)
	if terraformConfig.ProjectGlobal != "" {
		argsPaths[terraformConfig.ProjectGlobal] = true
	}

	// 4. Collect args paths from init items
	for _, item := range terraformConfig.Init {
		// Handle both single string (backward compatible) and list of strings
		if len(item.Args) == 0 {
			// If Args is empty, use the standard path: terraform/init/args.yaml
			argsPath := "terraform/init/args.yaml"
			if loader.CacheExists(argsPath) {
				argsPaths[argsPath] = true
			}
		} else {
			// Add all args paths from the list
			for _, argsPath := range item.Args {
				if argsPath != "" {
					argsPaths[argsPath] = true
				}
			}
		}
	}

	// If no args paths found from init items, try checking terraform/init/args.yaml directly
	// This is a fallback for cases where config.yaml doesn't specify args paths
	if len(argsPaths) == 0 {
		fmt.Printf("No args paths found in config, checking terraform/init/args.yaml...\n")
		argsPath := "terraform/init/args.yaml"
		if loader.CacheExists(argsPath) {
			argsPaths[argsPath] = true
		}
	}

	// 5. Collect args paths from modules
	for _, module := range terraformConfig.Modules {
		// Handle both single string (backward compatible) and list of strings
		for _, argsPath := range module.Args {
			if argsPath != "" {
				argsPaths[argsPath] = true
			}
		}
	}

	// 6. Collect args paths from projects
	for _, project := range terraformConfig.Projects {
		// Handle both single string (backward compatible) and list of strings
		for _, argsPath := range project.Args {
			if argsPath != "" {
				argsPaths[argsPath] = true
			}
		}
	}

	return argsPaths
}

// Note: inferInitArgsPath, scanInitArgsPaths, and discoverInitArgsPaths functions
// have been removed as terraform/init/ no longer uses subdirectories.
// All init files are now directly under terraform/init/ and use terraform/init/args.yaml

// loadArgsDefinitions loads and parses all args.yaml files
func loadArgsDefinitions(loader *template.Loader, argsPaths map[string]bool) ([]*renderer.ArgsDefinition, error) {
	fmt.Printf("Found %d args.yaml file(s)\n", len(argsPaths))

	var allArgsDefs []*renderer.ArgsDefinition
	for argsPath := range argsPaths {
		fmt.Printf("  Loading: %s\n", argsPath)
		content, err := loader.LoadTemplate(argsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load %s: %v\n", argsPath, err)
			continue
		}

		def, err := renderer.LoadArgsDefinition(content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", argsPath, err)
			continue
		}

		allArgsDefs = append(allArgsDefs, def)
	}

	if len(allArgsDefs) == 0 {
		return nil, fmt.Errorf("failed to load any args definitions")
	}

	return allArgsDefs, nil
}

// mergeArgsDefinitions merges all args definitions into a single config
func mergeArgsDefinitions(allArgsDefs []*renderer.ArgsDefinition) (renderer.ArgsData, map[string]string) {
	var allConfigValues []renderer.ArgsData
	var allComments []map[string]string

	for _, def := range allArgsDefs {
		configValues, comments := def.ToConfigValues()
		allConfigValues = append(allConfigValues, configValues)
		allComments = append(allComments, comments)
	}

	// Merge config values (later ones override earlier ones)
	reversed := make([]renderer.ArgsData, len(allConfigValues))
	for i, args := range allConfigValues {
		reversed[len(allConfigValues)-1-i] = args
	}
	mergedValues := renderer.MergeArgs(reversed...)

	// Merge comments (later ones override earlier ones)
	mergedComments := make(map[string]string)
	for i := len(allComments) - 1; i >= 0; i-- {
		for k, v := range allComments[i] {
			if _, exists := mergedComments[k]; !exists {
				mergedComments[k] = v
			}
		}
	}

	return mergedValues, mergedComments
}

// separateArgsByLevel separates args definitions by their level
func separateArgsByLevel(loader *template.Loader, argsPaths map[string]bool, projectGlobalPath string) ([]*renderer.ArgsDefinition, []*renderer.ArgsDefinition, []*renderer.ArgsDefinition, []*renderer.ArgsDefinition, []*renderer.ArgsDefinition) {
	var repoArgsDefs, terraformArgsDefs, projectArgsDefs, initArgsDefs, componentArgsDefs []*renderer.ArgsDefinition

	for argsPath := range argsPaths {
		content, err := loader.LoadTemplate(argsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load %s: %v\n", argsPath, err)
			continue
		}

		def, err := renderer.LoadArgsDefinition(content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", argsPath, err)
			continue
		}

		// Categorize by path
		switch argsPath {
		case "args.yaml":
			repoArgsDefs = append(repoArgsDefs, def)
		case "terraform/args.yaml":
			terraformArgsDefs = append(terraformArgsDefs, def)
		case "terraform/init/args.yaml":
			initArgsDefs = append(initArgsDefs, def)
		case projectGlobalPath:
			projectArgsDefs = append(projectArgsDefs, def)
		default:
			// Component-specific args files
			componentArgsDefs = append(componentArgsDefs, def)
		}
	}

	return repoArgsDefs, terraformArgsDefs, projectArgsDefs, initArgsDefs, componentArgsDefs
}

// reorganizeToHierarchicalStructure reorganizes config to hierarchical structure
// Structure: global (repo-level) -> terraform.global (repo + terraform) -> terraform.init.components -> terraform.projects[].global (repo + terraform + project) -> terraform.projects[].components[].parameters
func reorganizeToHierarchicalStructure(mergedValues renderer.ArgsData, terraformConfig *template.TerraformConfig, repoArgsDefs, terraformArgsDefs, projectArgsDefs, initArgsDefs []*renderer.ArgsDefinition) ArgsConfig {
	var result ArgsConfig

	// Extract global parameters from different levels
	repoGlobal := extractGlobalFromArgsDefs(repoArgsDefs)
	terraformGlobal := extractGlobalFromArgsDefs(terraformArgsDefs)
	projectGlobal := extractGlobalFromArgsDefs(projectArgsDefs)

	// Top-level global: only repo-level parameters
	result.Global = repoGlobal

	// Build terraform section
	result.Terraform.Version = terraformConfig.Version

	// terraform.global: repo-level + terraform-level parameters
	result.Terraform.Global = make(map[string]interface{})
	for k, v := range repoGlobal {
		result.Terraform.Global[k] = v
	}
	for k, v := range terraformGlobal {
		result.Terraform.Global[k] = v
	}

	// Extract init components from terraform/init/args.yaml
	// These components correspond to init items in config.yaml (backend, projects, atlantis)
	// Generate components based on config.yaml init items, with parameters from args.yaml
	initComponents := extractInitComponents(terraformConfig, initArgsDefs, mergedValues)
	if len(initComponents) > 0 {
		result.Terraform.Init = &renderer.InitSection{
			Components: initComponents,
		}
	}

	// Project names: use default list (prd, stg, corp, ...); ID injection and placeholders handle resolution
	projectNames := inferProjectNames(mergedValues)

	// Identify project-level components from config
	projectLevelComponents := identifyProjectLevelComponents(terraformConfig)

	// Identify module components from config
	moduleComponents := make(map[string]bool)
	for moduleName := range terraformConfig.Modules {
		moduleComponents[moduleName] = true
	}

	// Build projects list (always use default project names if no projects specified)
	result.Terraform.Projects = buildProjectsList(projectNames, mergedValues, projectLevelComponents, moduleComponents, repoGlobal, terraformGlobal, projectGlobal)

	return result
}

// extractInitComponents extracts components from init args definitions
// Returns a map of component name to component parameters
// Components are based on config.yaml init items, with parameters extracted from args.yaml
func extractInitComponents(terraformConfig *template.TerraformConfig, initArgsDefs []*renderer.ArgsDefinition, mergedValues renderer.ArgsData) map[string]interface{} {
	initComponents := make(map[string]interface{})

	// First, collect all init component names from config.yaml
	initComponentNames := make(map[string]bool)
	for _, initItem := range terraformConfig.Init {
		initComponentNames[initItem.Name] = true
	}

	// Extract components from init args definitions
	// This gives us parameters defined in terraform/init/args.yaml
	argsComponents := make(map[string]interface{})
	for _, def := range initArgsDefs {
		configValues, _ := def.ToConfigValues()
		if components, ok := configValues[renderer.FieldComponents]; ok {
			if componentsMap, ok := components.(map[string]interface{}); ok {
				for compName, compData := range componentsMap {
					argsComponents[compName] = compData
				}
			}
		}
	}

	// Generate init components based on config.yaml init items
	// For each init item in config.yaml, include it in the output with its parameters (if defined in args.yaml)
	for compName := range initComponentNames {
		if compParams, ok := argsComponents[compName]; ok {
			// Component has parameters defined in args.yaml
			initComponents[compName] = compParams
		} else {
			// Component exists in config.yaml but has no parameters defined in args.yaml
			// Generate empty object so it appears in the output
			// Use an empty map (not nil) so YAML encoder will output it as "component: {}"
			initComponents[compName] = make(map[string]interface{})
		}
	}

	return initComponents
}

// extractGlobalFromArgsDefs extracts global parameters from args definitions
func extractGlobalFromArgsDefs(argsDefs []*renderer.ArgsDefinition) map[string]interface{} {
	result := make(map[string]interface{})

	for _, def := range argsDefs {
		configValues, _ := def.ToConfigValues()
		if global, ok := configValues[renderer.FieldGlobal]; ok {
			if globalMap, ok := global.(map[string]interface{}); ok {
				for k, v := range globalMap {
					result[k] = v
				}
			}
		}
	}

	return result
}

// inferProjectNames returns default project names
// Always returns the 5 default project names: prd, stg, corp, static-res, atfs (matching default.yaml)
func inferProjectNames(mergedValues renderer.ArgsData) []string {
	// Always return default project names
	return getDefaultProjectNames()
}

// getDefaultProjectNames returns the default project names (short names used in default.yaml)
func getDefaultProjectNames() []string {
	return []string{
		"prd",
		"stg",
		"corp",
		"static-res",
		"atfs",
	}
}

// identifyProjectLevelComponents identifies project-level components from terraform config
func identifyProjectLevelComponents(terraformConfig *template.TerraformConfig) []string {
	projectComponents := make(map[string]bool)

	for _, project := range terraformConfig.Projects {
		// Handle multiple paths in Path list
		for _, path := range project.Path {
			if path != "" {
				baseName := filepath.Base(path)
				baseName = strings.TrimSuffix(baseName, ".tmpl")
				baseName = strings.TrimSuffix(baseName, ".tf")
				// Skip common non-component files
				if baseName != "" && baseName != "main" && baseName != "README" && baseName != "outputs" {
					projectComponents[baseName] = true
				}
			}
		}
	}

	result := make([]string, 0, len(projectComponents))
	for comp := range projectComponents {
		result = append(result, comp)
	}
	return result
}

// buildProjectsList builds the list of projects with their components
// Returns a slice of ProjectData structs with name and global fields first, then components
func buildProjectsList(projectNames []string, mergedValues renderer.ArgsData, projectLevelComponents []string, moduleComponents map[string]bool, repoGlobal, terraformGlobal, projectGlobal map[string]interface{}) []ProjectData {
	projectsList := make([]ProjectData, 0, len(projectNames))

	for _, projectName := range projectNames {
		// Skip empty project names
		if projectName == "" {
			continue
		}

		// Build components list
		componentsList := buildProjectComponents(mergedValues, projectLevelComponents, moduleComponents)

		// Create project data with ordered fields: name, global, components
		projectData := ProjectData{
			Name: projectName,
		}

		// Project-level global config: project_name, project_id and other project-specific params
		projectData.Global = make(map[string]interface{})
		for k, v := range projectGlobal {
			projectData.Global[k] = v
		}
		projectData.Global["project_name"] = projectName
		// project_id set by injectGeneratedProjectIDs when using convertToArgsConfig flow
		projectData.Global["project_id"] = projectName // fallback when ID not yet generated

		// Add components if any
		if len(componentsList) > 0 {
			projectData.Components = componentsList
		}

		projectsList = append(projectsList, projectData)
	}

	return projectsList
}

// buildProjectComponents builds the components list for a project
// Only includes project-level components, NOT module components
func buildProjectComponents(mergedValues renderer.ArgsData, projectLevelComponents []string, moduleComponents map[string]bool) []ComponentData {
	componentsList := make([]ComponentData, 0)

	components, ok := mergedValues[renderer.FieldComponents]
	if !ok {
		return componentsList
	}

	componentsMap, ok := components.(map[string]interface{})
	if !ok {
		return componentsList
	}

	// Only add project-level components, NOT module components
	// First, filter out module components from componentsMap
	filteredComponents := make(map[string]interface{})
	for compName, compData := range componentsMap {
		// Skip if it's a module component
		if moduleComponents[compName] {
			continue
		}
		// Only add if it's a project-level component
		isProjectLevel := false
		for _, plc := range projectLevelComponents {
			if compName == plc {
				isProjectLevel = true
				break
			}
		}
		if isProjectLevel {
			filteredComponents[compName] = compData
		}
	}

	// Build components list from filtered components
	for _, compName := range projectLevelComponents {
		if compData, exists := filteredComponents[compName]; exists {
			if compParams, ok := compData.(map[string]interface{}); ok {
				componentsList = append(componentsList, ComponentData{
					Name:       compName,
					Parameters: compParams,
				})
			}
		}
	}

	// Note: Module components are intentionally excluded from args.yaml
	// Modules are handled separately during rendering and don't need component parameters

	return componentsList
}

// reorganizeComments reorganizes comments according to hierarchical structure
func reorganizeComments(mergedComments map[string]string, terraformConfig *template.TerraformConfig, reorganizedData *ArgsConfig) map[string]string {
	result := make(map[string]string)

	// Copy global comments
	for k, v := range mergedComments {
		if strings.HasPrefix(k, "global.") {
			result[k] = v
			result[strings.Replace(k, "global.", "terraform.global.", 1)] = v
		}
	}

	projectLevelComponents := identifyProjectLevelComponents(terraformConfig)
	moduleComponents := make(map[string]bool)
	for moduleName := range terraformConfig.Modules {
		moduleComponents[moduleName] = true
	}

	// Reorganize comments for each project
	for _, projectData := range reorganizedData.Terraform.Projects {
		if projectData.Name == "" {
			continue
		}
		addProjectComments(&result, mergedComments, projectData.Name, projectLevelComponents, moduleComponents)
	}

	return result
}

// addProjectComments adds comments for a project
func addProjectComments(result *map[string]string, mergedComments map[string]string, projectName string, projectLevelComponents []string, moduleComponents map[string]bool) {
	// Project-level global comments
	for k, v := range mergedComments {
		if strings.HasPrefix(k, "global.") {
			newKey := fmt.Sprintf("terraform.projects.%s.global.%s", projectName, strings.TrimPrefix(k, "global."))
			(*result)[newKey] = v
		}
	}

	// Component comments
	for _, compName := range projectLevelComponents {
		compPrefix := fmt.Sprintf("components.%s", compName)
		for k, v := range mergedComments {
			if strings.HasPrefix(k, compPrefix+".") {
				paramName := strings.TrimPrefix(k, compPrefix+".")
				newKey := fmt.Sprintf("terraform.projects.%s.components.%s.parameters.%s", projectName, compName, paramName)
				(*result)[newKey] = v
			}
		}
	}

	// Module component comments (use module name directly)
	for moduleName := range moduleComponents {
		compPrefix := fmt.Sprintf("components.%s", moduleName)
		for k, v := range mergedComments {
			if strings.HasPrefix(k, compPrefix+".") {
				paramName := strings.TrimPrefix(k, compPrefix+".")
				newKey := fmt.Sprintf("terraform.projects.%s.components.%s.parameters.%s", projectName, moduleName, paramName)
				(*result)[newKey] = v
			}
		}
	}
}

// writeConfigFile writes the configuration to file based on format
func writeConfigFile(outputPath, format string, data *ArgsConfig, comments map[string]string) error {
	switch format {
	case "toml":
		return writeTOMLFile(outputPath, data, comments)
	case "yaml", "yml":
		return writeYAMLFile(outputPath, data, comments)
	default:
		return writeYAMLFile(outputPath, data, comments)
	}
}

// writeYAMLFile writes ArgsConfig to a YAML file
// Uses yaml.Encoder to ensure proper handling of MarshalYAML methods
func writeYAMLFile(path string, data *ArgsConfig, comments map[string]string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(4)
	defer encoder.Close()

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode YAML: %w", err)
	}

	// Add header comment
	header := "# blcli args configuration file\n# Generated by \"blcli init-args\"\n# You can customize template arguments here.\n\n"
	finalContent := header + buf.String()

	if err := os.WriteFile(path, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("✅ Generated args configuration: %s\n", path)
	return nil
}

// writeTOMLFile writes ArgsConfig to a TOML file using the library
func writeTOMLFile(path string, data *ArgsConfig, comments map[string]string) error {
	// Convert ArgsConfig to map[string]interface{} for TOML library
	tomlData := make(map[string]interface{})
	if len(data.Global) > 0 {
		tomlData[renderer.FieldGlobal] = data.Global
	}
	if data.Terraform.Version != "" || len(data.Terraform.Global) > 0 || data.Terraform.Init != nil || len(data.Terraform.Projects) > 0 {
		terraformMap := make(map[string]interface{})
		if data.Terraform.Version != "" {
			terraformMap["version"] = data.Terraform.Version
		}
		if len(data.Terraform.Global) > 0 {
			terraformMap[renderer.FieldGlobal] = data.Terraform.Global
		}
		if data.Terraform.Init != nil && len(data.Terraform.Init.Components) > 0 {
			terraformMap[renderer.FieldInit] = map[string]interface{}{
				renderer.FieldComponents: data.Terraform.Init.Components,
			}
		}
		if len(data.Terraform.Projects) > 0 {
			// Convert []ProjectData to []map[string]interface{}
			projectsList := make([]map[string]interface{}, len(data.Terraform.Projects))
			for i, project := range data.Terraform.Projects {
				projectMap := make(map[string]interface{})
				projectMap[renderer.FieldName] = project.Name
				if len(project.Global) > 0 {
					projectMap[renderer.FieldGlobal] = project.Global
				}
				if len(project.Components) > 0 {
					componentsList := make([]map[string]interface{}, len(project.Components))
					for j, comp := range project.Components {
						compMap := make(map[string]interface{})
						compMap[renderer.FieldName] = comp.Name
						compMap["parameters"] = comp.Parameters
						componentsList[j] = compMap
					}
					projectMap[renderer.FieldComponents] = componentsList
				}
				projectsList[i] = projectMap
			}
			terraformMap[renderer.FieldProjects] = projectsList
		}
		tomlData[renderer.FieldTerraform] = terraformMap
	}

	// Add Kubernetes section if present
	if data.Kubernetes.Version != "" || len(data.Kubernetes.Global) > 0 || len(data.Kubernetes.Projects) > 0 {
		kubernetesMap := make(map[string]interface{})
		if data.Kubernetes.Version != "" {
			kubernetesMap["version"] = data.Kubernetes.Version
		}
		if len(data.Kubernetes.Global) > 0 {
			kubernetesMap[renderer.FieldGlobal] = data.Kubernetes.Global
		}
		if len(data.Kubernetes.Projects) > 0 {
			// Convert []ProjectData to []map[string]interface{}
			projectsList := make([]map[string]interface{}, len(data.Kubernetes.Projects))
			for i, project := range data.Kubernetes.Projects {
				projectMap := make(map[string]interface{})
				projectMap[renderer.FieldName] = project.Name
				if len(project.Global) > 0 {
					projectMap[renderer.FieldGlobal] = project.Global
				}
				if len(project.Components) > 0 {
					componentsList := make([]map[string]interface{}, len(project.Components))
					for j, comp := range project.Components {
						compMap := make(map[string]interface{})
						compMap[renderer.FieldName] = comp.Name
						compMap["parameters"] = comp.Parameters
						componentsList[j] = compMap
					}
					projectMap[renderer.FieldComponents] = componentsList
				}
				projectsList[i] = projectMap
			}
			kubernetesMap[renderer.FieldProjects] = projectsList
		}
		tomlData["kubernetes"] = kubernetesMap
	}

	// Add Gitops section if present
	if len(data.Gitops) > 0 {
		tomlData["gitops"] = data.Gitops
	}

	// Marshal using TOML library
	content, err := toml.Marshal(tomlData)
	if err != nil {
		return fmt.Errorf("failed to marshal TOML: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Add header comment
	header := "# blcli args configuration file\n# Generated by \"blcli init-args\"\n# You can customize template arguments here.\n\n"
	finalContent := header + string(content)

	if err := os.WriteFile(path, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("✅ Generated args configuration: %s\n", path)
	return nil
}
