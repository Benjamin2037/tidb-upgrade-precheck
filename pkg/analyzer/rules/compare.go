// Package rules provides standardized rule definitions for upgrade precheck
package rules

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// MapDiff represents a difference between two map values
type MapDiff struct {
	Current interface{}
	Source  interface{}
	Target  interface{} // Optional, for three-way comparison
}

// CompareOptions provides options for map comparison
type CompareOptions struct {
	// IgnoredParams is a map of parameter names to ignore (top-level only)
	IgnoredParams map[string]bool
	// BasePath is used for nested map field paths
	BasePath string
}

// IsMapType checks if a value is a map type
func IsMapType(v interface{}) bool {
	if v == nil {
		return false
	}
	val := reflect.ValueOf(v)
	return val.Kind() == reflect.Map
}

// IsSliceType checks if a value is a slice or array type
func IsSliceType(v interface{}) bool {
	if v == nil {
		return false
	}
	val := reflect.ValueOf(v)
	return val.Kind() == reflect.Slice || val.Kind() == reflect.Array
}

// ConvertToMapStringInterface converts various map types to map[string]interface{}
func ConvertToMapStringInterface(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}

	// Direct conversion
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}

	// Try map[interface{}]interface{} (common from YAML unmarshaling)
	if m, ok := v.(map[interface{}]interface{}); ok {
		result := make(map[string]interface{})
		for k, val := range m {
			key := fmt.Sprintf("%v", k)
			result[key] = val
		}
		return result
	}

	// Use reflection for other map types
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Map {
		return nil
	}

	result := make(map[string]interface{})
	for _, key := range val.MapKeys() {
		keyStr := fmt.Sprintf("%v", key.Interface())
		result[keyStr] = val.MapIndex(key).Interface()
	}
	return result
}

// CompareMapsDeep compares two maps and returns only the differing fields
// Returns a map of field paths to their differences
func CompareMapsDeep(current, source interface{}, opts CompareOptions) map[string]MapDiff {
	result := make(map[string]MapDiff)

	// Convert to map[string]interface{} if possible
	currentMap := ConvertToMapStringInterface(current)
	sourceMap := ConvertToMapStringInterface(source)

	if currentMap == nil || sourceMap == nil {
		// If not both maps, fall back to simple comparison
		if fmt.Sprintf("%v", current) != fmt.Sprintf("%v", source) {
			result[""] = MapDiff{Current: current, Source: source}
		}
		return result
	}

	// Check all fields in current map
	for key, currentVal := range currentMap {
		sourceVal, exists := sourceMap[key]
		// Only check top-level parameter name for ignore list (not nested map fields)
		// For nested maps, we want to show all differences
		if opts.BasePath == "" && opts.IgnoredParams != nil && opts.IgnoredParams[key] {
			continue
		}

		// Build field path for recursive calls
		fieldPath := key
		if opts.BasePath != "" {
			fieldPath = fmt.Sprintf("%s.%s", opts.BasePath, key)
		}

		// Check if this field path (including nested paths) should be ignored
		if opts.IgnoredParams != nil {
			// Check full path (e.g., "log.file.filename")
			if opts.IgnoredParams[fieldPath] {
				continue
			}
			// Also check just the key for top-level parameters
			if opts.BasePath == "" && opts.IgnoredParams[key] {
				continue
			}
		}

		if !exists {
			// Field exists in current but not in source
			result[key] = MapDiff{Current: currentVal, Source: nil}
		} else if IsMapType(currentVal) && IsMapType(sourceVal) {
			// Recursively compare nested maps
			nestedOpts := CompareOptions{
				IgnoredParams: opts.IgnoredParams,
				BasePath:      fieldPath,
			}
			nestedDiffs := CompareMapsDeep(currentVal, sourceVal, nestedOpts)
			for nestedKey, nestedDiff := range nestedDiffs {
				if nestedKey == "" {
					result[key] = nestedDiff
				} else {
					result[fmt.Sprintf("%s.%s", key, nestedKey)] = nestedDiff
				}
			}
		} else if fmt.Sprintf("%v", currentVal) != fmt.Sprintf("%v", sourceVal) {
			// Simple value comparison
			result[key] = MapDiff{Current: currentVal, Source: sourceVal}
		}
	}

	// Check fields in source map that don't exist in current
	for key, sourceVal := range sourceMap {
		if _, exists := currentMap[key]; !exists {
			// Only check top-level parameter name for ignore list (not nested map fields)
			if opts.BasePath == "" && opts.IgnoredParams != nil && opts.IgnoredParams[key] {
				continue
			}
			result[key] = MapDiff{Current: nil, Source: sourceVal}
		}
	}

	return result
}

