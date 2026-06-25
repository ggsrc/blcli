package bootstrap

import (
	"fmt"
	"strings"

	"blcli/pkg/config"
	"blcli/pkg/renderer"
)

// PrintInitPreview prints what init would generate without writing files.
func PrintInitPreview(opts InitOptions) error {
	if len(opts.ArgsPaths) == 0 {
		return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
	}

	var allArgs []renderer.ArgsData
	for _, argsPath := range opts.ArgsPaths {
		args, err := renderer.LoadArgs(argsPath)
		if err != nil {
			return fmt.Errorf("failed to load args from %s: %w", argsPath, err)
		}
		allArgs = append(allArgs, args)
	}
	reversed := make([]renderer.ArgsData, len(allArgs))
	for i, a := range allArgs {
		reversed[len(allArgs)-1-i] = a
	}
	templateArgs := renderer.MergeArgs(reversed...)

	envPaths := ResolveInitEnvPaths(opts.ArgsPaths, opts.EnvPaths)
	var err error
	templateArgs, err = ApplyInitEnvOverrides(templateArgs, envPaths)
	if err != nil {
		return fmt.Errorf("failed to apply env overrides: %w", err)
	}

	cfg, err := config.LoadFromArgs(templateArgs)
	if err != nil {
		return fmt.Errorf("failed to load config from args: %w", err)
	}

	modules := opts.Modules
	if len(modules) == 0 {
		modules = []string{"terraform", "kubernetes", "gitops"}
	}

	outputDir := cfg.Global.Workspace
	if opts.OutputPath != "" {
		outputDir = opts.OutputPath
	}
	if outputDir == "" {
		outputDir = "."
	}

	fmt.Println()
	fmt.Println("=== Init Preview ===")
	fmt.Printf("Template repo: %s\n", opts.TemplateRepo)
	fmt.Printf("Output dir:    %s\n", outputDir)
	fmt.Printf("Modules:       %s\n", strings.Join(modules, ", "))
	if len(opts.ArgsPaths) > 0 {
		fmt.Printf("Args files:    %s\n", strings.Join(opts.ArgsPaths, ", "))
	}

	if containsModule(modules, "terraform") && cfg.Terraform != nil {
		fmt.Printf("\nTerraform projects (%d):\n", len(cfg.Terraform.Projects))
		for _, name := range cfg.Terraform.Projects {
			fmt.Printf("  - %s -> %s/terraform/gcp/%s\n", name, outputDir, name)
		}
		if len(cfg.Terraform.Projects) == 0 && cfg.Terraform.Name != "" {
			fmt.Printf("  - %s\n", cfg.Terraform.Name)
		}
	}
	if containsModule(modules, "kubernetes") && cfg.Kubernetes != nil {
		fmt.Printf("\nKubernetes: %s/kubernetes\n", outputDir)
	}
	if containsModule(modules, "gitops") && cfg.Gitops != nil {
		fmt.Printf("\nGitOps: %s/gitops\n", outputDir)
	}

	fmt.Println("\nNo files were written (preview mode).")
	return nil
}

func containsModule(modules []string, name string) bool {
	for _, m := range modules {
		if strings.EqualFold(m, name) {
			return true
		}
	}
	return false
}
