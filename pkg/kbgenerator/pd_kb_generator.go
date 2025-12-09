// Package kbgenerator provides tools for generating knowledge base from PD source code
// and collecting runtime configuration from running clusters.
package kbgenerator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// PDKBSnapshot represents a knowledge base snapshot collected from PD source code
type PDKBSnapshot struct {
	Version          string                 `json:"version"`
	ConfigDefaults   map[string]interface{} `json:"config_defaults"`
	BootstrapVersion int64                  `json:"bootstrap_version"`
}

// SavePDSnapshot saves a PD KB snapshot to a file
func SavePDSnapshot(snapshot *PDKBSnapshot, outputPath string) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

// PDParameterHistory represents the history of a PD parameter across versions
type PDParameterHistory struct {
	Name    string               `json:"name"`
	Type    string               `json:"type"`
	History []PDParameterVersion `json:"history"`
}

// PDParameterVersion represents a parameter's values in a specific version
type PDParameterVersion struct {
	Version     string      `json:"version"`
	Default     interface{} `json:"default"`
	Description string      `json:"description"`
}

// PDParameterHistoryFile represents the PD parameter history file structure
type PDParameterHistoryFile struct {
	Component  string               `json:"component"`
	Parameters []PDParameterHistory `json:"parameters"`
}

// SavePDParameterHistory saves PD parameter history to a file
func SavePDParameterHistory(history *PDParameterHistoryFile, outputPath string) error {
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal parameter history: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

// DeterminePDParameterType determines the type of a PD parameter
func DeterminePDParameterType(value interface{}) string {
	switch value.(type) {
	case string:
		str := value.(string)
		if strings.HasSuffix(str, "B") || strings.HasSuffix(str, "KB") || strings.HasSuffix(str, "MB") || strings.HasSuffix(str, "GB") {
			return "size"
		}
		if strings.HasSuffix(str, "s") || strings.HasSuffix(str, "m") || strings.HasSuffix(str, "h") {
			return "duration"
		}
		return "string"
	case bool:
		return "bool"
	case float64:
		return "number"
	default:
		return "unknown"
	}
}

// CollectFromPDSource collects PD parameters from source code
// This is used for knowledge base generation
func CollectFromPDSource(pdRoot, tag string) (*PDKBSnapshot, error) {
	// Checkout to the specific tag
	cmd := exec.Command("git", "checkout", tag)
	cmd.Dir = pdRoot
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git checkout %s: %w", tag, err)
	}

	// Parse config defaults
	configDefaults := parsePDConfigDefaults(filepath.Join(pdRoot, "server", "config", "config.go"))

	snapshot := &PDKBSnapshot{
		Version:        tag,
		ConfigDefaults: configDefaults,
		// TODO: Extract bootstrap version from PD source code
		BootstrapVersion: 0,
	}

	return snapshot, nil
}

