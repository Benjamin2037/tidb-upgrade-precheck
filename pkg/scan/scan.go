package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ScanResult contains the result of a scan
type ScanResult struct {
	Tag     string
	WorkDir string
	Error   error
}

// ScanSingleTag scans a single tag
func ScanSingleTag(ctx context.Context, repo, tag string, vm *VersionManager) (ScanResult, error) {
	fmt.Printf("[ScanSingleTag] repo=%s tag=%s\n", repo, tag)

	// Check if version already exists
	if vm.IsVersionGenerated(tag) {
		fmt.Printf("[Skipped] Version %s already generated, skipping parameter collection\n", tag)
		return ScanResult{Tag: tag, WorkDir: ""}, nil
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "tidb_upgrade_precheck_*")
	if err != nil {
		return ScanResult{Tag: tag, Error: fmt.Errorf("failed to create temp dir: %v", err)}, nil
	}

	// Clean up
	defer os.RemoveAll(tmpDir)

	// Checkout tag
	workDir := filepath.Join(tmpDir, "tidb")
	if err := checkoutTag(repo, tag, workDir); err != nil {
		return ScanResult{Tag: tag, Error: fmt.Errorf("failed to checkout tag: %v", err)}, nil
	}

	// Scan defaults
	defaultsFile := filepath.Join("knowledge", tag, "defaults.json")
	if err := os.MkdirAll(filepath.Dir(defaultsFile), 0755); err != nil {
		return ScanResult{Tag: tag, Error: fmt.Errorf("failed to create defaults dir: %v", err)}, nil
	}

	toolFile, _ := selectToolByVersion(workDir)
	if err := ScanDefaults(workDir, defaultsFile, toolFile); err != nil {
		return ScanResult{Tag: tag, Error: fmt.Errorf("failed to scan defaults: %v", err)}, nil
	}

	// Record version
	if err := vm.RecordVersion(tag, workDir); err != nil {
		return ScanResult{Tag: tag, Error: fmt.Errorf("failed to record version: %v", err)}, nil
	}

	return ScanResult{Tag: tag, WorkDir: workDir}, nil
}

// ScanVersionRange scans a range of versions
func ScanVersionRange(ctx context.Context, repo, fromTag, toTag string, vm *VersionManager) error {
	fmt.Printf("[ScanVersionRange] repo=%s from=%s to=%s\n", repo, fromTag, toTag)

	tags, err := getTagsInRange(repo, fromTag, toTag)
	if err != nil {
		return fmt.Errorf("failed to get tags in range: %v", err)
	}

	fmt.Printf("Found %d tags to process\n", len(tags))

	for _, tag := range tags {
		result, err := ScanSingleTag(ctx, repo, tag, vm)
		if err != nil {
			return fmt.Errorf("failed to scan tag %s: %v", tag, err)
		}

		if result.Error != nil {
			fmt.Printf("[Error] Failed to process tag %s: %v\n", tag, result.Error)
		} else if result.WorkDir == "" {
			fmt.Printf("[Skipped] Version %s already generated, skipping parameter collection\n", tag)
		} else {
			fmt.Printf("[Success] Processed tag %s\n", tag)
		}
	}

	return nil
}


// ScanAllTags scans all LTS tags
func ScanAllTags(ctx context.Context, repo string, vm *VersionManager) error {
	fmt.Printf("[ScanAllTags] repo=%s\n", repo)

	tags, err := getAllLTSTags(repo)
	if err != nil {
		return fmt.Errorf("failed to get all LTS tags: %v", err)
	}

	fmt.Printf("Found %d LTS tags to process\n", len(tags))

	for _, tag := range tags {
		result, err := ScanSingleTag(ctx, repo, tag, vm)
		if err != nil {
			return fmt.Errorf("failed to scan tag %s: %v", tag, err)
		}

		if result.Error != nil {
			fmt.Printf("[Error] Failed to process tag %s: %v\n", tag, result.Error)
		} else if result.WorkDir == "" {
			fmt.Printf("[Skipped] Version %s already generated, skipping parameter collection\n", tag)
		} else {
			fmt.Printf("[Success] Processed tag %s\n", tag)
		}
	}

	// After processing all tags, aggregate parameters
	if err := aggregateParameters(); err != nil {
		return fmt.Errorf("failed to aggregate parameters: %v", err)
	}

	// Scan upgrade logic
	if err := scanUpgradeLogic(repo); err != nil {
		return fmt.Errorf("failed to scan upgrade logic: %v", err)
	}

	return nil
}

