package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	defaultsTypes "github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// BaselineResult reuses ComponentState from pkg/collector
// We use a type alias with JSON marshalling that converts time.Time to string for backward compatibility
type BaselineResult struct {
	Version    string                              `json:"version"`
	Timestamp  string                              `json:"timestamp"` // RFC3339 format string
	Components map[string]collector.ComponentState `json:"components"`
}

func compareBaselineWithArgs(baselineFile, knowledgeDir, version string) {
	if baselineFile == "" || knowledgeDir == "" || version == "" {
		fmt.Fprintf(os.Stderr, "Error: --baseline, --knowledge, and --version are required\n")
		os.Exit(1)
	}

	// Load baseline
	baseline, err := loadBaseline(baselineFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load baseline: %v\n", err)
		os.Exit(1)
	}

	// Load knowledge base
	kb, err := loadKnowledgeBase(knowledgeDir, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load knowledge base: %v\n", err)
		os.Exit(1)
	}

	// Compare with detailed 1-1 matching
	hasDiff := false
	totalConfigParams := 0
	matchedConfigParams := 0
	totalVariables := 0
	matchedVariables := 0

	for component, baselineConfig := range baseline.Components {
		fmt.Printf("\n========================================\n")
		fmt.Printf("Comparing %s component (1-1 matching)\n", strings.ToUpper(component))
		fmt.Printf("========================================\n")

		kbConfig, ok := kb[component]
		if !ok {
			fmt.Printf("  ❌ Component %s not found in knowledge base\n", component)
			hasDiff = true
			continue
		}

		// Compare config parameters (1-1)
		// Extract values from ConfigDefaults (map[string]ParameterValue) to map[string]interface{}
		baselineConfigMap := extractConfigValues(baselineConfig.Config)
		if len(baselineConfigMap) > 0 {
			fmt.Printf("\n  Config Parameters:\n")
			fmt.Printf("    Baseline has: %d parameters\n", len(baselineConfigMap))
			fmt.Printf("    KB has: %d parameters\n", len(kbConfig.Config))

			configDiffs := compareConfigs(baselineConfigMap, kbConfig.Config)
			totalConfigParams += len(baselineConfigMap) + len(kbConfig.Config)

			if len(configDiffs) > 0 {
				hasDiff = true
				matchedConfigParams += len(baselineConfigMap) + len(kbConfig.Config) - len(configDiffs)
				fmt.Printf("    ❌ Found %d differences:\n", len(configDiffs))
				for _, diff := range configDiffs {
					fmt.Printf("      - %s\n", diff)
				}
			} else {
				matchedConfigParams += len(baselineConfigMap)
				fmt.Printf("    ✓ All %d config parameters match (1-1)\n", len(baselineConfigMap))
			}
		} else if len(kbConfig.Config) > 0 {
			fmt.Printf("\n  Config Parameters:\n")
			fmt.Printf("    ⚠️  Baseline has no config, but KB has %d parameters\n", len(kbConfig.Config))
			hasDiff = true
		}

		// Compare system variables (for TiDB) - 1-1
		if component == "tidb" {
			// Extract values from SystemVariables (map[string]ParameterValue) to map[string]interface{}
			baselineVars := extractVariableValues(baselineConfig.Variables)
			// Filter out MySQL compatibility variables
			filteredBaselineVars := make(map[string]interface{})
			for k, v := range baselineVars {
				if !isMySQLCompatibilityVar(k) {
					filteredBaselineVars[k] = normalizeValue(v)
				}
			}
			baselineVars = filteredBaselineVars

			fmt.Printf("\n  System Variables:\n")
			fmt.Printf("    Baseline has: %d variables\n", len(baselineVars))
			fmt.Printf("    KB has: %d variables\n", len(kbConfig.Variables))

			if len(baselineVars) > 0 || len(kbConfig.Variables) > 0 {
				varDiffs := compareVariables(baselineVars, kbConfig.Variables)
				totalVariables += len(baselineVars) + len(kbConfig.Variables)

				if len(varDiffs) > 0 {
					hasDiff = true
					matchedVariables += len(baselineVars) + len(kbConfig.Variables) - len(varDiffs)
					fmt.Printf("    ❌ Found %d differences:\n", len(varDiffs))
					for _, diff := range varDiffs {
						fmt.Printf("      - %s\n", diff)
					}
				} else {
					matchedVariables += len(baselineVars)
					fmt.Printf("    ✓ All %d system variables match (1-1)\n", len(baselineVars))
				}
			}
		}
	}

	// Summary
	fmt.Printf("\n========================================\n")
	fmt.Printf("Comparison Summary\n")
	fmt.Printf("========================================\n")
	fmt.Printf("Config Parameters: %d matched / %d total\n", matchedConfigParams, totalConfigParams)
	fmt.Printf("System Variables: %d matched / %d total\n", matchedVariables, totalVariables)

	if hasDiff {
		fmt.Printf("\n⚠️  Differences found between baseline and knowledge base\n")
		os.Exit(1)
	} else {
		fmt.Printf("\n✓ Baseline matches knowledge base perfectly!\n")
	}
}