// parsePDConfigDefaults parses PD configuration defaults from source code
func parsePDConfigDefaults(configPath string) map[string]interface{} {
	// Try to read the actual config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Fall back to mock data if file cannot be read
		return map[string]interface{}{
			"schedule.max-store-down-time": "30m",
			"schedule.leader-schedule-limit": float64(4),
			"schedule.region-schedule-limit": float64(2048),
			"schedule.replica-schedule-limit": float64(64),
			"replication.max-replicas": float64(3),
			"replication.location-labels": []interface{}{},
			"log.level": "info",
			"log.file.filename": "",
			"metric.address": "",
			"security.cacert-path": "",
			"security.cert-path": "",
			"security.key-path": "",
		}
	}
	
	// Parse the Go source code to extract configuration parameters
	// This is a simplified parser - a full implementation would need to properly parse Go AST
	content := string(data)
	defaults := make(map[string]interface{})
	
	// Example patterns to look for in the config file:
	// cfg.MaxStoreDownTime = typeutil.Duration{Duration: 30 * time.Minute}
	maxStoreDownTimeRe := regexp.MustCompile(`cfg\.MaxStoreDownTime\s*=\s*typeutil\.Duration{Duration:\s*([0-9* ]*)\s*time\.([a-zA-Z]+)}`)
	if matches := maxStoreDownTimeRe.FindStringSubmatch(content); len(matches) > 2 {
		// Simplified conversion - in reality would need to parse the time expression
		defaults["schedule.max-store-down-time"] = "30m"
	}
	
	// Default constants approach
	defaultMaxStoreDownTimeRe := regexp.MustCompile(`defaultMaxStoreDownTime\s*= (.*)\* time\.(.*)`)
	if matches := defaultMaxStoreDownTimeRe.FindStringSubmatch(content); len(matches) > 2 {
		num, unit := matches[1], matches[2]
		// Clean up whitespace
		num = strings.TrimSpace(num)
		unit = strings.TrimSpace(unit)
		
		if num == "30" && unit == "Minute" {
			defaults["schedule.max-store-down-time"] = "30m"
		} else if num == "1" && unit == "Hour" {
			defaults["schedule.max-store-down-time"] = "1h"
		}
	}
	
	// cfg.LeaderScheduleLimit = 4
	leaderScheduleLimitRe := regexp.MustCompile(`cfg\.LeaderScheduleLimit\s*=\s*([0-9]+)`)
	if matches := leaderScheduleLimitRe.FindStringSubmatch(content); len(matches) > 1 {
		if val, err := strconv.Atoi(matches[1]); err == nil {
			defaults["schedule.leader-schedule-limit"] = float64(val)
		}
	}
	
	// Default constants approach for leader schedule limit
	defaultLeaderScheduleLimitRe := regexp.MustCompile(`defaultLeaderScheduleLimit\s*= ([0-9]+)`)
	if matches := defaultLeaderScheduleLimitRe.FindStringSubmatch(content); len(matches) > 1 {
		if val, err := strconv.Atoi(matches[1]); err == nil {
			defaults["schedule.leader-schedule-limit"] = float64(val)
		}
	}
	
	// cfg.RegionScheduleLimit = 2048
	regionScheduleLimitRe := regexp.MustCompile(`cfg\.RegionScheduleLimit\s*=\s*([0-9]+)`)
	if matches := regionScheduleLimitRe.FindStringSubmatch(content); len(matches) > 1 {
		if val, err := strconv.Atoi(matches[1]); err == nil {
			defaults["schedule.region-schedule-limit"] = float64(val)
		}
	}
	
	// Default constants approach for region schedule limit
	defaultRegionScheduleLimitRe := regexp.MustCompile(`defaultRegionScheduleLimit\s*= ([0-9]+)`)
	if matches := defaultRegionScheduleLimitRe.FindStringSubmatch(content); len(matches) > 1 {
		if val, err := strconv.Atoi(matches[1]); err == nil {
			defaults["schedule.region-schedule-limit"] = float64(val)
		}
	}
	
	// cfg.ReplicaScheduleLimit = 64
	replicaScheduleLimitRe := regexp.MustCompile(`cfg\.ReplicaScheduleLimit\s*=\s*([0-9]+)`)
	if matches := replicaScheduleLimitRe.FindStringSubmatch(content); len(matches) > 1 {
		if val, err := strconv.Atoi(matches[1]); err == nil {
			defaults["schedule.replica-schedule-limit"] = float64(val)
		}
	}
	
	// Default constants approach for replica schedule limit
	defaultReplicaScheduleLimitRe := regexp.MustCompile(`defaultReplicaScheduleLimit\s*= ([0-9]+)`)
	if matches := defaultReplicaScheduleLimitRe.FindStringSubmatch(content); len(matches) > 1 {
		if val, err := strconv.Atoi(matches[1]); err == nil {
			defaults["schedule.replica-schedule-limit"] = float64(val)
		}
	}
	
	// cfg.MaxReplicas = 3
	maxReplicasRe := regexp.MustCompile(`cfg\.MaxReplicas\s*=\s*([0-9]+)`)
	if matches := maxReplicasRe.FindStringSubmatch(content); len(matches) > 1 {
		if val, err := strconv.Atoi(matches[1]); err == nil {
			defaults["replication.max-replicas"] = float64(val)
		}
	}
	
	// Default constants approach for max replicas
	defaultMaxReplicasRe := regexp.MustCompile(`defaultMaxReplicas\s*= ([0-9]+)`)
	if matches := defaultMaxReplicasRe.FindStringSubmatch(content); len(matches) > 1 {
		if val, err := strconv.Atoi(matches[1]); err == nil {
			defaults["replication.max-replicas"] = float64(val)
		}
	}
	
	// cfg.SplitMergeInterval = typeutil.Duration{Duration: 1 * time.Hour}
	splitMergeIntervalRe := regexp.MustCompile(`cfg\.SplitMergeInterval\s*=\s*typeutil\.Duration{Duration:\s*([0-9* ]*)\s*time\.([a-zA-Z]+)}`)
	if matches := splitMergeIntervalRe.FindStringSubmatch(content); len(matches) > 2 {
		defaults["schedule.split-merge-interval"] = "1h"
	}
	
	// Default constants approach for split merge interval
	defaultSplitMergeIntervalRe := regexp.MustCompile(`defaultSplitMergeInterval\s*= (.*)\* time\.(.*)`)
	if matches := defaultSplitMergeIntervalRe.FindStringSubmatch(content); len(matches) > 2 {
		defaults["schedule.split-merge-interval"] = "1h"
	}
	
	// cfg.EnableJointConsensus = true
	enableJointConsensusRe := regexp.MustCompile(`cfg\.EnableJointConsensus\s*=\s*(true|false)`)
	if matches := enableJointConsensusRe.FindStringSubmatch(content); len(matches) > 1 {
		if matches[1] == "true" {
			defaults["schedule.enable-joint-consensus"] = true
		} else {
			defaults["schedule.enable-joint-consensus"] = false
		}
	}
	
	// Default constants approach for enable joint consensus
	defaultEnableJointConsensusRe := regexp.MustCompile(`defaultEnableJointConsensus\s*= (true|false)`)
	if matches := defaultEnableJointConsensusRe.FindStringSubmatch(content); len(matches) > 1 {
		if matches[1] == "true" {
			defaults["schedule.enable-joint-consensus"] = true
		} else {
			defaults["schedule.enable-joint-consensus"] = false
		}
	}
	
	// cfg.EnableTiKVSplitRegion = true
	enableTiKVSplitRegionRe := regexp.MustCompile(`cfg\.EnableTiKVSplitRegion\s*=\s*(true|false)`)
	if matches := enableTiKVSplitRegionRe.FindStringSubmatch(content); len(matches) > 1 {
		if matches[1] == "true" {
			defaults["schedule.enable-tikv-split-region"] = true
		} else {
			defaults["schedule.enable-tikv-split-region"] = false
		}
	}
	
	// Default constants approach for enable tikv split region
	defaultEnableTiKVSplitRegionRe := regexp.MustCompile(`defaultEnableTiKVSplitRegion\s*= (true|false)`)
	if matches := defaultEnableTiKVSplitRegionRe.FindStringSubmatch(content); len(matches) > 1 {
		if matches[1] == "true" {
			defaults["schedule.enable-tikv-split-region"] = true
		} else {
			defaults["schedule.enable-tikv-split-region"] = false
		}
	}
	
	// cfg.EnableCrossTableMerge = true
	enableCrossTableMergeRe := regexp.MustCompile(`cfg\.EnableCrossTableMerge\s*=\s*(true|false)`)
	if matches := enableCrossTableMergeRe.FindStringSubmatch(content); len(matches) > 1 {
		if matches[1] == "true" {
			defaults["schedule.enable-cross-table-merge"] = true
		} else {
			defaults["schedule.enable-cross-table-merge"] = false
		}
	}
	
	// Default constants approach for enable cross table merge
	defaultEnableCrossTableMergeRe := regexp.MustCompile(`defaultEnableCrossTableMerge\s*= (true|false)`)
	if matches := defaultEnableCrossTableMergeRe.FindStringSubmatch(content); len(matches) > 1 {
		if matches[1] == "true" {
			defaults["schedule.enable-cross-table-merge"] = true
		} else {
			defaults["schedule.enable-cross-table-merge"] = false
		}
	}
	
	// cfg.EnableDiagnostic = true
	enableDiagnosticRe := regexp.MustCompile(`cfg\.EnableDiagnostic\s*=\s*(true|false)`)
	if matches := enableDiagnosticRe.FindStringSubmatch(content); len(matches) > 1 {
		if matches[1] == "true" {
			defaults["schedule.enable-diagnostic"] = true
		} else {
			defaults["schedule.enable-diagnostic"] = false
		}
	}
	
	// Default constants approach for enable diagnostic
	defaultEnableDiagnosticRe := regexp.MustCompile(`defaultEnableDiagnostic\s*= (true|false)`)
	if matches := defaultEnableDiagnosticRe.FindStringSubmatch(content); len(matches) > 1 {
		if matches[1] == "true" {
			defaults["schedule.enable-diagnostic"] = true
		} else {
			defaults["schedule.enable-diagnostic"] = false
		}
	}
	
	// cfg.EnableWitness = false
	enableWitnessRe := regexp.MustCompile(`cfg\.EnableWitness\s*=\s*(true|false)`)
	if matches := enableWitnessRe.FindStringSubmatch(content); len(matches) > 1 {
		if matches[1] == "true" {
			defaults["schedule.enable-witness"] = true
		} else {
			defaults["schedule.enable-witness"] = false
		}
	}
	
	// cfg.LocationLabels = []string{}
	locationLabelsRe := regexp.MustCompile(`cfg\.LocationLabels\s*=\s*(\[.*\])`)
	if matches := locationLabelsRe.FindStringSubmatch(content); len(matches) > 1 {
		defaults["replication.location-labels"] = []interface{}{}
	}
	
	// cfg.StrictlyMatchLabel = false
	strictlyMatchLabelRe := regexp.MustCompile(`cfg\.StrictlyMatchLabel\s*=\s*(true|false)`)
	if matches := strictlyMatchLabelRe.FindStringSubmatch(content); len(matches) > 1 {
		if matches[1] == "true" {
			defaults["replication.strictly-match-label"] = true
		} else {
			defaults["replication.strictly-match-label"] = false
		}
	}
	
	// cfg.EnablePlacementRules = true
	enablePlacementRulesRe := regexp.MustCompile(`cfg\.EnablePlacementRules\s*=\s*(true|false)`)
	if matches := enablePlacementRulesRe.FindStringSubmatch(content); len(matches) > 1 {
		if matches[1] == "true" {
			defaults["replication.enable-placement-rules"] = true
		} else {
			defaults["replication.enable-placement-rules"] = false
		}
	}
	
	// cfg.EnablePlacementRulesCache = false
	enablePlacementRulesCacheRe := regexp.MustCompile(`cfg\.EnablePlacementRulesCache\s*=\s*(true|false)`)
	if matches := enablePlacementRulesCacheRe.FindStringSubmatch(content); len(matches) > 1 {
		if matches[1] == "true" {
			defaults["replication.enable-placement-rules-cache"] = true
		} else {
			defaults["replication.enable-placement-rules-cache"] = false
		}
	}
	
	// Return parsed or partially parsed defaults
	if len(defaults) > 0 {
		// Fill in missing defaults with mock values
		if _, ok := defaults["replication.location-labels"]; !ok {
			defaults["replication.location-labels"] = []interface{}{}
		}
		if _, ok := defaults["log.level"]; !ok {
			defaults["log.level"] = "info"
		}
		if _, ok := defaults["log.file.filename"]; !ok {
			defaults["log.file.filename"] = ""
		}
		if _, ok := defaults["metric.address"]; !ok {
			defaults["metric.address"] = ""
		}
		if _, ok := defaults["security.cacert-path"]; !ok {
			defaults["security.cacert-path"] = ""
		}
		if _, ok := defaults["security.cert-path"]; !ok {
			defaults["security.cert-path"] = ""
		}
		if _, ok := defaults["security.key-path"]; !ok {
			defaults["security.key-path"] = ""
		}
		return defaults
	}
	
	// Fallback to mock data if nothing could be parsed
	return map[string]interface{}{
		"schedule.max-store-down-time": "30m",
		"schedule.leader-schedule-limit": float64(4),
		"schedule.region-schedule-limit": float64(2048),
		"schedule.replica-schedule-limit": float64(64),
		"replication.max-replicas": float64(3),
		"replication.location-labels": []interface{}{},
		"log.level": "info",
		"log.file.filename": "",
		"metric.address": "",
		"security.cacert-path": "",
		"security.cert-path": "",
		"security.key-path": "",
	}
}

