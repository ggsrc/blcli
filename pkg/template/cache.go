package template

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Cache manages template caching
type Cache struct {
	cacheDir string
	options  LoaderOptions
	syncOnce sync.Once
	syncErr  error
	isLocal  bool // Whether this is a local directory (skip GitHub sync)
}

// CacheMetadata stores metadata about a cached template
type CacheMetadata struct {
	ETag          string    `json:"etag"`
	LastModified  string    `json:"last_modified"`
	CachedAt      time.Time `json:"cached_at"`
	ContentLength int64     `json:"content_length"`
	Sha           string    `json:"sha,omitempty"` // GitHub file SHA
}

// NewCache creates a new cache manager
func NewCache(cacheDir string, options LoaderOptions) *Cache {
	return &Cache{
		cacheDir: cacheDir,
		options:  options,
		isLocal:  false,
	}
}

// SetLocalMode sets the cache to local mode (skip GitHub sync)
func (c *Cache) SetLocalMode(isLocal bool) {
	c.isLocal = isLocal
}

// Sync synchronizes the entire repository to local cache
// This should be called once at initialization
// For local paths, this is a no-op
func (c *Cache) Sync(repoInfo *RepoInfo, token string) error {
	// If this is a local directory, skip sync
	if c.isLocal {
		// Just verify the directory exists and is valid
		if !c.isValid() {
			return fmt.Errorf("local template directory is not valid: %s", c.cacheDir)
		}
		return nil
	}

	c.syncOnce.Do(func() {
		// Check if cache exists and is valid (unless force-update)
		if !c.options.ForceUpdate {
			if c.isValid() {
				// If using git and cache is a git repo, do a quick pull to update
				if isGitInstalled() && c.isGitRepository() {
					fmt.Printf("Cache exists, updating with git pull...\n")
					c.syncErr = c.syncRepository(repoInfo, token)
					if c.syncErr != nil {
						// Check cache age to determine if we should warn more strongly
						cacheAge := c.getCacheAge()
						if cacheAge > 7*24*time.Hour {
							fmt.Printf("⚠️  WARNING: Git update failed (%v)\n", c.syncErr)
							fmt.Printf("⚠️  Using cache that is %.0f days old - content may be outdated\n", cacheAge.Hours()/24)
							fmt.Printf("⚠️  Consider: 1) Check your GITHUB_TOKEN, 2) Use --force-update, 3) Clear cache\n")
						} else {
							fmt.Printf("Warning: git update failed (%v), using existing cache (age: %.1f hours)\n", c.syncErr, cacheAge.Hours())
						}
						c.syncErr = nil // Don't fail if pull fails, use existing cache
					} else {
						fmt.Printf("Cache updated successfully\n")
					}
				}
				// Cache exists and is valid, no need to sync
				return
			}
		}

		// Sync the repository
		fmt.Printf("Syncing template repository to cache...\n")
		c.syncErr = c.syncRepository(repoInfo, token)
		if c.syncErr != nil {
			fmt.Printf("Failed to sync cache: %v\n", c.syncErr)
		} else {
			fmt.Printf("Template repository synced successfully\n")
		}
	})
	return c.syncErr
}

// Load loads a template file from cache
func (c *Cache) Load(path string) (string, error) {
	cachedPath := filepath.Join(c.cacheDir, path)
	content, err := os.ReadFile(cachedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("template %s not found in cache", path)
		}
		return "", fmt.Errorf("failed to read cached template %s: %w", path, err)
	}
	return string(content), nil
}

// Exists checks if a template file exists in cache
func (c *Cache) Exists(path string) bool {
	cachedPath := filepath.Join(c.cacheDir, path)
	_, err := os.Stat(cachedPath)
	return err == nil
}

// ListModuleFiles lists all .tmpl files in a module directory from cache
func (c *Cache) ListModuleFiles(modulePath string) ([]string, error) {
	moduleCachePath := filepath.Join(c.cacheDir, modulePath)

	// Check if module directory exists in cache
	if _, err := os.Stat(moduleCachePath); os.IsNotExist(err) {
		return []string{}, nil
	}

	var files []string
	entries, err := os.ReadDir(moduleCachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read module directory %s: %w", moduleCachePath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Only include .tmpl files
		if strings.HasSuffix(name, ".tmpl") && !strings.HasSuffix(name, ".meta") {
			// Remove .tmpl extension to get the base filename
			baseName := strings.TrimSuffix(name, ".tmpl")
			files = append(files, baseName)
		}
	}

	return files, nil
}