type KBConfig struct {
	Config    map[string]interface{} `json:"config_defaults,omitempty"`
	Variables map[string]interface{} `json:"system_variables,omitempty"`
}

// extractConfigValues extracts values from ConfigDefaults (map[string]ParameterValue) to map[string]interface{}
func extractConfigValues(config defaultsTypes.ConfigDefaults) map[string]interface{} {
	result := make(map[string]interface{})
	for k, paramValue := range config {
		result[k] = paramValue.Value
	}
	return result
}

// extractVariableValues extracts values from SystemVariables (map[string]ParameterValue) to map[string]interface{}
func extractVariableValues(variables defaultsTypes.SystemVariables) map[string]interface{} {
	result := make(map[string]interface{})
	for k, paramValue := range variables {
		result[k] = paramValue.Value
	}
	return result
}

func loadBaseline(filename string) (*BaselineResult, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var baseline BaselineResult
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, err
	}

	return &baseline, nil
}

func loadKnowledgeBase(kbDir, version string) (map[string]KBConfig, error) {
	// Use pkg/collector/kbgenerator.LoadKnowledgeBase to load the knowledge base
	// kbDir is the base knowledge directory (e.g., "knowledge")
	kb, err := kbgenerator.LoadKnowledgeBase(kbDir, version)
	if err != nil {
		return nil, fmt.Errorf("failed to load knowledge base: %w", err)
	}

	result := make(map[string]KBConfig)

	// Convert from pkg/collector/kbgenerator format to KBConfig format
	for comp, componentKB := range kb {
		compData, ok := componentKB.(map[string]interface{})
		if !ok {
			continue
		}

		kbConfig := KBConfig{
			Config:    make(map[string]interface{}),
			Variables: make(map[string]interface{}),
		}

		// Extract config_defaults
		if configDefaults, ok := compData["config_defaults"].(map[string]interface{}); ok {
			for paramName, paramData := range configDefaults {
				// Handle ParameterValue format: {"value": ..., "type": ..., "description": ...}
				if paramValue, ok := paramData.(map[string]interface{}); ok {
					if value, exists := paramValue["value"]; exists {
						kbConfig.Config[paramName] = normalizeValue(value)
					}
				} else {
					// Handle direct value format (for backward compatibility)
					kbConfig.Config[paramName] = normalizeValue(paramData)
				}
			}
		}

		// Extract system_variables (for TiDB)
		if systemVars, ok := compData["system_variables"].(map[string]interface{}); ok {
			for varName, varData := range systemVars {
				// Handle ParameterValue format
				if varValue, ok := varData.(map[string]interface{}); ok {
					if value, exists := varValue["value"]; exists {
						kbConfig.Variables[varName] = normalizeValue(value)
					}
				} else {
					// Handle direct value format
					kbConfig.Variables[varName] = normalizeValue(varData)
				}
			}
		}

		result[comp] = kbConfig
	}

	return result, nil
}

