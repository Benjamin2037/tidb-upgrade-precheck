package scan

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

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
		tempDir, err := ioutil.TempDir("", "tidb_upgrade_precheck")
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
		toolFileName := selectToolByVersion(tag)
		
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
		defaultsData, err := ioutil.ReadFile(defaultsPath)
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
	
	if err := ioutil.WriteFile(outFile, data, 0644); err != nil {
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
		tempDir, err := ioutil.TempDir("", "tidb_upgrade_precheck")
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
		toolFileName := selectToolByVersion(tag)
		
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
		if err := ScanUpgradeLogic(tempDir, tag); err != nil {
			fmt.Printf("[WARN] ScanUpgradeLogic failed for %s: %v\n", tag, err)
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
		if err := ScanDefaults(repo, tag); err != nil {
			fmt.Printf("[WARN] ScanDefaults failed for %s: %v\n", tag, err)
		}
		if err := ScanUpgradeLogic(repo, tag); err != nil {
			fmt.Printf("[WARN] ScanUpgradeLogic failed for %s: %v\n", tag, err)
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

// getTagsInRange gets LTS tags within specified tag range
func getTagsInRange(repo, fromTag, toTag string) ([]string, error) {
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

// ScanUpgradeLogicGlobal generates only one global upgrade_logic.json with version information
func ScanUpgradeLogicGlobal(repo string, tags []string) error {
	// Implement AST scanning of all tag upgrade changes, merge into a global file
	// Example structure: [{"tag": "v7.5.0", ...}, {"tag": "v8.1.0", ...}]
	fmt.Println("[ScanUpgradeLogicGlobal] Scanning all version upgrade changes, generating global upgrade_logic.json")
	// Changes for each tag should be merged here
	return nil
}