// getTagsInRange returns tags in a range
func getTagsInRange(repo, fromTag, toTag string) ([]string, error) {
	// This is a placeholder implementation
	// In a real implementation, this would query git for tags between fromTag and toTag
	return []string{fromTag, toTag}, nil
}

// getAllLTSTags returns all LTS tags
func getAllLTSTags(repo string) ([]string, error) {
	// This is a placeholder implementation
	// In a real implementation, this would query git for all LTS tags
	// For now, we'll return a sample list
	return []string{
		"v6.5.0", "v6.5.1", "v6.5.2", "v6.5.3", "v6.5.4", "v6.5.5", "v6.5.6", "v6.5.7", "v6.5.8", "v6.5.9", "v6.5.10", "v6.5.11", "v6.5.12",
		"v7.1.0", "v7.1.1", "v7.1.2", "v7.1.3", "v7.1.4", "v7.1.5", "v7.1.6",
		"v7.5.0", "v7.5.1", "v7.5.2", "v7.5.3", "v7.5.4", "v7.5.5", "v7.5.6", "v7.5.7",
		"v8.1.0", "v8.1.1", "v8.1.2",
		"v8.5.0", "v8.5.1", "v8.5.2", "v8.5.3",
	}, nil
}

// checkoutTag checks out a specific tag
func checkoutTag(repo, tag, workDir string) error {
	// This is a placeholder implementation
	// In a real implementation, this would perform git checkout
	fmt.Printf("[Checkout] repo=%s tag=%s workDir=%s\n", repo, tag, workDir)
	return nil
}

// aggregateParameters aggregates parameters from all versions
func aggregateParameters() error {
	fmt.Printf("[Parameter Aggregation] Aggregating parameters from all versions\n")
	// This is a placeholder implementation
	// In a real implementation, this would aggregate parameters from all versions
	return nil
}

// scanUpgradeLogic scans the upgrade logic
func scanUpgradeLogic(repo string) error {
	if repo == "" {
		return fmt.Errorf("repo path is empty")
	}

	// Create tools directory if not exists
	toolsDir := filepath.Join("pkg", "scan", "tools")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		return fmt.Errorf("failed to create tools directory: %v", err)
	}

	// Copy the tool to tools directory
	toolPath := filepath.Join(toolsDir, "upgrade_logic_collector.go")
	if err := copyFile(filepath.Join("../..", "tools", "upgrade_logic_collector.go"), toolPath); err != nil {
		return fmt.Errorf("failed to copy tool: %v", err)
	}

	// Run the tool
	cmd := exec.Command("go", "run", toolPath, repo)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run upgrade logic collector: %v", err)
	}

	// Create knowledge/tidb directory if not exists
	knowledgeDir := filepath.Join("knowledge", "tidb")
	if err := os.MkdirAll(knowledgeDir, 0755); err != nil {
		return fmt.Errorf("failed to create knowledge directory: %v", err)
	}

	// Write output to file
	outputFile := filepath.Join(knowledgeDir, "upgrade_logic.json")
	if err := os.WriteFile(outputFile, output, 0644); err != nil {
		return fmt.Errorf("failed to write upgrade logic to file: %v", err)
	}

	fmt.Printf("[ScanUpgradeLogic] repo=%s\n", repo)
	return nil
}