// isMySQLCompatibilityVar checks if a variable is a MySQL compatibility variable
// These variables are placeholders for MySQL compatibility but don't affect TiDB behavior
func isMySQLCompatibilityVar(varName string) bool {
	return strings.HasPrefix(varName, "innodb_") ||
		strings.HasPrefix(varName, "myisam_") ||
		strings.HasPrefix(varName, "performance_schema_") ||
		strings.HasPrefix(varName, "query_cache_")
}

// normalizeValue normalizes values for comparison (handles type conversions)
func normalizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case float64:
		// Check if it's actually an integer
		if val == float64(int64(val)) {
			return int64(val)
		}
		return val
	case bool, string, int, int64, int32, int16, int8:
		return val
	case []interface{}:
		return val
	case map[string]interface{}:
		return val
	default:
		// Convert to string for unknown types
		return fmt.Sprintf("%v", val)
	}
}

func compareConfigs(baseline, kb map[string]interface{}) []string {
	var diffs []string

	// Track all parameters for 1-1 comparison
	allParams := make(map[string]bool)
	for param := range baseline {
		allParams[param] = true
	}
	for param := range kb {
		allParams[param] = true
	}

	// Sort parameters for consistent output
	sortedParams := make([]string, 0, len(allParams))
	for param := range allParams {
		sortedParams = append(sortedParams, param)
	}

	// Simple sort (could use sort.Strings for better ordering)
	for i := 0; i < len(sortedParams)-1; i++ {
		for j := i + 1; j < len(sortedParams); j++ {
			if sortedParams[i] > sortedParams[j] {
				sortedParams[i], sortedParams[j] = sortedParams[j], sortedParams[i]
			}
		}
	}

	// Compare each parameter 1-1
	for _, param := range sortedParams {
		// Skip if not user-visible parameter
		if !isUserVisibleParam(param) {
			continue
		}

		baselineValue, baselineExists := baseline[param]
		kbValue, kbExists := kb[param]

		if !baselineExists && !kbExists {
			continue // Should not happen
		}

		if !baselineExists {
			// Skip if KB has a placeholder value
			if !isPlaceholderValue(kbValue) {
				diffs = append(diffs, fmt.Sprintf("MISSING in baseline: %s (KB has: %v)", param, kbValue))
			}
			continue
		}

		if !kbExists {
			// Skip if baseline has a placeholder value
			if !isPlaceholderValue(baselineValue) {
				diffs = append(diffs, fmt.Sprintf("MISSING in KB: %s (baseline has: %v)", param, baselineValue))
			}
			continue
		}

		// Both exist, compare values with smart comparison
		if !smartValuesEqual(baselineValue, kbValue) {
			diffs = append(diffs, fmt.Sprintf("DIFFERENT: %s (baseline: %v, KB: %v)", param, baselineValue, kbValue))
		}
	}

	return diffs
}

