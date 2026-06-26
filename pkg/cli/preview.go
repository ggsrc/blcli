package cli

import (
	"fmt"
	"strings"
)

// printArgsConfigPreview summarizes an ArgsConfig before writing or after wizard.
func printArgsConfigPreview(cfg *ArgsConfig, outputPath string) {
	fmt.Println()
	fmt.Println("=== Configuration Preview ===")
	fmt.Printf("Output file:     %s\n", outputPath)

	if cfg.Global != nil {
		if v, ok := cfg.Global["workspace"].(string); ok && v != "" {
			fmt.Printf("Workspace:       %s\n", v)
		}
		if v, ok := cfg.Global["GlobalName"].(string); ok && v != "" {
			fmt.Printf("Global name:     %s\n", v)
		}
		if v, ok := cfg.Global["domain"].(string); ok && v != "" {
			fmt.Printf("Domain:          %s\n", v)
		}
	}

	if len(cfg.Terraform.Global) > 0 {
		if v, ok := cfg.Terraform.Global["OrganizationID"].(string); ok && v != "" {
			fmt.Printf("Organization ID: %s\n", v)
		}
		if v, ok := cfg.Terraform.Global["BillingAccountID"].(string); ok && v != "" {
			fmt.Printf("Billing account: %s\n", maskSecret(v))
		}
	}

	fmt.Println()
	fmt.Println("Modules:")
	if len(cfg.Terraform.Projects) > 0 || cfg.Terraform.Init != nil || len(cfg.Terraform.Global) > 0 {
		fmt.Printf("  terraform: %d project(s)", len(cfg.Terraform.Projects))
		if names := projectNamesFromArgs(cfg.Terraform.Projects); len(names) > 0 {
			fmt.Printf(" [%s]", strings.Join(names, ", "))
		}
		fmt.Println()
	}
	if len(cfg.Kubernetes.Projects) > 0 || len(cfg.Kubernetes.Global) > 0 {
		fmt.Printf("  kubernetes: %d project(s)", len(cfg.Kubernetes.Projects))
		if names := projectNamesFromArgs(cfg.Kubernetes.Projects); len(names) > 0 {
			fmt.Printf(" [%s]", strings.Join(names, ", "))
		}
		fmt.Println()
	}
	if len(cfg.Gitops) > 0 {
		fmt.Println("  gitops: configured")
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. blcli check args --args", outputPath, "-r <template-repo>")
	fmt.Println("  2. blcli init -a", outputPath, "-r <template-repo>")
	fmt.Println("  3. blcli apply all -d <workspace> --args", outputPath)
}

func projectNamesFromArgs(projects []ProjectData) []string {
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		if p.Name != "" {
			names = append(names, p.Name)
		}
	}
	return names
}

func maskSecret(value string) string {
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "****" + value[len(value)-4:]
}
