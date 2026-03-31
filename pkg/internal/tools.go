package internal

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// InstallHint returns platform-specific install command for a tool (empty if not defined).
// Commands are from official docs so users can copy-paste when the tool is not installed.
func InstallHint(tool string) string {
	osType := runtime.GOOS
	hints := map[string]map[string]string{
		"terraform": {
			"darwin": "brew tap hashicorp/tap && brew install hashicorp/tap/terraform",
			"linux":  "wget -qO- https://apt.releases.hashicorp.com/gpg | sudo gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg && echo \"deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main\" | sudo tee /etc/apt/sources.list.d/hashicorp.list > /dev/null && sudo apt update && sudo apt install -y terraform",
		},
		"kubectl": {
			"darwin": "brew install kubectl",
			"linux":  "curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.29/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg && echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.29/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list && sudo apt-get update && sudo apt-get install -y kubectl",
		},
		"argocd": {
			"darwin": "brew install argocd",
			"linux":  "curl -sSL -o argocd https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64 && chmod +x argocd && sudo mv argocd /usr/local/bin/argocd",
		},
		"gh": {
			"darwin": "brew install gh",
			"linux":  "sudo apt update && sudo apt install -y gh",
		},
		"istioctl": {
			"darwin": "brew install istioctl",
			"linux":  "curl -sL https://istio.io/downloadIstioctl | sh - && sudo mv $HOME/.istioctl/bin/istioctl /usr/local/bin/",
		},
	}
	if m, ok := hints[tool]; ok {
		if h, ok := m[osType]; ok {
			return h
		}
	}
	return ""
}

// platformLabel returns human-readable label for current OS (for install hints).
func platformLabel() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	default:
		return runtime.GOOS
	}
}

// CheckTools checks required and suggested external tools, with platform-specific install hints.
func CheckTools() {
	fmt.Println("Checking required and suggested tools...")
	fmt.Printf("当前平台: %s\n\n", platformLabel())

	// 必须安装 (Required): terraform, kubectl
	fmt.Println("必须安装 (Required):")
	requiredOK := true
	for _, name := range []string{"terraform", "kubectl"} {
		if ok, line := checkTool(name, toolVersionArgs(name)...); ok {
			fmt.Printf("  ✓ %s: %s\n", name, line)
		} else {
			fmt.Printf("  ✗ %s: not found\n", name)
			requiredOK = false
			if h := InstallHint(name); h != "" {
				fmt.Printf("    安装命令: %s\n", h)
			}
		}
	}
	fmt.Println()

	// 建议安装 (Suggested): argocd, gh, istioctl
	fmt.Println("建议安装 (Suggested):")
	for _, name := range []string{"argocd", "gh", "istioctl"} {
		if ok, line := checkTool(name, toolVersionArgs(name)...); ok {
			fmt.Printf("  ✓ %s: %s\n", name, line)
		} else {
			fmt.Printf("  ○ %s: not found\n", name)
			if h := InstallHint(name); h != "" {
				fmt.Printf("    安装命令: %s\n", h)
			}
		}
	}
	fmt.Println()

	if requiredOK {
		fmt.Println("All required tools are installed.")
	} else {
		fmt.Println("Some required tools are missing. Please install them before using blcli.")
	}
}

func toolVersionArgs(name string) []string {
	switch name {
	case "terraform":
		return []string{"version"}
	case "kubectl":
		return []string{"version", "--client"}
	case "argocd":
		return []string{"version", "--client"}
	case "gh":
		return []string{"--version"}
	case "istioctl":
		return []string{"version", "--short"}
	default:
		return []string{"--version"}
	}
}

// CheckAndInstallTools checks if tools are installed and installs them if missing
func CheckAndInstallTools(cfg ToolConfig) error {
	needsTerraform := false
	needsKubectl := false

	// Check terraform
	if !isToolInstalled("terraform") {
		needsTerraform = true
		fmt.Println("  ✗ terraform: not found, will install")
	} else {
		fmt.Println("  ✓ terraform: already installed")
	}

	// Check kubectl
	if !isToolInstalled("kubectl") {
		needsKubectl = true
		fmt.Println("  ✗ kubectl: not found, will install")
	} else {
		fmt.Println("  ✓ kubectl: already installed")
	}

	if needsTerraform {
		fmt.Println("\nInstalling terraform...")
		if err := installTerraform(cfg.TerraformVersion); err != nil {
			return fmt.Errorf("failed to install terraform: %w", err)
		}
		fmt.Println("  ✓ terraform: installed successfully")
	}

	if needsKubectl {
		fmt.Println("\nInstalling kubectl...")
		if err := installKubectl(cfg.KubectlVersion); err != nil {
			return fmt.Errorf("failed to install kubectl: %w", err)
		}
		fmt.Println("  ✓ kubectl: installed successfully")
	}

	return nil
}