func compareVariables(baseline, kb map[string]interface{}) []string {
	var diffs []string

	// Track all variables for 1-1 comparison
	allVars := make(map[string]bool)
	for varName := range baseline {
		allVars[varName] = true
	}
	for varName := range kb {
		allVars[varName] = true
	}

	// Sort variables for consistent output
	sortedVars := make([]string, 0, len(allVars))
	for varName := range allVars {
		sortedVars = append(sortedVars, varName)
	}

	// Simple sort
	for i := 0; i < len(sortedVars)-1; i++ {
		for j := i + 1; j < len(sortedVars); j++ {
			if sortedVars[i] > sortedVars[j] {
				sortedVars[i], sortedVars[j] = sortedVars[j], sortedVars[i]
			}
		}
	}

	// Compare each variable 1-1
	// Only compare global system variables (user-visible)
	for _, varName := range sortedVars {
		// Skip MySQL compatible variables (already filtered in compareBaselineWithArgs)
		// Skip session-only variables (not user-visible for upgrade precheck)
		// Only compare global system variables
		if !isGlobalSystemVariable(varName) {
			continue
		}

		baselineValue, baselineExists := baseline[varName]
		kbValue, kbExists := kb[varName]

		if !baselineExists && !kbExists {
			continue // Should not happen
		}

		if !baselineExists {
			// Skip if KB has a placeholder value
			if !isPlaceholderValue(kbValue) {
				diffs = append(diffs, fmt.Sprintf("MISSING in baseline: %s (KB has: %v)", varName, kbValue))
			}
			continue
		}

		if !kbExists {
			// Skip if baseline has a placeholder value
			if !isPlaceholderValue(baselineValue) {
				diffs = append(diffs, fmt.Sprintf("MISSING in KB: %s (baseline has: %v)", varName, baselineValue))
			}
			continue
		}

		// Both exist, compare values with smart comparison
		if !smartValuesEqual(baselineValue, kbValue) {
			diffs = append(diffs, fmt.Sprintf("DIFFERENT: %s (baseline: %v, KB: %v)", varName, baselineValue, kbValue))
		}
	}

	return diffs
}

func valuesEqual(a, b interface{}) bool {
	// Convert to strings for comparison (handles type differences)
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return aStr == bStr || reflect.DeepEqual(a, b)
}

// smartValuesEqual performs intelligent comparison that handles:
// 1. Unit format differences (64 vs 64MB, 100 vs 100s) - but be conservative
// 2. Case differences (text vs Text, info vs Level)
// 3. Empty vs nil
func smartValuesEqual(a, b interface{}) bool {
	// First try normal comparison
	if valuesEqual(a, b) {
		return true
	}

	aStr := strings.TrimSpace(fmt.Sprintf("%v", a))
	bStr := strings.TrimSpace(fmt.Sprintf("%v", b))

	// Handle empty/nil
	if (aStr == "" || aStr == "<nil>" || aStr == "nil") && (bStr == "" || bStr == "<nil>" || bStr == "nil") {
		return true
	}

	// Handle case-insensitive comparison for enum values
	if strings.EqualFold(aStr, bStr) {
		return true
	}

	// Handle unit format differences conservatively
	// Only match if numbers are the same and units are reasonable
	if matchUnitFormat(aStr, bStr) {
		return true
	}

	return false
}

// matchUnitFormat checks if two values are the same number with different units
// This is conservative - only matches obvious cases
func matchUnitFormat(a, b string) bool {
	// Extract numeric part and unit
	extractNumAndUnit := func(s string) (string, string) {
		s = strings.TrimSpace(s)
		// Find where number ends
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				if i > 0 {
					return s[:i], strings.TrimSpace(s[i:])
				}
				return "", s
			}
		}
		return s, ""
	}

	aNum, aUnit := extractNumAndUnit(a)
	bNum, bUnit := extractNumAndUnit(b)

	// Special case: 0 with any unit is equivalent to 0
	if aNum == "0" && bNum == "0" {
		return true
	}

	// If numbers are the same
	if aNum != "" && aNum == bNum && aNum != "0" {
		// Normalize units (MiB <-> MB, KiB <-> KB, GiB <-> GB, TiB <-> TB)
		normalizeUnit := func(u string) string {
			u = strings.TrimSpace(u)
			// Map binary units to decimal units for comparison
			unitMap := map[string]string{
				"MiB": "MB", "mib": "MB",
				"KiB": "KB", "kib": "KB",
				"GiB": "GB", "gib": "GB",
				"TiB": "TB", "tib": "TB",
			}
			if mapped, ok := unitMap[u]; ok {
				return mapped
			}
			return u
		}

		normAUnit := normalizeUnit(aUnit)
		normBUnit := normalizeUnit(bUnit)

		// Case 1: One has unit and other doesn't (unit format error)
		if (aUnit != "" && bUnit == "") || (aUnit == "" && bUnit != "") {
			unit := normAUnit
			if unit == "" {
				unit = normBUnit
			}
			// Common units that might be added incorrectly
			commonUnits := []string{"MB", "KB", "GB", "TB", "s", "ms", "h", "m"}
			for _, u := range commonUnits {
				if strings.EqualFold(unit, u) {
					return true
				}
			}
		}

		// Case 2: Both have units, check if they're equivalent (MiB vs MB, etc.)
		if aUnit != "" && bUnit != "" {
			if strings.EqualFold(normAUnit, normBUnit) {
				return true
			}
			// Check time units conversion (1m vs 60s, etc.)
			if isTimeUnit(aUnit) && isTimeUnit(bUnit) {
				// Try to convert and compare
				if compareTimeUnits(aNum, aUnit, bNum, bUnit) {
					return true
				}
			}
		}
	}

	return false
}

