package renderer

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ArgsDefinition represents the structure of args.yaml files
type ArgsDefinition struct {
	Version     string          `yaml:"version"`
	Parameters  ArgsParameters  `yaml:"parameters"`
	Validation  *ValidationDef  `yaml:"validation,omitempty"`
}

// ValidationDef holds top-level validation rules (e.g. unique)
type ValidationDef struct {
	Unique []UniqueRule `yaml:"unique,omitempty"`
}

// UniqueRule defines a uniqueness constraint on a path (e.g. terraform.projects[].name)
type UniqueRule struct {
	Path    string `yaml:"path"`
	Message string `yaml:"message"`
}

// ArgsParameters represents the parameters section
type ArgsParameters struct {
	Global     map[string]interface{}            `yaml:"global,omitempty"`
	Components map[string]map[string]interface{} `yaml:"components,omitempty"`
}

// LoadArgsDefinition loads an args definition from YAML content
func LoadArgsDefinition(content string) (*ArgsDefinition, error) {
	var def ArgsDefinition
	if err := yaml.Unmarshal([]byte(content), &def); err != nil {
		return nil, fmt.Errorf("failed to parse args definition: %w", err)
	}
	return &def, nil
}

// ToArgsData converts ArgsDefinition to ArgsData format
func (def *ArgsDefinition) ToArgsData() ArgsData {
	result := make(ArgsData)

	// Add global parameters
	if def.Parameters.Global != nil {
		result[FieldGlobal] = def.Parameters.Global
	}

	// Add component parameters
	if def.Parameters.Components != nil {
		result[FieldComponents] = def.Parameters.Components
	}

	return result
}

// ToConfigValues converts ArgsDefinition to actual config values (extracting defaults/examples)
// Returns a map with actual values and a map with comments
func (def *ArgsDefinition) ToConfigValues() (ArgsData, map[string]string) {
	values := make(ArgsData)
	comments := make(map[string]string)

	// Process global parameters
	if def.Parameters.Global != nil {
		globalValues := make(map[string]interface{})
		for key, param := range def.Parameters.Global {
			if paramMap, ok := param.(map[string]interface{}); ok {
				// Extract value: default > example > empty (with recursive processing)
				value := extractValueFromParam(paramMap, nil)

				globalValues[key] = value

				// Store comment
				commentKey := fmt.Sprintf("%s.%s", FieldGlobal, key)
				comments[commentKey] = buildComment(paramMap)
			} else {
				// If it's not a map, use it directly
				globalValues[key] = param
			}
		}
		values[FieldGlobal] = globalValues
	}

	// Process component parameters
	if def.Parameters.Components != nil {
		componentValues := make(map[string]interface{})
		for compName, compParams := range def.Parameters.Components {
			// compParams is already map[string]interface{} from ArgsParameters definition
			compValues := make(map[string]interface{})
			for key, param := range compParams {
				// Skip description field
				if key == "description" {
					// Store description as comment
					if desc, ok := param.(string); ok && desc != "" {
						commentKey := fmt.Sprintf("%s.%s.description", FieldComponents, compName)
						comments[commentKey] = desc
					}
					continue
				}

				if paramMap, ok := param.(map[string]interface{}); ok {
					// Extract value: default > example > empty (with recursive processing)
					value := extractValueFromParam(paramMap, []string{compName, key})
					compValues[key] = value

					// Store comment
					commentKey := fmt.Sprintf("%s.%s.%s", FieldComponents, compName, key)
					comments[commentKey] = buildComment(paramMap)
				} else {
					// If it's not a map, use it directly
					compValues[key] = param
				}
			}
			componentValues[compName] = compValues
		}
		values[FieldComponents] = componentValues
	}

	return values, comments
}

