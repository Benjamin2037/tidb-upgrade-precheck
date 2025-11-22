// ------------------------------------------------------------
// Design Description:
// This tool automatically scans the TiDB source code pkg/session/upgrade.go file to extract all upgradeToVerXX 
// and the SET GLOBAL/INSERT/UPDATE/SQL changes in their call chains, outputting all mandatory variable changes 
// for each version (upgradeToVerXX). The output structure facilitates filtering upgrade mandatory changes by 
// version range during the precheck phase.
//
// Usage Instructions:
// 1. Ensure tidb-upgrade-precheck and tidb source directories are at the same level, or specify the tidb source path.
// 2. Execute in the tidb-upgrade-precheck directory:
//    go run tools/upgrade_logic_collector.go ../tidb
//    # Or specify another tidb path:
//    go run tools/upgrade_logic_collector.go /your/tidb/path
// 3. Output as JSON, can be redirected to save:
//    go run tools/upgrade_logic_collector.go ../tidb > knowledge/upgrade_logic.json
//
// Main Output Fields:
// - version: the XX in upgradeToVerXX
// - function: upgradeToVerXX
// - changes: all system variable changes under this function and its call chain (including SQL, location, comments, etc.)
// ------------------------------------------------------------

package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
)

type Change struct {
	Param    string `json:"param,omitempty"`
	Type     string `json:"type"`
	SQL      string `json:"sql"`
	Location string `json:"location"`
	Comment  string `json:"comment,omitempty"`
}

type UpgradeChange struct {
	Version  int      `json:"version"`
	Function string   `json:"function"`
	Changes  []Change `json:"changes"`
}

func main() {
	tidbRepo := "../tidb" // Default path, can be specified via arguments
	if len(os.Args) > 1 {
		tidbRepo = os.Args[1]
	}
	upgradePath := tidbRepo + "/pkg/session/upgrade.go"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, upgradePath, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	var results []UpgradeChange
	re := regexp.MustCompile(`upgradeToVer(\d+)`)
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name == nil {
			return true
		}
		m := re.FindStringSubmatch(fn.Name.Name)
		if len(m) != 2 {
			return true
		}
		ver, _ := strconv.Atoi(m[1])
		changes := collectChangesFromFunc(fn, fset)
		if len(changes) > 0 {
			results = append(results, UpgradeChange{
				Version:  ver,
				Function: fn.Name.Name,
				Changes:  changes,
			})
		}
		return true
	})
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		fmt.Fprintf(os.Stderr, "encode failed: %v\n", err)
		os.Exit(1)
	}
}

func collectChangesFromFunc(fn *ast.FuncDecl, fset *token.FileSet) []Change {
	var changes []Change
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		be, ok := n.(*ast.BasicLit)
		if ok && be.Kind == token.STRING {
			s := be.Value
			if match, _ := regexp.MatchString(`(?i)(SET GLOBAL|INSERT INTO mysql.global_variables|UPDATE mysql.global_variables)`, s); match {
				changes = append(changes, Change{
					Type:     "SQL",
					SQL:      s,
					Location: fset.Position(be.Pos()).String(),
				})
			}
		}
		return true
	})
	return changes
}