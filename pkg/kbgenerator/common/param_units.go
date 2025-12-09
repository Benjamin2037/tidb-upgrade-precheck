// Package common provides parameter unit mapping for configuration extraction
package common

import (
	"fmt"
	"regexp"
	"strings"
)

// getParameterUnit determines the unit for a parameter based on its name
// Returns the unit suffix (e.g., "s" for seconds, "MB" for megabytes) or empty string if unknown
func getParameterUnit(paramName string) string {
	paramName = strings.ToLower(paramName)
	
	// Time-related parameters (duration)
	timeParams := []string{
		"lease", "timeout", "interval", "duration", "ttl", "expire", "age",
		"wait", "delay", "period", "refresh", "retry", "backoff",
		"heartbeat", "tick", "check", "scan", "gc", "compact",
	}
	for _, timeParam := range timeParams {
		if strings.Contains(paramName, timeParam) {
			return "s" // Default to seconds
		}
	}
	
	// Size-related parameters
	sizeParams := []string{
		"size", "capacity", "limit", "quota", "threshold", "max-size", "min-size",
		"buffer", "cache", "memory", "storage", "space", "reserve",
		"merge-region-size", "split-size", "region-size", "file-size",
		"block-size", "chunk-size", "batch-size", "entry-size",
	}
	for _, sizeParam := range sizeParams {
		if strings.Contains(paramName, sizeParam) {
			// Determine appropriate size unit based on parameter name
			if strings.Contains(paramName, "region") || strings.Contains(paramName, "merge") {
				return "MB" // Region sizes are typically in MB
			}
			if strings.Contains(paramName, "block") || strings.Contains(paramName, "chunk") {
				return "KB" // Block/chunk sizes are typically in KB
			}
			if strings.Contains(paramName, "cache") || strings.Contains(paramName, "memory") {
				return "MB" // Cache/memory sizes are typically in MB
			}
			return "MB" // Default to MB for size parameters
		}
	}
	
	// Count-related parameters (no unit)
	countParams := []string{
		"count", "num", "number", "workers", "threads", "concurrency",
		"pool-size", "queue-size", "batch-count",
	}
	for _, countParam := range countParams {
		if strings.Contains(paramName, countParam) {
			return "" // No unit for counts
		}
	}
	
	return ""
}

// applyParameterUnit applies the appropriate unit to a numeric value based on parameter name
// Only applies unit if the value is a number and doesn't already have a unit
func applyParameterUnit(paramName string, value interface{}) interface{} {
	// Check if value already has a unit (string ending with s, m, h, ms, B, KB, MB, GB, etc.)
	if strVal, ok := value.(string); ok {
		// Check if it already has a unit
		if matched, _ := regexp.MatchString(`^\d+(s|m|h|ms|us|ns|B|KB|MB|GB|TB)$`, strVal); matched {
			return value // Already has unit, don't modify
		}
	}
	
	unit := getParameterUnit(paramName)
	if unit == "" {
		return value
	}
	
	// Only apply unit if value is a number
	switch v := value.(type) {
	case float64:
		if v == float64(int64(v)) {
			// Integer value
			return fmt.Sprintf("%.0f%s", v, unit)
		}
		return fmt.Sprintf("%v%s", v, unit)
	case int, int32, int64:
		return fmt.Sprintf("%d%s", v, unit)
	}
	
	return value
}