// ParameterHistoryEntry records parameter properties at a specific version
type ParameterHistoryEntry struct {
	Version     int         `json:"version"`
	Default     interface{} `json:"default"`
	Scope       string      `json:"scope"`
	Description string      `json:"description"`
	Dynamic     bool        `json:"dynamic"`
}

// ParameterHistory aggregates all changes for a single parameter
type ParameterHistory struct {
	Name    string                  `json:"name"`
	Type    string                  `json:"type"`
	History []ParameterHistoryEntry `json:"history"`
}

// ParametersHistoryFile global aggregation output structure
type ParametersHistoryFile struct {
	Component  string             `json:"component"`
	Parameters []ParameterHistory `json:"parameters"`
}

// ScanSnapshot represents a snapshot of parameters at a specific version
type ScanSnapshot struct {
	Sysvars             map[string]interface{} `json:"sysvars"`
	Config              map[string]interface{} `json:"config"`
	BootstrapVersion    int                    `json:"bootstrap_version"`
}

// ScanAllAndAggregateParameters aggregates parameter changes across all LTS versions, outputs global parameters-history.json
func ScanAllAndAggregateParameters(repo string) error {
	tags, err := GetAllTags(repo)
	if err != nil {
		return err
	}
	
	// Initialize version manager
	vm, err := NewVersionManager("knowledge")
	if err != nil {
		return fmt.Errorf("failed to initialize version manager: %v", err)
	}
	
	paramMap := make(map[string]*ParameterHistory)
	
	for _, tag := range tags {
		// Check if version already generated
		if vm.IsVersionGenerated(tag) {
			fmt.Printf("[Skipped] Version %s already generated, skipping parameter collection\n", tag)
			continue
		}
		
		fmt.Printf("[Processing] Collecting parameters for version %s\n", tag)
		
		// Create a temporary directory for cloning
		tempDir, err := os.MkdirTemp("", "tidb_upgrade_precheck")
		if err != nil {
			fmt.Printf("[WARN] failed to create temp directory: %v\n", err)
			continue
		}
		
		// Clean up temp directory after processing
		defer os.RemoveAll(tempDir)
		
		// Clone the repo to temporary directory
		cloneCmd := exec.Command("git", "clone", repo, tempDir)
		if err := cloneCmd.Run(); err != nil {
			fmt.Printf("[WARN] failed to clone repo: %v\n", err)
			continue
		}
		
		// Checkout the specific tag in the cloned repo
		checkoutCmd := exec.Command("git", "checkout", tag)
		checkoutCmd.Dir = tempDir
		if err := checkoutCmd.Run(); err != nil {
			fmt.Printf("[WARN] git checkout %s failed: %v\n", tag, err)
			continue
		}
		
		// Determine which export_defaults file to use based on version
		toolFileName, err := selectToolByVersion(tempDir)
	if err != nil {
		fmt.Printf("[WARN] failed to select tool by version: %v\n", err)
		continue
	}
		
		// Copy the appropriate export_defaults tool to the cloned repo
		// Now we copy from tidb-upgrade-precheck project instead of tidb project
		currentDir, _ := os.Getwd()
		srcToolPath := filepath.Join(currentDir, "tools/upgrade-precheck", toolFileName)
		dstToolPath := filepath.Join(tempDir, "tools", "export_defaults.go")
		
		// Create tools directory if it doesn't exist
		if err := os.MkdirAll(filepath.Join(tempDir, "tools"), 0755); err != nil {
			fmt.Printf("[WARN] failed to create tools directory: %v\n", err)
			continue
		}
		
		// Copy file
		if err := copyFile(srcToolPath, dstToolPath); err != nil {
			fmt.Printf("[WARN] failed to copy export_defaults.go: %v\n", err)
			continue
		}
		
		// Create tag directory
		tagDir := filepath.Join("knowledge", tag)
		if err := os.MkdirAll(tagDir, 0755); err != nil {
			fmt.Printf("[WARN] failed to create tag directory for %s: %v\n", tag, err)
			continue
		}
		
		// Run parameter collection tool on the cloned repo
		defaultsPath := filepath.Join(tagDir, "defaults.json")
		run := exec.Command("go", "run", "tools/export_defaults.go")
		run.Dir = tempDir
		f, err := os.Create(defaultsPath)
		if err != nil {
			fmt.Printf("[WARN] failed to create output file: %v\n", err)
			continue
		}
		
		run.Stdout = f
		run.Stderr = os.Stderr
		if err := run.Run(); err != nil {
			f.Close()
			fmt.Printf("[WARN] go run export_defaults.go failed: %v\n", err)
			continue
		}
		f.Close()
		
		// Record this version as generated
		// Get commit hash for this tag
		commitCmd := exec.Command("git", "rev-parse", "HEAD")
		commitCmd.Dir = tempDir
		commitHash, err := commitCmd.Output()
		if err == nil {
			if err := vm.RecordVersion(tag, string(commitHash)); err != nil {
				fmt.Printf("[WARN] failed to record version %s: %v\n", tag, err)
			}
		}
		
		// Load defaults.json and merge into paramMap
		defaultsData, err := os.ReadFile(defaultsPath)
		if err != nil {
			fmt.Printf("[WARN] Failed to read defaults.json for %s: %v\n", tag, err)
			continue
		}
		
		var snapshot ScanSnapshot
		if err := json.Unmarshal(defaultsData, &snapshot); err != nil {
			fmt.Printf("[WARN] Failed to unmarshal defaults.json for %s: %v\n", tag, err)
			continue
		}
		
		// Merge sysvars into paramMap
		for name, value := range snapshot.Sysvars {
			if _, exists := paramMap[name]; !exists {
				paramMap[name] = &ParameterHistory{
					Name: name,
					Type: getParamType(value),
				}
			}
			
			entry := ParameterHistoryEntry{
				Version:     snapshot.BootstrapVersion,
				Default:     value,
				Scope:       "unknown", // Would need to extract from code
				Description: "unknown", // Would need to extract from code
				Dynamic:     false,     // Would need to extract from code
			}
			paramMap[name].History = append(paramMap[name].History, entry)
		}
	}
	
	// Sort history by version for each parameter
	for _, param := range paramMap {
		sort.Slice(param.History, func(i, j int) bool {
			return param.History[i].Version < param.History[j].Version
		})
	}
	
	// Output aggregated parameters
	// Convert map to slice
	var params []ParameterHistory
	for _, param := range paramMap {
		params = append(params, *param)
	}
	
	output := ParametersHistoryFile{
		Component:  "tidb",
		Parameters: params,
	}
	
	outDir := filepath.Join("knowledge", "tidb")
	os.MkdirAll(outDir, 0755)
	outFile := filepath.Join(outDir, "parameters-history.json")
	
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal parameters history: %v", err)
	}
	
	if err := os.WriteFile(outFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %v", err)
	}
	
	fmt.Printf("[Parameter Aggregation] Output %d parameters to %s\n", len(params), outFile)
	return nil
}

