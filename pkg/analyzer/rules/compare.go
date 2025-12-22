// Package rules provides standardized rule definitions for upgrade precheck
package rules

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
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

		// Build field path for recursive calls
		fieldPath := key
		if opts.BasePath != "" {
			fieldPath = fmt.Sprintf("%s.%s", opts.BasePath, key)
		}

		// Note: Parameter filtering is done at report generation time, not during comparison

		if !exists {
			// Field exists in current but not in source
			result[key] = MapDiff{Current: currentVal, Source: nil}
		} else if IsMapType(currentVal) && IsMapType(sourceVal) {
			// Recursively compare nested maps
			nestedOpts := CompareOptions{
				BasePath: fieldPath,
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
			// Note: Parameter filtering is done at report generation time, not during comparison
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
		BasePath: opts.BasePath,
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
		// Try to parse string as number to handle scientific notation
		str := val.String()
		// Try parsing as float64 first (handles scientific notation)
		if f, err := strconv.ParseFloat(str, 64); err == nil {
			// Successfully parsed as float, format it properly
			if f == float64(int64(f)) {
				// Whole number, format as integer to avoid scientific notation
				return fmt.Sprintf("%.0f", f)
			}
			// Decimal number, use %f to avoid scientific notation
			if f >= 1 && f < 1000000 {
				return fmt.Sprintf("%.6f", f)
			} else if f >= 0.001 && f < 1 {
				return fmt.Sprintf("%.9f", f)
			} else {
				return fmt.Sprintf("%.0f", f)
			}
		}
		// Not a number, return as quoted string
		return fmt.Sprintf("%q", v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Format integers without scientific notation
		return fmt.Sprintf("%d", val.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		// Format unsigned integers without scientific notation
		return fmt.Sprintf("%d", val.Uint())
	case reflect.Float32, reflect.Float64:
		// For floats, check if it's a whole number
		f := val.Float()
		if f == float64(int64(f)) {
			// Whole number, format as integer to avoid scientific notation
			return fmt.Sprintf("%.0f", f)
		}
		// Decimal number, use %f to avoid scientific notation
		// Determine appropriate precision based on magnitude
		if f >= 1 && f < 1000000 {
			// For numbers in this range, use up to 6 decimal places
			return fmt.Sprintf("%.6f", f)
		} else if f >= 0.001 && f < 1 {
			// For small decimals, use more precision
			return fmt.Sprintf("%.9f", f)
		} else {
			// For very large numbers, use %f with no decimal places (they should be integers anyway)
			return fmt.Sprintf("%.0f", f)
		}
	default:
		// Try to convert to string and parse as number if possible
		str := fmt.Sprintf("%v", v)
		// Try parsing as float64 to handle scientific notation
		if f, err := strconv.ParseFloat(str, 64); err == nil {
			// Successfully parsed as float, format it properly
			if f == float64(int64(f)) {
				// Whole number, format as integer to avoid scientific notation
				return fmt.Sprintf("%.0f", f)
			}
			// Decimal number, use %f to avoid scientific notation
			if f >= 1 && f < 1000000 {
				return fmt.Sprintf("%.6f", f)
			} else if f >= 0.001 && f < 1 {
				return fmt.Sprintf("%.9f", f)
			} else {
				return fmt.Sprintf("%.0f", f)
			}
		}
		// Not a number, return as-is
		return str
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
			BasePath: "",
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

// CompareValues compares two values properly, handling numeric types to avoid scientific notation issues
// Returns true if values are equal, false otherwise
func CompareValues(v1, v2 interface{}) bool {
	if v1 == nil && v2 == nil {
		return true
	}
	if v1 == nil || v2 == nil {
		return false
	}

	// Try to convert both values to float64 for numeric comparison
	// This handles cases where one value is a number and the other is a string (e.g., "1.44e+06")
	var f1, f2 float64
	var ok1, ok2 bool

	val1 := reflect.ValueOf(v1)
	val2 := reflect.ValueOf(v2)

	// Try to get numeric value from v1
	if isNumeric(val1) {
		f1 = toFloat64(val1)
		ok1 = true
	} else if val1.Kind() == reflect.String {
		// Try parsing string as float (handles scientific notation)
		if parsed, err := strconv.ParseFloat(val1.String(), 64); err == nil {
			f1 = parsed
			ok1 = true
		}
	}

	// Try to get numeric value from v2
	if isNumeric(val2) {
		f2 = toFloat64(val2)
		ok2 = true
	} else if val2.Kind() == reflect.String {
		// Try parsing string as float (handles scientific notation)
		if parsed, err := strconv.ParseFloat(val2.String(), 64); err == nil {
			f2 = parsed
			ok2 = true
		}
	}

	// If both values can be converted to float64, compare numerically
	if ok1 && ok2 {
		return f1 == f2
	}

	// For non-numeric types or when parsing fails, use string comparison with proper formatting
	return FormatValue(v1) == FormatValue(v2)
}

// isNumeric checks if a reflect.Value is a numeric type
func isNumeric(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// toFloat64 converts a reflect.Value to float64
func toFloat64(v reflect.Value) float64 {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint())
	case reflect.Float32, reflect.Float64:
		return v.Float()
	default:
		return 0
	}
}

// ToNumeric converts an interface{} value to a numeric value (float64)
// Returns the numeric value and true if conversion was successful
// This function handles various numeric types and string representations
func ToNumeric(v interface{}) (float64, bool) {
	if v == nil {
		return 0, false
	}

	// Try to parse as float64
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int8:
		return float64(val), true
	case int16:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint8:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	case float32:
		return float64(val), true
	case float64:
		return val, true
	case string:
		// Try parsing string as float
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// filenameOnlyParams contains parameters that should be compared by filename only (ignoring path)
// This is used for special comparison strategy during rule evaluation
var filenameOnlyParams = map[string]bool{
	"log.file.filename":    true,
	"log.slow-query-file":  true,
	"log-file":             true,
}

// IsFilenameOnlyParameter checks if a parameter should be compared by filename only (ignoring path)
// This is used during rule evaluation for special comparison strategy, not for filtering
func IsFilenameOnlyParameter(paramName string) bool {
	return filenameOnlyParams[paramName]
}
