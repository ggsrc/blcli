// Package validator runs validation rules from template args definitions against merged args data.
// Rules are declared in bl-template as validation list (each item is a map with kind + params);
// supported kinds: required, stringLength, pattern, format, enum, numberRange.
// Top-level validation.unique is also supported (path + message).
package validator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"blcli/pkg/renderer"
)

// Rule runs a single validation rule. Each rule is a map with "kind" and kind-specific params.
func Rule(value interface{}, rule map[string]interface{}) error {
	if rule == nil {
		return nil
	}
	kind, _ := rule["kind"].(string)
	switch kind {
	case "required":
		return ruleRequired(value, rule)
	case "stringLength":
		return ruleStringLength(value, rule)
	case "pattern":
		return rulePattern(value, rule)
	case "format":
		return ruleFormat(value, rule)
	case "enum":
		return ruleEnum(value, rule)
	case "numberRange":
		return ruleNumberRange(value, rule)
	default:
		if kind != "" {
			return fmt.Errorf("validation: unknown rule kind %q", kind)
		}
	}
	return nil
}

func ruleRequired(value interface{}, rule map[string]interface{}) error {
	if value == nil {
		return fmt.Errorf("%s", getMessage(rule, "value is required"))
	}
	if s, ok := value.(string); ok && strings.TrimSpace(s) == "" {
		return fmt.Errorf("%s", getMessage(rule, "value is required"))
	}
	return nil
}

func ruleStringLength(value interface{}, rule map[string]interface{}) error {
	s, ok := toString(value)
	if !ok {
		return fmt.Errorf("%s", getMessage(rule, "expected string"))
	}
	min, hasMin := getInt(rule, "min")
	max, hasMax := getInt(rule, "max")
	n := len(s)
	if hasMin && n < min {
		return fmt.Errorf("%s", getMessage(rule, fmt.Sprintf("length must be at least %d", min)))
	}
	if hasMax && n > max {
		return fmt.Errorf("%s", getMessage(rule, fmt.Sprintf("length must be at most %d", max)))
	}
	return nil
}

func rulePattern(value interface{}, rule map[string]interface{}) error {
	s, ok := toString(value)
	if !ok {
		return fmt.Errorf("%s", getMessage(rule, "expected string"))
	}
	pattern, _ := rule["value"].(string)
	if pattern == "" {
		pattern, _ = rule["pattern"].(string)
	}
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("validation pattern invalid: %w", err)
	}
	if !re.MatchString(s) {
		return fmt.Errorf("%s", getMessage(rule, "value does not match required pattern"))
	}
	return nil
}

func ruleFormat(value interface{}, rule map[string]interface{}) error {
	s, ok := toString(value)
	if !ok {
		return fmt.Errorf("%s", getMessage(rule, "expected string"))
	}
	format, _ := rule["value"].(string)
	if format == "" {
		format, _ = rule["format"].(string)
	}
	switch format {
	case "email":
		// simple email check
		if !regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).MatchString(s) {
			return fmt.Errorf("%s", getMessage(rule, "invalid email format"))
		}
	case "numeric", "number":
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("%s", getMessage(rule, "expected numeric value"))
		}
	default:
		// optional custom regex via "pattern" in same rule
		if pattern, _ := rule["pattern"].(string); pattern != "" {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("validation format pattern invalid: %w", err)
			}
			if !re.MatchString(s) {
				return fmt.Errorf("%s", getMessage(rule, "invalid format"))
			}
		}
	}
	return nil
}