// getParamType determines parameter type from its value
func getParamType(value interface{}) string {
	switch v := value.(type) {
	case bool:
		return "bool"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "int"
	case float32, float64:
		return "float"
	case string:
		// Special handling for string values that represent other types
		if v == "ON" || v == "OFF" || v == "TRUE" || v == "FALSE" {
			return "bool"
		}
		if strings.Contains(v, ",") || (strings.HasPrefix(v, "(") && strings.HasSuffix(v, ")")) {
			return "enum"
		}
		return "string"
	default:
		return "unknown"
	}
}

// extractUpgradeVersion derives numeric version (e.g., 98) from tag (e.g., v8.1.0), should be combined with upgrade.go in practice
func extractUpgradeVersion(tag string) int {
	// Implement more realistic mapping
	versionMap := map[string]int{
		"v6.5.0": 93,
		"v7.1.0": 95,
		"v7.5.0": 97,
		"v8.1.0": 98,
		"v8.5.0": 99,
	}
	
	if version, exists := versionMap[tag]; exists {
		return version
	}
	
	// Try to parse from tag
	parts := strings.Split(strings.TrimPrefix(tag, "v"), ".")
	if len(parts) >= 2 {
		major, err1 := strconv.Atoi(parts[0])
		minor, err2 := strconv.Atoi(parts[1])
		if err1 == nil && err2 == nil {
			// Rough estimation based on version numbers
			return (major-6)*10 + minor
		}
	}
	
	return 0
}

