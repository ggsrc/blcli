package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"blcli/pkg/bootstrap"
)

var (
	forceUpdate  bool
	cacheExpiry  time.Duration
	argsPaths    []string // Support multiple args files
	envPaths     []string // Support env override files
	overwrite    bool     // Allow overwriting existing blcli-managed directories
	profilePath  string   // Path to save CPU profile (empty = no profiling)
	outputPath   string   // Output directory path (overrides workspace in config)
	modulesSlice []string // Modules to initialize (empty = all)
)

// NewInitCommand creates the init command
func NewInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [template-repo]",
		Short: "Initialize projects defined in blcli.config.toml",
		Long: `Initialize projects defined in blcli.config.toml.

This will initialize all configured modules (or only those specified by --modules):
- All terraform projects listed in terraform.projects
- Kubernetes project
- GitOps project

Use --modules to initialize specific modules (default: all). Template repository can be given as first positional argument.

Template Repository:
  Templates are loaded from a GitHub repository or local path.
  
  For private repositories, authentication is required. The tool will automatically try to get a GitHub token from:
  1. GITHUB_TOKEN environment variable
  2. gh cli (gh auth token) if GitHub CLI is installed and authenticated
  
  If a token is available, the tool will use GitHub API for private repositories, otherwise it will use
  raw.githubusercontent.com for public repositories.

Template Repository Format:
  - GitHub: github.com/user/repo (defaults to main branch)
  - GitHub: github.com/user/repo@branch (specific branch/tag)
  - Local path: /absolute/path/to/template
  - Local path: ./relative/path/to/template
  - Local path: $HOME/code/bl-template (with environment variable expansion)

Cache Management:
  Templates are cached locally to improve performance. You can control caching behavior with:
  - --force-update: Always fetch from remote, ignoring cache
  - --cache-expiry: Set how long templates are cached (default: 24h, 0 = no expiry)
  
  Cache location: ~/.blcli/templates/{repo_name}/`,
		Example: `  # Initialize all modules (template repo from default)
  blcli init -a args.yaml

  # Specify template repo as positional argument
  blcli init ./bl-template -a args.yaml
  blcli init github.com/user/repo -a args.yaml

  # Initialize specific modules
  blcli init -a args.yaml --modules terraform
  blcli init -a args.yaml -m kubernetes -m gitops

  # Use custom template repository (public) with short flag
  blcli init github.com/ggsrc/infra-template -a args.yaml

  # Use GitHub repository with specific branch/tag
  blcli init github.com/user/repo@v1.0.0 -a args.yaml

  # Use local template repository
  blcli init /Users/username/code/bl-template -a args.yaml
  blcli init ./bl-template -a args.yaml
  blcli init $HOME/code/bl-template -a args.yaml

  # Use private template repository (requires authentication)
  export GITHUB_TOKEN=your_token_here
  blcli init github.com/ggsrc/bl-template -a args.yaml

  # Use multiple args files (earlier files override later ones)
  blcli init github.com/user/repo -a base-args.yaml -a override-args.yaml

  # Use default .env next to args.yaml automatically
  blcli init ./bl-template -a args.yaml

  # Or specify .env overrides explicitly
  blcli init ./bl-template -a args.yaml --env-file .env

  # Force update templates (ignore cache)
  blcli init github.com/user/repo -a args.yaml -f

  # Set custom cache expiry
  blcli init github.com/user/repo -a args.yaml --cache-expiry=1h
  blcli init github.com/user/repo -a args.yaml --cache-expiry=30m
  blcli init github.com/user/repo -a args.yaml --cache-expiry=0

  # Specify output directory (overrides workspace in config)
  blcli init github.com/user/repo -a args.yaml -o workspace/output
  blcli init ./bl-template -a args.yaml --modules terraform -o workspace/output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Template repo: first positional or default
			templateRepoValue := "github.com/ggsrc/infra-template"
			if len(args) > 0 {
				templateRepoValue = args[0]
			}

			// Support multiple args files (earlier files override later ones)
			argsFiles := argsPaths

			// Args file is now required
			if len(argsFiles) == 0 {
				return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
			}

			// Modules: empty = all (bootstrap expands to terraform, kubernetes, gitops)
			modules := modulesSlice

			return bootstrap.ExecuteInit(bootstrap.InitOptions{
				Modules:      modules,
				TemplateRepo: templateRepoValue,
				ArgsPaths:    argsFiles,
				EnvPaths:     envPaths,
				ForceUpdate:  forceUpdate,
				CacheExpiry:  cacheExpiry,
				Overwrite:    overwrite,
				ProfilePath:  profilePath,
				OutputPath:   outputPath,
				Quiet:        false, // Show progress by default
			})
		},
	}

	// Init-specific flags (template-repo is positional)
	cmd.Flags().StringArrayVarP(&argsPaths, "args", "a", nil,
		"Path to YAML or TOML file with template arguments and blcli configuration (required, can be specified multiple times, earlier files override later ones)")
	cmd.Flags().StringArrayVar(&envPaths, "env-file", nil,
		"Path to .env file with args overrides (can be specified multiple times, earlier files override later ones). If omitted, blcli auto-loads .env next to the first args file when present")
	cmd.Flags().StringArrayVarP(&modulesSlice, "modules", "m", nil,
		"Modules to initialize: terraform, kubernetes, gitops (default: all). Can be specified multiple times")
	cmd.Flags().BoolVarP(&forceUpdate, "force-update", "f", false,
		"Force update templates from remote repository, ignoring cache")
	cmd.Flags().DurationVar(&cacheExpiry, "cache-expiry", 24*time.Hour,
		"Cache expiry duration for templates (e.g., 1h, 30m, 0 = no expiry)")
	cmd.Flags().BoolVarP(&overwrite, "overwrite", "w", false,
		"Allow overwriting existing blcli-managed directories (use with caution)")
	cmd.Flags().StringVar(&profilePath, "profile", "",
		"Enable CPU profiling and save to file (e.g., --profile=cpu.prof). Use 'go tool pprof cpu.prof' to analyze")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "",
		"Output directory path for generated files (overrides workspace in config file). Default: use workspace from config")

	return cmd
}
