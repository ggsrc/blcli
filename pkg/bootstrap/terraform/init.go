package terraform

import (
	"fmt"
	"path/filepath"
	"strings"

	"blcli/pkg/internal"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

// InitializeInitItems initializes all terraform init items
// workspaceRoot is the root directory of the workspace (where files will be generated)
// Only processes init items that are present in terraform.init.components
func InitializeInitItems(terraformConfig *template.TerraformConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData, workspaceRoot string, data map[string]interface{}) error {
	// Get available init components from terraform.init.components
	availableInitComponents := getAvailableInitComponents(templateArgs)

	for _, initItem := range terraformConfig.Init {
		// Skip if component is not in terraform.init.components
		if !availableInitComponents[initItem.Name] {
			continue
		}

		if len(initItem.Path) == 0 {
			continue // Skip if no paths specified
		}

		for _, path := range initItem.Path {
			if err := processInitItemPath(initItem, path, templateLoader, templateArgs, workspaceRoot, data); err != nil {
				return err
			}
		}
	}
	return nil
}

// getAvailableInitComponents extracts available init components from terraform.init.components
func getAvailableInitComponents(templateArgs renderer.ArgsData) map[string]bool {
	available := make(map[string]bool)

	// Check terraform.init.components
	if terraform, ok := templateArgs[renderer.FieldTerraform]; ok {
		terraformMap := toMapStringInterface(terraform)
		if terraformMap != nil {
			if init, ok := terraformMap[renderer.FieldInit]; ok {
				initMap := toMapStringInterface(init)
				if initMap != nil {
				if components, ok := initMap[renderer.FieldComponents]; ok {
					componentsMap := toMapStringInterface(components)
					for compName := range componentsMap {
						available[compName] = true
					}
				}
				}
			}
		}
	}

	return available
}

// processInitItemPath processes a single path for an init item
func processInitItemPath(initItem template.InitItem, path string, templateLoader *template.Loader, templateArgs renderer.ArgsData, workspaceRoot string, data map[string]interface{}) error {
	// Load template
	tmplContent, err := templateLoader.LoadTemplate(path)
	if err != nil {
		return buildTemplateLoadError(initItem.Name, path, err)
	}

	// Extract init item specific args, including component-specific parameters
	initArgs := ExtractInitItemArgsForComponent(templateArgs, initItem.Args, initItem.Name)

	// Determine output filename from template path
	outputFileName := RemoveTmplExtension(filepath.Base(path))

	// Validate and render destination path
	outputDir, err := validateAndRenderDestination(initItem, data, templateArgs)
	if err != nil {
		return err
	}

	// Prepare data and args for template rendering
	perData, perArgs := prepareInitItemData(data, initArgs, outputDir)

	// Render template
	perContent, err := template.RenderWithArgs(tmplContent, perData, perArgs)
	if err != nil {
		return fmt.Errorf("failed to render template %s: %w", path, err)
	}

	// Write rendered content to file
	outputPath, err := writeInitItemFile(workspaceRoot, outputDir, outputFileName, perContent)
	if err != nil {
		return err
	}

	fmt.Printf("Initialized terraform init item: %s -> %s\n", initItem.Name, outputPath)
	return nil
}

// buildTemplateLoadError builds a detailed error message for template loading failures
func buildTemplateLoadError(itemName, path string, err error) error {
	errMsg := fmt.Sprintf("\n❌ Failed to load template for init item '%s'\n", itemName)
	errMsg += fmt.Sprintf("   Template path: %s\n", path)
	errMsg += fmt.Sprintf("   Error: %v\n", err)

	// Check if it's a 404 error (file not found)
	if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
		errMsg += "\n   💡 Possible issues:\n"
		errMsg += fmt.Sprintf("      - Template file does not exist in repository: %s\n", path)
		errMsg += "      - Check if the path in config.yaml is correct\n"
		errMsg += "      - Verify the file exists in the template repository\n"

		// Check if file might exist without .tmpl extension
		if strings.HasSuffix(path, ".tmpl") {
			pathWithoutTmpl := strings.TrimSuffix(path, ".tmpl")
			errMsg += fmt.Sprintf("      - ⚠️  File configured with .tmpl extension, check if file exists as: %s\n", pathWithoutTmpl)
		} else {
			pathWithTmpl := path + ".tmpl"
			errMsg += fmt.Sprintf("      - ⚠️  File configured without .tmpl extension, check if file exists as: %s\n", pathWithTmpl)
		}

		// Suggest alternative paths if the path looks suspicious
		if strings.Contains(path, "tf-backendvariable") {
			errMsg += "      - ⚠️  Path contains 'tf-backendvariable', did you mean 'tf-backend/variable'?\n"
		}
	}

	return fmt.Errorf("%s", errMsg)
}

