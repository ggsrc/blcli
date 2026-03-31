package bootstrap

import (
	"fmt"
	"strings"
)

// ExecutionPlan represents an execution plan for apply operations
type ExecutionPlan struct {
	Module string     // terraform, kubernetes, gitops
	Items  []PlanItem // Ordered list of execution items
	DryRun bool       // Whether this is a dry-run
}

// PlanItem represents a single item in the execution plan
type PlanItem struct {
	Step         int      // Step number (1-indexed)
	Name         string   // Component/item name (e.g., "init/0-backend", "gcp/prd/variables")
	Directory    string   // Directory where command will be executed
	Command      string   // Command to execute
	Args         []string // Command arguments
	Dependencies []string // Dependencies (component names this depends on)
	Description  string   // Optional description
}

// PrintExecutionPlan prints the execution plan in a formatted way
func PrintExecutionPlan(plan ExecutionPlan) {
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if plan.DryRun {
		fmt.Println("📋 Execution Plan (DRY RUN)")
	} else {
		fmt.Println("📋 Execution Plan")
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Module: %s\n", plan.Module)
	fmt.Printf("Total Steps: %d\n\n", len(plan.Items))

	for _, item := range plan.Items {
		fmt.Printf("  Step %d: %s\n", item.Step, item.Name)
		if item.Directory != "" {
			fmt.Printf("    Directory: %s\n", item.Directory)
		}
		if item.Description != "" {
			fmt.Printf("    Description: %s\n", item.Description)
		}
		if len(item.Dependencies) > 0 {
			fmt.Printf("    Dependencies: %s\n", strings.Join(item.Dependencies, ", "))
		}
		// Build full command
		fullCommand := item.Command
		if len(item.Args) > 0 {
			fullCommand += " " + strings.Join(item.Args, " ")
		}
		fmt.Printf("    Command: %s\n", fullCommand)
		fmt.Println()
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if plan.DryRun {
		fmt.Println("⚠️  DRY RUN MODE: No changes will be applied")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}
}

// BuildTerraformPlan builds an execution plan for Terraform apply
func BuildTerraformPlan(initDirs []string, projectComponents map[string][]string, terraformDir string) ExecutionPlan {
	var items []PlanItem
	step := 1

	// Add init directories
	for _, initSubDir := range initDirs {
		initDir := fmt.Sprintf("init/%s", initSubDir)
		items = append(items, PlanItem{
			Step:         step,
			Name:         initDir,
			Directory:    fmt.Sprintf("%s/init/%s", terraformDir, initSubDir),
			Command:      "terraform",
			Args:         []string{"apply", "-auto-approve"},
			Dependencies: []string{},
			Description:  fmt.Sprintf("Apply Terraform init directory: %s", initSubDir),
		})
		step++
	}

	// Add project components
	for projectName, components := range projectComponents {
		for _, componentName := range components {
			itemName := fmt.Sprintf("gcp/%s/%s", projectName, componentName)
			items = append(items, PlanItem{
				Step:         step,
				Name:         itemName,
				Directory:    fmt.Sprintf("%s/gcp/%s", terraformDir, projectName),
				Command:      "terraform",
				Args:         []string{"apply", "-auto-approve"},
				Dependencies: []string{}, // Will be filled by dependency resolution
				Description:  fmt.Sprintf("Apply Terraform component %s in project %s", componentName, projectName),
			})
			step++
		}
	}

	return ExecutionPlan{
		Module: "terraform",
		Items:  items,
		DryRun: false,
	}
}

// BuildKubernetesPlan builds an execution plan for Kubernetes apply
func BuildKubernetesPlan(components []componentInfo, kubernetesDir string) ExecutionPlan {
	var items []PlanItem
	step := 1

	for _, comp := range components {
		itemName := fmt.Sprintf("%s/%s", comp.projectName, comp.componentName)
		items = append(items, PlanItem{
			Step:         step,
			Name:         itemName,
			Directory:    comp.componentDir,
			Command:      "kubectl",
			Args:         []string{"apply", "-k", comp.componentDir},
			Dependencies: []string{}, // Will be filled by dependency resolution
			Description:  fmt.Sprintf("Apply Kubernetes component %s in project %s", comp.componentName, comp.projectName),
		})
		step++
	}

	return ExecutionPlan{
		Module: "kubernetes",
		Items:  items,
		DryRun: false,
	}
}

// BuildGitOpsPlan builds an execution plan for GitOps apply
func BuildGitOpsPlan(applications []applicationInfo, gitopsDir string) ExecutionPlan {
	var items []PlanItem
	step := 1

	for _, app := range applications {
		itemName := fmt.Sprintf("%s/%s", app.projectName, app.appName)
		items = append(items, PlanItem{
			Step:         step,
			Name:         itemName,
			Directory:    app.appDir,
			Command:      "kubectl",
			Args:         []string{"apply", "-f", app.appYaml},
			Dependencies: []string{},
			Description:  fmt.Sprintf("Apply GitOps Application %s in project %s", app.appName, app.projectName),
		})
		step++
	}

	return ExecutionPlan{
		Module: "gitops",
		Items:  items,
		DryRun: false,
	}
}

// componentInfo is used internally for Kubernetes plan building
type componentInfo struct {
	projectName   string
	componentName string
	componentDir  string
}

// applicationInfo is used internally for GitOps plan building
type applicationInfo struct {
	projectName string
	appName     string
	appDir      string
	appYaml     string
}
