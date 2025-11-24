package scan

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// UpgradeChange represents a change in the upgrade logic
type UpgradeChange struct {
	Version     int         `json:"version"`
	Type        string      `json:"type"`
	Target      string      `json:"target"`
	Value       interface{} `json:"value"`
	Scope       string      `json:"scope"`
	Description string      `json:"description"`
	SQL         string      `json:"sql,omitempty"`
}

// UpgradeLogic represents the upgrade logic for a component
type UpgradeLogic struct {
	Component string         `json:"component"`
	Changes   []UpgradeChange `json:"changes"`
}

// ScanUpgradeLogic analyzes upgrade.go and PR/commit to extract mandatory upgrade changes
func ScanUpgradeLogic(repo, tag string) error {
	fmt.Printf("[ScanUpgradeLogic] repo=%s tag=%s\n", repo, tag)
	
	// Initialize version manager
	vm, err := NewVersionManager("knowledge")
	if err != nil {
		return fmt.Errorf("failed to initialize version manager: %v", err)
	}
	
	// Check if version already generated
	if vm.IsVersionGenerated(tag) {
		fmt.Printf("[Skipped] Version %s already generated, skipping upgrade logic collection\n", tag)
		return nil
	}
	
	// 1. git checkout tag
	cmd := exec.Command("git", "checkout", tag)
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git checkout %s failed: %w", tag, err)
	}
	
	// 2. Parse upgrade.go AST
	upgradeFilePath := filepath.Join(repo, "session", "upgrade.go")
	if _, err := os.Stat(upgradeFilePath); os.IsNotExist(err) {
		// Try alternative path
		upgradeFilePath = filepath.Join(repo, "pkg", "session", "upgrade.go")
		if _, err := os.Stat(upgradeFilePath); os.IsNotExist(err) {
			return fmt.Errorf("upgrade.go not found in %s", repo)
		}
	}
	
	// Parse the Go file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, upgradeFilePath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse upgrade.go: %v", err)
	}
	
	// Find all upgradeToVer functions
	upgradeChanges := []UpgradeChange{}
	
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		
		// Check if this is an upgradeToVer function
		if !strings.HasPrefix(funcDecl.Name.Name, "upgradeToVer") {
			continue
		}
		
		// Extract version number
		versionStr := strings.TrimPrefix(funcDecl.Name.Name, "upgradeToVer")
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			continue
		}
		
		// Parse function body for SQL statements and changes
		changes := extractChangesFromFunction(funcDecl, version)
		upgradeChanges = append(upgradeChanges, changes...)
	}
	
	// Create upgrade logic structure
	upgradeLogic := UpgradeLogic{
		Component: "tidb",
		Changes:   upgradeChanges,
	}
	
	// Write to file
	outDir := filepath.Join("knowledge", tag)
	os.MkdirAll(outDir, 0755)
	outputPath := filepath.Join(outDir, "upgrade_logic.json")
	
	data, err := json.MarshalIndent(upgradeLogic, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal upgrade logic: %v", err)
	}
	
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %v", err)
	}
	
	// Record this version as generated
	// Get commit hash for this tag
	commitCmd := exec.Command("git", "rev-parse", "HEAD")
	commitCmd.Dir = repo
	commitHash, err := commitCmd.Output()
	if err == nil {
		if err := vm.RecordVersion(tag, string(commitHash)); err != nil {
			fmt.Printf("[WARN] failed to record version %s: %v\n", tag, err)
		}
	}
	
	fmt.Printf("Extracted %d upgrade changes into %s\n", len(upgradeChanges), outputPath)
	return nil
}

// extractChangesFromFunction extracts changes from an upgrade function
func extractChangesFromFunction(funcDecl *ast.FuncDecl, version int) []UpgradeChange {
	var changes []UpgradeChange
	
	// Look for SQL statements in function body
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.CallExpr:
			// Look for calls to mustExecute, doReentrantDDL, etc.
			if ident, ok := stmt.Fun.(*ast.Ident); ok {
				switch ident.Name {
				case "mustExecute", "doReentrantDDL":
					if len(stmt.Args) > 0 {
						if basicLit, ok := stmt.Args[0].(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
							sql := strings.Trim(basicLit.Value, "\"`")
							
							// Try to identify the type of change from the SQL
							change := parseSQLStatement(sql, version)
							if change != nil {
								changes = append(changes, *change)
							}
						}
					}
				}
			}
		}
		return true
	})
	
	return changes
}

