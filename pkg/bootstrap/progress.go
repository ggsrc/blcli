package bootstrap

import (
	"fmt"
	"os"
	"strings"
	"time"

	"blcli/pkg/state"
)

// ProgressTracker tracks and displays progress of operations
type ProgressTracker struct {
	progress *state.Progress
	quiet    bool // If true, don't print progress updates
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(operationID, operationType string, quiet bool) (*ProgressTracker, error) {
	now := time.Now()
	progress := &state.Progress{
		OperationID: operationID,
		Type:        operationType,
		StartedAt:   now,
		UpdatedAt:   now,
		Status:      "pending",
		Modules:     make(map[string]state.ModuleProgress),
	}

	tracker := &ProgressTracker{
		progress: progress,
		quiet:    quiet,
	}

	// Save initial progress
	if err := state.SaveProgress(progress); err != nil {
		return nil, fmt.Errorf("failed to save initial progress: %w", err)
	}

	return tracker, nil
}

// LoadProgressTracker loads an existing progress tracker
func LoadProgressTracker(operationID string, quiet bool) (*ProgressTracker, error) {
	progress, err := state.LoadProgress(operationID)
	if err != nil {
		return nil, fmt.Errorf("failed to load progress: %w", err)
	}
	if progress == nil {
		return nil, fmt.Errorf("progress not found: %s", operationID)
	}

	return &ProgressTracker{
		progress: progress,
		quiet:    quiet,
	}, nil
}

// StartOperation marks the operation as started
func (pt *ProgressTracker) StartOperation() error {
	pt.progress.Status = "in_progress"
	pt.progress.UpdatedAt = time.Now()
	return pt.save()
}

// CompleteOperation marks the operation as completed
func (pt *ProgressTracker) CompleteOperation() error {
	pt.progress.Status = "completed"
	pt.progress.UpdatedAt = time.Now()
	pt.progress.CompletedSteps = pt.progress.TotalSteps
	pt.progress.CurrentStep = pt.progress.TotalSteps
	return pt.save()
}

// FailOperation marks the operation as failed
func (pt *ProgressTracker) FailOperation(errorMsg string) error {
	pt.progress.Status = "failed"
	pt.progress.UpdatedAt = time.Now()
	return pt.save()
}

// StartModule starts tracking a module
func (pt *ProgressTracker) StartModule(moduleName string, totalSteps int) error {
	now := time.Now()
	moduleProgress := state.ModuleProgress{
		Name:           moduleName,
		Status:         "in_progress",
		Progress:       0,
		TotalSteps:     totalSteps,
		CompletedSteps: 0,
		Steps:          []state.StepProgress{},
		StartedAt:      &now,
	}

	pt.progress.Modules[moduleName] = moduleProgress
	pt.progress.UpdatedAt = time.Now()

	if !pt.quiet {
		pt.updateDisplay()
	}

	return pt.save()
}

// CompleteModule marks a module as completed
func (pt *ProgressTracker) CompleteModule(moduleName string) error {
	module, exists := pt.progress.Modules[moduleName]
	if !exists {
		return fmt.Errorf("module not found: %s", moduleName)
	}

	now := time.Now()
	module.Status = "completed"
	// Recalculate from actual steps
	module.TotalSteps = len(module.Steps)
	completedCount := 0
	for _, s := range module.Steps {
		if s.Status == "completed" {
			completedCount++
		}
	}
	module.CompletedSteps = completedCount
	module.Progress = 100
	module.CompletedAt = &now
	pt.progress.Modules[moduleName] = module

	// Recalculate overall steps from all modules
	totalCompleted := 0
	totalSteps := 0
	for _, m := range pt.progress.Modules {
		totalCompleted += m.CompletedSteps
		totalSteps += len(m.Steps)
	}
	pt.progress.CompletedSteps = totalCompleted
	pt.progress.TotalSteps = totalSteps
	pt.progress.UpdatedAt = time.Now()

	if !pt.quiet {
		pt.updateDisplay()
	}

	return pt.save()
}

// FailModule marks a module as failed
func (pt *ProgressTracker) FailModule(moduleName, errorMsg string) error {
	module, exists := pt.progress.Modules[moduleName]
	if !exists {
		return fmt.Errorf("module not found: %s", moduleName)
	}

	module.Status = "failed"
	module.ErrorMessage = errorMsg
	pt.progress.Modules[moduleName] = module

	pt.progress.UpdatedAt = time.Now()

	if !pt.quiet {
		pt.updateDisplay()
	}

	return pt.save()
}

// SkipModule marks a module as skipped
func (pt *ProgressTracker) SkipModule(moduleName string) error {
	module, exists := pt.progress.Modules[moduleName]
	if !exists {
		module = state.ModuleProgress{
			Name:   moduleName,
			Status: "skipped",
			Steps:  []state.StepProgress{},
		}
	} else {
		module.Status = "skipped"
	}

	pt.progress.Modules[moduleName] = module
	pt.progress.UpdatedAt = time.Now()

	if !pt.quiet {
		pt.updateDisplay()
	}

	return pt.save()
}

// StartStep starts tracking a step within a module
func (pt *ProgressTracker) StartStep(moduleName, stepName string) error {
	module, exists := pt.progress.Modules[moduleName]
	if !exists {
		// Create module if it doesn't exist
		module = state.ModuleProgress{
			Name:   moduleName,
			Status: "in_progress",
			Steps:  []state.StepProgress{},
		}
	}

	now := time.Now()
	step := state.StepProgress{
		Name:      stepName,
		Status:    "in_progress",
		StartedAt: &now,
	}

	// Check if step already exists (for resume)
	stepIndex := -1
	for i, s := range module.Steps {
		if s.Name == stepName {
			stepIndex = i
			break
		}
	}

	if stepIndex >= 0 {
		// Update existing step (resume scenario)
		module.Steps[stepIndex] = step
	} else {
		// Add new step
		module.Steps = append(module.Steps, step)
		module.TotalSteps = len(module.Steps) // Update total steps to match actual steps
		// Recalculate total steps from all modules
		totalSteps := 0
		for _, m := range pt.progress.Modules {
			totalSteps += len(m.Steps) // Use actual step count
		}
		pt.progress.TotalSteps = totalSteps
	}

	module.Status = "in_progress"
	pt.progress.Modules[moduleName] = module
	pt.progress.CurrentStep = pt.progress.CompletedSteps + 1
	pt.progress.UpdatedAt = time.Now()

	if !pt.quiet {
		pt.updateDisplay()
	}

	return pt.save()
}

// CompleteStep marks a step as completed
func (pt *ProgressTracker) CompleteStep(moduleName, stepName string) error {
	module, exists := pt.progress.Modules[moduleName]
	if !exists {
		return fmt.Errorf("module not found: %s", moduleName)
	}

	// Find step
	stepIndex := -1
	for i, s := range module.Steps {
		if s.Name == stepName {
			stepIndex = i
			break
		}
	}

	if stepIndex < 0 {
		return fmt.Errorf("step not found: %s in module %s", stepName, moduleName)
	}

	step := module.Steps[stepIndex]
	now := time.Now()
	step.Status = "completed"
	step.CompletedAt = &now

	// Calculate duration
	if step.StartedAt != nil {
		duration := now.Sub(*step.StartedAt)
		durationStr := formatDuration(duration)
		step.Duration = &durationStr
	}

	module.Steps[stepIndex] = step
	// Recalculate completed steps from actual step status
	completedCount := 0
	for _, s := range module.Steps {
		if s.Status == "completed" {
			completedCount++
		}
	}
	module.CompletedSteps = completedCount
	module.TotalSteps = len(module.Steps) // Ensure TotalSteps matches actual steps
	if module.TotalSteps > 0 {
		module.Progress = int(float64(module.CompletedSteps) / float64(module.TotalSteps) * 100)
		if module.Progress > 100 {
			module.Progress = 100
		}
	}
	pt.progress.Modules[moduleName] = module

	// Recalculate overall completed steps from all modules
	totalCompleted := 0
	totalSteps := 0
	for _, m := range pt.progress.Modules {
		totalCompleted += m.CompletedSteps
		totalSteps += len(m.Steps)
	}
	pt.progress.CompletedSteps = totalCompleted
	pt.progress.TotalSteps = totalSteps
	pt.progress.UpdatedAt = time.Now()

	if !pt.quiet {
		pt.updateDisplay()
	}

	return pt.save()
}

// FailStep marks a step as failed
func (pt *ProgressTracker) FailStep(moduleName, stepName, errorMsg string) error {
	module, exists := pt.progress.Modules[moduleName]
	if !exists {
		return fmt.Errorf("module not found: %s", moduleName)
	}

	// Find step
	stepIndex := -1
	for i, s := range module.Steps {
		if s.Name == stepName {
			stepIndex = i
			break
		}
	}

	if stepIndex < 0 {
		return fmt.Errorf("step not found: %s in module %s", stepName, moduleName)
	}

	step := module.Steps[stepIndex]
	now := time.Now()
	step.Status = "failed"
	step.CompletedAt = &now
	step.ErrorMessage = errorMsg

	if step.StartedAt != nil {
		duration := now.Sub(*step.StartedAt)
		durationStr := formatDuration(duration)
		step.Duration = &durationStr
	}

	module.Steps[stepIndex] = step
	pt.progress.Modules[moduleName] = module

	pt.progress.UpdatedAt = time.Now()

	if !pt.quiet {
		pt.updateDisplay()
	}

	return pt.save()
}

// SkipStep marks a step as skipped
func (pt *ProgressTracker) SkipStep(moduleName, stepName string) error {
	module, exists := pt.progress.Modules[moduleName]
	if !exists {
		return fmt.Errorf("module not found: %s", moduleName)
	}

	// Find step
	stepIndex := -1
	for i, s := range module.Steps {
		if s.Name == stepName {
			stepIndex = i
			break
		}
	}

	if stepIndex < 0 {
		// Add new skipped step
		step := state.StepProgress{
			Name:   stepName,
			Status: "skipped",
		}
		module.Steps = append(module.Steps, step)
		module.TotalSteps++
		pt.progress.TotalSteps++
	} else {
		step := module.Steps[stepIndex]
		step.Status = "skipped"
		module.Steps[stepIndex] = step
	}

	pt.progress.Modules[moduleName] = module
	pt.progress.UpdatedAt = time.Now()

	if !pt.quiet {
		pt.updateDisplay()
	}

	return pt.save()
}

// GetProgress returns the current progress
func (pt *ProgressTracker) GetProgress() *state.Progress {
	return pt.progress
}

// save saves progress to file
func (pt *ProgressTracker) save() error {
	return state.SaveProgress(pt.progress)
}

// updateDisplay updates the progress display on console
func (pt *ProgressTracker) updateDisplay() {
	// Clear previous line and print updated progress
	fmt.Print("\r\033[K") // Move cursor to beginning of line and clear to end

	// Calculate overall progress
	overallProgress := 0
	if pt.progress.TotalSteps > 0 {
		overallProgress = int(float64(pt.progress.CompletedSteps) / float64(pt.progress.TotalSteps) * 100)
	}

	// Build progress bar
	barWidth := 30
	var filled int
	if pt.progress.TotalSteps > 0 {
		filled = int(float64(barWidth) * float64(pt.progress.CompletedSteps) / float64(pt.progress.TotalSteps))
		if filled > barWidth {
			filled = barWidth
		}
		if filled < 0 {
			filled = 0
		}
	} else {
		filled = 0
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// Print progress
	typeStr := pt.progress.Type
	if len(typeStr) > 0 {
		typeStr = strings.ToUpper(typeStr[:1]) + typeStr[1:]
	}
	fmt.Printf("🚀 %s... [%s] %d%% (%d/%d steps)",
		typeStr,
		bar,
		overallProgress,
		pt.progress.CompletedSteps,
		pt.progress.TotalSteps)

	// Print current module/step if available
	if pt.progress.CurrentStep > 0 && pt.progress.CurrentStep <= pt.progress.TotalSteps {
		// Find current module and step
		for moduleName, module := range pt.progress.Modules {
			if module.Status == "in_progress" {
				for _, step := range module.Steps {
					if step.Status == "in_progress" {
						fmt.Printf(" | Current: %s/%s", moduleName, step.Name)
						break
					}
				}
				break
			}
		}
	}

	os.Stdout.Sync()
}

// PrintSummary prints a summary of the progress
func (pt *ProgressTracker) PrintSummary() {
	fmt.Println() // New line after progress bar
	typeStr := pt.progress.Type
	if len(typeStr) > 0 {
		typeStr = strings.ToUpper(typeStr[:1]) + typeStr[1:]
	}
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📊 %s Summary\n", typeStr)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Overall status
	statusIcon := getProgressStatusIcon(pt.progress.Status)
	fmt.Printf("Status: %s %s\n", statusIcon, pt.progress.Status)
	fmt.Printf("Total Steps: %d | Completed: %d\n\n", pt.progress.TotalSteps, pt.progress.CompletedSteps)

	// Module details
	for moduleName, module := range pt.progress.Modules {
		moduleIcon := getProgressStatusIcon(module.Status)
		fmt.Printf("%s %s (%d%%)\n", moduleIcon, moduleName, module.Progress)

		for _, step := range module.Steps {
			stepIcon := getProgressStatusIcon(step.Status)
			fmt.Printf("  %s %s", stepIcon, step.Name)
			if step.Duration != nil {
				fmt.Printf(" (%s)", *step.Duration)
			}
			if step.ErrorMessage != "" {
				fmt.Printf(" - Error: %s", step.ErrorMessage)
			}
			fmt.Println()
		}
		fmt.Println()
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// formatDuration formats a duration as a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	} else if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		if seconds > 0 {
			return fmt.Sprintf("%dm%ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes > 0 {
			return fmt.Sprintf("%dh%dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
}

// getProgressStatusIcon returns an icon for progress status
func getProgressStatusIcon(status string) string {
	switch status {
	case "completed":
		return "✅"
	case "in_progress":
		return "🔄"
	case "failed":
		return "❌"
	case "skipped":
		return "⏭️"
	case "pending":
		return "⏸"
	default:
		return "❓"
	}
}

// GenerateOperationID generates a unique operation ID
func GenerateOperationID(operationType string) string {
	timestamp := time.Now().Format("20060102-150405")
	// Simple hash of timestamp for uniqueness (in production, could use UUID)
	return fmt.Sprintf("op-%s-%s", timestamp, operationType[:3])
}