// CompareMapsThreeWay compares three maps (current, source, target) and returns differences
// This is useful for upgrade scenarios where we need to compare current vs source vs target
func CompareMapsThreeWay(current, source, target interface{}, opts CompareOptions) map[string]MapDiff {
	result := make(map[string]MapDiff)

	// First, compare source and target to find default value changes
	sourceTargetDiffs := CompareMapsDeep(source, target, CompareOptions{
		IgnoredParams: opts.IgnoredParams,
		BasePath:      opts.BasePath,
	})

	// Then, compare current with source and target
	currentSourceDiffs := CompareMapsDeep(current, source, opts)
	currentTargetDiffs := CompareMapsDeep(current, target, opts)

	// Merge all differences
	allKeys := make(map[string]bool)
	for k := range sourceTargetDiffs {
		allKeys[k] = true
	}
	for k := range currentSourceDiffs {
		allKeys[k] = true
	}
	for k := range currentTargetDiffs {
		allKeys[k] = true
	}

	for key := range allKeys {
		diff := MapDiff{}
		if sd, ok := sourceTargetDiffs[key]; ok {
			diff.Source = sd.Source
			diff.Target = sd.Current // In sourceTargetDiffs, Current is actually target
		}
		if csd, ok := currentSourceDiffs[key]; ok {
			diff.Current = csd.Current
			if diff.Source == nil {
				diff.Source = csd.Source
			}
		}
		if ctd, ok := currentTargetDiffs[key]; ok {
			if diff.Current == nil {
				diff.Current = ctd.Current
			}
			if diff.Target == nil {
				diff.Target = ctd.Source // In currentTargetDiffs, Source is actually target
			}
		}
		result[key] = diff
	}

	return result
}

