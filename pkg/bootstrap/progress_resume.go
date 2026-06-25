package bootstrap

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"blcli/pkg/state"
)

// progressResumeOptions configures incomplete-operation resume handling.
type progressResumeOptions struct {
	OperationType string
	Quiet         bool
	NoResume      bool
	Stdin         io.Reader
	Stdout        io.Writer
}

// ResolveProgressTracker loads an in-progress operation when the user agrees to resume,
// or starts a new tracker otherwise.
func ResolveProgressTracker(opts progressResumeOptions) (*ProgressTracker, error) {
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}

	incomplete, err := state.FindIncompleteProgress()
	if err != nil {
		return nil, fmt.Errorf("failed to check for incomplete operations: %w", err)
	}

	var candidates []*state.Progress
	for _, p := range incomplete {
		if p != nil && p.Type == opts.OperationType {
			candidates = append(candidates, p)
		}
	}

	if len(candidates) == 0 || opts.NoResume {
		return newProgressTrackerForOperation(opts.OperationType, opts.Quiet)
	}

	latest := latestIncompleteProgress(candidates)
	if latest == nil {
		return newProgressTrackerForOperation(opts.OperationType, opts.Quiet)
	}

	if !opts.Quiet {
		fmt.Fprintf(opts.Stdout, "\nFound incomplete %s operation: %s (started %s)\n",
			opts.OperationType, latest.OperationID, latest.StartedAt.Format(time.RFC3339))
		fmt.Fprintf(opts.Stdout, "Resume this operation and skip completed modules? [y/N]: ")
	}

	answer, err := readYesNo(opts.Stdin, false)
	if err != nil {
		return nil, err
	}
	if !answer {
		if !opts.Quiet {
			fmt.Fprintln(opts.Stdout, "Starting a new operation.")
		}
		return newProgressTrackerForOperation(opts.OperationType, opts.Quiet)
	}

	tracker, err := LoadProgressTracker(latest.OperationID, opts.Quiet)
	if err != nil {
		return nil, fmt.Errorf("failed to resume operation %s: %w", latest.OperationID, err)
	}
	if err := tracker.StartOperation(); err != nil {
		return nil, fmt.Errorf("failed to resume operation %s: %w", latest.OperationID, err)
	}
	if !opts.Quiet {
		fmt.Fprintf(opts.Stdout, "Resuming operation %s\n\n", latest.OperationID)
	}
	return tracker, nil
}

func newProgressTrackerForOperation(operationType string, quiet bool) (*ProgressTracker, error) {
	operationID := GenerateOperationID(operationType)
	tracker, err := NewProgressTracker(operationID, operationType, quiet)
	if err != nil {
		return nil, err
	}
	if err := tracker.StartOperation(); err != nil {
		return nil, err
	}
	return tracker, nil
}

func latestIncompleteProgress(progresses []*state.Progress) *state.Progress {
	var latest *state.Progress
	for _, p := range progresses {
		if p == nil {
			continue
		}
		if latest == nil || p.UpdatedAt.After(latest.UpdatedAt) {
			latest = p
		}
	}
	return latest
}

func readYesNo(r io.Reader, defaultYes bool) (bool, error) {
	reader, ok := r.(*os.File)
	if ok {
		info, err := reader.Stat()
		if err == nil && (info.Mode()&os.ModeCharDevice) == 0 {
			return defaultYes, nil
		}
	}

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, err
		}
		return defaultYes, nil
	}

	switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
	case "y", "yes":
		return true, nil
	case "n", "no", "":
		return false, nil
	default:
		return false, nil
	}
}

// ModuleAlreadyCompleted reports whether a module finished or was skipped in the tracker.
func ModuleAlreadyCompleted(tracker *ProgressTracker, moduleName string) bool {
	if tracker == nil || tracker.progress == nil {
		return false
	}
	module, ok := tracker.progress.Modules[moduleName]
	if !ok {
		return false
	}
	return module.Status == "completed" || module.Status == "skipped"
}
