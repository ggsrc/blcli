package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromoteSubdirComponents(t *testing.T) {
	base := filepath.Join("workspace", "test-promote")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir base: %v", err)
	}
	defer os.RemoveAll(base)

	// Create corp/vpc-peering/vpc-peering.tf
	corpDir := filepath.Join(base, "corp")
	peeringSubdir := filepath.Join(corpDir, "vpc-peering")
	if err := os.MkdirAll(peeringSubdir, 0o755); err != nil {
		t.Fatalf("mkdir corp/vpc-peering: %v", err)
	}
	tfContent := "# test vpc peering"
	if err := os.WriteFile(filepath.Join(peeringSubdir, "vpc-peering.tf"), []byte(tfContent), 0o644); err != nil {
		t.Fatalf("write vpc-peering.tf: %v", err)
	}

	subdirComponents := map[string][]string{
		"corp": {"vpc-peering"},
	}
	projectsToApply := []string{"corp"}

	if err := promoteSubdirComponents(base, subdirComponents, projectsToApply); err != nil {
		t.Fatalf("promoteSubdirComponents: %v", err)
	}

	// After promote: corp/vpc-peering.tf should exist, corp/vpc-peering/ should be removed
	dstPath := filepath.Join(corpDir, "vpc-peering.tf")
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		t.Errorf("expected %s to exist after promote", dstPath)
	} else if err != nil {
		t.Errorf("stat %s: %v", dstPath, err)
	}
	if got, _ := os.ReadFile(dstPath); !strings.Contains(string(got), tfContent) {
		t.Errorf("vpc-peering.tf content = %q, want containing %q", string(got), tfContent)
	}
	if _, err := os.Stat(peeringSubdir); err == nil {
		t.Errorf("subdir %s should be removed after promote", peeringSubdir)
	}
}

func TestPromoteSubdirComponents_skipsProjectNotInList(t *testing.T) {
	base := filepath.Join("workspace", "test-promote-skip")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir base: %v", err)
	}
	defer os.RemoveAll(base)

	prdSubdir := filepath.Join(base, "prd", "vpc-peering")
	if err := os.MkdirAll(prdSubdir, 0o755); err != nil {
		t.Fatalf("mkdir prd/vpc-peering: %v", err)
	}
	if err := os.WriteFile(filepath.Join(prdSubdir, "vpc-peering.tf"), []byte("# prd"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	subdirComponents := map[string][]string{
		"prd": {"vpc-peering"},
	}
	// projectsToApply does not include prd -> prd should be skipped
	projectsToApply := []string{"corp"}

	if err := promoteSubdirComponents(base, subdirComponents, projectsToApply); err != nil {
		t.Fatalf("promoteSubdirComponents: %v", err)
	}

	// prd/vpc-peering/ should still exist (not promoted)
	if _, err := os.Stat(prdSubdir); os.IsNotExist(err) {
		t.Errorf("prd/vpc-peering should still exist when prd not in projectsToApply")
	}
}
