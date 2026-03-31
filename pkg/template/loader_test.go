package template

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseRepoURL(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOwner  string
		wantRepo   string
		wantBranch string
		wantErr    bool
	}{
		// SSH format
		{
			name:       "SSH format with .git",
			input:      "git@github.com:ggsrc/bl-template.git",
			wantOwner:  "ggsrc",
			wantRepo:   "bl-template",
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name:       "SSH format with .git and branch",
			input:      "git@github.com:ggsrc/bl-template.git@main",
			wantOwner:  "ggsrc",
			wantRepo:   "bl-template",
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name:       "SSH format without .git",
			input:      "git@github.com:ggsrc/bl-template",
			wantOwner:  "ggsrc",
			wantRepo:   "bl-template",
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name:       "SSH format without .git with branch",
			input:      "git@github.com:ggsrc/bl-template@develop",
			wantOwner:  "ggsrc",
			wantRepo:   "bl-template",
			wantBranch: "develop",
			wantErr:    false,
		},
		// HTTPS format
		{
			name:       "HTTPS format with .git",
			input:      "https://github.com/ggsrc/bl-template.git",
			wantOwner:  "ggsrc",
			wantRepo:   "bl-template",
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name:       "HTTPS format with .git and branch",
			input:      "https://github.com/ggsrc/bl-template.git@main",
			wantOwner:  "ggsrc",
			wantRepo:   "bl-template",
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name:       "HTTPS format without .git",
			input:      "https://github.com/ggsrc/bl-template",
			wantOwner:  "ggsrc",
			wantRepo:   "bl-template",
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name:       "HTTPS format without .git with branch",
			input:      "https://github.com/ggsrc/bl-template@develop",
			wantOwner:  "ggsrc",
			wantRepo:   "bl-template",
			wantBranch: "develop",
			wantErr:    false,
		},
		// Short format
		{
			name:       "Short format with .git",
			input:      "github.com/ggsrc/bl-template.git",
			wantOwner:  "ggsrc",
			wantRepo:   "bl-template",
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name:       "Short format with .git and branch",
			input:      "github.com/ggsrc/bl-template.git@main",
			wantOwner:  "ggsrc",
			wantRepo:   "bl-template",
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name:       "Short format without .git",
			input:      "github.com/ggsrc/bl-template",
			wantOwner:  "ggsrc",
			wantRepo:   "bl-template",
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name:       "Short format without .git with branch",
			input:      "github.com/ggsrc/bl-template@develop",
			wantOwner:  "ggsrc",
			wantRepo:   "bl-template",
			wantBranch: "develop",
			wantErr:    false,
		},
		// Edge cases
		{
			name:       "Short format with @branch (no .git)",
			input:      "github.com/user/repo@feature/test",
			wantOwner:  "user",
			wantRepo:   "repo",
			wantBranch: "feature/test",
			wantErr:    false,
		},
		// Invalid formats
		{
			name:    "Invalid format - missing parts",
			input:   "github.com/user",
			wantErr: true,
		},
		{
			name:    "Invalid format - empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRepoURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRepoURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Owner != tt.wantOwner {
				t.Errorf("parseRepoURL() Owner = %v, want %v", got.Owner, tt.wantOwner)
			}
			if got.Repo != tt.wantRepo {
				t.Errorf("parseRepoURL() Repo = %v, want %v", got.Repo, tt.wantRepo)
			}
			if got.Branch != tt.wantBranch {
				t.Errorf("parseRepoURL() Branch = %v, want %v", got.Branch, tt.wantBranch)
			}
		})
	}
}

func TestNormalizeRepoURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "SSH format with .git",
			input: "git@github.com:ggsrc/bl-template.git",
			want:  "https://raw.githubusercontent.com/ggsrc/bl-template/main",
		},
		{
			name:  "HTTPS format with .git and branch",
			input: "https://github.com/ggsrc/bl-template.git@develop",
			want:  "https://raw.githubusercontent.com/ggsrc/bl-template/develop",
		},
		{
			name:  "Short format without .git",
			input: "github.com/ggsrc/bl-template",
			want:  "https://raw.githubusercontent.com/ggsrc/bl-template/main",
		},
		{
			name:  "Short format with branch",
			input: "github.com/ggsrc/bl-template@feature/test",
			want:  "https://raw.githubusercontent.com/ggsrc/bl-template/feature/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeRepoURL(tt.input)
			if got != tt.want {
				t.Errorf("normalizeRepoURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeRepoName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "HTTPS URL",
			input: "https://github.com/ggsrc/bl-template.git",
			want:  "github_com_ggsrc_bl-template_git",
		},
		{
			name:  "SSH URL",
			input: "git@github.com:ggsrc/bl-template.git",
			want:  "git@github_com:ggsrc_bl-template_git",
		},
		{
			name:  "Short format",
			input: "github.com/ggsrc/bl-template",
			want:  "github_com_ggsrc_bl-template",
		},
		{
			name:  "With branch",
			input: "github.com/ggsrc/bl-template@main",
			want:  "github_com_ggsrc_bl-template@main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeRepoName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeRepoName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheAge(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := filepath.Join(os.TempDir(), "test-cache-age")
	defer os.RemoveAll(tempDir)

	// Create cache directory
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cache := NewCache(tempDir, LoaderOptions{})

	// Test 1: Fresh cache (just created)
	age := cache.getCacheAge()
	if age > 1*time.Second {
		t.Errorf("Fresh cache age = %v, want < 1 second", age)
	}

	// Test 2: Set cache to be old (simulate by changing modification time)
	oldTime := time.Now().Add(-8 * 24 * time.Hour) // 8 days ago
	if err := os.Chtimes(tempDir, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to change cache time: %v", err)
	}

	age = cache.getCacheAge()
	expectedAge := 8 * 24 * time.Hour
	tolerance := 1 * time.Hour
	if age < expectedAge-tolerance || age > expectedAge+tolerance {
		t.Errorf("Old cache age = %v, want approximately %v", age, expectedAge)
	}

	// Test 3: Non-existent cache
	nonExistentCache := NewCache(filepath.Join(tempDir, "non-existent"), LoaderOptions{})
	age = nonExistentCache.getCacheAge()
	if age != 0 {
		t.Errorf("Non-existent cache age = %v, want 0", age)
	}
}
