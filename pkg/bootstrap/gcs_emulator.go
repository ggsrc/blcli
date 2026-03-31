package bootstrap

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// GCSEmulator manages a local GCS emulator for testing
type GCSEmulator struct {
	cmd        *exec.Cmd
	port       int
	dataDir    string
	endpoint   string
	httpClient *http.Client
}

// NewGCSEmulator creates a new GCS emulator instance
func NewGCSEmulator(port int, dataDir string) *GCSEmulator {
	if port == 0 {
		port = 4443 // Default port for fake-gcs-server
	}
	if dataDir == "" {
		dataDir = filepath.Join(os.TempDir(), "blcli-gcs-emulator")
	}

	return &GCSEmulator{
		port:     port,
		dataDir:  dataDir,
		endpoint: fmt.Sprintf("http://localhost:%d", port),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Start starts the GCS emulator
func (e *GCSEmulator) Start(ctx context.Context) error {
	// Check if fake-gcs-server is available
	if !e.isAvailable() {
		return fmt.Errorf("fake-gcs-server not found. Install it with: go install github.com/fsouza/fake-gcs-server/fake-gcs-server@latest")
	}

	// Create data directory
	if err := os.MkdirAll(e.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create emulator data directory: %w", err)
	}

	// Start fake-gcs-server
	e.cmd = exec.CommandContext(ctx, "fake-gcs-server",
		"-backend", "filesystem",
		"-filesystem-root", e.dataDir,
		"-scheme", "http",
		"-host", "0.0.0.0",
		"-port", fmt.Sprintf("%d", e.port),
		"-public-host", "localhost",
	)

	e.cmd.Stdout = os.Stdout
	e.cmd.Stderr = os.Stderr

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GCS emulator: %w", err)
	}

	// Wait for emulator to be ready
	if err := e.waitForReady(ctx, 30*time.Second); err != nil {
		e.Stop()
		return fmt.Errorf("GCS emulator failed to start: %w", err)
	}

	return nil
}

// Stop stops the GCS emulator
func (e *GCSEmulator) Stop() error {
	if e.cmd != nil && e.cmd.Process != nil {
		return e.cmd.Process.Kill()
	}
	return nil
}

// Endpoint returns the emulator endpoint URL
func (e *GCSEmulator) Endpoint() string {
	return e.endpoint
}

// isAvailable checks if fake-gcs-server is installed
func (e *GCSEmulator) isAvailable() bool {
	_, err := exec.LookPath("fake-gcs-server")
	return err == nil
}

// waitForReady waits for the emulator to be ready
func (e *GCSEmulator) waitForReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for GCS emulator to be ready")
			}

			// Try to connect to the emulator
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", e.port), 1*time.Second)
			if err == nil {
				conn.Close()
				// Try a simple HTTP request
				resp, err := e.httpClient.Get(fmt.Sprintf("%s/storage/v1/b", e.endpoint))
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == 200 || resp.StatusCode == 404 {
						return nil // Emulator is ready
					}
				}
			}
		}
	}
}

// SetupTerraformBackend configures terraform to use the emulator
// This sets environment variables that terraform will use
func (e *GCSEmulator) SetupTerraformBackend() []string {
	// Set GCS emulator endpoint via environment variables
	// fake-gcs-server uses STORAGE_EMULATOR_HOST environment variable
	// This tells Google Cloud libraries to use the emulator instead of real GCS
	return []string{
		fmt.Sprintf("STORAGE_EMULATOR_HOST=localhost:%d", e.port),
	}
}