// validateAndRenderDestination validates and renders the destination path for an init item
func validateAndRenderDestination(initItem template.InitItem, data map[string]interface{}, templateArgs renderer.ArgsData) (string, error) {
	if initItem.Destination == "" {
		return "", fmt.Errorf("destination is required for init item '%s'", initItem.Name)
	}

	// Render destination path (supports template variables like {{.GlobalName}})
	renderedDest, err := template.RenderWithArgs(initItem.Destination, data, templateArgs)
	if err != nil {
		return "", fmt.Errorf("failed to render destination '%s' for init item '%s': %w", initItem.Destination, initItem.Name, err)
	}

	return renderedDest, nil
}

// prepareInitItemData prepares data and args for template rendering
func prepareInitItemData(data map[string]interface{}, initArgs renderer.ArgsData, outputDir string) (map[string]interface{}, renderer.ArgsData) {
	// Copy data and add FolderName
	perData := make(map[string]interface{}, len(data)+1)
	for k, v := range data {
		perData[k] = v
	}
	// Expose current folder name to templates (used by main.tf, etc.)
	perData["FolderName"] = filepath.Base(outputDir)

	// Copy args
	perArgs := make(renderer.ArgsData, len(initArgs))
	for k, v := range initArgs {
		perArgs[k] = v
	}

	return perData, perArgs
}

// writeInitItemFile writes the rendered content to the output file
func writeInitItemFile(workspaceRoot, outputDir, outputFileName, content string) (string, error) {
	// Build full output path: workspaceRoot + outputDir + filename
	fullOutputDir := filepath.Join(workspaceRoot, outputDir)

	// Ensure output directory exists
	if err := internal.EnsureDir(fullOutputDir); err != nil {
		return "", fmt.Errorf("failed to create output directory %s: %w", fullOutputDir, err)
	}

	// Write rendered content to file
	outputPath := filepath.Join(fullOutputDir, outputFileName)
	if err := internal.WriteFileIfAbsent(outputPath, content); err != nil {
		return "", fmt.Errorf("failed to write init file %s: %w", outputPath, err)
	}

	return outputPath, nil
}

// BuildInitPlan returns init directories split by prepare flag, for use in apply init.
// Paths are relative to terraform dir (e.g. "init/0-terraform-statestore").
func BuildInitPlan(terraformConfig *template.TerraformConfig, templateArgs renderer.ArgsData, data map[string]interface{}) (prepareDirs, initDirs []string, err error) {
	available := getAvailableInitComponents(templateArgs)
	for _, initItem := range terraformConfig.Init {
		if !available[initItem.Name] || len(initItem.Path) == 0 {
			continue
		}
		outputDir, err := validateAndRenderDestination(initItem, data, templateArgs)
		if err != nil {
			return nil, nil, err
		}
		// outputDir is relative to workspace (e.g. "terraform/init/0-terraform-statestore"); convert to relative to terraform dir
		outputDir = strings.TrimSuffix(outputDir, "/")
		rel := strings.TrimPrefix(outputDir, "terraform/")
		if initItem.Prepare {
			prepareDirs = append(prepareDirs, rel)
		} else {
			initDirs = append(initDirs, rel)
		}
	}
	return prepareDirs, initDirs, nil
}