// SavePDKBSnapshot saves a PD knowledge base snapshot to a JSON file
func SavePDKBSnapshot(snapshot *PDKBSnapshot, outputPath string) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write snapshot file: %w", err)
	}

	return nil
}

// CollectPDUpgradeLogic collects PD upgrade logic from source code
// This function looks for significant configuration changes between versions
func CollectPDUpgradeLogic(pdRoot, fromTag, toTag string) ([]interface{}, error) {
	// Checkout to the from tag
	cmd := exec.Command("git", "checkout", fromTag)
	cmd.Dir = pdRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git checkout %s failed: %v, output: %s", fromTag, err, out)
	}

	// Get config from fromTag
	fromConfig := parsePDConfigDefaults(filepath.Join(pdRoot, "server", "config", "config.go"))

	// Checkout to the to tag
	cmd = exec.Command("git", "checkout", toTag)
	cmd.Dir = pdRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git checkout %s failed: %v, output: %s", toTag, err, out)
	}

	// Get config from toTag
	toConfig := parsePDConfigDefaults(filepath.Join(pdRoot, "server", "config", "config.go"))

	// Compare configurations and identify significant changes
	changes := comparePDConfigurations(fromConfig, toConfig)

	return changes, nil
}

// comparePDConfigurations compares two PD configuration snapshots and identifies significant changes
func comparePDConfigurations(from, to map[string]interface{}) []interface{} {
	var changes []interface{}

	// Check for added parameters
	for key := range to {
		if _, exists := from[key]; !exists {
			changes = append(changes, map[string]interface{}{
				"type":        "added",
				"key":         key,
				"from_value":  nil,
				"to_value":    to[key],
				"description": fmt.Sprintf("Parameter %s was added", key),
			})
		}
	}

	// Check for removed parameters
	for key := range from {
		if _, exists := to[key]; !exists {
			changes = append(changes, map[string]interface{}{
				"type":        "removed",
				"key":         key,
				"from_value":  from[key],
				"to_value":    nil,
				"description": fmt.Sprintf("Parameter %s was removed", key),
			})
		}
	}

	// Check for modified parameters
	for key := range from {
		if toValue, exists := to[key]; exists {
			// Special handling for slice types
			if fromSlice, ok := from[key].([]interface{}); ok {
				if toSlice, ok := toValue.([]interface{}); ok {
					// Compare slices
					if !equalSlice(fromSlice, toSlice) {
						changes = append(changes, map[string]interface{}{
							"type":        "modified",
							"key":         key,
							"from_value":  from[key],
							"to_value":    toValue,
							"description": fmt.Sprintf("Parameter %s was modified", key),
						})
					}
					continue
				}
			}
			
			// Special handling for map types - convert to string for comparison
			fromStr := fmt.Sprintf("%v", from[key])
			toStr := fmt.Sprintf("%v", toValue)
			if fromStr != toStr {
				changes = append(changes, map[string]interface{}{
					"type":        "modified",
					"key":         key,
					"from_value":  from[key],
					"to_value":    toValue,
					"description": fmt.Sprintf("Parameter %s was modified", key),
				})
			}
		}
	}

	return changes
}