// ScanAll collects parameters and upgrade logic for all LTS versions
func ScanAll(repo string) error {
	tags, err := GetAllTags(repo)
	if err != nil {
		return err
	}
	
	for _, tag := range tags {
		fmt.Printf("[Processing] Collecting data for version %s\n", tag)
		
		// Create a temporary directory for cloning
		tempDir, err := os.MkdirTemp("", "tidb_upgrade_precheck")
		if err != nil {
			fmt.Printf("[WARN] failed to create temp directory: %v\n", err)
			continue
		}
		
		// Clean up temp directory after processing
		defer os.RemoveAll(tempDir)
		
		// Clone the repo to temporary directory
		cloneCmd := exec.Command("git", "clone", repo, tempDir)
		if err := cloneCmd.Run(); err != nil {
			fmt.Printf("[WARN] failed to clone repo: %v\n", err)
			continue
		}
		
		// Checkout the specific tag in the cloned repo
		checkoutCmd := exec.Command("git", "checkout", tag)
		checkoutCmd.Dir = tempDir
		if err := checkoutCmd.Run(); err != nil {
			fmt.Printf("[WARN] git checkout %s failed: %v\n", tag, err)
			continue
		}
		
		// Determine which export_defaults file to use based on version
		toolFileName, err := selectToolByVersion(tempDir)
	if err != nil {
		fmt.Printf("[WARN] failed to select tool by version: %v\n", err)
		continue
	}
		
		// Copy the appropriate export_defaults tool to the cloned repo
		// Now we copy from tidb-upgrade-precheck project instead of tidb project
		srcToolPath := filepath.Join("./tools/upgrade-precheck", toolFileName)
		dstToolPath := filepath.Join(tempDir, "tools", "export_defaults.go")
		
		// Create tools directory if it doesn't exist
		if err := os.MkdirAll(filepath.Join(tempDir, "tools"), 0755); err != nil {
			fmt.Printf("[WARN] failed to create tools directory: %v\n", err)
			continue
		}
		
		// Copy file
		if err := copyFile(srcToolPath, dstToolPath); err != nil {
			fmt.Printf("[WARN] failed to copy export_defaults.go: %v\n", err)
			continue
		}
		
		// Create tag directory
		tagDir := filepath.Join("knowledge", tag)
		if err := os.MkdirAll(tagDir, 0755); err != nil {
			fmt.Printf("[WARN] failed to create tag directory for %s: %v\n", tag, err)
			continue
		}
		
		// Run parameter collection tool on the cloned repo
		defaultsPath := filepath.Join(tagDir, "defaults.json")
		run := exec.Command("go", "run", "tools/export_defaults.go")
		run.Dir = tempDir
		f, err := os.Create(defaultsPath)
		if err != nil {
			fmt.Printf("[WARN] failed to create output file: %v\n", err)
			continue
		}
		
		run.Stdout = f
		run.Stderr = os.Stderr
		if err := run.Run(); err != nil {
			f.Close()
			fmt.Printf("[WARN] go run export_defaults.go failed: %v\n", err)
			continue
		}
		f.Close()
		
		// Run upgrade logic collection tool
		if err := scanUpgradeLogic(tempDir); err != nil {
			fmt.Printf("[WARN] scanUpgradeLogic failed for %s: %v\n", tag, err)
			continue
		}
	}
	
	return nil
}