func isToolInstalled(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func checkTool(name string, args ...string) (bool, string) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return false, ""
	}
	line := ""
	for _, l := range splitLines(string(out)) {
		if l != "" {
			line = l
			break
		}
	}
	return true, line
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func installTerraform(version string) error {
	if version == "" || version == "latest" {
		version = "latest"
	}

	osType := runtime.GOOS
	arch := runtime.GOARCH

	// Map Go arch to Terraform arch
	terraformArch := arch
	if arch == "amd64" {
		terraformArch = "amd64"
	} else if arch == "arm64" {
		terraformArch = "arm64"
	}

	var downloadURL string
	var binaryName string

	switch osType {
	case "darwin":
		if terraformArch == "arm64" {
			downloadURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_darwin_arm64.zip", version, version)
		} else {
			downloadURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_darwin_amd64.zip", version, version)
		}
		binaryName = "terraform"
	case "linux":
		downloadURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_linux_%s.zip", version, version, terraformArch)
		binaryName = "terraform"
	case "windows":
		downloadURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_windows_%s.zip", version, version, terraformArch)
		binaryName = "terraform.exe"
	default:
		return fmt.Errorf("unsupported OS: %s", osType)
	}

	// For latest version, we need to get the actual version number
	if version == "latest" {
		actualVersion, err := getLatestTerraformVersion()
		if err != nil {
			return fmt.Errorf("failed to get latest terraform version: %w", err)
		}
		version = actualVersion
		// Rebuild URL with actual version
		switch osType {
		case "darwin":
			if terraformArch == "arm64" {
				downloadURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_darwin_arm64.zip", version, version)
			} else {
				downloadURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_darwin_amd64.zip", version, version)
			}
		case "linux":
			downloadURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_linux_%s.zip", version, version, terraformArch)
		case "windows":
			downloadURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_windows_%s.zip", version, version, terraformArch)
		}
	}

	return downloadAndExtractZip(downloadURL, binaryName, "terraform")
}

func installKubectl(version string) error {
	if version == "" || version == "latest" {
		version = "latest"
	}

	osType := runtime.GOOS
	arch := runtime.GOARCH

	var downloadURL string
	var binaryName string

	// Map Go arch to kubectl arch
	kubectlArch := arch
	if arch == "amd64" {
		kubectlArch = "amd64"
	} else if arch == "arm64" {
		kubectlArch = "arm64"
	}

	switch osType {
	case "darwin":
		if kubectlArch == "arm64" {
			downloadURL = fmt.Sprintf("https://dl.k8s.io/release/%s/bin/darwin/arm64/kubectl", version)
		} else {
			downloadURL = fmt.Sprintf("https://dl.k8s.io/release/%s/bin/darwin/amd64/kubectl", version)
		}
		binaryName = "kubectl"
	case "linux":
		downloadURL = fmt.Sprintf("https://dl.k8s.io/release/%s/bin/linux/%s/kubectl", version, kubectlArch)
		binaryName = "kubectl"
	case "windows":
		downloadURL = fmt.Sprintf("https://dl.k8s.io/release/%s/bin/windows/%s/kubectl.exe", version, kubectlArch)
		binaryName = "kubectl.exe"
	default:
		return fmt.Errorf("unsupported OS: %s", osType)
	}

	// For latest version, get the stable version
	if version == "latest" {
		stableVersion, err := getLatestKubectlVersion()
		if err != nil {
			return fmt.Errorf("failed to get latest kubectl version: %w", err)
		}
		version = stableVersion
		// Rebuild URL with actual version (version already includes 'v' prefix)
		switch osType {
		case "darwin":
			if kubectlArch == "arm64" {
				downloadURL = fmt.Sprintf("https://dl.k8s.io/release/%s/bin/darwin/arm64/kubectl", version)
			} else {
				downloadURL = fmt.Sprintf("https://dl.k8s.io/release/%s/bin/darwin/amd64/kubectl", version)
			}
		case "linux":
			downloadURL = fmt.Sprintf("https://dl.k8s.io/release/%s/bin/linux/%s/kubectl", version, kubectlArch)
		case "windows":
			downloadURL = fmt.Sprintf("https://dl.k8s.io/release/%s/bin/windows/%s/kubectl.exe", version, kubectlArch)
		}
	} else {
		// Ensure version has 'v' prefix for kubectl
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}
		// Rebuild URL with version
		switch osType {
		case "darwin":
			if kubectlArch == "arm64" {
				downloadURL = fmt.Sprintf("https://dl.k8s.io/release/%s/bin/darwin/arm64/kubectl", version)
			} else {
				downloadURL = fmt.Sprintf("https://dl.k8s.io/release/%s/bin/darwin/amd64/kubectl", version)
			}
		case "linux":
			downloadURL = fmt.Sprintf("https://dl.k8s.io/release/%s/bin/linux/%s/kubectl", version, kubectlArch)
		case "windows":
			downloadURL = fmt.Sprintf("https://dl.k8s.io/release/%s/bin/windows/%s/kubectl.exe", version, kubectlArch)
		}
	}

	return downloadBinary(downloadURL, binaryName, "kubectl")
}

