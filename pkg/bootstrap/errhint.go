package bootstrap

import (
	"fmt"
	"strings"
)

// errHint describes an actionable recovery hint for a common failure.
type errHint struct {
	Title   string
	Steps   []string
	DocLink string
}

// PrintFailureHints writes recovery suggestions for known error patterns.
func PrintFailureHints(operation string, err error) {
	if err == nil {
		return
	}
	msg := strings.ToLower(err.Error())
	hint := matchFailureHint(operation, msg)
	if hint == nil {
		return
	}

	fmt.Printf("\nSuggested next steps (%s):\n", hint.Title)
	for i, step := range hint.Steps {
		fmt.Printf("  %d. %s\n", i+1, step)
	}
	if hint.DocLink != "" {
		fmt.Printf("  Docs: %s\n", hint.DocLink)
	}
}

func matchFailureHint(operation, msg string) *errHint {
	switch {
	case strings.Contains(msg, "args file is required"):
		return &errHint{
			Title: "missing args file",
			Steps: []string{
				"Generate a starter config: blcli init-args -r <template-repo> -o args.yaml",
				"Re-run with: --args args.yaml",
			},
			DocLink: "https://github.com/ggsrc/blcli#quick-start-5-minutes",
		}
	case strings.Contains(msg, "validation failed"):
		return &errHint{
			Title: "args validation failed",
			Steps: []string{
				"Inspect parameter definitions: blcli explain -r <template-repo> -m terraform",
				"Fix required fields in args.yaml (billing account, project IDs, domains)",
				"Re-run init after correcting args",
			},
			DocLink: "https://github.com/ggsrc/blcli/blob/main/docs/zh/USAGE.md",
		}
	case strings.Contains(msg, "backend") && strings.Contains(msg, "terraform"):
		return &errHint{
			Title: "terraform backend not initialized",
			Steps: []string{
				"Apply terraform init stage first: blcli apply init -d <workspace>/terraform --args args.yaml",
				"Or run: blcli apply terraform -d <workspace>/terraform --args args.yaml",
			},
			DocLink: "https://github.com/ggsrc/blcli/blob/main/README.md#missing-backend-configuration",
		}
	case strings.Contains(msg, "kubeconfig") || strings.Contains(msg, "kubernetes"):
		return &errHint{
			Title: "kubernetes access",
			Steps: []string{
				"Fetch cluster credentials: gcloud container clusters get-credentials <cluster> --region <region>",
				"Or pass an explicit kubeconfig: blcli apply kubernetes --kubeconfig <path>",
				"Verify access: kubectl get nodes",
			},
			DocLink: "https://github.com/ggsrc/blcli/blob/main/README.md#troubleshooting",
		}
	case strings.Contains(msg, "argocd") || strings.Contains(msg, "gitops") || strings.Contains(msg, "application"):
		return &errHint{
			Title: "gitops / argocd",
			Steps: []string{
				"Confirm Argo CD is running: kubectl get pods -n argocd",
				"Use SSH repo URLs in args/.env for Argo CD apps",
				"Re-run: blcli apply gitops -d <workspace>/gitops --args args.yaml",
			},
			DocLink: "https://github.com/ggsrc/blcli/blob/main/README.md#argocd-ssh-authentication-failures",
		}
	case strings.Contains(msg, "409") || strings.Contains(msg, "already exists"):
		return &errHint{
			Title: "resource already exists",
			Steps: []string{
				"Import existing resources into terraform state, or remove duplicates in GCP",
				"See README troubleshooting for import examples",
			},
			DocLink: "https://github.com/ggsrc/blcli/blob/main/README.md#409-resource-already-exists-conflicts",
		}
	case strings.Contains(operation, "apply") && strings.Contains(msg, "terraform"):
		return &errHint{
			Title: "terraform apply failed",
			Steps: []string{
				"Re-run only terraform: blcli apply terraform -d <workspace>/terraform --args args.yaml",
				"Preview changes: add --dry-run or run terraform plan in the failing directory",
				"After fixing, resume: blcli apply all -d <workspace> --args args.yaml",
			},
			DocLink: "https://github.com/ggsrc/blcli/blob/main/README.md#troubleshooting",
		}
	default:
		return nil
	}
}

// WrapApplyError attaches context and prints hints for apply failures.
func WrapApplyError(operation string, err error) error {
	if err == nil {
		return nil
	}
	PrintFailureHints(operation, err)
	return err
}
