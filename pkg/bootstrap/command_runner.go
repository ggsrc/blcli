package bootstrap

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type externalCommandOutputMode string

const (
	externalCommandStream           externalCommandOutputMode = "stream"
	externalCommandCapture          externalCommandOutputMode = "capture"
	externalCommandCaptureAndStream externalCommandOutputMode = "capture_and_stream"
)

type ExternalCommandSpec struct {
	Name string
	Args []string
	Dir  string
	Env  []string
}

type ExternalCommandResult struct {
	Name    string
	Command string
	Args    []string
	Dir     string
	Output  []byte
}

type externalCommandExecutorFunc func(ctx context.Context, spec ExternalCommandSpec, mode externalCommandOutputMode) (ExternalCommandResult, error)

var externalCommandExecutor externalCommandExecutorFunc = executeExternalCommand

func externalCommandString(name string, args []string) string {
	parts := append([]string{name}, args...)
	for i, part := range parts {
		parts[i] = shellQuoteArg(part)
	}
	return strings.Join(parts, " ")
}

func shellQuoteArg(arg string) string {
	if arg == "" {
		return "''"
	}
	if !strings.ContainsAny(arg, " \t\n'\"\\$`") {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
}

func runExternalCommandResult(ctx context.Context, spec ExternalCommandSpec, mode externalCommandOutputMode) (ExternalCommandResult, error) {
	return externalCommandExecutor(ctx, spec, mode)
}

func executeExternalCommand(ctx context.Context, spec ExternalCommandSpec, mode externalCommandOutputMode) (ExternalCommandResult, error) {
	result := ExternalCommandResult{
		Name:    spec.Name,
		Command: externalCommandString(spec.Name, spec.Args),
		Args:    append([]string(nil), spec.Args...),
		Dir:     spec.Dir,
	}

	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	if spec.Env != nil {
		cmd.Env = spec.Env
	}

	switch mode {
	case externalCommandCapture, externalCommandCaptureAndStream:
		var stdout, stderr bytes.Buffer
		if mode == externalCommandCaptureAndStream {
			cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
			cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
		} else {
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
		}

		err := cmd.Run()
		if err != nil {
			combinedOutput := append(stdout.Bytes(), stderr.Bytes()...)
			result.Output = combinedOutput
			if stderr.Len() > 0 {
				return result, fmt.Errorf("%w: %s", err, stderr.String())
			}
			return result, err
		}

		result.Output = stdout.Bytes()
		return result, nil
	default:
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return result, cmd.Run()
	}
}
