package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// BlcliMarkerFile is the name of the marker file that indicates a directory was created by blcli
	BlcliMarkerFile = ".blcli.marker"
)

// TerraformMarkerData is the structured content of .blcli.marker under workspace/terraform (for apply init order and project apply).
type TerraformMarkerData struct {
	InitPrepareDirs       []string            `yaml:"init_prepare_dirs"`        // Init items with prepare: true (e.g. 0-terraform-statestore)
	InitDirs              []string            `yaml:"init_dirs"`               // Init items with prepare: false
	DependencyOrder       []string            `yaml:"dependency_order"`         // Topological order of project/component (e.g. prd/vpc, corp/vpc-peering)
	SubdirComponents      map[string][]string `yaml:"subdir_components"`        // Project -> components rendered in subdir (to be promoted after first apply)
	SubdirComponentLayers map[string]int     `yaml:"subdir_component_layers"`  // "project/component" -> 1-based layer for promote round (dynamic apply)
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	return os.MkdirAll(path, 0o755)
}

// WriteFileIfAbsent writes a file only if it doesn't exist
func WriteFileIfAbsent(path, contents string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(contents), 0o644)
}

// WriteFile writes a file, overwriting if it already exists (used when init is run with --overwrite)
func WriteFile(path, contents string) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(contents), 0o644)
}

// CheckBlcliMarker checks if a directory was created by blcli by looking for the marker file
func CheckBlcliMarker(dir string) (bool, error) {
	markerPath := filepath.Join(dir, BlcliMarkerFile)
	_, err := os.Stat(markerPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// CreateBlcliMarker creates a marker file in the specified directory to indicate it was created by blcli
func CreateBlcliMarker(dir string) error {
	markerPath := filepath.Join(dir, BlcliMarkerFile)
	markerContent := "# This directory was created by blcli\n# Do not delete this file\n"
	return os.WriteFile(markerPath, []byte(markerContent), 0o644)
}

// WriteTerraformMarker writes the terraform directory marker with init plan (prepare dirs + init dirs) for apply init.
func WriteTerraformMarker(terraformDir string, prepareDirs, initDirs []string) error {
	return WriteTerraformMarkerWithDeps(terraformDir, prepareDirs, initDirs, nil, nil, nil)
}

// WriteTerraformMarkerWithDeps writes the terraform marker including project dependency order, subdir components, and their layers.
func WriteTerraformMarkerWithDeps(terraformDir string, prepareDirs, initDirs []string, dependencyOrder []string, subdirComponents map[string][]string, subdirComponentLayers map[string]int) error {
	markerPath := filepath.Join(terraformDir, BlcliMarkerFile)
	data := TerraformMarkerData{
		InitPrepareDirs:       prepareDirs,
		InitDirs:              initDirs,
		DependencyOrder:       dependencyOrder,
		SubdirComponents:      subdirComponents,
		SubdirComponentLayers: subdirComponentLayers,
	}
	body, err := yaml.Marshal(&data)
	if err != nil {
		return fmt.Errorf("marshal terraform marker: %w", err)
	}
	header := "# This directory was created by blcli\n# Do not delete this file\n"
	content := header + string(body)
	return os.WriteFile(markerPath, []byte(content), 0o644)
}

// ReadTerraformMarker reads the terraform marker and returns init plan if present.
// Returns (nil, nil) when the file does not exist or does not contain init plan (old format).
func ReadTerraformMarker(terraformDir string) (*TerraformMarkerData, error) {
	markerPath := filepath.Join(terraformDir, BlcliMarkerFile)
	raw, err := os.ReadFile(markerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	// Skip comment lines for YAML parse
	content := string(raw)
	if idx := strings.Index(content, "init_prepare_dirs"); idx >= 0 {
		content = content[idx:]
	} else {
		return nil, nil
	}
	var data TerraformMarkerData
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		return nil, fmt.Errorf("parse terraform marker: %w", err)
	}
	return &data, nil
}
