package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"blcli/pkg/internal"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

const kubernetesInitScriptPath = "kubernetes/init.sh"

func runKubernetesInitScript(templateLoader *template.Loader, templateArgs renderer.ArgsData, workspace, kubernetesDir string, overwrite bool) error {
	if templateLoader == nil || !templateLoader.CacheExists(kubernetesInitScriptPath) {
		return nil
	}

	absKubernetesDir, err := filepath.Abs(kubernetesDir)
	if err != nil {
		return fmt.Errorf("resolve kubernetes dir: %w", err)
	}

	scriptContent, err := templateLoader.LoadTemplate(kubernetesInitScriptPath)
	if err != nil {
		return fmt.Errorf("load kubernetes init script: %w", err)
	}

	rendered, err := template.RenderWithArgs(scriptContent, map[string]interface{}{}, templateArgs)
	if err != nil {
		return fmt.Errorf("render kubernetes init script: %w", err)
	}

	scriptPath := filepath.Join(absKubernetesDir, "init.sh")
	if overwrite {
		if err := internal.WriteFile(scriptPath, rendered); err != nil {
			return fmt.Errorf("write kubernetes init script: %w", err)
		}
	} else {
		if err := internal.WriteFileIfAbsent(scriptPath, rendered); err != nil {
			return fmt.Errorf("write kubernetes init script: %w", err)
		}
	}

	if err := os.Chmod(scriptPath, 0o755); err != nil {
		return fmt.Errorf("chmod kubernetes init script: %w", err)
	}

	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = absKubernetesDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("BLCLI_WORKSPACE=%s", workspace),
		fmt.Sprintf("BLCLI_KUBERNETES_DIR=%s", absKubernetesDir),
		fmt.Sprintf("BLCLI_KEYS_DIR=%s", filepath.Join(absKubernetesDir, "keys")),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("execute kubernetes init script: %w", err)
	}

	return nil
}
