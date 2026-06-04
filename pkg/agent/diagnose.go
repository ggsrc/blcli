package agent

import (
	"strings"
)

const FailureDiagnosisSchemaVersion = "blcli.failure-diagnosis/v1"

// FailureDiagnosis is a machine-readable classification for a blcli/runtime error.
type FailureDiagnosis struct {
	SchemaVersion   string   `json:"schema_version" yaml:"schema_version"`
	Category        string   `json:"category" yaml:"category"`
	Confidence      string   `json:"confidence" yaml:"confidence"`
	MatchedKeywords []string `json:"matched_keywords" yaml:"matched_keywords"`
	Summary         string   `json:"summary" yaml:"summary"`
	LikelyCause     string   `json:"likely_cause" yaml:"likely_cause"`
	NextSteps       []string `json:"next_steps" yaml:"next_steps"`
	RepairCommands  []string `json:"repair_commands" yaml:"repair_commands"`
	References      []string `json:"references,omitempty" yaml:"references,omitempty"`
}

type diagnosisRule struct {
	category       string
	keywords       []string
	summary        string
	likelyCause    string
	nextSteps      []string
	repairCommands []string
	references     []string
}

var diagnosisRules = []diagnosisRule{
	{
		category:    "gcp_project_soft_deleted",
		keywords:    []string{"delete_requested", "project has been scheduled for deletion"},
		summary:     "The GCP project appears to be in DELETE_REQUESTED state.",
		likelyCause: "A project with the same ID was deleted recently and still exists in the soft-delete window.",
		nextSteps: []string{
			"Undelete the project before re-running terraform.",
			"After undelete, refresh or import the Terraform state for the affected project.",
		},
		repairCommands: []string{
			"gcloud projects undelete <project-id>",
			"blcli apply terraform -d ./workspace/output/terraform --project <project>",
		},
		references: []string{"README.md#GCP projects in DELETE_REQUESTED state"},
	},
	{
		category:    "billing_disabled",
		keywords:    []string{"billing_disabled", "billing has not been enabled", "project billing must be enabled"},
		summary:     "Billing is disabled or detached for the target GCP project.",
		likelyCause: "The project was restored, created outside the expected flow, or no billing account is attached.",
		nextSteps: []string{
			"Attach a valid billing account to the project.",
			"Re-run only the failed terraform stage after billing is active.",
		},
		repairCommands: []string{
			"gcloud beta billing projects link <project-id> --billing-account <billing-account-id>",
			"blcli apply terraform -d ./workspace/output/terraform --project <project>",
		},
		references: []string{"README.md#Billing disabled on restored projects"},
	},
	{
		category:    "resource_already_exists",
		keywords:    []string{"already exists", "alreadyexists", " error 409", "status 409", "http 409", "conflict"},
		summary:     "A resource already exists outside the current Terraform or Kubernetes state.",
		likelyCause: "The resource was created manually, by a previous run, or by another state backend.",
		nextSteps: []string{
			"Identify the existing resource and decide whether to import it or rename it.",
			"Do not delete production resources only to satisfy the local state.",
		},
		repairCommands: []string{
			"terraform import <resource-address> <resource-id>",
			"blcli apply terraform -d ./workspace/output/terraform --project <project>",
		},
		references: []string{"README.md#409 resource-already-exists conflicts"},
	},
	{
		category:    "state_lock_conflict",
		keywords:    []string{"error acquiring the state lock", "state lock", "statelock", "config.lock", "lock file"},
		summary:     "A local or remote state/config lock is blocking the operation.",
		likelyCause: "Another process is running, a previous command exited abruptly, or kubectl left a stale config lock.",
		nextSteps: []string{
			"Check whether another Terraform, kubectl, or blcli process is still active.",
			"Only remove a lock after confirming no active process owns it.",
		},
		repairCommands: []string{
			"terraform force-unlock <lock-id>",
			"rm ~/.kube/config.lock",
		},
		references: []string{"README.md#kubectl config.lock file error"},
	},
	{
		category:    "missing_backend_configuration",
		keywords:    []string{"missing backend configuration", "backend configuration block is missing", "backend \"gcs\""},
		summary:     "The Terraform directory is missing the expected backend configuration.",
		likelyCause: "The project was rendered without backend.tf or the command is running from the wrong directory.",
		nextSteps: []string{
			"Verify the generated terraform directory contains backend.tf.",
			"Re-run blcli init for the affected module if generated files are stale.",
		},
		repairCommands: []string{
			"blcli init ../bl-template -a args.yaml -m terraform --overwrite",
			"blcli apply terraform -d ./workspace/output/terraform --project <project>",
		},
		references: []string{"README.md#Missing backend configuration"},
	},
	{
		category:    "prevent_destroy",
		keywords:    []string{"prevent_destroy", "instance cannot be destroyed", "lifecycle.prevent_destroy"},
		summary:     "Terraform is blocked by a prevent_destroy lifecycle rule.",
		likelyCause: "The plan would replace or destroy a protected resource, commonly the state bucket.",
		nextSteps: []string{
			"Inspect the Terraform plan and confirm why the protected resource would be destroyed.",
			"Fix the configuration drift instead of disabling prevent_destroy by default.",
		},
		repairCommands: []string{
			"terraform plan",
			"terraform state show <resource-address>",
		},
		references: []string{"README.md#Terraform state bucket prevent_destroy error"},
	},
	{
		category:    "quota_exceeded",
		keywords:    []string{"quota exceeded", "quota was exceeded", "resource_exhausted", "insufficient regional quota", "cpu quota"},
		summary:     "The target cloud account or region does not have enough quota.",
		likelyCause: "The requested infrastructure exceeds available CPU, address, API, or service quota.",
		nextSteps: []string{
			"Check quota in the target region and project.",
			"Request quota increases or reduce the requested resource size.",
		},
		repairCommands: []string{
			"gcloud compute regions describe <region> --format=json",
			"blcli apply terraform -d ./workspace/output/terraform --project <project>",
		},
		references: []string{"README.md#GKE node pool CPU quota exceeded"},
	},
	{
		category:    "permission_insufficient",
		keywords:    []string{"permission_denied", "forbidden", "access denied", "iam.serviceaccounts.actas", "does not have permission", " error 403", "status 403"},
		summary:     "The active identity does not have enough permission.",
		likelyCause: "The gcloud, kubectl, GitHub, or ArgoCD identity lacks a required role for the target resource.",
		nextSteps: []string{
			"Confirm the active account/context and target project or cluster.",
			"Grant the least required role, then re-run only the failed stage.",
		},
		repairCommands: []string{
			"gcloud auth list",
			"kubectl config current-context",
			"blcli check",
		},
	},
	{
		category:    "credential_invalid",
		keywords:    []string{"could not find default credentials", "application default credentials", "invalid_grant", "unauthorized", "authentication failed", "no auth provider found"},
		summary:     "Credentials are missing, expired, or invalid.",
		likelyCause: "The local environment is not authenticated for the provider required by the failing stage.",
		nextSteps: []string{
			"Refresh credentials for the failing provider.",
			"Confirm the command is using the intended account and context.",
		},
		repairCommands: []string{
			"gcloud auth login",
			"gcloud auth application-default login",
			"gh auth login",
			"blcli check",
		},
	},
	{
		category:    "dependency_missing",
		keywords:    []string{"command not found", "executable file not found", "no such file or directory", "terraform: not found", "kubectl: not found", "helm: not found", "gh: not found"},
		summary:     "A required local dependency is not installed or not on PATH.",
		likelyCause: "The current shell cannot find terraform, kubectl, helm, gh, gcloud, or another required tool.",
		nextSteps: []string{
			"Run blcli check to identify missing tools.",
			"Install the missing tool and restart the shell if PATH changed.",
		},
		repairCommands: []string{
			"blcli check",
		},
	},
	{
		category:    "network_problem",
		keywords:    []string{"i/o timeout", "connection refused", "connection reset", "no such host", "tls handshake timeout", "temporary failure in name resolution", "network is unreachable"},
		summary:     "The operation failed due to network connectivity.",
		likelyCause: "The local machine cannot reach a cloud API, GitHub, Kubernetes API server, or ArgoCD endpoint.",
		nextSteps: []string{
			"Check network connectivity, DNS, proxy, VPN, and endpoint availability.",
			"Retry only after the endpoint is reachable.",
		},
		repairCommands: []string{
			"blcli check",
			"kubectl cluster-info",
		},
	},
	{
		category:    "argocd_ssh_auth",
		keywords:    []string{"usesshagent", "permission denied (publickey)", "ssh: handshake failed", "repository not accessible", "failed to list refs"},
		summary:     "ArgoCD cannot authenticate to the Git repository over SSH.",
		likelyCause: "The deploy key, repository URL, or sealed secret contains incompatible SSH settings.",
		nextSteps: []string{
			"Verify the GitOps repo URL uses SSH format.",
			"Regenerate the ArgoCD repository secret without unsupported useSshAgent fields.",
		},
		repairCommands: []string{
			"blcli init ../bl-template -a args.yaml -m gitops --overwrite",
			"blcli apply gitops -d ./workspace/output/gitops --args args.yaml",
		},
		references: []string{"README.md#ArgoCD SSH authentication failures"},
	},
}

