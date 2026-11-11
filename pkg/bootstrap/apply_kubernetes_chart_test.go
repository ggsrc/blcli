package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectHelmChartDoesNotInferRedis(t *testing.T) {
	if _, err := detectHelmChart(t.TempDir(), "redis", ""); err == nil {
		t.Fatal("expected an error for redis without explicit or local chart")
	}
}

func TestDetectHelmChartPrefersLocalChart(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("apiVersion: v2\nname: redis\nversion: 2.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chart, err := detectHelmChart(dir, "redis", "")
	if err != nil || chart != dir {
		t.Fatalf("got %q, %v; want local chart %q", chart, err, dir)
	}
}
