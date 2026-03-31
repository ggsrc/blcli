package bootstrap

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"blcli/pkg/config"
	"blcli/pkg/internal"
	"blcli/pkg/renderer"
	"blcli/pkg/state"
	"blcli/pkg/template"
	"blcli/pkg/validator"
)

// Profiler interface for optional performance tracking
type Profiler interface {
	TimeStep(name string, fn func() error) error
}

// InitOptions holds options for init command
type InitOptions struct {
	Modules      []string
	ArgsPaths    []string // Args file paths (required, earlier ones override later ones)
	EnvPaths     []string // Env file paths with high-priority args overrides (optional)
	TemplateRepo string
	ForceUpdate  bool
	CacheExpiry  time.Duration
	Overwrite    bool   // If true, allow overwriting existing blcli-managed directories
	ProfilePath  string // Path to save CPU profile (empty = no profiling)
	OutputPath   string // Output directory path (overrides workspace in config if specified)
	Quiet        bool   // If true, don't show progress updates
}

// DestroyOptions holds options for destroy command
type DestroyOptions struct {
	Modules   []string
	ArgsPaths []string // Args file paths (required)
}

// ExecuteInit executes the init command
func ExecuteInit(opts InitOptions) error {
	// Initialize profiler if profile path is specified
	var profiler *internal.Profiler
	var err error
	if opts.ProfilePath != "" {
		profiler, err = internal.NewProfiler(opts.ProfilePath)
		if err != nil {
			return fmt.Errorf("failed to initialize profiler: %w", err)
		}
		defer profiler.Stop()
		defer profiler.PrintTimings()
	}

	// Args file is now required
	if len(opts.ArgsPaths) == 0 {
		return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
	}

	// Load template args from multiple files (earlier files override later ones)
	var templateArgs renderer.ArgsData
	var allArgs []renderer.ArgsData
	for _, argsPath := range opts.ArgsPaths {
		fmt.Printf("Loading args from: %s\n", argsPath)
		args, err := renderer.LoadArgs(argsPath)
		if err != nil {
			return fmt.Errorf("failed to load args file %s: %w", argsPath, err)
		}
		allArgs = append(allArgs, args)
	}
	// Merge args: earlier files override later ones
	// So we reverse the order when merging
	if len(allArgs) > 0 {
		reversed := make([]renderer.ArgsData, len(allArgs))
		for i, args := range allArgs {
			reversed[len(allArgs)-1-i] = args
		}
		templateArgs = renderer.MergeArgs(reversed...)
	}
	templateArgs, err = ApplyInitEnvOverrides(templateArgs, ResolveInitEnvPaths(opts.ArgsPaths, opts.EnvPaths))
	if err != nil {
		return fmt.Errorf("failed to apply env overrides: %w", err)
	}

	// Load configuration from args file
	cfg, err := config.LoadFromArgs(templateArgs)
	if err != nil {
		return fmt.Errorf("failed to load config from args: %w", err)
	}

	// Override workspace with --output if specified
	if opts.OutputPath != "" {
		cfg.Global.Workspace = opts.OutputPath
	}

	// Ensure state directory exists
	if err := state.EnsureStateDir(); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Check and install required tools
	fmt.Println("Checking required tools...")
	toolCfg := internal.ToolConfig{
		TerraformVersion: cfg.Global.TerraformVersion,
		KubectlVersion:   cfg.Global.KubectlVersion,
	}
	if err := internal.CheckAndInstallTools(toolCfg); err != nil {
		return fmt.Errorf("error checking/installing tools: %w", err)
	}
	fmt.Println()

	// Load state
	st, err := state.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Load template loader
	var templateLoader *template.Loader
	if opts.TemplateRepo != "" {
		fmt.Printf("Loading templates from: %s\n", opts.TemplateRepo)
		loaderOptions := template.LoaderOptions{
			ForceUpdate: opts.ForceUpdate,
			CacheExpiry: opts.CacheExpiry,
		}
		if loaderOptions.CacheExpiry == 0 {
			// Default to 24 hours if not specified
			loaderOptions.CacheExpiry = 24 * time.Hour
		}
		templateLoader = template.NewLoaderWithOptions(opts.TemplateRepo, loaderOptions)
		if opts.ForceUpdate {
			fmt.Println("Force update enabled: templates will be synced from remote")
		}
		// Sync cache once at initialization
		if err := templateLoader.SyncCache(); err != nil {
			return fmt.Errorf("failed to sync template cache: %w", err)
		}
		// Validate merged args against template param definitions (before writing any files)
		if err := validator.Run(templateArgs, templateLoader); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	// Determine modules to initialize
	modules := opts.Modules
	if len(modules) == 0 {
		modules = []string{"terraform", "kubernetes", "gitops"}
	} else {
		// Expand "all" to all modules
		var expandedModules []string
		for _, module := range modules {
			if module == "all" {
				expandedModules = append(expandedModules, "terraform", "kubernetes", "gitops")
			} else {
				expandedModules = append(expandedModules, module)
			}
		}
		modules = expandedModules
	}

	// Initialize progress tracker
	operationID := GenerateOperationID("init")
	progressTracker, err := NewProgressTracker(operationID, "init", opts.Quiet)
	if err != nil {
		fmt.Printf("Warning: failed to initialize progress tracker: %v\n", err)
		progressTracker = nil
	}

	if progressTracker != nil {
		if err := progressTracker.StartOperation(); err != nil {
			fmt.Printf("Warning: failed to start progress tracking: %v\n", err)
		}
		defer func() {
			if progressTracker != nil {
				if err != nil {
					progressTracker.FailOperation(fmt.Sprintf("%v", err))
				} else {
					progressTracker.CompleteOperation()
				}
				progressTracker.PrintSummary()
			}
		}()
	}

	// Initialize modules
	for _, module := range modules {
		moduleName := module

		// Start module tracking
		if progressTracker != nil {
			// Estimate total steps (will be updated as steps are added)
			if err := progressTracker.StartModule(moduleName, 1); err != nil {
				fmt.Printf("Warning: failed to track module %s: %v\n", moduleName, err)
			}
		}

		if profiler != nil {
			err = profiler.TimeStep(fmt.Sprintf("Initialize %s module", moduleName), func() error {
				initErr := initializeModule(moduleName, cfg, templateLoader, templateArgs, opts.Overwrite, &st, profiler, progressTracker)
				if progressTracker != nil {
					if initErr != nil {
						progressTracker.FailModule(moduleName, fmt.Sprintf("%v", initErr))
					} else {
						progressTracker.CompleteModule(moduleName)
					}
				}
				return initErr
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
		} else {
			err = initializeModule(moduleName, cfg, templateLoader, templateArgs, opts.Overwrite, &st, nil, progressTracker)
			if progressTracker != nil {
				if err != nil {
					progressTracker.FailModule(moduleName, fmt.Sprintf("%v", err))
				} else {
					progressTracker.CompleteModule(moduleName)
				}
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
		}
	}

	// Save state
	if err := state.Save(st); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// initializeModule initializes a single module
func initializeModule(module string, cfg config.BlcliConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData, overwrite bool, st *state.BlcliState, profiler Profiler, progressTracker *ProgressTracker) error {
	switch module {
	case "terraform":
		if cfg.Terraform == nil {
			fmt.Fprintln(os.Stderr, "No [terraform] section found in config; skipping terraform init.")
			return nil
		}
		var names []string
		var err error
		if profiler != nil {
			err = profiler.TimeStep("Bootstrap terraform", func() error {
				var bootstrapErr error
				names, bootstrapErr = BootstrapTerraformWithProfiler(cfg.Global, cfg.Terraform, templateLoader, templateArgs, overwrite, profiler, progressTracker)
				return bootstrapErr
			})
		} else {
			names, err = BootstrapTerraformWithProfiler(cfg.Global, cfg.Terraform, templateLoader, templateArgs, overwrite, nil, progressTracker)
		}
		if err != nil {
			return err
		}
		for _, name := range names {
			state.RecordTerraformProject(st, name)
		}
	case "kubernetes":
		if cfg.Kubernetes == nil {
			fmt.Fprintln(os.Stderr, "No [kubernetes] section found in config; skipping kubernetes init.")
			return nil
		}
		if err := BootstrapKubernetes(cfg.Global, cfg.Kubernetes, templateLoader, templateArgs, overwrite); err != nil {
			return err
		}
		state.RecordKubernetesProject(st, cfg.Kubernetes.Name)
	case "gitops":
		if cfg.Gitops == nil {
			fmt.Fprintln(os.Stderr, "No [gitops] section found in config; skipping gitops init.")
			return nil
		}
		if err := BootstrapGitops(cfg.Global, cfg.Gitops, templateLoader, templateArgs); err != nil {
			return err
		}
		state.RecordGitopsProject(st, cfg.Gitops.Name)
	default:
		return fmt.Errorf("unknown init target `%s`. Supported targets: terraform, kubernetes, gitops", module)
	}
	return nil
}

// ExecuteDestroy executes the destroy command
func ExecuteDestroy(opts DestroyOptions) error {
	// Args file is required
	if len(opts.ArgsPaths) == 0 {
		return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
	}

	// Load args from files
	var allArgs []renderer.ArgsData
	for _, argsPath := range opts.ArgsPaths {
		args, err := renderer.LoadArgs(argsPath)
		if err != nil {
			return fmt.Errorf("failed to load args file %s: %w", argsPath, err)
		}
		allArgs = append(allArgs, args)
	}
	// Merge args: earlier files override later ones
	if len(allArgs) == 0 {
		return fmt.Errorf("no valid args files loaded")
	}
	reversed := make([]renderer.ArgsData, len(allArgs))
	for i, args := range allArgs {
		reversed[len(allArgs)-1-i] = args
	}
	mergedArgs := renderer.MergeArgs(reversed...)

	// Load configuration from args file
	cfg, err := config.LoadFromArgs(mergedArgs)
	if err != nil {
		return fmt.Errorf("failed to load config from args: %w", err)
	}

	modules := opts.Modules
	if len(modules) == 0 {
		modules = []string{"terraform", "kubernetes", "gitops"}
	}

	// Confirm destruction
	fmt.Fprintln(os.Stderr, "⚠️  WARNING: This will destroy initialized projects and may run 'terraform destroy' on your infrastructure!")
	fmt.Fprintln(os.Stderr, "This action cannot be undone. Use only on test/non-production environments unless you are absolutely sure.")
	fmt.Fprintln(os.Stderr)
	if len(modules) > 0 {
		fmt.Fprintf(os.Stderr, "Modules to destroy: %s\n\n", strings.Join(modules, ", "))
	}

	fmt.Fprint(os.Stderr, "Type 'yes' to confirm: ")
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(text)) != "yes" {
		fmt.Fprintln(os.Stderr, "Destroy cancelled. No changes were made.")
		return nil
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Proceeding with destroy...")
	fmt.Fprintln(os.Stderr)

	// Destroy modules
	for _, module := range modules {
		switch module {
		case "terraform":
			if cfg.Terraform == nil {
				fmt.Fprintln(os.Stderr, "No [terraform] section found in config; nothing to destroy for terraform.")
				continue
			}
			// Additional safety confirmation for terraform destroy: require typing organization ID or global name if available.
			expected := cfg.Global.OrganizationID
			label := "organization ID (organization_id)"
			if expected == "" {
				expected = cfg.Global.Name
				label = "global name (global.name)"
			}
			if expected != "" {
				fmt.Fprintf(os.Stderr, "Extra safety check for terraform destroy. Type %q (%s) to confirm: ", expected, label)
				text2, _ := reader.ReadString('\n')
				if strings.TrimSpace(text2) != expected {
					fmt.Fprintln(os.Stderr, "Terraform destroy cancelled for safety. organization/global name did not match.")
					continue
				}
			}
			if err := DestroyTerraform(cfg.Global, cfg.Terraform); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		case "kubernetes":
			if cfg.Kubernetes == nil {
				fmt.Fprintln(os.Stderr, "No [kubernetes] section found in config; nothing to destroy for kubernetes.")
				continue
			}
			if err := DestroyKubernetes(cfg.Global, cfg.Kubernetes); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		case "gitops":
			if cfg.Gitops == nil {
				fmt.Fprintln(os.Stderr, "No [gitops] section found in config; nothing to destroy for gitops.")
				continue
			}
			if err := DestroyGitops(cfg.Global, cfg.Gitops); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		default:
			fmt.Fprintf(os.Stderr, "Unknown destroy target `%s`. Supported targets: terraform, kubernetes, gitops.\n", module)
		}
	}

	return nil
}
