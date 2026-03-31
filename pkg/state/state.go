package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type BlcliState struct {
	TerraformProjects  []string `json:"terraform_projects"`
	KubernetesProjects []string `json:"kubernetes_projects"`
	GitopsProjects     []string `json:"gitops_projects"`
}

// GetStateDir returns the path to the .blcli directory in user's home directory
func GetStateDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	stateDir := filepath.Join(homeDir, ".blcli")
	return stateDir, nil
}

// GetStatePath returns the path to the state.json file
func GetStatePath() (string, error) {
	stateDir, err := GetStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "state.json"), nil
}

// EnsureStateDir creates the .blcli directory if it doesn't exist
func EnsureStateDir() error {
	stateDir, err := GetStateDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(stateDir, 0o755)
}

// Load loads the state from the default location (~/.blcli/state.json)
func Load() (BlcliState, error) {
	statePath, err := GetStatePath()
	if err != nil {
		return BlcliState{}, err
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return BlcliState{}, nil
		}
		return BlcliState{}, fmt.Errorf("failed to read state file: %w", err)
	}

	var st BlcliState
	if err := json.Unmarshal(data, &st); err != nil {
		return BlcliState{}, fmt.Errorf("failed to parse state file: %w", err)
	}
	return st, nil
}

// Save saves the state to the default location (~/.blcli/state.json)
func Save(st BlcliState) error {
	if err := EnsureStateDir(); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	statePath, err := GetStatePath()
	if err != nil {
		return err
	}

	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(statePath, raw, 0o644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

func RecordTerraformProject(st *BlcliState, project string) {
	st.TerraformProjects = append(st.TerraformProjects, project)
	st.TerraformProjects = dedupeStrings(st.TerraformProjects)
}

func RecordKubernetesProject(st *BlcliState, project string) {
	st.KubernetesProjects = append(st.KubernetesProjects, project)
	st.KubernetesProjects = dedupeStrings(st.KubernetesProjects)
}

func RecordGitopsProject(st *BlcliState, project string) {
	st.GitopsProjects = append(st.GitopsProjects, project)
	st.GitopsProjects = dedupeStrings(st.GitopsProjects)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var result []string
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}
