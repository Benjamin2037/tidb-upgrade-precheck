package collectparams

import (
	"bufio"
	"encoding/json"
	"os"
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

// CollectUpgradeLogic parses bootstrap.go to extract all variable forced changes within upgradeToVerXX functions

func CollectUpgradeLogic(bootstrapPath, outputPath string) error {
	f, err := os.Open(bootstrapPath)
	if err != nil {
		return err
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
	return enc.Encode(results)
}

// main example
// func main() {
// 	err := CollectUpgradeLogic("/path/to/bootstrap.go", "./upgrade_logic.json")
// 	if err != nil {
// 		fmt.Println("collect failed:", err)
// 	}
// }