// isTimeUnit checks if a unit is a time unit
func isTimeUnit(u string) bool {
	timeUnits := []string{"s", "ms", "us", "ns", "m", "h", "d"}
	u = strings.TrimSpace(strings.ToLower(u))
	for _, tu := range timeUnits {
		if u == tu {
			return true
		}
	}
	return false
}

// compareTimeUnits compares two time values with different units
func compareTimeUnits(num1, unit1, num2, unit2 string) bool {
	// Convert to seconds for comparison
	toSeconds := func(numStr, unit string) (float64, bool) {
		var num float64
		if _, err := fmt.Sscanf(numStr, "%f", &num); err != nil {
			return 0, false
		}
		unit = strings.TrimSpace(strings.ToLower(unit))
		switch unit {
		case "ns":
			return num / 1e9, true
		case "us":
			return num / 1e6, true
		case "ms":
			return num / 1000, true
		case "s":
			return num, true
		case "m":
			return num * 60, true
		case "h":
			return num * 3600, true
		case "d":
			return num * 86400, true
		default:
			return 0, false
		}
	}

	sec1, ok1 := toSeconds(num1, unit1)
	sec2, ok2 := toSeconds(num2, unit2)
	if !ok1 || !ok2 {
		return false
	}

	// Allow small floating point differences
	diff := sec1 - sec2
	if diff < 0 {
		diff = -diff
	}
	return diff < 0.001 // 1ms tolerance
}