// FormatValue formats a value in a readable way
func FormatValue(v interface{}) string {
	if v == nil {
		return "<nil>"
	}

	// For map/slice types, use JSON formatting for readability
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		jsonBytes, err := json.MarshalIndent(v, "", "  ")
		if err == nil {
			return string(jsonBytes)
		}
		// Fall back to simple format if JSON marshaling fails
		return fmt.Sprintf("%v", v)
	case reflect.String:
		return fmt.Sprintf("%q", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// FormatValueDiff formats the difference between current and source values in a clear, readable way
func FormatValueDiff(current, source interface{}) string {
	var currentStr, sourceStr string

	// Format current value
	if current == nil {
		currentStr = "<not set>"
	} else {
		currentStr = FormatValue(current)
	}

	// Format source value
	if source == nil {
		sourceStr = "<not set>"
	} else {
		sourceStr = FormatValue(source)
	}

	// For simple values (non-map, non-slice), use compact format
	if !IsMapType(current) && !IsMapType(source) && !IsSliceType(current) && !IsSliceType(source) {
		return fmt.Sprintf("Current: %s | Source Default: %s", currentStr, sourceStr)
	}

	// For complex types, use multi-line format
	return fmt.Sprintf("Current Value:\n%s\n\nSource Default:\n%s", currentStr, sourceStr)
}

// FormatValueDiffThreeWay formats the difference between current, source, and target values
func FormatValueDiffThreeWay(current, source, target interface{}) string {
	var currentStr, sourceStr, targetStr string

	// Format values
	if current == nil {
		currentStr = "<not set>"
	} else {
		currentStr = FormatValue(current)
	}

	if source == nil {
		sourceStr = "<not set>"
	} else {
		sourceStr = FormatValue(source)
	}

	if target == nil {
		targetStr = "<not set>"
	} else {
		targetStr = FormatValue(target)
	}

	// For simple values, use compact format
	if !IsMapType(current) && !IsMapType(source) && !IsMapType(target) &&
		!IsSliceType(current) && !IsSliceType(source) && !IsSliceType(target) {
		return fmt.Sprintf("Current: %s | Source Default: %s | Target Default: %s", currentStr, sourceStr, targetStr)
	}

	// For complex types, use multi-line format
	return fmt.Sprintf("Current Value:\n%s\n\nSource Default:\n%s\n\nTarget Default:\n%s", currentStr, sourceStr, targetStr)
}

// FormatDefaultChangeDiff formats the difference for default value changes
// For map types, it only shows differing fields between source and target defaults
// For simple types, it uses compact format
func FormatDefaultChangeDiff(current, source, target interface{}, ignoredParams map[string]bool) string {
	// For map types, do deep comparison to find only differing fields
	if IsMapType(source) && IsMapType(target) {
		// Compare source and target to find differences
		opts := CompareOptions{
			IgnoredParams: ignoredParams,
			BasePath:      "",
		}
		sourceTargetDiffs := CompareMapsDeep(source, target, opts)

		if len(sourceTargetDiffs) == 0 {
			// No differences between source and target, but check if current differs
			if IsMapType(current) {
				currentSourceDiffs := CompareMapsDeep(current, source, opts)
				currentTargetDiffs := CompareMapsDeep(current, target, opts)
				if len(currentSourceDiffs) > 0 || len(currentTargetDiffs) > 0 {
					// Current differs from defaults, but source == target
					var parts []string
					parts = append(parts, "Current value differs from defaults (source and target defaults are the same):")
					allDiffs := make(map[string]MapDiff)
					for k, v := range currentSourceDiffs {
						allDiffs[k] = v
					}
					for k, v := range currentTargetDiffs {
						if _, exists := allDiffs[k]; !exists {
							allDiffs[k] = v
						}
					}
					for fieldPath, diff := range allDiffs {
						if fieldPath == "" {
							parts = append(parts, fmt.Sprintf("  - Current: %s | Default: %s", FormatValue(diff.Current), FormatValue(diff.Source)))
						} else {
							parts = append(parts, fmt.Sprintf("  - %s: Current: %s | Default: %s", fieldPath, FormatValue(diff.Current), FormatValue(diff.Source)))
						}
					}
					return strings.Join(parts, "\n")
				}
			}
			return "No differences between source and target defaults"
		}

		// Show differences between source and target
		var parts []string
		parts = append(parts, "Default value changes (Source → Target):")
		for fieldPath, diff := range sourceTargetDiffs {
			if fieldPath == "" {
				parts = append(parts, fmt.Sprintf("  - %s → %s", FormatValue(diff.Source), FormatValue(diff.Current)))
			} else {
				parts = append(parts, fmt.Sprintf("  - %s: %s → %s", fieldPath, FormatValue(diff.Source), FormatValue(diff.Current)))
			}
		}

		// If current is also a map and differs from target, show that too
		if IsMapType(current) {
			currentTargetDiffs := CompareMapsDeep(current, target, opts)
			if len(currentTargetDiffs) > 0 {
				parts = append(parts, "")
				parts = append(parts, "Current value vs Target default:")
				for fieldPath, diff := range currentTargetDiffs {
					if fieldPath == "" {
						parts = append(parts, fmt.Sprintf("  - Current: %s | Target Default: %s", FormatValue(diff.Current), FormatValue(diff.Source)))
					} else {
						parts = append(parts, fmt.Sprintf("  - %s: Current: %s | Target Default: %s", fieldPath, FormatValue(diff.Current), FormatValue(diff.Source)))
					}
				}
			}
		}

		return strings.Join(parts, "\n")
	}

	// For non-map types, use simple format
	currentStr := FormatValue(current)
	sourceStr := FormatValue(source)
	targetStr := FormatValue(target)

	// Check if source and target are different
	if fmt.Sprintf("%v", source) != fmt.Sprintf("%v", target) {
		return fmt.Sprintf("Source Default: %s → Target Default: %s\nCurrent: %s", sourceStr, targetStr, currentStr)
	}

	return fmt.Sprintf("Current: %s | Source Default: %s | Target Default: %s", currentStr, sourceStr, targetStr)
}

// getNestedMapValue gets a value from a nested map using a path (e.g., ["backup", "num-threads"])
func getNestedMapValue(m map[string]interface{}, path []string) interface{} {
	if m == nil || len(path) == 0 {
		return nil
	}

	current := m
	for i, key := range path {
		if i == len(path)-1 {
			// Last key, return the value
			return current[key]
		}
		// Not the last key, go deeper
		if nextMap, ok := current[key].(map[string]interface{}); ok {
			current = nextMap
		} else {
			return nil
		}
	}
	return nil
}

// ExtractFileName extracts the filename from a file path
// Returns the filename without the directory path
func ExtractFileName(filePath interface{}) string {
	if filePath == nil {
		return ""
	}
	pathStr := fmt.Sprintf("%v", filePath)
	if pathStr == "" {
		return ""
	}
	// Handle both Unix and Windows path separators
	pathStr = strings.ReplaceAll(pathStr, "\\", "/")
	parts := strings.Split(pathStr, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return pathStr
}

// CompareFileNames compares two file paths by filename only (ignoring directory path)
// Returns true if filenames are the same, false otherwise
func CompareFileNames(path1, path2 interface{}) bool {
	filename1 := ExtractFileName(path1)
	filename2 := ExtractFileName(path2)
	return filename1 == filename2 && filename1 != ""
}

// IsPathParameter checks if a parameter name indicates a path-related parameter
// Returns true if the parameter name contains path-related keywords
func IsPathParameter(paramName string) bool {
	paramNameLower := strings.ToLower(paramName)
	pathKeywords := []string{
		"path", "dir", "file", "log", "data", "deploy", "temp", "tmp",
		"storage", "socket", "home", "root", "cache", "config",
	}
	for _, keyword := range pathKeywords {
		if strings.Contains(paramNameLower, keyword) {
			return true
		}
	}
	return false
}
