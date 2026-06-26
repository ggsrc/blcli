package bootstrap

import (
	"fmt"
	"time"

	"blcli/pkg/renderer"
	"blcli/pkg/template"
	"blcli/pkg/validator"
)

// CheckArgsOptions holds options for validating args files against template definitions.
type CheckArgsOptions struct {
	ArgsPaths    []string
	EnvPaths     []string
	TemplateRepo string
	ForceUpdate  bool
	CacheExpiry  time.Duration
}

// ExecuteCheckArgs validates merged args against template parameter definitions.
func ExecuteCheckArgs(opts CheckArgsOptions) error {
	if len(opts.ArgsPaths) == 0 {
		return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
	}
	if opts.TemplateRepo == "" {
		return fmt.Errorf("template repository is required. Use -r/--template-repo")
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
	mergedArgs := renderer.MergeArgs(reversed...)

	envPaths := ResolveInitEnvPaths(opts.ArgsPaths, opts.EnvPaths)
	withEnv, err := ApplyInitEnvOverrides(mergedArgs, envPaths)
	if err != nil {
		return fmt.Errorf("failed to apply env overrides: %w", err)
	}
	mergedArgs = withEnv

	cacheExpiry := opts.CacheExpiry
	if cacheExpiry == 0 {
		cacheExpiry = 24 * time.Hour
	}
	loader := template.NewLoaderWithOptions(opts.TemplateRepo, template.LoaderOptions{
		ForceUpdate: opts.ForceUpdate,
		CacheExpiry: cacheExpiry,
	})
	if err := loader.SyncCache(); err != nil {
		return fmt.Errorf("failed to sync template repository: %w", err)
	}

	templateLoader := validator.NewTemplateLoader(func(path string) (string, error) {
		return loader.LoadTemplate(path)
	})

	fmt.Printf("Validating args against template: %s\n", opts.TemplateRepo)
	if err := validator.Run(mergedArgs, templateLoader); err != nil {
		PrintFailureHints("check args", err)
		return fmt.Errorf("validation failed: %w", err)
	}

	fmt.Println("✅ Args validation passed")
	return nil
}
