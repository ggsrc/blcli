package state

import (
	"time"
)

// StatusResult represents the overall status result
type StatusResult struct {
	Type        string            `json:"type" yaml:"type"` // "terraform", "kubernetes", "gitops", "all"
	Terraform   *TerraformStatus  `json:"terraform,omitempty" yaml:"terraform,omitempty"`
	Kubernetes  *KubernetesStatus `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	GitOps      *GitOpsStatus     `json:"gitops,omitempty" yaml:"gitops,omitempty"`
	Summary     StatusSummary     `json:"summary" yaml:"summary"`
	GeneratedAt time.Time         `json:"generated_at" yaml:"generated_at"`
}

// StatusSummary provides a high-level summary
type StatusSummary struct {
	TotalResources    int    `json:"total_resources" yaml:"total_resources"`
	HealthyResources  int    `json:"healthy_resources" yaml:"healthy_resources"`
	DegradedResources int    `json:"degraded_resources" yaml:"degraded_resources"`
	OverallStatus     string `json:"overall_status" yaml:"overall_status"` // "healthy", "degraded", "unknown"
}

// TerraformStatus represents Terraform resources status
type TerraformStatus struct {
	InitDirs []TerraformDirStatus `json:"init_dirs,omitempty" yaml:"init_dirs,omitempty"`
	Projects []TerraformDirStatus `json:"projects,omitempty" yaml:"projects,omitempty"`
	Modules  []TerraformDirStatus `json:"modules,omitempty" yaml:"modules,omitempty"`
	Summary  TerraformSummary     `json:"summary" yaml:"summary"`
}

// TerraformDirStatus represents status of a single Terraform directory
type TerraformDirStatus struct {
	Name         string    `json:"name" yaml:"name"`
	Path         string    `json:"path" yaml:"path"`
	Status       string    `json:"status" yaml:"status"` // "initialized", "not_initialized", "error"
	Resources    int       `json:"resources" yaml:"resources"`
	Created      int       `json:"created" yaml:"created"`
	Changed      int       `json:"changed" yaml:"changed"`
	Destroyed    int       `json:"destroyed" yaml:"destroyed"`
	LastUpdated  time.Time `json:"last_updated,omitempty" yaml:"last_updated,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty" yaml:"error_message,omitempty"`
}

// TerraformSummary provides Terraform status summary
type TerraformSummary struct {
	TotalDirs      int    `json:"total_dirs" yaml:"total_dirs"`
	Initialized    int    `json:"initialized" yaml:"initialized"`
	NotInitialized int    `json:"not_initialized" yaml:"not_initialized"`
	TotalResources int    `json:"total_resources" yaml:"total_resources"`
	Status         string `json:"status" yaml:"status"` // "healthy", "degraded", "unknown"
}

// KubernetesStatus represents Kubernetes resources status
type KubernetesStatus struct {
	Projects []KubernetesProjectStatus `json:"projects" yaml:"projects"`
	Summary  KubernetesSummary         `json:"summary" yaml:"summary"`
}

// KubernetesProjectStatus represents status of a Kubernetes project
type KubernetesProjectStatus struct {
	Name       string                      `json:"name" yaml:"name"`
	Components []KubernetesComponentStatus `json:"components" yaml:"components"`
	Summary    KubernetesComponentSummary  `json:"summary" yaml:"summary"`
}

// KubernetesComponentStatus represents status of a Kubernetes component
type KubernetesComponentStatus struct {
	Name         string         `json:"name" yaml:"name"`
	Path         string         `json:"path" yaml:"path"`
	Namespace    string         `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Deployments  ResourceStatus `json:"deployments" yaml:"deployments"`
	StatefulSets ResourceStatus `json:"statefulsets" yaml:"statefulsets"`
	Services     ResourceStatus `json:"services" yaml:"services"`
	Status       string         `json:"status" yaml:"status"` // "healthy", "degraded", "unknown", "not_found"
	ErrorMessage string         `json:"error_message,omitempty" yaml:"error_message,omitempty"`
}

// KubernetesComponentSummary provides component-level summary
type KubernetesComponentSummary struct {
	TotalComponents int `json:"total_components" yaml:"total_components"`
	Healthy         int `json:"healthy" yaml:"healthy"`
	Degraded        int `json:"degraded" yaml:"degraded"`
	NotFound        int `json:"not_found" yaml:"not_found"`
}

// ResourceStatus represents status of Kubernetes resources
type ResourceStatus struct {
	Total   int `json:"total" yaml:"total"`
	Ready   int `json:"ready" yaml:"ready"`
	Pending int `json:"pending" yaml:"pending"`
	Failed  int `json:"failed" yaml:"failed"`
}

// KubernetesSummary provides Kubernetes status summary
type KubernetesSummary struct {
	TotalProjects   int    `json:"total_projects" yaml:"total_projects"`
	TotalComponents int    `json:"total_components" yaml:"total_components"`
	Healthy         int    `json:"healthy" yaml:"healthy"`
	Degraded        int    `json:"degraded" yaml:"degraded"`
	Status          string `json:"status" yaml:"status"` // "healthy", "degraded", "unknown"
}

// GitOpsStatus represents GitOps/ArgoCD status
type GitOpsStatus struct {
	Projects []GitOpsProjectStatus `json:"projects" yaml:"projects"`
	Summary  GitOpsSummary         `json:"summary" yaml:"summary"`
}

// GitOpsProjectStatus represents status of a GitOps project
type GitOpsProjectStatus struct {
	Name         string            `json:"name" yaml:"name"`
	Applications []GitOpsAppStatus `json:"applications" yaml:"applications"`
	Summary      GitOpsAppSummary  `json:"summary" yaml:"summary"`
}

// GitOpsAppStatus represents status of an ArgoCD Application
type GitOpsAppStatus struct {
	Name         string    `json:"name" yaml:"name"`
	Namespace    string    `json:"namespace" yaml:"namespace"`
	SyncStatus   string    `json:"sync_status" yaml:"sync_status"`     // "Synced", "OutOfSync", "Unknown"
	HealthStatus string    `json:"health_status" yaml:"health_status"` // "Healthy", "Degraded", "Unknown"
	Revision     string    `json:"revision,omitempty" yaml:"revision,omitempty"`
	LastSynced   time.Time `json:"last_synced,omitempty" yaml:"last_synced,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty" yaml:"error_message,omitempty"`
}

// GitOpsAppSummary provides application-level summary
type GitOpsAppSummary struct {
	TotalApps int `json:"total_apps" yaml:"total_apps"`
	Synced    int `json:"synced" yaml:"synced"`
	OutOfSync int `json:"out_of_sync" yaml:"out_of_sync"`
	Healthy   int `json:"healthy" yaml:"healthy"`
	Degraded  int `json:"degraded" yaml:"degraded"`
}

// GitOpsSummary provides GitOps status summary
type GitOpsSummary struct {
	TotalProjects int    `json:"total_projects" yaml:"total_projects"`
	TotalApps     int    `json:"total_apps" yaml:"total_apps"`
	Synced        int    `json:"synced" yaml:"synced"`
	OutOfSync     int    `json:"out_of_sync" yaml:"out_of_sync"`
	Status        string `json:"status" yaml:"status"` // "healthy", "degraded", "unknown"
}