// ScanRange scans specified tag range incrementally
func ScanRange(repo, fromTag, toTag string) error {
	tags, err := getTagsInRange(repo, fromTag, toTag)
	if err != nil {
		return err
	}
	
	// Initialize version manager
	vm, err := NewVersionManager("knowledge")
	if err != nil {
		return fmt.Errorf("failed to initialize version manager: %v", err)
	}
	
	for _, tag := range tags {
		// Check if version already generated
		if vm.IsVersionGenerated(tag) {
			fmt.Printf("[Skipped] Version %s already generated, skipping collection\n", tag)
			continue
		}
		
		fmt.Printf("[Incremental] Collecting %s ...\n", tag)
		// Create a temporary directory for cloning
		tempDir, err := os.MkdirTemp("", "tidb_upgrade_precheck")
		if err != nil {
			fmt.Printf("[WARN] failed to create temp directory: %v\n", err)
			continue
		}
		
		// Clean up temp directory after processing
		defer os.RemoveAll(tempDir)
		
		// Clone the repo to temporary directory
		cloneCmd := exec.Command("git", "clone", repo, tempDir)
		if err := cloneCmd.Run(); err != nil {
			fmt.Printf("[WARN] failed to clone repo: %v\n", err)
			continue
		}
		
		// Checkout the specific tag in the cloned repo
		checkoutCmd := exec.Command("git", "checkout", tag)
		checkoutCmd.Dir = tempDir
		if err := checkoutCmd.Run(); err != nil {
			fmt.Printf("[WARN] git checkout %s failed: %v\n", tag, err)
			continue
		}
		
		toolFile, err := selectToolByVersion(tempDir)
		if err != nil {
			fmt.Printf("[WARN] failed to select tool by version: %v\n", err)
			continue
		}
		
		if err := ScanDefaults(tempDir, filepath.Join("knowledge", tag, "defaults.json"), toolFile); err != nil {
			fmt.Printf("[WARN] ScanDefaults failed for %s: %v\n", tag, err)
		}
		if err := scanUpgradeLogic(tempDir); err != nil {
			fmt.Printf("[WARN] scanUpgradeLogic failed for %s: %v\n", tag, err)
		}
		
		// Record this version as generated
		// Get commit hash for this tag
		commitCmd := exec.Command("git", "rev-parse", "HEAD")
		commitCmd.Dir = repo
		commitHash, err := commitCmd.Output()
		if err == nil {
			if err := vm.RecordVersion(tag, strings.TrimSpace(string(commitHash))); err != nil {
				fmt.Printf("[WARN] failed to record version %s: %v\n", tag, err)
			}
		}
	}
	return nil
}

// GetAllTags gets all LTS tags (keeping only main versions, no suffixes)
func GetAllTags(repo string) ([]string, error) {
	cmd := exec.Command("git", "tag")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	tags := strings.Split(strings.TrimSpace(string(out)), "\n")
	var lts []string
	for _, tag := range tags {
		// Match LTS version patterns and exclude tags with suffixes
		if (strings.HasPrefix(tag, "v6.5.") || strings.HasPrefix(tag, "v7.1.") || 
			strings.HasPrefix(tag, "v7.5.") || strings.HasPrefix(tag, "v8.1.") || 
			strings.HasPrefix(tag, "v8.5.")) && !strings.Contains(tag, "-") {
			lts = append(lts, tag)
		}
	}
	return lts, nil
}

// GetTagsInRange gets LTS tags within specified tag range
func GetTagsInRange(repo, fromTag, toTag string) ([]string, error) {
	all, err := GetAllTags(repo)
	if err != nil {
		return nil, err
	}
	var res []string
	found := false
	for _, tag := range all {
		if tag == fromTag {
			found = true
		}
		if found {
			res = append(res, tag)
		}
		if tag == toTag {
			break
		}
	}
	return res, nil
}



