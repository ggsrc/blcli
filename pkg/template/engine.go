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

// defaultFunc implements the default template function with Sprig's argument order.
// Go templates pass a piped value as the last argument, so both forms
//
//	{{ default "fallback" .Value }}
//	{{ .Value | default "fallback" }}
//
// call defaultFunc("fallback", .Value). It returns val unless val is nil (unset or
// missing key) or the empty string. Unlike Sprig, an explicit false or 0 is kept,
// so `enabled: false` in args survives a `default true` in the template.
func defaultFunc(defaultVal interface{}, val interface{}) interface{} {
	if val == nil {
		return defaultVal
	}
	if str, ok := val.(string); ok && str == "" {
		return defaultVal
	}
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

	// Merge args into dataMap. Layers are applied in a fixed order, lowest
	// precedence first, so a key present in two layers always resolves the same way:
	//  1. data (backward compatibility)
	//  2. args.global flattened to top level (it may already be merged from several sources)
	//  3. args.terraform.global flattened to top level (more specific than global)
	//  4. top-level args keys: component parameters and values the caller has already
	//     flattened with their own precedence
	if globalMap := toMapStringInterface(args[renderer.FieldGlobal]); globalMap != nil {
		for gk, gv := range globalMap {
			dataMap[gk] = gv
		}
	}
	if terraformMap := toMapStringInterface(args[renderer.FieldTerraform]); terraformMap != nil {
		if globalMap := toMapStringInterface(terraformMap[renderer.FieldGlobal]); globalMap != nil {
			for gk, gv := range globalMap {
				dataMap[gk] = gv
			}
		}
	}
	for k, v := range args {
		dataMap[k] = v
	}

	return Render(tmplStr, dataMap)
}
