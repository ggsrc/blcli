package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTerraformMarker_ReadTerraformMarker_roundtrip(t *testing.T) {
	dir := filepath.Join("workspace", "test-marker")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	defer os.RemoveAll(dir)

	prepareDirs := []string{"init/0-terraform-statestore"}
	initDirs := []string{"init/1-my-org-projects", "init/2-atlantis"}

	if err := WriteTerraformMarker(dir, prepareDirs, initDirs); err != nil {
		t.Fatalf("WriteTerraformMarker: %v", err)
	}

	got, err := ReadTerraformMarker(dir)
	if err != nil {
		t.Fatalf("ReadTerraformMarker: %v", err)
	}
	if got == nil {
		t.Fatal("ReadTerraformMarker returned nil")
	}
	if len(got.InitPrepareDirs) != 1 || got.InitPrepareDirs[0] != "init/0-terraform-statestore" {
		t.Errorf("InitPrepareDirs = %v, want [init/0-terraform-statestore]", got.InitPrepareDirs)
	}
	if len(got.InitDirs) != 2 || got.InitDirs[0] != "init/1-my-org-projects" || got.InitDirs[1] != "init/2-atlantis" {
		t.Errorf("InitDirs = %v, want [init/1-my-org-projects init/2-atlantis]", got.InitDirs)
	}
}

func TestWriteTerraformMarkerWithDeps_ReadTerraformMarker_roundtrip(t *testing.T) {
	dir := filepath.Join("workspace", "test-marker-deps")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	defer os.RemoveAll(dir)

	prepareDirs := []string{"init/0-terraform-statestore"}
	initDirs := []string{"init/1-my-org-projects"}
	dependencyOrder := []string{"prd/vpc", "stg/vpc", "corp/vpc", "prd/vpc-peering", "stg/vpc-peering", "corp/vpc-peering"}
	subdirComponents := map[string][]string{
		"prd":  {"vpc-peering"},
		"stg":  {"vpc-peering"},
		"corp": {"vpc-peering"},
	}
	subdirComponentLayers := map[string]int{
		"prd/vpc-peering":  1,
		"stg/vpc-peering":  1,
		"corp/vpc-peering": 1,
	}

	if err := WriteTerraformMarkerWithDeps(dir, prepareDirs, initDirs, dependencyOrder, subdirComponents, subdirComponentLayers); err != nil {
		t.Fatalf("WriteTerraformMarkerWithDeps: %v", err)
	}

	got, err := ReadTerraformMarker(dir)
	if err != nil {
		t.Fatalf("ReadTerraformMarker: %v", err)
	}
	if got == nil {
		t.Fatal("ReadTerraformMarker returned nil")
	}
	if len(got.DependencyOrder) != len(dependencyOrder) {
		t.Errorf("DependencyOrder len = %d, want %d", len(got.DependencyOrder), len(dependencyOrder))
	}
	for i, s := range dependencyOrder {
		if i < len(got.DependencyOrder) && got.DependencyOrder[i] != s {
			t.Errorf("DependencyOrder[%d] = %q, want %q", i, got.DependencyOrder[i], s)
		}
	}
	if len(got.SubdirComponents) != len(subdirComponents) {
		t.Errorf("SubdirComponents len = %d, want %d", len(got.SubdirComponents), len(subdirComponents))
	}
	for proj, comps := range subdirComponents {
		gotComps := got.SubdirComponents[proj]
		if len(gotComps) != len(comps) || (len(comps) > 0 && gotComps[0] != comps[0]) {
			t.Errorf("SubdirComponents[%q] = %v, want %v", proj, gotComps, comps)
		}
	}
	for key, layer := range subdirComponentLayers {
		if got.SubdirComponentLayers == nil || got.SubdirComponentLayers[key] != layer {
			t.Errorf("SubdirComponentLayers[%q] = %v, want %d", key, got.SubdirComponentLayers[key], layer)
		}
	}
}

func TestReadTerraformMarker_noFile_returnsNilNil(t *testing.T) {
	got, err := ReadTerraformMarker("workspace/nonexistent-terraform-dir")
	if err != nil {
		t.Fatalf("ReadTerraformMarker: %v", err)
	}
	if got != nil {
		t.Errorf("ReadTerraformMarker returned %v, want nil", got)
	}
}

func TestReadTerraformMarker_oldFormat_returnsNilNil(t *testing.T) {
	dir := filepath.Join("workspace", "test-marker-old")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	defer os.RemoveAll(dir)
	if err := CreateBlcliMarker(dir); err != nil {
		t.Fatalf("CreateBlcliMarker: %v", err)
	}

	got, err := ReadTerraformMarker(dir)
	if err != nil {
		t.Fatalf("ReadTerraformMarker: %v", err)
	}
	if got != nil {
		t.Errorf("ReadTerraformMarker (old format) returned %v, want nil", got)
	}
}
