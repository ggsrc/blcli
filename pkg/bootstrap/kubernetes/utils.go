package kubernetes

import (
	"path/filepath"
	"strings"
)

// RemoveTmplExtension removes .tmpl extension from a filename
func RemoveTmplExtension(filename string) string {
	return strings.TrimSuffix(filename, ".tmpl")
}

// IsTemplatePath returns true if the path should be rendered as a Go template.
// Only paths ending with .tmpl are rendered; other files (e.g. .yaml with Prometheus syntax) are copied as-is.
func IsTemplatePath(path string) bool {
	return strings.HasSuffix(path, ".tmpl")
}

// GetComponentBasePath returns the longest common directory prefix of the given paths.
// Used to preserve subdirectory structure when writing component files (e.g. alert-rules/vmoperator.yaml).
func GetComponentBasePath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	base := filepath.Dir(paths[0])
	for _, p := range paths[1:] {
		b := filepath.ToSlash(base)
		pp := filepath.ToSlash(p)
		for base != "." && b != "." && !strings.HasPrefix(pp, b+"/") && pp != b {
			base = filepath.Dir(base)
			b = filepath.ToSlash(base)
		}
	}
	return base
}
