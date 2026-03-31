package validator

import (
	"fmt"

	"blcli/pkg/renderer"
)

// TemplateLoader loads template file content by path (e.g. args.yaml).
type TemplateLoader interface {
	LoadTemplate(path string) (string, error)
}

// Run validates merged args against parameter definitions loaded from the template.
// It loads args definitions from standard paths (args.yaml, terraform/args.yaml, etc.),
// runs field-level validation rules, then top-level unique rules.
// Returns nil if valid, or an error with path/kind/message.
func Run(data renderer.ArgsData, loader TemplateLoader) error {
	if loader == nil {
		return nil
	}
	// Load args definitions from template (same paths as init-args)
	paths := []string{
		"args.yaml",
		"terraform/args.yaml",
		"terraform/init/args.yaml",
		"terraform/project/args.yaml",
	}
	var rootDef, tfDef, initDef, projectDef *renderer.ArgsDefinition
	for _, p := range paths {
		content, err := loader.LoadTemplate(p)
		if err != nil {
			continue
		}
		def, err := renderer.LoadArgsDefinition(content)
		if err != nil {
			continue
		}
		switch p {
		case "args.yaml":
			rootDef = def
		case "terraform/args.yaml":
			tfDef = def
		case "terraform/init/args.yaml":
			initDef = def
		case "terraform/project/args.yaml":
			projectDef = def
		}
	}

	// 1) Validate global (merged from root + terraform.global)
	globalMap := toMapFromInterface(data[renderer.FieldGlobal])
	if globalMap == nil {
		globalMap = make(map[string]interface{})
	}
	// Merge terraform.global into scope for param defs
	globalDef := mergeGlobalParamDefs(rootDef, tfDef)
	for key, value := range globalMap {
		paramDef := getParamDef(globalDef, key)
		rules := RulesFromParam(paramDef)
		for _, rule := range rules {
			if err := Rule(value, rule); err != nil {
				return fmt.Errorf("global.%s: %w", key, err)
			}
		}
	}

	// 2) Validate terraform.init (global + components)
	if initDef != nil {
		tfSection := toMapFromInterface(data[renderer.FieldTerraform])
		if tfSection != nil {
			initSection := toMapFromInterface(tfSection[renderer.FieldInit])
			if initSection != nil {
				if g := toMapFromInterface(initSection[renderer.FieldGlobal]); g != nil {
					for key, value := range g {
						paramDef := getParamDefFromGlobal(initDef.Parameters.Global, key)
						rules := RulesFromParam(paramDef)
						for _, rule := range rules {
							if err := Rule(value, rule); err != nil {
								return fmt.Errorf("terraform.init.global.%s: %w", key, err)
							}
						}
					}
				}
				comps := toMapFromInterface(initSection[renderer.FieldComponents])
				if comps != nil {
					for compName, compVal := range comps {
						compDefMap := initDef.Parameters.Components[compName]
						if compDefMap == nil {
							continue
						}
						compMap := toMapFromInterface(compVal)
						if compMap == nil {
							continue
						}
						for paramKey, value := range compMap {
							paramDef := getParamDefFromGlobal(compDefMap, paramKey)
							rules := RulesFromParam(paramDef)
							for _, rule := range rules {
								if err := Rule(value, rule); err != nil {
									return fmt.Errorf("terraform.init.components.%s.%s: %w", compName, paramKey, err)
								}
							}
						}
					}
				}
			}
		}
	}

	// 3) Validate terraform.projects[].global with project def
	if projectDef != nil && projectDef.Parameters.Global != nil {
		tfSection := toMapFromInterface(data[renderer.FieldTerraform])
		if tfSection != nil {
			projects := toSliceFromInterface(tfSection[renderer.FieldProjects])
			for i, p := range projects {
				projMap := toMapFromInterface(p)
				if projMap == nil {
					continue
				}
				global := toMapFromInterface(projMap[renderer.FieldGlobal])
				if global == nil {
					continue
				}
				for key, value := range global {
					paramDef := getParamDefFromGlobal(projectDef.Parameters.Global, key)
					rules := RulesFromParam(paramDef)
					for _, rule := range rules {
						if err := Rule(value, rule); err != nil {
							return fmt.Errorf("terraform.projects[%d].global.%s: %w", i, key, err)
						}
					}
				}
			}
		}
	}

	// 4) Top-level validation.unique (from any loaded def)
	var uniqueRules []struct{ Path, Message string }
	for _, def := range []*renderer.ArgsDefinition{rootDef, tfDef, initDef, projectDef} {
		if def == nil || def.Validation == nil {
			continue
		}
		for _, u := range def.Validation.Unique {
			if u.Path != "" {
				uniqueRules = append(uniqueRules, struct{ Path, Message string }{u.Path, u.Message})
			}
		}
	}
	for _, u := range uniqueRules {
		if err := CheckUnique(data, u.Path, u.Message); err != nil {
			return err
		}
	}

	return nil
}

func mergeGlobalParamDefs(root, tf *renderer.ArgsDefinition) map[string]interface{} {
	out := make(map[string]interface{})
	if root != nil && root.Parameters.Global != nil {
		for k, v := range root.Parameters.Global {
			out[k] = v
		}
	}
	if tf != nil && tf.Parameters.Global != nil {
		for k, v := range tf.Parameters.Global {
			out[k] = v
		}
	}
	return out
}

func getParamDef(globalDef map[string]interface{}, key string) map[string]interface{} {
	if globalDef == nil {
		return nil
	}
	v := globalDef[key]
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	if m, ok := v.(map[interface{}]interface{}); ok {
		out := make(map[string]interface{})
		for k, val := range m {
			if s, ok := k.(string); ok {
				out[s] = val
			}
		}
		return out
	}
	return nil
}

func getParamDefFromGlobal(global map[string]interface{}, key string) map[string]interface{} {
	if global == nil {
		return nil
	}
	v := global[key]
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	if m, ok := v.(map[interface{}]interface{}); ok {
		out := make(map[string]interface{})
		for k, val := range m {
			if s, ok := k.(string); ok {
				out[s] = val
			}
		}
		return out
	}
	return nil
}

// Ensure template.Loader satisfies TemplateLoader
var _ TemplateLoader = (*templateLoaderAdapter)(nil)

type templateLoaderAdapter struct {
	LoadTemplateFunc func(path string) (string, error)
}

func (a *templateLoaderAdapter) LoadTemplate(path string) (string, error) {
	return a.LoadTemplateFunc(path)
}

// NewTemplateLoader adapts a function to TemplateLoader.
func NewTemplateLoader(loadTemplate func(path string) (string, error)) TemplateLoader {
	return &templateLoaderAdapter{LoadTemplateFunc: loadTemplate}
}

// toMapFromInterface converts map[string]interface{} or map[interface{}]interface{} to map[string]interface{}.
func toMapFromInterface(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	if m, ok := v.(renderer.ArgsData); ok {
		return m
	}
	if m, ok := v.(map[interface{}]interface{}); ok {
		out := make(map[string]interface{})
		for k, val := range m {
			if s, ok := k.(string); ok {
				out[s] = val
			}
		}
		return out
	}
	return nil
}

// toSliceFromInterface returns []interface{} from slice types.
func toSliceFromInterface(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	if s, ok := v.([]interface{}); ok {
		return s
	}
	return nil
}