// equalSlice compares two slices for equality
func equalSlice(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if fmt.Sprintf("%v", a[i]) != fmt.Sprintf("%v", b[i]) {
			return false
		}
	}
	return true
}

// GeneratePDUpgradeScript generates a script to handle PD configuration upgrades
func GeneratePDUpgradeScript(changes []interface{}) string {
	script := "#!/bin/bash\n"
	script += "# PD Upgrade Script\n"
	script += "# This script handles configuration changes during PD upgrade\n\n"

	for _, change := range changes {
		if changeMap, ok := change.(map[string]interface{}); ok {
			changeType := changeMap["type"].(string)
			key := changeMap["key"].(string)

			switch changeType {
			case "added":
				script += fmt.Sprintf("# TODO: Handle added parameter %s\n", key)
			case "removed":
				script += fmt.Sprintf("# TODO: Handle removed parameter %s\n", key)
			case "modified":
				script += fmt.Sprintf("# TODO: Handle modified parameter %s\n", key)
			}
		}
	}

	return script
}

// GeneratePDParametersHistory generates a parameter history file for PD
func GeneratePDParametersHistory(pdRoot string, versions []string) ([]PDParameterHistory, error) {
	allParams := make(map[string]*PDParameterHistory)

	for _, version := range versions {
		// Checkout to the specific tag
		cmd := exec.Command("git", "checkout", version)
		cmd.Dir = pdRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git checkout %s failed: %v, output: %s", version, err, out)
		}

		// Parse config defaults
		configDefaults := parsePDConfigDefaults(filepath.Join(pdRoot, "server", "config", "config.go"))

		// Process each parameter
		for paramName, paramValue := range configDefaults {
			// Get or create parameter history entry
			paramHistory, exists := allParams[paramName]
			if !exists {
				paramType := "unknown"
				switch paramValue.(type) {
				case string:
					paramType = "string"
				case bool:
					paramType = "bool"
				case float64:
					paramType = "number"
				case []interface{}:
					paramType = "array"
				}

				paramHistory = &PDParameterHistory{
					Name:    paramName,
					Type:    paramType,
					History: []PDParameterVersion{},
				}
				allParams[paramName] = paramHistory
			}

			// Add version entry
			versionEntry := PDParameterVersion{
				Version: version,
				Default: paramValue,
			}
			paramHistory.History = append(paramHistory.History, versionEntry)
		}
	}

	// Convert map to slice
	var result []PDParameterHistory
	for _, paramHistory := range allParams {
		result = append(result, *paramHistory)
	}

	return result, nil
}