// Clear clears the cache
func (c *Cache) Clear() error {
	return os.RemoveAll(c.cacheDir)
}

// isValid checks if the cache directory exists and contains files
func (c *Cache) isValid() bool {
	// Check if cache directory exists
	if _, err := os.Stat(c.cacheDir); os.IsNotExist(err) {
		return false
	}

	// Recursively check for .tmpl files
	return c.hasTemplateFiles(c.cacheDir)
}

// getCacheAge returns the age of the cache directory
func (c *Cache) getCacheAge() time.Duration {
	info, err := os.Stat(c.cacheDir)
	if err != nil {
		return 0
	}
	return time.Since(info.ModTime())
}

// hasTemplateFiles recursively checks if a directory contains .tmpl files
func (c *Cache) hasTemplateFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if c.hasTemplateFiles(filepath.Join(dir, entry.Name())) {
				return true
			}
		} else if strings.HasSuffix(entry.Name(), ".tmpl") {
			return true
		}
	}

	return false
}

// isGitInstalled checks if git is installed and available
func isGitInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// isGitRepository checks if the cache directory is a git repository
func (c *Cache) isGitRepository() bool {
	gitDir := filepath.Join(c.cacheDir, ".git")
	info, err := os.Stat(gitDir)
	return err == nil && info.IsDir()
}

// buildGitRepoURL builds the git repository URL from RepoInfo
func buildGitRepoURL(repoInfo *RepoInfo, token string) string {
	// Always use HTTPS URL, token will be passed via environment variable or git credential helper
	return fmt.Sprintf("https://github.com/%s/%s.git", repoInfo.Owner, repoInfo.Repo)
}

// setupGitAuth configures git authentication using token
func setupGitAuth(cmd *exec.Cmd, token string) {
	env := os.Environ()
	// Disable terminal prompts
	env = append(env, "GIT_TERMINAL_PROMPT=0")

	if token != "" {
		// Set GITHUB_TOKEN environment variable for git credential helper
		// This is used as a fallback if URL-based authentication fails
		env = append(env, fmt.Sprintf("GITHUB_TOKEN=%s", token))
		// Disable SSH by setting GIT_SSH_COMMAND to empty
		// This prevents git from using SSH even if URL rewriting is configured
		env = append(env, "GIT_SSH_COMMAND=")
		// Note: The token is already embedded in the cloneURL (https://token@github.com/...)
		// which is the primary authentication method
	}

	cmd.Env = env
}

