package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"blcli/pkg/config"
	"blcli/pkg/internal"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

// toMapStringInterface converts YAML/TOML decoded map types to map[string]interface{}
func toMapStringInterface(v interface{}) map[string]interface{} {
	switch m := v.(type) {
	case map[string]interface{}:
		return m
	case renderer.ArgsData:
		return map[string]interface{}(m)
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for k, val := range m {
			if keyStr, ok := k.(string); ok {
				result[keyStr] = val
			}
		}
		return result
	default:
		return nil
	}
}

// BootstrapGitops bootstraps GitOps projects by rendering app manifests per (project, app)
// using gitops/config.yaml app-templates and default.yaml (argocd.project, apps).
// Output layout: workspace/gitops/{project}/{app_name}/{template_name}.yaml
func BootstrapGitops(global config.GlobalConfig, project *config.ProjectConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData) error {
	if templateLoader == nil {
		return fmt.Errorf("template repository is required for gitops bootstrap")
	}

	gitopsData := toMapStringInterface(templateArgs["gitops"])
	if gitopsData == nil || (gitopsData["argocd"] == nil && gitopsData["apps"] == nil) {
		return fmt.Errorf("gitops section required in args (run blcli init-args and ensure args has gitops.argocd and gitops.apps)")
	}

	gitopsCfg, err := templateLoader.LoadGitopsConfig()
	if err != nil {
		return fmt.Errorf("failed to load gitops config: %w", err)
	}

	// argocd.project: [stg, prd, ...]
	var projectNames []string
	if argocd := toMapStringInterface(gitopsData["argocd"]); argocd != nil {
		if proj, _ := argocd["project"]; proj != nil {
			switch v := proj.(type) {
			case []interface{}:
				for _, p := range v {
					if s, ok := p.(string); ok && s != "" {
						projectNames = append(projectNames, s)
					}
				}
			case []string:
				projectNames = append(projectNames, v...)
			}
		}
	}
	if len(projectNames) == 0 {
		return fmt.Errorf("gitops.argocd.project is required and must be a non-empty list (e.g. [stg, prd])")
	}

	// apps: [{ name, kind, image, repo, project: [{ name, parameters }] }, ...]
	appsList, _ := gitopsData["apps"].([]interface{})
	if len(appsList) == 0 {
		return fmt.Errorf("gitops.apps is required and must be a non-empty list")
	}

	workspace := config.WorkspacePath(global)
	gitopsRoot := filepath.Join(workspace, "gitops")
	if err := internal.EnsureDir(gitopsRoot); err != nil {
		return fmt.Errorf("failed to create gitops root dir: %w", err)
	}

	for _, projectName := range projectNames {
		for _, a := range appsList {
			app := toMapStringInterface(a)
			if app == nil {
				continue
			}
			appName, _ := app["name"].(string)
			if appName == "" {
				continue
			}
			kind, _ := app["kind"].(string)
			if kind == "" {
				kind = template.AppKindDeployment
			}
			templates := gitopsCfg.GetTemplatesByKind(kind)
			if len(templates) == 0 {
				continue
			}

			// Find project entry for this (app, projectName)
			projList, _ := app["project"].([]interface{})
			var params map[string]interface{}
			for _, pe := range projList {
				pm := toMapStringInterface(pe)
				if pm == nil {
					continue
				}
				if n, _ := pm["name"].(string); n == projectName {
					params = toMapStringInterface(pm["parameters"])
					break
				}
			}
			if params == nil {
				continue
			}

			// Build template context: params as-is, plus APP_NAME/APP_IMAGE/APP_VERSION for templates that use them
			ctx := make(renderer.ArgsData)
			for k, v := range params {
				ctx[k] = v
			}
			appImage, _ := app["image"].(string)
			appVersion := "1.0.0"
			if rev, ok := params["ApplicationRevision"]; ok {
				if s, ok := rev.(string); ok {
					appVersion = s
				}
			}
			ctx["APP_NAME"] = appName
			ctx["Name"] = appName
			ctx["APP_IMAGE"] = appImage
			ctx["APP_VERSION"] = appVersion

			appDir := filepath.Join(gitopsRoot, projectName, appName)
			if err := internal.EnsureDir(appDir); err != nil {
				return fmt.Errorf("failed to create app dir %s: %w", appDir, err)
			}

			for _, t := range templates {
				tmplContent, err := templateLoader.LoadTemplate(t.Path)
				if err != nil {
					return fmt.Errorf("failed to load template %s: %w", t.Path, err)
				}
				rendered, err := template.Render(tmplContent, ctx)
				if err != nil {
					return fmt.Errorf("failed to render %s for %s/%s: %w", t.Path, projectName, appName, err)
				}
				// Replace literal placeholders in case template uses APP_NAME/APP_IMAGE/APP_VERSION as plain text
				rendered = strings.ReplaceAll(rendered, "APP_NAME", appName)
				rendered = strings.ReplaceAll(rendered, "APP_IMAGE", appImage)
				rendered = strings.ReplaceAll(rendered, "APP_VERSION", appVersion)
				outPath := filepath.Join(appDir, t.Name+".yaml")
				if err := os.WriteFile(outPath, []byte(rendered), 0o644); err != nil {
					return fmt.Errorf("failed to write %s: %w", outPath, err)
				}
			}
			fmt.Printf("Initialized gitops app: %s (project: %s) -> %s\n", appName, projectName, appDir)
		}
	}

	return nil
}

// DestroyGitops removes generated gitops output under workspace/gitops.
func DestroyGitops(global config.GlobalConfig, project *config.ProjectConfig) error {
	workspace := config.WorkspacePath(global)
	gitopsRoot := filepath.Join(workspace, "gitops")
	if _, err := os.Stat(gitopsRoot); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(gitopsRoot)
}