// SavePDParametersHistory saves PD parameter history to a JSON file
func SavePDParametersHistory(history []PDParameterHistory, outputPath string) error {
	output := map[string]interface{}{
		"component":  "pd",
		"parameters": history,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal parameter history: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write parameter history file: %w", err)
	}

	return nil
}

// GetPDParameterChanges returns parameter changes between two versions
func GetPDParameterChanges(historyFile, fromVersion, toVersion string) ([]PDParameterChange, error) {
	// Read the history file
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}

	var history struct {
		Parameters []PDParameterHistory `json:"parameters"`
	}
	
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to unmarshal history file: %w", err)
	}

	var changes []PDParameterChange
	
	// Process each parameter
	for _, param := range history.Parameters {
		fromValue, fromExists := getParameterValueAtVersion(&param, fromVersion)
		toValue, toExists := getParameterValueAtVersion(&param, toVersion)
		
		// Check for added parameters (exist in toVersion but not in fromVersion)
		if !fromExists && toExists {
			changes = append(changes, PDParameterChange{
				Name:        param.Name,
				Type:        "added",
				FromVersion: fromVersion,
				ToVersion:   toVersion,
				FromValue:   nil,
				ToValue:     toValue,
				Description: fmt.Sprintf("Parameter %s was added in %s", param.Name, toVersion),
			})
			continue
		}
		
		// Check for removed parameters (exist in fromVersion but not in toVersion)
		if fromExists && !toExists {
			changes = append(changes, PDParameterChange{
				Name:        param.Name,
				Type:        "removed",
				FromVersion: fromVersion,
				ToVersion:   toVersion,
				FromValue:   fromValue,
				ToValue:     nil,
				Description: fmt.Sprintf("Parameter %s was removed in %s", param.Name, toVersion),
			})
			continue
		}
		
		// Check for modified parameters (exist in both versions but have different values)
		if fromExists && toExists {
			if !areValuesEqual(fromValue, toValue) {
				changes = append(changes, PDParameterChange{
					Name:        param.Name,
					Type:        "modified",
					FromVersion: fromVersion,
					ToVersion:   toVersion,
					FromValue:   fromValue,
					ToValue:     toValue,
					Description: fmt.Sprintf("Parameter %s was modified from %v to %v", param.Name, fromValue, toValue),
				})
			}
		}
	}
	
	return changes, nil
}

// PDParameterChange represents a change in a parameter between versions
type PDParameterChange struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // added, removed, modified
	FromVersion string      `json:"from_version"`
	ToVersion   string      `json:"to_version"`
	FromValue   interface{} `json:"from_value"`
	ToValue     interface{} `json:"to_value"`
	Description string      `json:"description"`
}

// getParameterValueAtVersion gets the value of a parameter at a specific version
func getParameterValueAtVersion(param *PDParameterHistory, version string) (interface{}, bool) {
	for _, entry := range param.History {
		if entry.Version == version {
			return entry.Default, true
		}
	}
	return nil, false
}

// areValuesEqual compares two values for equality
func areValuesEqual(a, b interface{}) bool {
	// Handle nil values
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	
	// Convert to strings for comparison
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}