// syncRepositoryWithGit syncs repository using git clone/pull
func (c *Cache) syncRepositoryWithGit(repoInfo *RepoInfo, token string) error {
	if repoInfo == nil {
		return fmt.Errorf("repository info not available")
	}

	// Check if it's already a git repository
	if c.isGitRepository() {
		// Update existing repository with git pull
		fmt.Printf("Updating template repository with git pull...\n")

		// Always update remote URL to match current token state
		// Clean owner and repo
		owner := strings.TrimPrefix(repoInfo.Owner, "https://")
		owner = strings.TrimPrefix(owner, "http://")
		owner = strings.TrimPrefix(owner, "github.com/")
		repo := strings.TrimPrefix(repoInfo.Repo, "https://")
		repo = strings.TrimPrefix(repo, "http://")
		repo = strings.TrimPrefix(repo, "github.com/")
		repo = strings.TrimSuffix(repo, ".git")

		var httpsURL string
		if token != "" {
			// Use token in URL
			httpsURL = fmt.Sprintf("https://%s@github.com/%s/%s.git", token, owner, repo)
		} else {
			// No token - use plain HTTPS URL (will use git credentials or SSH)
			httpsURL = fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
		}

		cmd := exec.Command("git", "remote", "set-url", "origin", httpsURL)
		cmd.Dir = c.cacheDir
		setupGitAuth(cmd, token)
		if _, err := cmd.CombinedOutput(); err != nil {
			// If setting URL fails, continue anyway
			fmt.Printf("Warning: failed to update remote URL: %v\n", err)
		}

		// Fetch latest changes
		cmd = exec.Command("git", "fetch", "origin")
		cmd.Dir = c.cacheDir
		setupGitAuth(cmd, token)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to fetch: %w, output: %s", err, string(output))
		}

		// Checkout the specified branch/tag
		cmd = exec.Command("git", "checkout", repoInfo.Branch)
		cmd.Dir = c.cacheDir
		setupGitAuth(cmd, token)
		if _, err := cmd.CombinedOutput(); err != nil {
			// If branch doesn't exist locally, try to checkout remote branch
			cmd = exec.Command("git", "checkout", "-b", repoInfo.Branch, fmt.Sprintf("origin/%s", repoInfo.Branch))
			cmd.Dir = c.cacheDir
			setupGitAuth(cmd, token)
			if output2, err2 := cmd.CombinedOutput(); err2 != nil {
				return fmt.Errorf("failed to checkout %s: %w, output: %s", repoInfo.Branch, err2, string(output2))
			}
		}

		// Reset to remote branch to ensure we're up to date
		cmd = exec.Command("git", "reset", "--hard", fmt.Sprintf("origin/%s", repoInfo.Branch))
		cmd.Dir = c.cacheDir
		setupGitAuth(cmd, token)
		if _, err := cmd.CombinedOutput(); err != nil {
			// If reset fails, try pull instead
			cmd = exec.Command("git", "pull", "origin", repoInfo.Branch)
			cmd.Dir = c.cacheDir
			setupGitAuth(cmd, token)
			if output2, err2 := cmd.CombinedOutput(); err2 != nil {
				return fmt.Errorf("failed to update repository: %w, output: %s", err2, string(output2))
			}
		}

		fmt.Printf("Template repository updated successfully\n")
		return nil
	}

	// Clone repository for the first time
	fmt.Printf("Cloning template repository with git...\n")

	// Remove existing cache directory if it exists but is not a git repo
	if _, err := os.Stat(c.cacheDir); err == nil {
		if err := os.RemoveAll(c.cacheDir); err != nil {
			return fmt.Errorf("failed to remove existing cache directory: %w", err)
		}
	}

	// Build clone URL with token if available
	// Always build from repoInfo to avoid URL duplication issues
	// Clean owner and repo to ensure they don't contain URL prefixes
	cleanOwner := strings.TrimPrefix(repoInfo.Owner, "https://")
	cleanOwner = strings.TrimPrefix(cleanOwner, "http://")
	cleanOwner = strings.TrimPrefix(cleanOwner, "github.com/")
	cleanRepo := strings.TrimPrefix(repoInfo.Repo, "https://")
	cleanRepo = strings.TrimPrefix(cleanRepo, "http://")
	cleanRepo = strings.TrimPrefix(cleanRepo, "github.com/")
	// Remove .git suffix if present
	cleanRepo = strings.TrimSuffix(cleanRepo, ".git")

	var cloneURL string
	if token != "" {
		// Clean token: remove any URL prefixes that might have been accidentally included
		cleanToken := strings.TrimPrefix(token, "https://")
		cleanToken = strings.TrimPrefix(cleanToken, "http://")
		cleanToken = strings.TrimPrefix(cleanToken, "github.com/")
		// Use token in URL for authentication (HTTPS)
		cloneURL = fmt.Sprintf("https://%s@github.com/%s/%s.git", cleanToken, cleanOwner, cleanRepo)
	} else {
		// Build URL without token
		cloneURL = fmt.Sprintf("https://github.com/%s/%s.git", cleanOwner, cleanRepo)
	}

	// Clone with depth=1 for faster initial clone (shallow clone)
	// Don't use url.insteadOf as it can cause URL rewriting issues
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", repoInfo.Branch, cloneURL, c.cacheDir)
	setupGitAuth(cmd, token)
	if _, err := cmd.CombinedOutput(); err != nil {
		// If shallow clone fails (e.g., branch is a tag), try without depth
		cmd = exec.Command("git", "clone", "--branch", repoInfo.Branch, cloneURL, c.cacheDir)
		setupGitAuth(cmd, token)
		if output2, err2 := cmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("failed to clone repository: %w, output: %s", err2, string(output2))
		}
	}

	fmt.Printf("Template repository cloned successfully\n")
	return nil
}

// syncRepository downloads the entire repository to cache
// It tries to use git first, falls back to GitHub API if git is not available
func (c *Cache) syncRepository(repoInfo *RepoInfo, token string) error {
	if repoInfo == nil {
		return fmt.Errorf("repository info not available")
	}

	// Try to use git if available
	if isGitInstalled() {
		if err := c.syncRepositoryWithGit(repoInfo, token); err != nil {
			fmt.Printf("Warning: git sync failed (%v), falling back to GitHub API...\n", err)
			// Fall back to GitHub API method
			return c.syncRepositoryWithAPI(repoInfo, token)
		}
		return nil
	}

	// Fall back to GitHub API if git is not installed
	fmt.Printf("Git is not installed, using GitHub API to sync templates...\n")
	return c.syncRepositoryWithAPI(repoInfo, token)
}

