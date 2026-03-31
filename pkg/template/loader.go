package template

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LoaderOptions configures template loader behavior
type LoaderOptions struct {
	ForceUpdate bool          // If true, always fetch from remote, ignoring cache
	CacheExpiry time.Duration // How long to cache templates (0 = no expiry, default 24h)
}

// Loader loads templates from a GitHub repository or local directory
type Loader struct {
	repoURL     string
	options     LoaderOptions
	token       string    // GitHub token for private repos
	repoInfo    *RepoInfo // Parsed repository information
	cache       *Cache    // Cache manager
	isLocalPath bool      // Whether this is a local file path
	localPath   string    // Local directory path (if isLocalPath is true)
}

// NewLoader creates a new template loader
func NewLoader(repoURL string) *Loader {
	return NewLoaderWithOptions(repoURL, LoaderOptions{
		ForceUpdate: false,
		CacheExpiry: 24 * time.Hour, // Default: cache for 24 hours
	})
}

// isLocalPath checks if the given path is a local file system path
func isLocalPath(path string) bool {
	// Check for absolute paths
	if strings.HasPrefix(path, "/") {
		return true
	}
	// Check for relative paths
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		return true
	}
	// Check for file:// protocol
	if strings.HasPrefix(path, "file://") {
		return true
	}
	// Check if it's a Windows absolute path (C:\, D:\, etc.)
	if len(path) >= 3 && path[1] == ':' && (path[2] == '/' || path[2] == '\\') {
		return true
	}
	return false
}

// normalizeLocalPath normalizes a local path to an absolute path
func normalizeLocalPath(path string) (string, error) {
	// Remove file:// prefix if present
	if strings.HasPrefix(path, "file://") {
		path = strings.TrimPrefix(path, "file://")
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve local path: %w", err)
	}

	// Check if path exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("local path does not exist: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("local path is not a directory: %s", absPath)
	}

	return absPath, nil
}

// NewLoaderWithOptions creates a new template loader with custom options
func NewLoaderWithOptions(repoURL string, options LoaderOptions) *Loader {
	// Check if this is a local path
	if isLocalPath(repoURL) {
		localPath, err := normalizeLocalPath(repoURL)
		if err != nil {
			// If normalization fails, still create loader but it will fail later
			localPath = repoURL
		}

		// For local paths, use the path directly as cache directory
		// Create cache manager with local path
		cache := NewCache(localPath, options)
		cache.SetLocalMode(true) // Mark as local mode

		return &Loader{
			repoURL:     repoURL,
			options:     options,
			token:       "",
			repoInfo:    nil,
			cache:       cache,
			isLocalPath: true,
			localPath:   localPath,
		}
	}

	// Convert github.com/user/repo to GitHub raw content URL
	// Support both github.com/user/repo and full URLs
	normalizedURL := normalizeRepoURL(repoURL)

	// Use a cache directory in the user's home directory
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, ".blcli", "templates", sanitizeRepoName(repoURL))

	// Try to get GitHub token from environment or gh cli
	token := getGitHubToken()

	// Parse repo info
	repoInfo, _ := parseRepoURL(repoURL)

	// Create cache manager
	cache := NewCache(cacheDir, options)

	return &Loader{
		repoURL:     normalizedURL,
		options:     options,
		token:       token,
		repoInfo:    repoInfo,
		cache:       cache,
		isLocalPath: false,
		localPath:   "",
	}
}

// getGitHubToken tries to get a GitHub token from:
// 1. GITHUB_TOKEN environment variable
// 2. gh cli (gh auth token)
func getGitHubToken() string {
	// Try environment variable first
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}

	// Try gh cli
	cmd := exec.Command("gh", "auth", "token")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	return ""
}

// RepoInfo holds parsed repository information
type RepoInfo struct {
	Owner  string
	Repo   string
	Branch string
}

