package bootstrap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	k8sbootstrap "blcli/pkg/bootstrap/kubernetes"
	"blcli/pkg/renderer"
)

const (
	argocdGitRepositoryURLKey   = "GitRepositoryURL"
	argocdGitRepositoriesKey    = "GitRepositories"
	argocdGitSSHSecretNameKey   = "GitSSHSecretName"
	argocdGitSSHSecretKeyKey    = "GitSSHSecretKey"
	argocdURLKey                = "ArgoCDURL"
	argocdDexGitHubEnabledKey   = "DexGitHubEnabled"
	argocdDexGitHubClientIDKey  = "DexGitHubClientID"
	argocdDexGitHubClientSecret = "DexGitHubClientSecret"
	argocdDexGitHubOrgsKey      = "DexGitHubOrgs"
)

// ApplyInitEnvOverrides loads one or more .env files and overlays supported values on top of args data.
// Earlier env files override later ones, mirroring --args semantics.
func ApplyInitEnvOverrides(base renderer.ArgsData, envPaths []string) (renderer.ArgsData, error) {
	if len(envPaths) == 0 {
		return base, nil
	}

	var allOverrides []renderer.ArgsData
	for _, envPath := range envPaths {
		envMap, err := loadEnvFile(envPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load env file %s: %w", envPath, err)
		}

		override, err := buildEnvOverrideArgs(base, envMap)
		if err != nil {
			return nil, fmt.Errorf("failed to build env override from %s: %w", envPath, err)
		}
		if len(override) > 0 {
			allOverrides = append(allOverrides, override)
		}
	}

	if len(allOverrides) == 0 {
		return base, nil
	}

	reversed := make([]renderer.ArgsData, len(allOverrides))
	for i, override := range allOverrides {
		reversed[len(allOverrides)-1-i] = override
	}
	mergedOverride := renderer.MergeArgs(reversed...)
	return renderer.MergeArgs(base, mergedOverride), nil
}

func ResolveInitEnvPaths(argsPaths, envPaths []string) []string {
	if len(envPaths) > 0 {
		return envPaths
	}
	if len(argsPaths) == 0 {
		return nil
	}

	defaultEnvPath := filepath.Join(filepath.Dir(argsPaths[0]), ".env")
	if _, err := os.Stat(defaultEnvPath); err == nil {
		return []string{defaultEnvPath}
	}

	return nil
}

func loadEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		idx := strings.IndexRune(line, '=')
		if idx <= 0 {
			return nil, fmt.Errorf("invalid line %d: expected KEY=VALUE", lineNo)
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		value = stripEnvQuotes(value)
		result[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func stripEnvQuotes(value string) string {
	if len(value) < 2 {
		return value
	}
	if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
		(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
		return value[1 : len(value)-1]
	}
	return value
}

func buildEnvOverrideArgs(base renderer.ArgsData, env map[string]string) (renderer.ArgsData, error) {
	override := make(renderer.ArgsData)

	tfGlobal := make(map[string]interface{})
	if organizationID := firstNonEmpty(env,
		"BLCLI_TERRAFORM_ORGANIZATION_ID",
		"BLCLI_ORGANIZATION_ID",
		"TERRAFORM_ORGANIZATION_ID",
		"ORGANIZATION_ID",
	); organizationID != "" {
		tfGlobal["OrganizationID"] = organizationID
	}
	if billingAccountID := firstNonEmpty(env,
		"BLCLI_TERRAFORM_BILLING_ACCOUNT_ID",
		"BLCLI_BILLING_ACCOUNT_ID",
		"TERRAFORM_BILLING_ACCOUNT_ID",
		"BILLING_ACCOUNT_ID",
	); billingAccountID != "" {
		tfGlobal["BillingAccountID"] = billingAccountID
	}
	if len(tfGlobal) > 0 {
		tfOverride := buildTerraformGlobalOverrides(base, tfGlobal)
		if len(tfOverride) > 0 {
			override[renderer.FieldTerraform] = tfOverride
		}
	}

	k8sOverride, err := buildKubernetesArgoCDOverrides(base, env)
	if err != nil {
		return nil, err
	}
	if len(k8sOverride) > 0 {
		override["kubernetes"] = k8sOverride
	}

	gitopsOverride := buildGitopsOverrides(base, env)
	if len(gitopsOverride) > 0 {
		override["gitops"] = gitopsOverride
	}

	return override, nil
}

func buildArgoCDParameterOverrides(env map[string]string, projectName string) (map[string]interface{}, error) {
	params := make(map[string]interface{})

	projectKey := strings.ToUpper(projectName)

	if repoURL := firstNonEmpty(env,
		fmt.Sprintf("BLCLI_ARGOCD_%s_GIT_REPOSITORY_URL", projectKey),
		"BLCLI_ARGOCD_GIT_REPOSITORY_URL",
		"ARGOCD_GIT_REPOSITORY_URL",
	); repoURL != "" {
		params[argocdGitRepositoryURLKey] = repoURL
	}

	if reposJSON := firstNonEmpty(env,
		fmt.Sprintf("BLCLI_ARGOCD_%s_GIT_REPOSITORIES_JSON", projectKey),
		"BLCLI_ARGOCD_GIT_REPOSITORIES_JSON",
		"ARGOCD_GIT_REPOSITORIES_JSON",
	); reposJSON != "" {
		var repositories []interface{}
		if err := json.Unmarshal([]byte(reposJSON), &repositories); err != nil {
			return nil, fmt.Errorf("invalid BLCLI_ARGOCD_GIT_REPOSITORIES_JSON: %w", err)
		}
		params[argocdGitRepositoriesKey] = repositories
	}

	if sshSecretName := firstNonEmpty(env,
		fmt.Sprintf("BLCLI_ARGOCD_%s_GIT_SSH_SECRET_NAME", projectKey),
		"BLCLI_ARGOCD_GIT_SSH_SECRET_NAME",
		"ARGOCD_GIT_SSH_SECRET_NAME",
	); sshSecretName != "" {
		params[argocdGitSSHSecretNameKey] = sshSecretName
	}

	if sshSecretKey := firstNonEmpty(env,
		fmt.Sprintf("BLCLI_ARGOCD_%s_GIT_SSH_SECRET_KEY", projectKey),
		"BLCLI_ARGOCD_GIT_SSH_SECRET_KEY",
		"ARGOCD_GIT_SSH_SECRET_KEY",
	); sshSecretKey != "" {
		params[argocdGitSSHSecretKeyKey] = sshSecretKey
	}

	if argocdURL := firstNonEmpty(env,
		fmt.Sprintf("BLCLI_ARGOCD_%s_URL", projectKey),
		"BLCLI_ARGOCD_URL",
		"ARGOCD_URL",
	); argocdURL != "" {
		params[argocdURLKey] = argocdURL
	}

	clientID := firstNonEmpty(env,
		fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_CLIENT_ID", projectKey),
		fmt.Sprintf("BLCLI_ARGOCD_%s_GITHUB_AUTH_CLIENT_ID", projectKey),
		"BLCLI_ARGOCD_DEX_GITHUB_CLIENT_ID",
		"BLCLI_ARGOCD_GITHUB_AUTH_CLIENT_ID",
		"ARGOCD_DEX_GITHUB_CLIENT_ID",
		"ARGOCD_GITHUB_AUTH_CLIENT_ID",
	)
	if clientID != "" {
		params[argocdDexGitHubClientIDKey] = clientID
		params[argocdDexGitHubEnabledKey] = true
	}

	clientSecret := firstNonEmpty(env,
		fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_CLIENT_SECRET", projectKey),
		fmt.Sprintf("BLCLI_ARGOCD_%s_GITHUB_AUTH_CLIENT_SECRET", projectKey),
		"BLCLI_ARGOCD_DEX_GITHUB_CLIENT_SECRET",
		"BLCLI_ARGOCD_GITHUB_AUTH_CLIENT_SECRET",
		"ARGOCD_DEX_GITHUB_CLIENT_SECRET",
		"ARGOCD_GITHUB_AUTH_CLIENT_SECRET",
	)
	if clientSecret != "" {
		params[argocdDexGitHubClientSecret] = clientSecret
		params[argocdDexGitHubEnabledKey] = true
	}

	if enabledValue := firstNonEmpty(env,
		fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_ENABLED", projectKey),
		"BLCLI_ARGOCD_DEX_GITHUB_ENABLED",
		"ARGOCD_DEX_GITHUB_ENABLED",
	); enabledValue != "" {
		params[argocdDexGitHubEnabledKey] = parseEnvBool(enabledValue)
	}

	if orgsValue := firstNonEmpty(env,
		fmt.Sprintf("BLCLI_ARGOCD_%s_DEX_GITHUB_ORGS", projectKey),
		fmt.Sprintf("BLCLI_ARGOCD_%s_GITHUB_AUTH_ORGS", projectKey),
		"BLCLI_ARGOCD_DEX_GITHUB_ORGS",
		"BLCLI_ARGOCD_GITHUB_AUTH_ORGS",
		"ARGOCD_DEX_GITHUB_ORGS",
		"ARGOCD_GITHUB_AUTH_ORGS",
	); orgsValue != "" {
		params[argocdDexGitHubOrgsKey] = splitCSVList(orgsValue)
		params[argocdDexGitHubEnabledKey] = true
	}

	if dexConfig := buildDexGitHubConfig(params); dexConfig != nil {
		// Keep both key styles so templates and older parameter shapes resolve consistently.
		params["DexConfig"] = dexConfig
		params["dex-config"] = dexConfig
	}

	if rbacPrefix := firstNonEmpty(env,
		fmt.Sprintf("BLCLI_ARGOCD_%s_RBAC_GROUP_PREFIX", projectKey),
		"BLCLI_ARGOCD_RBAC_GROUP_PREFIX",
	); rbacPrefix != "" {
		params["RBACRoleBindingsPrefix"] = rbacPrefix
	}

	return params, nil
}

func buildDexGitHubConfig(params map[string]interface{}) map[string]interface{} {
	clientID, _ := params[argocdDexGitHubClientIDKey].(string)
	clientSecret, _ := params[argocdDexGitHubClientSecret].(string)
	orgs := toStringSlice(params[argocdDexGitHubOrgsKey])

	if clientID == "" && clientSecret == "" && len(orgs) == 0 {
		return nil
	}

	config := map[string]interface{}{
		"connectors": []interface{}{
			map[string]interface{}{
				"type": "github",
				"id":   "github",
				"name": "GitHub",
				"config": map[string]interface{}{
					"clientID":     clientID,
					"clientSecret": clientSecret,
					"orgs":         buildDexOrgEntries(orgs),
				},
			},
		},
	}
	return config
}

func buildDexOrgEntries(orgs []string) []interface{} {
	if len(orgs) == 0 {
		return nil
	}
	result := make([]interface{}, 0, len(orgs))
	for _, org := range orgs {
		result = append(result, map[string]interface{}{"name": org})
	}
	return result
}

func buildTerraformGlobalOverrides(base renderer.ArgsData, global map[string]interface{}) map[string]interface{} {
	terraform, ok := base[renderer.FieldTerraform]
	if !ok {
		return map[string]interface{}{
			renderer.FieldGlobal: global,
		}
	}

	tfMap := cloneStringMap(toStringMap(terraform))
	if tfMap == nil {
		return map[string]interface{}{
			renderer.FieldGlobal: global,
		}
	}

	existingGlobal := cloneStringMap(toStringMap(tfMap[renderer.FieldGlobal]))
	if existingGlobal == nil {
		existingGlobal = make(map[string]interface{})
	}
	for key, value := range global {
		existingGlobal[key] = value
	}
	tfMap[renderer.FieldGlobal] = existingGlobal
	return tfMap
}

func buildKubernetesArgoCDOverrides(base renderer.ArgsData, env map[string]string) (map[string]interface{}, error) {
	kubernetes, ok := base["kubernetes"]
	if !ok {
		return nil, nil
	}

	k8sMap := toStringMap(kubernetes)
	if k8sMap == nil {
		return nil, nil
	}

	projects, ok := k8sMap[renderer.FieldProjects].([]interface{})
	if !ok || len(projects) == 0 {
		return nil, nil
	}

	overrideProjects := make([]interface{}, 0, len(projects))
	foundArgocd := false
	for _, item := range projects {
		projectMap := cloneStringMap(toStringMap(item))
		if projectMap == nil {
			continue
		}
		projectName, _ := projectMap[renderer.FieldName].(string)
		projectParams, err := buildArgoCDParameterOverrides(env, projectName)
		if err != nil {
			return nil, err
		}

		components, _ := projectMap[renderer.FieldComponents].([]interface{})
		overrideComponents := make([]interface{}, 0, len(components))

		for _, component := range components {
			componentMap := cloneStringMap(toStringMap(component))
			if componentMap == nil {
				continue
			}

			componentName, _ := componentMap[renderer.FieldName].(string)
			if k8sbootstrap.NormalizeComponentName(componentName) == "argocd" {
				if len(projectParams) == 0 {
					overrideComponents = append(overrideComponents, componentMap)
					continue
				}
				existingParams := cloneStringMap(toStringMap(componentMap["parameters"]))
				if existingParams == nil {
					existingParams = make(map[string]interface{})
				}
				for key, value := range projectParams {
					existingParams[key] = value
				}
				if prefix, _ := projectParams["RBACRoleBindingsPrefix"].(string); prefix != "" {
					existingParams["RBACRoleBindings"] = rewriteRBACRoleBindings(existingParams["RBACRoleBindings"], prefix)
				}
				delete(existingParams, "RBACRoleBindingsPrefix")
				componentMap["parameters"] = existingParams
				foundArgocd = true
			}
			overrideComponents = append(overrideComponents, componentMap)
		}

		projectMap[renderer.FieldComponents] = overrideComponents
		overrideProjects = append(overrideProjects, projectMap)
	}

	if !foundArgocd {
		return nil, nil
	}

	return map[string]interface{}{
		renderer.FieldProjects: overrideProjects,
	}, nil
}

func buildGitopsOverrides(base renderer.ArgsData, env map[string]string) map[string]interface{} {
	gitops, ok := base["gitops"]
	if !ok {
		return nil
	}
	gitopsMap := cloneStringMap(toStringMap(gitops))
	if gitopsMap == nil {
		return nil
	}

	apps, _ := gitopsMap["apps"].([]interface{})
	if len(apps) == 0 {
		return nil
	}

	found := false
	updatedApps := make([]interface{}, 0, len(apps))
	for _, appItem := range apps {
		appMap := cloneStringMap(toStringMap(appItem))
		if appMap == nil {
			continue
		}
		appName, _ := appMap["name"].(string)
		projects, _ := appMap["project"].([]interface{})
		updatedProjects := make([]interface{}, 0, len(projects))
		for _, projectItem := range projects {
			projectMap := cloneStringMap(toStringMap(projectItem))
			if projectMap == nil {
				continue
			}
			projectName, _ := projectMap["name"].(string)
			params := cloneStringMap(toStringMap(projectMap["parameters"]))
			if params == nil {
				params = make(map[string]interface{})
			}
			if sourceRepo := firstNonEmpty(env,
				fmt.Sprintf("BLCLI_GITOPS_%s_%s_SOURCE_REPO_URL", strings.ToUpper(projectName), envKeyPart(appName)),
				fmt.Sprintf("BLCLI_GITOPS_%s_SOURCE_REPO_URL", strings.ToUpper(projectName)),
				"BLCLI_GITOPS_SOURCE_REPO_URL",
			); sourceRepo != "" {
				params["SourceRepoURL"] = sourceRepo
				found = true
			}
			if appRepo := firstNonEmpty(env,
				fmt.Sprintf("BLCLI_GITOPS_%s_%s_APPLICATION_REPO", strings.ToUpper(projectName), envKeyPart(appName)),
				fmt.Sprintf("BLCLI_GITOPS_%s_APPLICATION_REPO", envKeyPart(appName)),
				"BLCLI_GITOPS_APPLICATION_REPO",
			); appRepo != "" {
				params["ApplicationRepo"] = appRepo
				found = true
			}
			projectMap["parameters"] = params
			updatedProjects = append(updatedProjects, projectMap)
		}
		appMap["project"] = updatedProjects
		updatedApps = append(updatedApps, appMap)
	}

	if !found {
		return nil
	}
	gitopsMap["apps"] = updatedApps
	return gitopsMap
}

func envKeyPart(value string) string {
	value = strings.ToUpper(value)
	re := regexp.MustCompile(`[^A-Z0-9]+`)
	value = re.ReplaceAllString(value, "_")
	return strings.Trim(value, "_")
}

func rewriteRBACRoleBindings(v interface{}, prefix string) []interface{} {
	bindings, ok := v.([]interface{})
	if !ok || len(bindings) == 0 {
		return defaultRBACRoleBindings(prefix)
	}
	result := make([]interface{}, 0, len(bindings))
	for _, item := range bindings {
		binding := cloneStringMap(toStringMap(item))
		if binding == nil {
			continue
		}
		group, _ := binding["group"].(string)
		if group != "" {
			if idx := strings.Index(group, ":"); idx >= 0 {
				binding["group"] = prefix + group[idx:]
			} else {
				binding["group"] = prefix
			}
		}
		result = append(result, binding)
	}
	return result
}

func defaultRBACRoleBindings(prefix string) []interface{} {
	return []interface{}{
		map[string]interface{}{
			"group": prefix + ":Infra",
			"role":  "role:org-admin",
		},
		map[string]interface{}{
			"group": prefix + ":backend",
			"role":  "role:org-backend",
		},
	}
}

func firstNonEmpty(values map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return ""
}

func parseEnvBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func splitCSVList(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func toStringSlice(v interface{}) []string {
	switch value := v.(type) {
	case []string:
		return value
	case []interface{}:
		out := make([]string, 0, len(value))
		for _, item := range value {
			s := stringifyInterface(item)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func stringifyInterface(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}

func toStringMap(v interface{}) map[string]interface{} {
	switch value := v.(type) {
	case map[string]interface{}:
		return value
	case renderer.ArgsData:
		return map[string]interface{}(value)
	case map[interface{}]interface{}:
		result := make(map[string]interface{}, len(value))
		for key, item := range value {
			keyStr, ok := key.(string)
			if !ok {
				continue
			}
			result[keyStr] = item
		}
		return result
	default:
		return nil
	}
}

func cloneStringMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
