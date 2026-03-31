package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Progress represents the progress of an operation (init or apply)
type Progress struct {
	OperationID    string                    `json:"operation_id" yaml:"operation_id"`
	Type           string                    `json:"type" yaml:"type"` // "init" or "apply"
	StartedAt      time.Time                 `json:"started_at" yaml:"started_at"`
	UpdatedAt      time.Time                 `json:"updated_at" yaml:"updated_at"`
	Status         string                    `json:"status" yaml:"status"` // "pending", "in_progress", "completed", "failed", "cancelled"
	TotalSteps     int                       `json:"total_steps" yaml:"total_steps"`
	CompletedSteps int                       `json:"completed_steps" yaml:"completed_steps"`
	CurrentStep    int                       `json:"current_step" yaml:"current_step"`
	Modules        map[string]ModuleProgress `json:"modules" yaml:"modules"`
}

// ModuleProgress represents progress of a single module
type ModuleProgress struct {
	Name           string         `json:"name" yaml:"name"`
	Status         string         `json:"status" yaml:"status"`     // "pending", "in_progress", "completed", "failed", "skipped"
	Progress       int            `json:"progress" yaml:"progress"` // 0-100
	TotalSteps     int            `json:"total_steps" yaml:"total_steps"`
	CompletedSteps int            `json:"completed_steps" yaml:"completed_steps"`
	Steps          []StepProgress `json:"steps" yaml:"steps"`
	StartedAt      *time.Time     `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`
	ErrorMessage   string         `json:"error_message,omitempty" yaml:"error_message,omitempty"`
}

// StepProgress represents progress of a single step
type StepProgress struct {
	Name         string     `json:"name" yaml:"name"`
	Status       string     `json:"status" yaml:"status"` // "pending", "in_progress", "completed", "failed", "skipped"
	StartedAt    *time.Time `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`
	Duration     *string    `json:"duration,omitempty" yaml:"duration,omitempty"` // e.g., "10s", "2m30s"
	ErrorMessage string     `json:"error_message,omitempty" yaml:"error_message,omitempty"`
}

// GetProgressDir returns the path to the progress directory
func GetProgressDir() (string, error) {
	stateDir, err := GetStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "progress"), nil
}

// GetProgressPath returns the path to a progress file
func GetProgressPath(operationID string) (string, error) {
	progressDir, err := GetProgressDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(progressDir, operationID+".yaml"), nil
}

// EnsureProgressDir creates the progress directory if it doesn't exist
func EnsureProgressDir() error {
	progressDir, err := GetProgressDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(progressDir, 0o755)
}

// LoadProgress loads progress from file
func LoadProgress(operationID string) (*Progress, error) {
	progressPath, err := GetProgressPath(operationID)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(progressPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Progress file doesn't exist
		}
		return nil, fmt.Errorf("failed to read progress file: %w", err)
	}

	var progress Progress
	if err := yaml.Unmarshal(data, &progress); err != nil {
		return nil, fmt.Errorf("failed to parse progress file: %w", err)
	}

	return &progress, nil
}

// SaveProgress saves progress to file
func SaveProgress(progress *Progress) error {
	if err := EnsureProgressDir(); err != nil {
		return fmt.Errorf("failed to create progress directory: %w", err)
	}

	progressPath, err := GetProgressPath(progress.OperationID)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(progress)
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	if err := os.WriteFile(progressPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write progress file: %w", err)
	}

	return nil
}

// FindIncompleteProgress finds incomplete progress files
func FindIncompleteProgress() ([]*Progress, error) {
	progressDir, err := GetProgressDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(progressDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Progress{}, nil
		}
		return nil, fmt.Errorf("failed to read progress directory: %w", err)
	}

	var incomplete []*Progress
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		operationID := strings.TrimSuffix(entry.Name(), ".yaml")
		progress, err := LoadProgress(operationID)
		if err != nil {
			continue // Skip invalid files
		}

		if progress != nil && (progress.Status == "in_progress" || progress.Status == "pending") {
			incomplete = append(incomplete, progress)
		}
	}

	return incomplete, nil
}