// DiagnoseFailure classifies an error message and returns actionable repair guidance.
func DiagnoseFailure(message string) FailureDiagnosis {
	normalized := strings.ToLower(message)
	for _, rule := range diagnosisRules {
		matches := matchedKeywords(normalized, rule.keywords)
		if len(matches) == 0 {
			continue
		}

		return FailureDiagnosis{
			SchemaVersion:   FailureDiagnosisSchemaVersion,
			Category:        rule.category,
			Confidence:      confidenceForMatches(matches),
			MatchedKeywords: matches,
			Summary:         rule.summary,
			LikelyCause:     rule.likelyCause,
			NextSteps:       rule.nextSteps,
			RepairCommands:  rule.repairCommands,
			References:      rule.references,
		}
	}

	return FailureDiagnosis{
		SchemaVersion: FailureDiagnosisSchemaVersion,
		Category:      "unknown",
		Confidence:    "low",
		Summary:       "No known failure pattern matched the provided message.",
		LikelyCause:   "The error may be specific to the template, provider, cluster, or local environment.",
		NextSteps: []string{
			"Re-run the failed command with the narrowest module or project scope.",
			"Inspect the command output immediately above the first error line.",
			"Run blcli check to validate local dependencies.",
		},
		RepairCommands: []string{
			"blcli check",
		},
	}
}

func matchedKeywords(message string, keywords []string) []string {
	var matches []string
	for _, keyword := range keywords {
		if strings.Contains(message, strings.ToLower(keyword)) {
			matches = append(matches, keyword)
		}
	}
	return matches
}

func confidenceForMatches(matches []string) string {
	if len(matches) >= 2 {
		return "high"
	}
	return "medium"
}
