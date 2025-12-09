package kbgenerator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// UpgradeVarChange records variables that are forcibly changed during the upgrade process
// Method: setGlobalSysVar, writeGlobalSysVar, initGlobalVariableIfNotExists, ...
type UpgradeVarChange struct {
	Version  string `json:"version"`
	FuncName string `json:"func_name"`
	VarName  string `json:"var_name"`
	Value    string `json:"value"`
	Method   string `json:"method"`
	Comment  string `json:"comment,omitempty"`
}

// UpgradeLogicSnapshot represents a snapshot of upgrade logic collected from TiDB source code
// This is part of the knowledge base used for upgrade compatibility checking
type UpgradeLogicSnapshot struct {
	Changes []UpgradeVarChange `json:"changes"`
}

// CollectUpgradeLogicFromSource parses bootstrap.go to extract all variable forced changes within upgradeToVerXX functions
// This is used for knowledge base generation to identify forced parameter changes during upgrades
func CollectUpgradeLogicFromSource(bootstrapPath string) (*UpgradeLogicSnapshot, error) {
	f, err := os.Open(bootstrapPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var (
		inUpgradeFunc bool
		curFunc       string
		curVersion    string
		curComment    string
		results       []UpgradeVarChange
	)
	// Match upgradeToVerXX function definition
	funcRe := regexp.MustCompile(`^func (upgradeToVer(\d+))\b`)
	// Match setGlobalSysVar/variable writing calls
	setVarRe := regexp.MustCompile(`(setGlobalSysVar|writeGlobalSysVar|initGlobalVariableIfNotExists)\s*\(\s*([^,]+)\s*,\s*([^,\)]+)`) // method, variable name, value
	// Match mustExecute/REPLACE INTO/SetGlobalSysVar statements
	mustExecRe := regexp.MustCompile(`mustExecute\(.*(REPLACE|UPDATE|INSERT).*?([a-zA-Z0-9_]+).*?([a-zA-Z0-9_]+).*?([a-zA-Z0-9_]+).*?([0-9a-zA-Z_"']+)`)
	// Match SetGlobalSysVar calls
	setGlobalRe := regexp.MustCompile(`SetGlobalSysVar\(.*?([a-zA-Z0-9_]+).*?,\s*([a-zA-Z0-9_"']+)`)
	// Match function comments
	commentRe := regexp.MustCompile(`^//\s*(.*)`)

	for scanner.Scan() {
		line := scanner.Text()
		if m := funcRe.FindStringSubmatch(line); m != nil {
			inUpgradeFunc = true
			curFunc = m[1]
			curVersion = m[2]
			curComment = ""
			continue
		}
		if inUpgradeFunc {
			if strings.HasPrefix(line, "}") { // Function end
				inUpgradeFunc = false
				continue
			}
			if m := setVarRe.FindStringSubmatch(line); m != nil {
				method := m[1]
				varName := strings.Trim(m[2], "\" ")
				value := strings.Trim(m[3], "\" ")
				results = append(results, UpgradeVarChange{
					Version:  curVersion,
					FuncName: curFunc,
					VarName:  varName,
					Value:    value,
					Method:   method,
					Comment:  curComment,
				})
				continue
			}
			if m := mustExecRe.FindStringSubmatch(line); m != nil {
				results = append(results, UpgradeVarChange{
					Version:  curVersion,
					FuncName: curFunc,
					VarName:  m[3],
					Value:    m[5],
					Method:   "mustExecute",
					Comment:  curComment,
				})
				continue
			}
			if m := setGlobalRe.FindStringSubmatch(line); m != nil {
				results = append(results, UpgradeVarChange{
					Version:  curVersion,
					FuncName: curFunc,
					VarName:  m[1],
					Value:    m[2],
					Method:   "SetGlobalSysVar",
					Comment:  curComment,
				})
				continue
			}
			if m := commentRe.FindStringSubmatch(line); m != nil {
				curComment = m[1]
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	
	return &UpgradeLogicSnapshot{
		Changes: results,
	}, nil
}

// CompareVersions compares two versions of TiDB and returns the upgrade logic differences
func CompareVersions(repoRoot, fromTag, toTag string) ([]UpgradeVarChange, error) {
	// Checkout to the from tag
	cmd := exec.Command("git", "checkout", fromTag)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git checkout %s failed: %v, output: %s", fromTag, err, out)
	}

	// Get upgrade logic from fromTag
	fromBootstrapPath := filepath.Join(repoRoot, "pkg", "session", "bootstrap.go")
	fromLogic, err := CollectUpgradeLogicFromSource(fromBootstrapPath)
	if err != nil {
		return nil, fmt.Errorf("failed to collect upgrade logic from %s: %v", fromTag, err)
	}

	// Checkout to the to tag
	cmd = exec.Command("git", "checkout", toTag)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git checkout %s failed: %v, output: %s", toTag, err, out)
	}

	// Get upgrade logic from toTag
	toBootstrapPath := filepath.Join(repoRoot, "pkg", "session", "bootstrap.go")
	toLogic, err := CollectUpgradeLogicFromSource(toBootstrapPath)
	if err != nil {
		return nil, fmt.Errorf("failed to collect upgrade logic from %s: %v", toTag, err)
	}

	// Compare and identify differences
	changes := compareUpgradeLogic(fromLogic, toLogic)
	return changes, nil
}

// compareUpgradeLogic compares two upgrade logic snapshots and identifies differences
func compareUpgradeLogic(from, to *UpgradeLogicSnapshot) []UpgradeVarChange {
	// Convert to maps for easier lookup
	fromMap := make(map[string]UpgradeVarChange)
	toMap := make(map[string]UpgradeVarChange)

	for _, change := range from.Changes {
		key := fmt.Sprintf("%s-%s", change.Version, change.VarName)
		fromMap[key] = change
	}

	for _, change := range to.Changes {
		key := fmt.Sprintf("%s-%s", change.Version, change.VarName)
		toMap[key] = change
	}

	// Identify added changes
	var changes []UpgradeVarChange
	for key, change := range toMap {
		if _, exists := fromMap[key]; !exists {
			changes = append(changes, change)
		}
	}

	return changes
}

// SaveUpgradeLogic saves the upgrade logic snapshot to a JSON file
func SaveUpgradeLogic(snapshot *UpgradeLogicSnapshot, outputPath string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	
	// Output to JSON
	outF, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outF.Close()
	
	enc := json.NewEncoder(outF)
	enc.SetIndent("", "  ")
	return enc.Encode(snapshot)
}

// main example
// func main() {
// 	err := CollectUpgradeLogic("/path/to/bootstrap.go", "./upgrade_logic.json")
// 	if err != nil {
// 		fmt.Println("collect failed:", err)
// 	}
// }