func getLatestTerraformVersion() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/hashicorp/terraform/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Simple JSON parsing - look for "tag_name"
	content := string(body)
	start := strings.Index(content, `"tag_name":`)
	if start == -1 {
		return "", fmt.Errorf("could not find tag_name in response")
	}
	start += len(`"tag_name":`)
	start = strings.Index(content[start:], `"`) + start + 1
	end := strings.Index(content[start:], `"`) + start

	version := content[start:end]
	version = strings.TrimPrefix(version, "v")
	return version, nil
}

func getLatestKubectlVersion() (string, error) {
	resp, err := http.Get("https://dl.k8s.io/release/stable.txt")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	version := strings.TrimSpace(string(body))
	// Keep 'v' prefix for kubectl URLs
	return version, nil
}

func downloadAndExtractZip(url, binaryName, toolName string) error {
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("blcli-%s-*", toolName))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "download.zip")
	if err := downloadFile(url, zipPath); err != nil {
		return err
	}

	// Extract zip
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	var binaryData []byte
	for _, f := range r.File {
		if f.Name == binaryName {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			binaryData, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return err
			}
			break
		}
	}

	if binaryData == nil {
		return fmt.Errorf("binary %s not found in zip", binaryName)
	}

	return installBinary(binaryData, binaryName, toolName)
}

func downloadBinary(url, binaryName, toolName string) error {
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("blcli-%s-*", toolName))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := filepath.Join(tmpDir, binaryName)
	if err := downloadFile(url, binaryPath); err != nil {
		return err
	}

	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		return err
	}

	return installBinary(binaryData, binaryName, toolName)
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func installBinary(data []byte, binaryName, toolName string) error {
	// Determine install location
	var installPath string
	path := os.Getenv("PATH")
	paths := strings.Split(path, string(os.PathListSeparator))

	// Try common locations
	candidates := []string{
		"/usr/local/bin",
		"/usr/bin",
		filepath.Join(os.Getenv("HOME"), "bin"),
		filepath.Join(os.Getenv("HOME"), ".local", "bin"),
	}

	// Add PATH directories
	for _, p := range paths {
		if p != "" {
			candidates = append(candidates, p)
		}
	}

	// Find first writable directory
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			if testFile := filepath.Join(candidate, ".blcli-test"); os.WriteFile(testFile, []byte("test"), 0644) == nil {
				os.Remove(testFile)
				installPath = filepath.Join(candidate, binaryName)
				break
			}
		}
	}

	if installPath == "" {
		// Fallback to user's home bin
		homeBin := filepath.Join(os.Getenv("HOME"), "bin")
		if err := os.MkdirAll(homeBin, 0755); err == nil {
			installPath = filepath.Join(homeBin, binaryName)
		} else {
			return fmt.Errorf("could not find writable directory to install %s", toolName)
		}
	}

	if err := os.WriteFile(installPath, data, 0755); err != nil {
		return err
	}

	fmt.Printf("  Installed %s to %s\n", toolName, installPath)
	return nil
}
