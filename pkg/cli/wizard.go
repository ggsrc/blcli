package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// wizardAnswers holds interactive init-args wizard responses.
type wizardAnswers struct {
	Org              string
	Profile          string
	OutputPath       string
	Workspace        string
	BillingAccountID string
	OrganizationID   string
	Domain           string
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func promptLine(label, defaultValue string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", label, defaultValue)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}

func promptYesNo(label string, defaultYes bool) (bool, error) {
	defaultLabel := "y/N"
	if defaultYes {
		defaultLabel = "Y/n"
	}
	answer, err := promptLine(fmt.Sprintf("%s (%s)", label, defaultLabel), "")
	if err != nil {
		return false, err
	}
	if answer == "" {
		return defaultYes, nil
	}
	switch strings.ToLower(answer) {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return defaultYes, nil
	}
}

// runInitArgsWizard collects required fields for a starter args.yaml.
func runInitArgsWizard(defaults wizardAnswers) (wizardAnswers, error) {
	if !isInteractive() {
		return defaults, fmt.Errorf("wizard requires an interactive terminal (stdin is not a TTY)")
	}

	fmt.Println("blcli init-args wizard — answer a few questions to generate args.yaml.")
	fmt.Println()

	var err error
	answers := defaults

	answers.Org, err = promptLine("Organization short name (used in GCP project IDs)", defaults.Org)
	if err != nil {
		return answers, err
	}
	answers.Profile, err = promptLine("Template profile (minimal or full)", defaults.Profile)
	if err != nil {
		return answers, err
	}
	answers.OutputPath, err = promptLine("Output args file path", defaults.OutputPath)
	if err != nil {
		return answers, err
	}
	answers.Workspace, err = promptLine("Workspace directory for generated infra", defaults.Workspace)
	if err != nil {
		return answers, err
	}
	answers.OrganizationID, err = promptLine("GCP Organization ID (use 0 for personal account)", defaults.OrganizationID)
	if err != nil {
		return answers, err
	}
	answers.BillingAccountID, err = promptLine("GCP Billing Account ID (01XXXX-YYYYYY-ZZZZZZ)", defaults.BillingAccountID)
	if err != nil {
		return answers, err
	}
	answers.Domain, err = promptLine("Primary domain (optional)", defaults.Domain)
	if err != nil {
		return answers, err
	}

	return answers, nil
}

func confirmWritePreview() (bool, error) {
	return promptYesNo("Write this configuration to disk?", true)
}

func applyWizardOverrides(cfg *ArgsConfig, w wizardAnswers) {
	if cfg.Global == nil {
		cfg.Global = make(map[string]interface{})
	}
	if cfg.Terraform.Global == nil {
		cfg.Terraform.Global = make(map[string]interface{})
	}
	if w.Workspace != "" {
		cfg.Global["workspace"] = w.Workspace
	}
	if w.Domain != "" {
		cfg.Global["domain"] = w.Domain
	}
	if w.OrganizationID != "" {
		cfg.Terraform.Global["OrganizationID"] = w.OrganizationID
	}
	if w.BillingAccountID != "" {
		cfg.Terraform.Global["BillingAccountID"] = w.BillingAccountID
	}
}
