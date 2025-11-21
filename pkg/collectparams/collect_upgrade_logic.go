package collectparams

import (
	"bufio"
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// UpgradeVarChange 记录升级过程中强制变更的变量
// Method: setGlobalSysVar, writeGlobalSysVar, initGlobalVariableIfNotExists, ...
type UpgradeVarChange struct {
	Version  string `json:"version"`
	FuncName string `json:"func_name"`
	VarName  string `json:"var_name"`
	Value    string `json:"value"`
	Method   string `json:"method"`
	Comment  string `json:"comment,omitempty"`
}

// CollectUpgradeLogic 解析 bootstrap.go，提取所有 upgradeToVerXX 函数内的变量强制变更

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
	// 匹配 upgradeToVerXX 函数定义
	funcRe := regexp.MustCompile(`^func (upgradeToVer(\d+))\b`)
	// 匹配 setGlobalSysVar/写变量的调用
	setVarRe := regexp.MustCompile(`(setGlobalSysVar|writeGlobalSysVar|initGlobalVariableIfNotExists)\s*\(\s*([^,]+)\s*,\s*([^,\)]+)`) // 方法, 变量名, 值
	// 匹配 mustExecute/REPLACE INTO/SetGlobalSysVar 语句
	mustExecRe := regexp.MustCompile(`mustExecute\(.*(REPLACE|UPDATE|INSERT).*?([a-zA-Z0-9_]+).*?([a-zA-Z0-9_]+).*?([a-zA-Z0-9_]+).*?([0-9a-zA-Z_"']+)`)
	// 匹配 SetGlobalSysVar 调用
	setGlobalRe := regexp.MustCompile(`SetGlobalSysVar\(.*?([a-zA-Z0-9_]+).*?,\s*([a-zA-Z0-9_"']+)`)
	// 匹配函数注释
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
			if strings.HasPrefix(line, "}") { // 函数结束
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
	// 输出到 JSON
	outF, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outF.Close()
	enc := json.NewEncoder(outF)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

// main 示例
// func main() {
// 	err := CollectUpgradeLogic("/path/to/bootstrap.go", "./upgrade_logic.json")
// 	if err != nil {
// 		fmt.Println("collect failed:", err)
// 	}
// }