// syncRepositoryWithAPI downloads the entire repository using GitHub API
func (c *Cache) syncRepositoryWithAPI(repoInfo *RepoInfo, token string) error {
	// Start from root directory
	return c.syncDirectory("", repoInfo, token)
}

// syncDirectory recursively syncs a directory from GitHub to cache
func (c *Cache) syncDirectory(dirPath string, repoInfo *RepoInfo, token string) error {
	// Clean owner and repo to ensure they don't contain URL prefixes
	owner := strings.TrimPrefix(repoInfo.Owner, "https://")
	owner = strings.TrimPrefix(owner, "http://")
	owner = strings.TrimPrefix(owner, "github.com/")
	repo := strings.TrimPrefix(repoInfo.Repo, "https://")
	repo = strings.TrimPrefix(repo, "http://")
	repo = strings.TrimPrefix(repo, "github.com/")
	repo = strings.TrimSuffix(repo, ".git")

	// Use GitHub API to list directory contents
	var apiURL string
	if dirPath == "" {
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/contents?ref=%s",
			owner, repo, repoInfo.Branch)
	} else {
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
			owner, repo, dirPath, repoInfo.Branch)
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header if token is available
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch directory %s: %w", dirPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to fetch directory %s: HTTP %d - %s", dirPath, resp.StatusCode, string(body))
	}

	var entries []struct {
		Type        string `json:"type"`
		Name        string `json:"name"`
		Path        string `json:"path"`
		Content     string `json:"content"`      // Base64 encoded for files (may be empty for large files)
		DownloadURL string `json:"download_url"` // URL to download file content if content is empty
		Sha         string `json:"sha"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return fmt.Errorf("failed to parse directory listing: %w", err)
	}

	// Process each entry
	for _, entry := range entries {
		entryPath := entry.Path
		cachedPath := filepath.Join(c.cacheDir, entryPath)

		if entry.Type == "dir" {
			// Recursively sync subdirectory
			if err := c.syncDirectory(entryPath, repoInfo, token); err != nil {
				return fmt.Errorf("failed to sync directory %s: %w", entryPath, err)
			}
		} else if entry.Type == "file" {
			// Only sync .tmpl files and config.yaml files
			if strings.HasSuffix(entry.Name, ".tmpl") || entry.Name == "config.yaml" || strings.HasSuffix(entry.Name, ".yaml") {
				var content []byte
				var err error

				// Try to get content from base64-encoded field first
				if entry.Content != "" {
					content, err = base64.StdEncoding.DecodeString(entry.Content)
					if err != nil {
						return fmt.Errorf("failed to decode content for %s: %w", entryPath, err)
					}
				} else if entry.DownloadURL != "" {
					// If content is empty, download from download_url
					content, err = c.downloadFile(entry.DownloadURL, token)
					if err != nil {
						return fmt.Errorf("failed to download %s: %w", entryPath, err)
					}
				} else {
					// Both content and download_url are empty, skip this file
					fmt.Printf("Warning: file %s has no content or download_url, skipping\n", entryPath)
					continue
				}

				// Save to cache
				if err := os.MkdirAll(filepath.Dir(cachedPath), 0755); err != nil {
					return fmt.Errorf("failed to create cache directory: %w", err)
				}

				if err := os.WriteFile(cachedPath, content, 0644); err != nil {
					return fmt.Errorf("failed to write cache file %s: %w", cachedPath, err)
				}

				// Save metadata
				metadata := &CacheMetadata{
					ETag:          resp.Header.Get("ETag"),
					LastModified:  resp.Header.Get("Last-Modified"),
					CachedAt:      time.Now(),
					ContentLength: int64(len(content)),
					Sha:           entry.Sha,
				}
				metadataPath := cachedPath + ".meta"
				if err := c.saveMetadata(metadataPath, metadata); err != nil {
					fmt.Printf("Warning: failed to save metadata for %s: %v\n", entryPath, err)
				}
			}
		}
	}

	return nil
}

// downloadFile downloads a file from a URL
func (c *Cache) downloadFile(url string, token string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header if token is available
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to download file: HTTP %d - %s", resp.StatusCode, string(body))
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	return content, nil
}

// saveMetadata saves cache metadata to a file
func (c *Cache) saveMetadata(metadataPath string, metadata *CacheMetadata) error {
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}
