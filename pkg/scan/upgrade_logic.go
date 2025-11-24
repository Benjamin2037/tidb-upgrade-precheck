package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// UpgradeChange represents a change in upgrade logic
type UpgradeChange struct {
	Version  int    `json:"version"`
	Function string `json:"function"`
	Changes  []struct {
		Type     string `json:"type"`
		SQL      string `json:"sql,omitempty"`
		Location string `json:"location"`
	} `json:"changes"`
}

// VersionRange represents a range of versions for incremental upgrade analysis
type VersionRange struct {
	FromVersion int `json:"from_version"`
	ToVersion   int `json:"to_version"`
}

// GetAllUpgradeChanges collects upgrade changes from all versions
func GetAllUpgradeChanges() ([]UpgradeChange, error) {
	fmt.Println("[GetAllUpgradeChanges] Collecting upgrade changes from all versions")

	// This would typically read from the generated upgrade_logic.json
	knowledgeDir := filepath.Join("..", "..", "knowledge", "tidb")
	outputFile := filepath.Join(knowledgeDir, "upgrade_logic.json")

	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		// Try alternative path
		altKnowledgeDir := filepath.Join("knowledge", "tidb")
		outputFile = filepath.Join(altKnowledgeDir, "upgrade_logic.json")
		if _, err := os.Stat(outputFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("upgrade_logic.json not found, please run ScanUpgradeLogic first")
		}
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read upgrade_logic.json: %v", err)
	}
	
	fmt.Printf("[GetAllUpgradeChanges] Read %d bytes from %s\n", len(data), outputFile)

	var changes []UpgradeChange
	if err := json.Unmarshal(data, &changes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal upgrade logic: %v", err)
	}

	// Sort by version
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Version < changes[j].Version
	})

	return changes, nil
}

// GetIncrementalUpgradeChanges collects upgrade changes for a specific version range
func GetIncrementalUpgradeChanges(fromVersion, toVersion string) ([]UpgradeChange, error) {
	fmt.Printf("[GetIncrementalUpgradeChanges] Collecting upgrade changes from version %s to %s\n", fromVersion, toVersion)

	allChanges, err := GetAllUpgradeChanges()
	if err != nil {
		return nil, fmt.Errorf("failed to get all upgrade changes: %v", err)
	}

	fromVer, err := parseVersionString(fromVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid from version %s: %v", fromVersion, err)
	}

	toVer, err := parseVersionString(toVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid to version %s: %v", toVersion, err)
	}

	var incrementalChanges []UpgradeChange
	for _, change := range allChanges {
		// Include changes where version is > fromVer and <= toVer
		if change.Version > fromVer && change.Version <= toVer {
			incrementalChanges = append(incrementalChanges, change)
		}
	}

	return incrementalChanges, nil
}

// parseVersionString parses a version string like "v6.5.0" and returns the major.minor version as an integer (e.g., 65)
func parseVersionString(version string) (int, error) {
	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")
	
	// Split by dots
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid version format")
	}
	
	// Parse major and minor versions
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid major version: %v", err)
	}
	
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minor version: %v", err)
	}
	
	// Return combined version as major*10 + minor
	return major*10 + minor, nil
}

// ScanUpgradeLogicGlobal scans upgrade logic globally
func ScanUpgradeLogicGlobal(repo string, versionRange *VersionRange) error {
	// This is a placeholder implementation
	// In a real implementation, this would scan upgrade logic globally
	fmt.Printf("[ScanUpgradeLogicGlobal] repo=%s\n", repo)
	if versionRange != nil {
		fmt.Printf("[ScanUpgradeLogicGlobal] from_version=%d to_version=%d\n", versionRange.FromVersion, versionRange.ToVersion)
	}
	return nil
}
