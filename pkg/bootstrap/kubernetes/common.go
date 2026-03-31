package kubernetes

import (
	"blcli/pkg/config"
	"blcli/pkg/renderer"
)

// Profiler interface for performance profiling (defined in bootstrap package)
type Profiler interface {
	TimeStep(name string, fn func() error) error
}

// toMapStringInterface converts various map types to map[string]interface{}
// This is a common pattern when dealing with YAML/TOML decoded data which can be
// map[string]interface{}, map[interface{}]interface{}, or renderer.ArgsData
func toMapStringInterface(v interface{}) map[string]interface{} {
	switch m := v.(type) {
	case map[string]interface{}:
		return m
	case renderer.ArgsData:
		return map[string]interface{}(m)
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for k, val := range m {
			if keyStr, ok := k.(string); ok {
				result[keyStr] = val
			}
		}
		return result
	default:
		return nil
	}
}

// PrepareKubernetesData prepares data map for kubernetes templates
// All parameters should come from args.yaml (via templateArgs), which will be automatically
// merged by RenderWithArgs. Returns empty map as all values come from templateArgs.
func PrepareKubernetesData(global config.GlobalConfig, projectName string) map[string]interface{} {
	// All parameters should come from templateArgs via RenderWithArgs
	// ProjectName should be in kubernetes.projects[].global.ProjectName (if projectName is provided)
	return map[string]interface{}{}
}