// parseSQLStatement parses an SQL statement and tries to identify the change type
func parseSQLStatement(sql string, version int) *UpgradeChange {
	sql = strings.ToUpper(strings.TrimSpace(sql))
	
	// Handle SET GLOBAL statements
	setGlobalRegex := regexp.MustCompile(`SET\s+GLOBAL\s+(\w+)\s*=\s*['"]?([^'"]+)['"]?`)
	if matches := setGlobalRegex.FindStringSubmatch(sql); len(matches) > 2 {
		return &UpgradeChange{
			Version:     version,
			Type:        "set_global",
			Target:      matches[1],
			Value:       matches[2],
			Scope:       "global",
			Description: "Setting global variable during upgrade",
			SQL:         sql,
		}
	}
	
	// Handle ALTER TABLE statements
	alterTableRegex := regexp.MustCompile(`ALTER\s+TABLE\s+(\S+)\s+(.*)`)
	if matches := alterTableRegex.FindStringSubmatch(sql); len(matches) > 2 {
		table := matches[1]
		operation := matches[2]
		
		// Determine operation type
		var opType string
		if strings.Contains(operation, "ADD COLUMN") {
			opType = "add_column"
		} else if strings.Contains(operation, "DROP COLUMN") {
			opType = "drop_column"
		} else if strings.Contains(operation, "MODIFY") || strings.Contains(operation, "CHANGE") {
			opType = "alter"
		} else {
			opType = "alter"
		}
		
		return &UpgradeChange{
			Version:     version,
			Type:        opType,
			Target:      table,
			Value:       operation,
			Scope:       "system_table",
			Description: "Altering table structure during upgrade",
			SQL:         sql,
		}
	}
	
	// Handle CREATE TABLE statements
	createTableRegex := regexp.MustCompile(`CREATE\s+TABLE\s+(\S+)`)
	if matches := createTableRegex.FindStringSubmatch(sql); len(matches) > 1 {
		return &UpgradeChange{
			Version:     version,
			Type:        "create_table",
			Target:      matches[1],
			Value:       sql,
			Scope:       "system_table",
			Description: "Creating new system table during upgrade",
			SQL:         sql,
		}
	}
	
	// Handle INSERT/UPDATE/DELETE statements
	if strings.HasPrefix(sql, "INSERT") {
		table := extractTableNameFromDML(sql, "INSERT")
		return &UpgradeChange{
			Version:     version,
			Type:        "insert",
			Target:      table,
			Value:       sql,
			Scope:       "system_table",
			Description: "Inserting data during upgrade",
			SQL:         sql,
		}
	}
	
	if strings.HasPrefix(sql, "UPDATE") {
		table := extractTableNameFromDML(sql, "UPDATE")
		return &UpgradeChange{
			Version:     version,
			Type:        "update",
			Target:      table,
			Value:       sql,
			Scope:       "system_table",
			Description: "Updating data during upgrade",
			SQL:         sql,
		}
	}
	
	if strings.HasPrefix(sql, "DELETE") {
		table := extractTableNameFromDML(sql, "DELETE")
		return &UpgradeChange{
			Version:     version,
			Type:        "delete",
			Target:      table,
			Value:       sql,
			Scope:       "system_table",
			Description: "Deleting data during upgrade",
			SQL:         sql,
		}
	}
	
	return nil
}

// extractTableNameFromDML extracts table name from DML statements
func extractTableNameFromDML(sql, operation string) string {
	regex := regexp.MustCompile(fmt.Sprintf(`%s\s+(?:INTO|FROM)?\s+(\S+)`, operation))
	if matches := regex.FindStringSubmatch(sql); len(matches) > 1 {
		return matches[1]
	}
	return "unknown"
}

// extractTargetFromSQL extracts target from SQL statements
func extractTargetFromSQL(sql string) string {
	// SET GLOBAL var = val
	setRegex := regexp.MustCompile(`(?i)SET\s+GLOBAL\s+(\w+)\s*=`)
	if matches := setRegex.FindStringSubmatch(sql); len(matches) > 1 {
		return matches[1]
	}
	
	// INSERT INTO mysql.GLOBAL_VARIABLES
	insertRegex := regexp.MustCompile(`(?i)INSERT\s+INTO\s+(\w+\.\w+|\w+)`)
	if matches := insertRegex.FindStringSubmatch(sql); len(matches) > 1 {
		return matches[1]
	}
	
	// UPDATE mysql.GLOBAL_VARIABLES
	updateRegex := regexp.MustCompile(`(?i)UPDATE\s+(\w+\.\w+|\w+)`)
	if matches := updateRegex.FindStringSubmatch(sql); len(matches) > 1 {
		return matches[1]
	}
	
	return "unknown"
}