// isPlaceholderValue checks if a value is a placeholder that shouldn't be compared
func isPlaceholderValue(v interface{}) bool {
	if v == nil {
		return true
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", v))
	// Common placeholders
	placeholders := []string{"", "None", "none", "nil", "<nil>", "null", "NULL"}
	for _, p := range placeholders {
		if s == p {
			return true
		}
	}
	// Check for default constant patterns (Def*)
	if strings.HasPrefix(s, "Def") || strings.HasPrefix(s, "def") {
		return true
	}
	// Check for other placeholder patterns
	placeholderPatterns := []string{
		"millis", "concurrency", "mb", "gb", "kb",
		"Level", "Text", "Disable", "Enable",
		"ByCompensatedSize", "unistore", // store type placeholder
	}
	for _, pattern := range placeholderPatterns {
		if strings.EqualFold(s, pattern) {
			return true
		}
	}
	// Check for empty arrays/objects
	if s == "[]" || s == "{}" || s == "[ ]" || s == "{ }" {
		return true
	}
	return false
}

// isRuntimeOnlyParam checks if a parameter is runtime-specific and shouldn't be compared
func isRuntimeOnlyParam(paramName string) bool {
	paramNameLower := strings.ToLower(paramName)

	// Runtime-specific parameters that vary by deployment
	runtimePrefixes := []string{
		"instance.",
		"log.file.filename",
		"log.file.max-backups",
		"log.file.max-days",
		"advertise-address",
		"host",
		"port",
		"socket",
		"data-dir",
		"storage.data-dir",
		"raftdb-path",
		"temp-path",
		"log-backup.temp-path",
		"log.file.",
		"log-format", // May vary by deployment
		"log-level",  // May vary by deployment
		"log.format",
		"log.level",
	}

	// Exact matches
	runtimeExact := []string{
		"path",  // PD path is runtime-specific
		"store", // Store type (tikv vs unistore) is runtime-specific
	}

	for _, exact := range runtimeExact {
		if paramNameLower == strings.ToLower(exact) {
			return true
		}
	}

	for _, prefix := range runtimePrefixes {
		if strings.HasPrefix(paramNameLower, strings.ToLower(prefix)) {
			return true
		}
	}

	return false
}

// isUserVisibleParam checks if a parameter is user-visible and should be compared
// Only user-visible configuration parameters should be compared
func isUserVisibleParam(paramName string) bool {
	paramNameLower := strings.ToLower(paramName)

	// Skip runtime-specific parameters
	if isRuntimeOnlyParam(paramName) {
		return false
	}

	// Skip compatibility parameters (MySQL compatible, not TiDB-specific)
	if isCompatibilityParam(paramName) {
		return false
	}

	// Skip internal/useless parameters
	if isUselessParam(paramName) {
		return false
	}

	// Skip internal prefixes (these are not user-visible)
	internalPrefixes := []string{
		"performance.txn-",   // Internal transaction parameters
		"performance.max-",   // Internal performance parameters
		"pd-client.",         // Internal PD client parameters
		"tikv-client.",       // Internal TiKV client parameters
		"raftdb.",            // Internal RaftDB parameters
		"rocksdb.",           // Internal RocksDB parameters
		"server.snap-",       // Internal snapshot parameters
		"quota.",             // Internal quota parameters (runtime-specific)
		"storage.scheduler-", // Internal scheduler parameters
		"storage.ttl-",       // Internal TTL parameters
		"storage.worker-",    // Internal worker parameters
		"backup.",            // Internal backup parameters
		"cdc.",               // Internal CDC parameters
		"log.",               // Internal log parameters (except user-visible ones)
		"log-",               // Internal log parameters
		"security.",          // Internal security parameters (runtime-specific)
		"delay-",             // Internal delay parameters
		"graceful-",          // Internal graceful shutdown parameters
		"memory-usage-",      // Runtime-specific memory parameters
	}

	for _, prefix := range internalPrefixes {
		if strings.HasPrefix(paramNameLower, prefix) {
			return false
		}
	}

	// All other parameters are considered user-visible
	return true
}

// isCompatibilityParam checks if a parameter is for MySQL compatibility only
func isCompatibilityParam(paramName string) bool {
	paramNameLower := strings.ToLower(paramName)

	// MySQL compatibility prefixes
	compatPrefixes := []string{
		"binlog.",
		"opentracing.",
		"experimental.",
		"compatible-",
		"enable-forwarding",
		"enable-tcp4-only",
		"isolation-read.",
	}

	for _, prefix := range compatPrefixes {
		if strings.HasPrefix(paramNameLower, prefix) {
			return true
		}
	}

	return false
}

// isUselessParam checks if a parameter is useless or deprecated
func isUselessParam(paramName string) bool {
	paramNameLower := strings.ToLower(paramName)

	// Useless/deprecated parameters
	uselessParams := []string{
		"ballast-object-size",
		"max-ballast-object-size",
		"tidb_config",                                           // Complex JSON object, not useful for comparison
		"performance.max-procs",                                 // Runtime-specific
		"performance.max-txn-ttl",                               // Internal parameter
		"performance.txn-entry-size-limit",                      // Internal parameter
		"performance.txn-total-size-limit",                      // Internal parameter
		"pd-client.pd-server-timeout",                           // Internal parameter
		"tikv-client.async-commit.total-key-size-limit",         // Internal parameter
		"raftdb.max-total-wal-size",                             // Internal parameter
		"rocksdb.max-total-wal-size",                            // Internal parameter (unit format may differ)
		"server.snap-max-total-size",                            // Internal parameter
		"memory-usage-limit",                                    // Runtime-specific (depends on available memory)
		"quota.background-read-bandwidth",                       // Runtime-specific
		"quota.background-write-bandwidth",                      // Runtime-specific
		"quota.foreground-read-bandwidth",                       // Runtime-specific
		"quota.foreground-write-bandwidth",                      // Runtime-specific
		"quota.max-delay-duration",                              // Runtime-specific
		"rocksdb.defaultcf.hard-pending-compaction-bytes-limit", // Internal parameter
		"rocksdb.defaultcf.level0-slowdown-writes-trigger",      // Internal parameter
		"rocksdb.defaultcf.level0-stop-writes-trigger",          // Internal parameter
		"rocksdb.defaultcf.compression-per-level",               // Complex array, format may differ
		"rocksdb.defaultcf.compaction-pri",                      // Enum value format may differ
		"rocksdb.defaultcf.compaction-style",                    // Enum value format may differ
		"rocksdb.defaultcf.bottommost-level-compression",        // Enum value format may differ
		"rocksdb.defaultcf.disable-write-stall",                 // Internal parameter
		"rocksdb.defaultcf.enable-compaction-guard",             // Internal parameter
		"security.auto-tls",                                     // Runtime-specific (depends on deployment)
		"log-rotation-timespan",                                 // Runtime-specific
		"delay-clean-table-lock",                                // Internal parameter
		"graceful-wait-before-shutdown",                         // Runtime-specific
		"performance.server-memory-quota",                       // Runtime-specific
		"backup.auto-tune-refresh-interval",                     // Time format may differ (1m vs 60s)
		"cdc.min-ts-interval",                                   // Time format may differ (200ms vs millis)
		"cdc.incremental-scan-speed-limit",                      // Unit format may differ (MiB vs MB)
		"backup.s3-multi-part-size",                             // Unit format may differ (MiB vs MB)
	}

	for _, useless := range uselessParams {
		if paramNameLower == strings.ToLower(useless) {
			return true
		}
	}

	return false
}

// isGlobalSystemVariable checks if a system variable is global (user-visible for upgrade precheck)
// Session-only variables are not relevant for upgrade precheck
func isGlobalSystemVariable(varName string) bool {
	varNameLower := strings.ToLower(varName)

	// Skip MySQL compatible variables (already filtered in compareBaselineWithArgs, but double-check)
	if strings.HasPrefix(varNameLower, "innodb_") ||
		strings.HasPrefix(varNameLower, "myisam_") ||
		strings.HasPrefix(varNameLower, "performance_schema_") ||
		strings.HasPrefix(varNameLower, "query_cache_") {
		return false
	}

	// Skip internal/system variables that are not user-configurable
	internalVars := []string{
		"tidb_config",                     // Complex JSON object, not useful
		"version",                         // Read-only
		"version_comment",                 // Read-only
		"version_compile_os",              // Read-only
		"version_compile_machine",         // Read-only
		"have_ssl",                        // Read-only
		"have_openssl",                    // Read-only
		"ssl_ca",                          // Read-only
		"ssl_cert",                        // Read-only
		"ssl_key",                         // Read-only
		"license",                         // Read-only
		"datadir",                         // Read-only
		"socket",                          // Read-only
		"hostname",                        // Read-only
		"port",                            // Read-only
		"system_time_zone",                // Read-only
		"tidb_current_ts",                 // Session-only
		"tidb_batch_insert",               // Session-only
		"tidb_batch_delete",               // Session-only
		"tidb_batch_commit",               // Session-only
		"tidb_checksum_table_concurrency", // Session-only
		"tidb_metric_schema_step",         // Session-only
		"tidb_wait_split_region_finish",   // Session-only
		"tidb_wait_split_region_timeout",  // Session-only
		"tidb_allow_remove_auto_inc",      // Session-only
		"timestamp",                       // Session-only
		"tidb_read_staleness",             // Session-only
	}

	for _, internal := range internalVars {
		if varNameLower == strings.ToLower(internal) {
			return false
		}
	}

	// All other TiDB system variables are considered global and user-visible
	// (they have ScopeGlobal or ScopeGlobal|ScopeSession)
	return true
}