// parseRepoURL parses a repository URL into components
// Supports formats:
//   - github.com/user/repo[@branch]
//   - https://github.com/user/repo.git[@branch]
//   - git@github.com:user/repo.git[@branch]
//   - https://raw.githubusercontent.com/user/repo/branch
func parseRepoURL(repo string) (*RepoInfo, error) {
	// Handle SSH format: git@github.com:user/repo.git[@branch]
	if strings.HasPrefix(repo, "git@github.com:") {
		// Remove git@github.com: prefix
		repo = strings.TrimPrefix(repo, "git@github.com:")

		// Check if branch/tag is specified with @ (before removing .git)
		branch := "main"
		if strings.Contains(repo, "@") {
			// Find the last @ to support branches with / in them
			idx := strings.LastIndex(repo, "@")
			if idx > 0 {
				branch = repo[idx+1:]
				repo = repo[:idx]
			}
		}

		// Remove .git suffix if present (after extracting branch)
		repo = strings.TrimSuffix(repo, ".git")

		// Split owner and repo name
		parts := strings.Split(repo, "/")
		if len(parts) == 2 {
			return &RepoInfo{
				Owner:  parts[0],
				Repo:   parts[1],
				Branch: branch,
			}, nil
		}
		return nil, fmt.Errorf("invalid SSH repository format: %s", repo)
	}

	// Handle HTTPS format: https://github.com/user/repo.git[@branch]
	if strings.HasPrefix(repo, "https://github.com/") || strings.HasPrefix(repo, "http://github.com/") {
		// Remove protocol prefix
		repo = strings.TrimPrefix(repo, "https://")
		repo = strings.TrimPrefix(repo, "http://")

		// Check if branch/tag is specified with @ (before removing .git)
		branch := "main"
		if strings.Contains(repo, "@") {
			parts := strings.Split(repo, "@")
			if len(parts) == 2 {
				repo = parts[0]   // github.com/user/repo.git
				branch = parts[1] // branch name
			}
		}

		// Remove .git suffix if present (after extracting branch)
		repo = strings.TrimSuffix(repo, ".git")

		// Parse github.com/user/repo format
		if strings.HasPrefix(repo, "github.com/") {
			parts := strings.Split(repo, "/")
			if len(parts) >= 3 {
				return &RepoInfo{
					Owner:  parts[1],
					Repo:   parts[2],
					Branch: branch,
				}, nil
			}
		}
		return nil, fmt.Errorf("invalid HTTPS repository format: %s", repo)
	}

	// Handle raw.githubusercontent.com URLs
	if strings.Contains(repo, "raw.githubusercontent.com") {
		parts := strings.Split(repo, "/")
		if len(parts) >= 5 {
			return &RepoInfo{
				Owner:  parts[3],
				Repo:   parts[4],
				Branch: parts[5],
			}, nil
		}
		return nil, fmt.Errorf("unsupported raw.githubusercontent.com URL format: %s", repo)
	}

	// Parse github.com/user/repo[@branch] format (short format)
	if strings.HasPrefix(repo, "github.com/") {
		// Check if branch/tag is specified with @ (before splitting)
		branch := "main"
		if strings.Contains(repo, "@") {
			// Find the last @ to support branches with / in them
			idx := strings.LastIndex(repo, "@")
			if idx > 0 {
				branch = repo[idx+1:]
				repo = repo[:idx]
			}
		}

		// Remove .git suffix if present (after extracting branch)
		repo = strings.TrimSuffix(repo, ".git")

		parts := strings.Split(repo, "/")
		if len(parts) >= 3 {
			return &RepoInfo{
				Owner:  parts[1],
				Repo:   parts[2],
				Branch: branch,
			}, nil
		}
	}

	return nil, fmt.Errorf("invalid repository format: %s", repo)
}

// normalizeRepoURL converts github.com/user/repo to GitHub raw content URL format
// Supports formats:
//   - github.com/user/repo (defaults to main branch)
//   - github.com/user/repo@branch
//   - github.com/user/repo@tag
//   - https://raw.githubusercontent.com/user/repo/branch (full URL)
func normalizeRepoURL(repo string) string {
	info, err := parseRepoURL(repo)
	if err != nil {
		// Fallback to original behavior
		return repo
	}
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", info.Owner, info.Repo, info.Branch)
}

// sanitizeRepoName creates a safe directory name from repo URL
func sanitizeRepoName(repo string) string {
	repo = strings.ReplaceAll(repo, "https://", "")
	repo = strings.ReplaceAll(repo, "http://", "")
	repo = strings.ReplaceAll(repo, "/", "_")
	repo = strings.ReplaceAll(repo, ".", "_")
	return repo
}

// SyncCache synchronizes the entire repository to local cache
// This should be called once at initialization, before any LoadTemplate calls
// For local paths, this is a no-op since we read directly from the local directory
func (l *Loader) SyncCache() error {
	if l.isLocalPath {
		// For local paths, no sync needed - we read directly from the directory
		return nil
	}
	return l.cache.Sync(l.repoInfo, l.token)
}

// IsLocalPath returns whether this loader is using a local path
func (l *Loader) IsLocalPath() bool {
	return l.isLocalPath
}

// LoadTemplate loads a template file from the local cache
// This method only reads from cache and does NOT trigger network requests
// SyncCache() must be called first to ensure cache is populated
func (l *Loader) LoadTemplate(path string) (string, error) {
	// Ensure cache is synced first
	if err := l.SyncCache(); err != nil {
		return "", fmt.Errorf("failed to sync cache: %w", err)
	}

	return l.cache.Load(path)
}

// CacheExists checks if a template file exists in cache (without loading it)
func (l *Loader) CacheExists(path string) bool {
	return l.cache.Exists(path)
}

// ListModuleFiles lists all .tmpl files in a module directory from cache
// Returns a list of file names (without .tmpl extension) that exist in cache
func (l *Loader) ListModuleFiles(modulePath string) ([]string, error) {
	return l.cache.ListModuleFiles(modulePath)
}

// ClearCache clears the cache for this loader
func (l *Loader) ClearCache() error {
	return l.cache.Clear()
}
