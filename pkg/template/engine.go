package template

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"blcli/pkg/renderer"
)

// toMapStringInterface converts various map types to map[string]interface{}
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

// Note: Embedded templates have been removed as the tool now relies entirely on
// templates from the template repository. All templates are loaded dynamically
// via template.Loader from the specified GitHub repository.

// defaultFunc implements the default template function
// Usage in template: {{ .Value | default "defaultValue" }}
// In Go template pipeline, the first argument is the piped value, second is the function argument
// Returns the value if it's not empty, otherwise returns the default value
func defaultFunc(val interface{}, defaultVal interface{}) interface{} {
	if val == nil {
		return defaultVal
	}
	// Check if value is empty string
	if str, ok := val.(string); ok {
		if str == "" {
			return defaultVal
		}
		return str
	}
	// For other types, return the value if not nil
	return val
}

// quoteFunc implements the quote template function
// Wraps a string in double quotes, escaping any double quotes inside
func quoteFunc(s interface{}) string {
	if s == nil {
		return `""`
	}
	str := fmt.Sprintf("%v", s)
	// Escape double quotes
	escaped := strconv.Quote(str)
	return escaped
}

// replaceFunc replaces all occurrences of old in s with new (for GCP-friendly names: underscore to hyphen).
// Usage in template: {{ replace .name "_" "-" }}
func replaceFunc(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}

// getTemplateFuncMap returns the function map for templates
func getTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"default": defaultFunc,
		"quote":   quoteFunc,
		"replace": replaceFunc,
	}
}

// Render renders a template string with the given data
func Render(tmplStr string, data interface{}) (string, error) {
	tmpl, err := template.New("").Funcs(getTemplateFuncMap()).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// RenderWithArgs renders a template with merged data from ArgsData
// All template variables should come from args, data is only used for backward compatibility
func RenderWithArgs(tmplStr string, data interface{}, args renderer.ArgsData) (string, error) {
	// Convert data to a map for easier merging (backward compatibility)
	dataMap := make(map[string]interface{})

	// Convert data to map[string]interface{} if it's a map
	if dataMapInput, ok := data.(map[string]interface{}); ok {
		for k, val := range dataMapInput {
			dataMap[k] = val
		}
	}

	// Merge args into dataMap (args take precedence for conflicting keys)
	// Also flatten global and terraform.global to top level for easier template access
	for k, v := range args {
		dataMap[k] = v

		// Flatten global to top level (args.global may already be merged from multiple sources)
		if k == renderer.FieldGlobal {
			globalMap := toMapStringInterface(v)
			if globalMap != nil {
				for gk, gv := range globalMap {
					dataMap[gk] = gv
				}
			}
		}

		// Flatten terraform.global to top level.
		// terraform.global is more specific than top-level global and should override
		// conflicting keys during rendering.
		if k == renderer.FieldTerraform {
			terraformMap := toMapStringInterface(v)
			if terraformMap != nil {
				if tfGlobal, ok := terraformMap[renderer.FieldGlobal]; ok {
					globalMap := toMapStringInterface(tfGlobal)
					if globalMap != nil {
						for gk, gv := range globalMap {
							dataMap[gk] = gv
						}
					}
				}
			}
		}
	}

	return Render(tmplStr, dataMap)
}
