package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// ArgsData holds template arguments loaded from YAML or TOML file
type ArgsData map[string]interface{}

// LoadArgs loads template arguments from a YAML or TOML file
// It automatically detects the format based on file extension:
// - .toml -> TOML format
// - .yaml, .yml -> YAML format
// - no extension or unknown -> tries TOML first, then YAML
func LoadArgs(path string) (ArgsData, error) {
	if path == "" {
		return make(ArgsData), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read args file %s: %w", path, err)
	}

	// Detect format based on file extension
	ext := strings.ToLower(filepath.Ext(path))

	var args ArgsData

	switch ext {
	case ".toml":
		// Try TOML format
		if err := toml.Unmarshal(data, &args); err != nil {
			return nil, fmt.Errorf("failed to parse TOML args file %s: %w", path, err)
		}
	case ".yaml", ".yml":
		// Try YAML format
		if err := yaml.Unmarshal(data, &args); err != nil {
			return nil, fmt.Errorf("failed to parse YAML args file %s: %w", path, err)
		}
	default:
		// No extension or unknown extension - try both formats
		// Try TOML first
		if err := toml.Unmarshal(data, &args); err == nil {
			// Successfully parsed as TOML
			return args, nil
		}

		// Try YAML
		if err := yaml.Unmarshal(data, &args); err != nil {
			return nil, fmt.Errorf("failed to parse args file %s (tried both TOML and YAML): %w", path, err)
		}
	}

	return args, nil
}

// MergeArgs merges multiple ArgsData maps, with later maps taking precedence
// Performs deep merge for nested maps
func MergeArgs(argsList ...ArgsData) ArgsData {
	result := make(ArgsData)
	for _, args := range argsList {
		result = deepMerge(result, args)
	}
	return result
}

// deepMerge performs a deep merge of two ArgsData maps
// Values from src override values in dst
func deepMerge(dst, src ArgsData) ArgsData {
	if dst == nil {
		dst = make(ArgsData)
	}

	for k, srcVal := range src {
		if dstVal, exists := dst[k]; exists {
			// Both exist, try to merge if both are maps
			if dstMap, ok := dstVal.(map[string]interface{}); ok {
				if srcMap, ok := srcVal.(map[string]interface{}); ok {
					// Both are maps, merge recursively
					dst[k] = deepMergeMap(dstMap, srcMap)
					continue
				}
			}
			// Try ArgsData type
			if dstMap, ok := dstVal.(ArgsData); ok {
				if srcMap, ok := srcVal.(ArgsData); ok {
					dst[k] = deepMergeMap(map[string]interface{}(dstMap), map[string]interface{}(srcMap))
					continue
				}
			}
			// Try map[interface{}]interface{} (from YAML)
			if dstMap, ok := dstVal.(map[interface{}]interface{}); ok {
				if srcMap, ok := srcVal.(map[interface{}]interface{}); ok {
					dst[k] = deepMergeInterfaceMap(dstMap, srcMap)
					continue
				}
			}
		}
		// Key doesn't exist in dst, or types don't match, just set it
		dst[k] = srcVal
	}

	return dst
}

// deepMergeMap merges two map[string]interface{} maps
func deepMergeMap(dst, src map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy dst first
	for k, v := range dst {
		result[k] = v
	}

	// Merge src
	for k, srcVal := range src {
		if dstVal, exists := result[k]; exists {
			// Both exist, try to merge if both are maps
			if dstMap, ok := dstVal.(map[string]interface{}); ok {
				if srcMap, ok := srcVal.(map[string]interface{}); ok {
					result[k] = deepMergeMap(dstMap, srcMap)
					continue
				}
			}
			// Try ArgsData type
			if dstMap, ok := dstVal.(ArgsData); ok {
				if srcMap, ok := srcVal.(ArgsData); ok {
					result[k] = deepMergeMap(map[string]interface{}(dstMap), map[string]interface{}(srcMap))
					continue
				}
			}
		}
		// Key doesn't exist or types don't match, just set it
		result[k] = srcVal
	}

	return result
}

// deepMergeInterfaceMap merges two map[interface{}]interface{} maps
func deepMergeInterfaceMap(dst, src map[interface{}]interface{}) map[interface{}]interface{} {
	result := make(map[interface{}]interface{})

	// Copy dst first
	for k, v := range dst {
		result[k] = v
	}

	// Merge src
	for k, srcVal := range src {
		if dstVal, exists := result[k]; exists {
			// Both exist, try to merge if both are maps
			if dstMap, ok := dstVal.(map[interface{}]interface{}); ok {
				if srcMap, ok := srcVal.(map[interface{}]interface{}); ok {
					result[k] = deepMergeInterfaceMap(dstMap, srcMap)
					continue
				}
			}
		}
		// Key doesn't exist or types don't match, just set it
		result[k] = srcVal
	}

	return result
}

// GetString gets a string value from ArgsData with a default
func (a ArgsData) GetString(key string, defaultValue string) string {
	if val, ok := a[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// GetMap gets a map value from ArgsData
// It handles both map[string]interface{} and ArgsData types
func (a ArgsData) GetMap(key string) map[string]interface{} {
	if val, ok := a[key]; ok {
		// Try direct map[string]interface{} first
		if m, ok := val.(map[string]interface{}); ok {
			return m
		}
		// Try ArgsData (which is also map[string]interface{})
		if m, ok := val.(ArgsData); ok {
			return map[string]interface{}(m)
		}
		// Try map[interface{}]interface{} (common in YAML parsing)
		if m, ok := val.(map[interface{}]interface{}); ok {
			result := make(map[string]interface{})
			for k, v := range m {
				if keyStr, ok := k.(string); ok {
					result[keyStr] = v
				}
			}
			return result
		}
	}
	return make(map[string]interface{})
}

// GetSlice gets a slice value from ArgsData
func (a ArgsData) GetSlice(key string) []interface{} {
	if val, ok := a[key]; ok {
		if s, ok := val.([]interface{}); ok {
			return s
		}
	}
	return []interface{}{}
}