// extractValueFromParam extracts a value from a parameter definition, handling nested structures
// Priority: default > example > generated from type definition
func extractValueFromParam(paramMap map[string]interface{}, path []string) interface{} {
	// First, check if there's a direct default or example value
	if defaultVal, ok := paramMap["default"]; ok {
		return defaultVal
	}
	if exampleVal, ok := paramMap["example"]; ok {
		return exampleVal
	}

	// If no direct value, check the type and generate appropriate value
	paramType := getString(paramMap, "type", "string")

	switch paramType {
	case "array", "list":
		// Check if items have example or default
		if items, ok := paramMap["items"]; ok {
			// Handle both map[string]interface{} and map[interface{}]interface{} (from YAML)
			var itemsMap map[string]interface{}
			if m, ok := items.(map[string]interface{}); ok {
				itemsMap = m
			} else if m, ok := items.(map[interface{}]interface{}); ok {
				// Convert map[interface{}]interface{} to map[string]interface{}
				itemsMap = make(map[string]interface{})
				for k, v := range m {
					if keyStr, ok := k.(string); ok {
						itemsMap[keyStr] = v
					}
				}
			}

			if itemsMap != nil {
				// Check if items have example
				if exampleVal, ok := itemsMap["example"]; ok {
					// If example is an array, return it; otherwise wrap in array
					if arr, ok := exampleVal.([]interface{}); ok {
						return arr
					}
					return []interface{}{exampleVal}
				}
				// Check if items have default
				if defaultVal, ok := itemsMap["default"]; ok {
					if arr, ok := defaultVal.([]interface{}); ok {
						return arr
					}
					return []interface{}{defaultVal}
				}
				// If items is an object, generate an example object
				itemsType := getString(itemsMap, "type", "")
				if itemsType == "object" {
					if properties, ok := itemsMap["properties"]; ok {
						// Handle both map[string]interface{} and map[interface{}]interface{}
						var propsMap map[string]interface{}
						if m, ok := properties.(map[string]interface{}); ok {
							propsMap = m
						} else if m, ok := properties.(map[interface{}]interface{}); ok {
							// Convert map[interface{}]interface{} to map[string]interface{}
							propsMap = make(map[string]interface{})
							for k, v := range m {
								if keyStr, ok := k.(string); ok {
									propsMap[keyStr] = v
								}
							}
						}

						if propsMap != nil && len(propsMap) > 0 {
							// Generate an example object from properties
							exampleObj := generateExampleFromProperties(propsMap, append(path, "items"))
							// Always return array with one example object if we have properties
							// This shows the structure even if some fields are empty
							return []interface{}{exampleObj}
						}
					}
				}
			}
		}
		// No items definition or no example, return empty array
		return []interface{}{}

	case "object", "map":
		// Check if properties have examples
		if properties, ok := paramMap["properties"]; ok {
			// Handle both map[string]interface{} and map[interface{}]interface{}
			var propsMap map[string]interface{}
			if m, ok := properties.(map[string]interface{}); ok {
				propsMap = m
			} else if m, ok := properties.(map[interface{}]interface{}); ok {
				// Convert map[interface{}]interface{} to map[string]interface{}
				propsMap = make(map[string]interface{})
				for k, v := range m {
					if keyStr, ok := k.(string); ok {
						propsMap[keyStr] = v
					}
				}
			}

			if propsMap != nil {
				return generateExampleFromProperties(propsMap, path)
			}
		}
		// Check if there's an example object directly
		if exampleVal, ok := paramMap["example"]; ok {
			if obj, ok := exampleVal.(map[string]interface{}); ok {
				return obj
			} else if obj, ok := exampleVal.(map[interface{}]interface{}); ok {
				// Convert map[interface{}]interface{} to map[string]interface{}
				result := make(map[string]interface{})
				for k, v := range obj {
					if keyStr, ok := k.(string); ok {
						result[keyStr] = v
					}
				}
				return result
			}
		}
		// No properties or example, return empty object
		return make(map[string]interface{})

	case "boolean", "bool":
		return false

	case "number", "integer", "int":
		return 0

	default:
		return ""
	}
}

// generateExampleFromProperties generates an example object from properties definition
func generateExampleFromProperties(properties map[string]interface{}, path []string) map[string]interface{} {
	result := make(map[string]interface{})

	for propName, propDef := range properties {
		// Handle both map[string]interface{} and map[interface{}]interface{} (from YAML)
		var propMap map[string]interface{}
		if m, ok := propDef.(map[string]interface{}); ok {
			propMap = m
		} else if m, ok := propDef.(map[interface{}]interface{}); ok {
			// Convert map[interface{}]interface{} to map[string]interface{}
			propMap = make(map[string]interface{})
			for k, v := range m {
				if keyStr, ok := k.(string); ok {
					propMap[keyStr] = v
				}
			}
		}

		if propMap != nil {
			// Recursively extract value from property definition
			value := extractValueFromParam(propMap, append(path, propName))
			// Always include the value if it has example or default, or if it's required
			// This ensures we generate a useful example object
			hasExample := false
			hasDefault := false
			if _, ok := propMap["example"]; ok {
				hasExample = true
			}
			if _, ok := propMap["default"]; ok {
				hasDefault = true
			}
			required := false
			if req, ok := propMap["required"].(bool); ok {
				required = req
			}

			// Include if:
			// 1. Has example or default (value will be non-empty)
			// 2. Is required (even if empty, to show structure)
			// 3. Value is false or 0 (valid values)
			// 4. Value is not empty string
			if hasExample || hasDefault || required {
				result[propName] = value
			} else if value == false || value == 0 {
				result[propName] = value
			} else if value != nil && value != "" {
				result[propName] = value
			}
		} else {
			// If it's not a map, use it directly
			result[propName] = propDef
		}
	}

	return result
}

// getString safely gets a string value from a map
func getString(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// getDefaultValue returns a default value based on type
func getDefaultValue(paramType string) interface{} {
	switch paramType {
	case "array", "list":
		return []interface{}{}
	case "object", "map":
		return make(map[string]interface{})
	case "boolean", "bool":
		return false
	case "number", "integer", "int":
		return 0
	default:
		return ""
	}
}

// buildComment builds a comment string from parameter metadata
func buildComment(paramMap map[string]interface{}) string {
	var parts []string

	if desc, ok := paramMap["description"].(string); ok && desc != "" {
		parts = append(parts, desc)
	}

	if required, ok := paramMap["required"].(bool); ok && required {
		parts = append(parts, "(required)")
	}

	if example, ok := paramMap["example"]; ok && example != nil {
		parts = append(parts, fmt.Sprintf("example: %v", example))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " - ")
}