func ruleEnum(value interface{}, rule map[string]interface{}) error {
	var allowed []string
	if v, ok := rule["values"].([]interface{}); ok {
		for _, item := range v {
			if s, ok := item.(string); ok {
				allowed = append(allowed, s)
			}
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	s, ok := toString(value)
	if !ok {
		return fmt.Errorf("%s", getMessage(rule, "expected string"))
	}
	for _, a := range allowed {
		if s == a {
			return nil
		}
	}
	return fmt.Errorf("%s", getMessage(rule, fmt.Sprintf("value must be one of: %s", strings.Join(allowed, ", "))))
}

func ruleNumberRange(value interface{}, rule map[string]interface{}) error {
	var num float64
	switch v := value.(type) {
	case float64:
		num = v
	case int:
		num = float64(v)
	case int64:
		num = float64(v)
	case string:
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("%s", getMessage(rule, "expected number"))
		}
		num = n
	default:
		return fmt.Errorf("%s", getMessage(rule, "expected number"))
	}
	min, hasMin := getFloat(rule, "min")
	max, hasMax := getFloat(rule, "max")
	if hasMin && num < min {
		return fmt.Errorf("%s", getMessage(rule, fmt.Sprintf("must be >= %v", min)))
	}
	if hasMax && num > max {
		return fmt.Errorf("%s", getMessage(rule, fmt.Sprintf("must be <= %v", max)))
	}
	return nil
}

func getMessage(rule map[string]interface{}, defaultMsg string) string {
	if msg, ok := rule["message"].(string); ok && msg != "" {
		return msg
	}
	return defaultMsg
}

func toString(v interface{}) (string, bool) {
	if v == nil {
		return "", false
	}
	if s, ok := v.(string); ok {
		return s, true
	}
	return fmt.Sprintf("%v", v), true
}

func getInt(m map[string]interface{}, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}

func getFloat(m map[string]interface{}, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// RulesFromParam returns validation rules from a parameter definition map.
// It reads the "validation" key (list of rule maps). Legacy "required" and "pattern"
// at top level are also converted to rules if validation list is empty.
func RulesFromParam(param map[string]interface{}) []map[string]interface{} {
	if param == nil {
		return nil
	}
	if v, ok := param["validation"].([]interface{}); ok {
		var out []map[string]interface{}
		for _, item := range v {
			m := toRuleMap(item)
			if m != nil {
				out = append(out, m)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	// Legacy: single required / pattern
	var rules []map[string]interface{}
	if req, ok := param["required"].(bool); ok && req {
		rules = append(rules, map[string]interface{}{"kind": "required"})
	}
	if pattern, ok := param["pattern"].(string); ok && pattern != "" {
		rules = append(rules, map[string]interface{}{"kind": "pattern", "value": pattern})
	}
	return rules
}

// toRuleMap converts a rule from YAML (possibly map[interface{}]interface{}) to map[string]interface{}.
func toRuleMap(item interface{}) map[string]interface{} {
	if m, ok := item.(map[string]interface{}); ok {
		return m
	}
	if m, ok := item.(map[interface{}]interface{}); ok {
		out := make(map[string]interface{})
		for k, v := range m {
			if s, ok := k.(string); ok {
				out[s] = v
			}
		}
		return out
	}
	return nil
}

// GetValue gets a value from ArgsData by dot path (no array index).
// e.g. "global.OrganizationID" -> data["global"].(map)["OrganizationID"]
func GetValue(data renderer.ArgsData, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	var current interface{} = data
	for _, p := range parts {
		if current == nil {
			return nil, false
		}
		m, ok := toMap(current)
		if !ok {
			return nil, false
		}
		current = m[p]
	}
	if current == nil {
		return nil, false
	}
	return current, true
}

// GetValues collects all values at a path that may contain one array segment.
// Path format: "terraform.projects[].global.ProjectName" -> collect ProjectName for each project.
// Returns (values, true) or (nil, false) if path invalid or no values.
func GetValues(data renderer.ArgsData, path string) ([]interface{}, bool) {
	// Split by "[]" to get segments: "terraform.projects[].global.ProjectName" -> ["terraform.projects", "global.ProjectName"]
	segments := splitPathWithArray(path)
	if len(segments) == 0 {
		return nil, false
	}
	var collect func(interface{}, int) []interface{}
	collect = func(node interface{}, segIndex int) []interface{} {
		if node == nil {
			return nil
		}
		if segIndex >= len(segments) {
			return []interface{}{node}
		}
		seg := segments[segIndex]
		if seg == "[]" {
			// current node must be slice; recurse on each element
			slice, ok := toSlice(node)
			if !ok {
				return nil
			}
			var out []interface{}
			for _, item := range slice {
				out = append(out, collect(item, segIndex+1)...)
			}
			return out
		}
		m, ok := toMap(node)
		if !ok {
			return nil
		}
		keys := strings.Split(seg, ".")
		var current interface{} = m
		for _, k := range keys {
			if current == nil {
				return nil
			}
			mm, ok := toMap(current)
			if !ok {
				return nil
			}
			current = mm[k]
		}
		return collect(current, segIndex+1)
	}
	// Start from data, first segment is key path into data
	first := segments[0]
	if first == "[]" {
		return nil, false
	}
	keys := strings.Split(first, ".")
	var current interface{} = data
	for _, k := range keys {
		if current == nil {
			return nil, false
		}
		m, ok := toMap(current)
		if !ok {
			return nil, false
		}
		current = m[k]
	}
	vals := collect(current, 1)
	return vals, len(vals) >= 0
}

func splitPathWithArray(path string) []string {
	var segments []string
	var buf strings.Builder
	for i := 0; i < len(path); i++ {
		if path[i] == '[' && i+2 <= len(path) && path[i:i+2] == "[]" {
			if buf.Len() > 0 {
				segments = append(segments, strings.TrimSuffix(buf.String(), "."))
				buf.Reset()
			}
			segments = append(segments, "[]")
			i++
			continue
		}
		buf.WriteByte(path[i])
	}
	if buf.Len() > 0 {
		segments = append(segments, strings.TrimSuffix(buf.String(), "."))
	}
	return segments
}

func toMap(v interface{}) (map[string]interface{}, bool) {
	if m, ok := v.(map[string]interface{}); ok {
		return m, true
	}
	if m, ok := v.(renderer.ArgsData); ok {
		return m, true
	}
	// YAML unmarshal may produce map[interface{}]interface{}
	if m, ok := v.(map[interface{}]interface{}); ok {
		out := make(map[string]interface{})
		for k, val := range m {
			if s, ok := k.(string); ok {
				out[s] = val
			}
		}
		return out, true
	}
	return nil, false
}

func toSlice(v interface{}) ([]interface{}, bool) {
	if s, ok := v.([]interface{}); ok {
		return s, true
	}
	return nil, false
}

// CheckUnique checks that all values at the given path are unique.
func CheckUnique(data renderer.ArgsData, path, message string) error {
	vals, _ := GetValues(data, path)
	seen := make(map[string]bool)
	for _, v := range vals {
		var key string
		if v != nil {
			key = fmt.Sprintf("%v", v)
		}
		if seen[key] {
			if message != "" {
				return fmt.Errorf("%s", message)
			}
			return fmt.Errorf("duplicate value at %q: %v", path, v)
		}
		seen[key] = true
	}
	return nil
}
