package bootstrap

import (
	"fmt"
	"strings"
)

type progressStepLogger struct {
	tracker  *ProgressTracker
	module   string
	step     string
	location string
}

func startProgressStep(progressTracker *ProgressTracker, moduleName, stepName, command, location string) progressStepLogger {
	logger := progressStepLogger{
		tracker:  progressTracker,
		module:   moduleName,
		step:     stepName,
		location: location,
	}
	if progressTracker != nil {
		progressTracker.StartStepWithCommand(moduleName, stepName, command)
	}
	return logger
}

func (l progressStepLogger) recordExternal(result ExternalCommandResult) {
	if l.tracker == nil {
		return
	}
	l.recordCommandOutput(result.Command, result.Output)
}

func (l progressStepLogger) recordTerraform(result TerraformCommandResult) {
	if l.tracker == nil {
		return
	}
	l.recordCommandOutput(result.Command, result.Output)
}

func (l progressStepLogger) recordCommandOutput(command string, outputBytes []byte) {
	if l.tracker == nil {
		return
	}

	output := strings.TrimSpace(string(outputBytes))
	if output == "" {
		output = "(no captured output)"
	}
	l.tracker.RecordStepOutput(l.module, l.step, fmt.Sprintf("$ %s\n%s", command, output))
}

func (l progressStepLogger) fail(err error, location string) {
	if l.tracker == nil || err == nil {
		return
	}
	if location == "" {
		location = l.location
	}
	l.tracker.FailStepWithContext(l.module, l.step, fmt.Sprintf("%v", err), location)
}

func (l progressStepLogger) complete() {
	if l.tracker == nil {
		return
	}
	l.tracker.CompleteStep(l.module, l.step)
}
