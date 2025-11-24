package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// UpgradeChange represents a change in upgrade logic
type UpgradeChange struct {
	Version  int    `json:"version"`
	Function string `json:"function"`
	Changes  []struct {
		Type        string `json:"type"`
		SQL         string `json:"sql,omitempty"`
		Location    string `json:"location"`
		Variable    string `json:"variable,omitempty"`
		ForcedValue string `json:"forced_value,omitempty"`
	} `json:"changes"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run upgrade_logic_collector.go <tidb_repo_path>")
		os.Exit(1)
	}

	tidbRepo := os.Args[1]
	upgradeGoPath := filepath.Join(tidbRepo, "pkg", "session", "upgrade.go")

	// Parse the Go file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, upgradeGoPath, nil, parser.ParseComments)
	if err != nil {
		fmt.Printf("Error parsing file: %v\n", err)
		os.Exit(1)
	}

	// Find all upgradeToVer functions
	var upgrades []UpgradeChange

	// Regular expression to match SQL statements involving mysql.global_variables
	sqlRegex := regexp.MustCompile(`(?i)(INSERT|UPDATE|DELETE|CREATE|ALTER|DROP|SET)\s+(?:.*?)(?:mysql\.global_variables|%n\.%n)`)

	// Walk through the AST
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			// Check if function name matches upgradeToVer pattern
			if strings.HasPrefix(x.Name.Name, "upgradeToVer") {
				upgrade := UpgradeChange{
					Function: x.Name.Name,
				}

				// Extract version number from function name
				versionStr := strings.TrimPrefix(x.Name.Name, "upgradeToVer")
				var version int
				fmt.Sscanf(versionStr, "%d", &version)
				upgrade.Version = version

				// Look for SQL statements in function body
				ast.Inspect(x.Body, func(n ast.Node) bool {
					switch stmt := n.(type) {
					case *ast.CallExpr:
						// Check if this is a call to mustExecute or executeSQL or doReentrantDDL
						if sel, ok := stmt.Fun.(*ast.Ident); ok {
							// Direct function call like mustExecute(...), executeSQL(...) or doReentrantDDL(...)
							if sel.Name == "mustExecute" || sel.Name == "executeSQL" || sel.Name == "doReentrantDDL" {
								// Extract SQL from arguments
								for _, arg := range stmt.Args {
									if basicLit, ok := arg.(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
										sqlValue := strings.Trim(basicLit.Value, "\"`")
										if sqlRegex.MatchString(sqlValue) {
											sqlType, _, variable, forcedValue := parseSQLQuery(sqlValue)
											if sqlType != "" {
												change := struct {
													Type        string `json:"type"`
													SQL         string `json:"sql,omitempty"`
													Location    string `json:"location"`
													Variable    string `json:"variable,omitempty"`
													ForcedValue string `json:"forced_value,omitempty"`
												}{
													Type:        sqlType,
													SQL:         basicLit.Value,
													Location:    fmt.Sprintf("%s:%d:%d", upgradeGoPath, fset.Position(basicLit.Pos()).Line, fset.Position(basicLit.Pos()).Column),
													Variable:    variable,
													ForcedValue: forcedValue,
												}
												upgrade.Changes = append(upgrade.Changes, change)
											}
										}
									}
									// Handle fmt.Sprintf calls
									if call, ok := arg.(*ast.CallExpr); ok {
										if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
											if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "fmt" && sel.Sel.Name == "Sprintf" {
												// Extract the format string
												if len(call.Args) > 0 {
													if basicLit, ok := call.Args[0].(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
														sqlValue := strings.Trim(basicLit.Value, "\"`")
														if sqlRegex.MatchString(sqlValue) {
															sqlType, _, variable, forcedValue := parseSQLQuery(sqlValue)
															if sqlType != "" {
																change := struct {
																	Type        string `json:"type"`
																	SQL         string `json:"sql,omitempty"`
																	Location    string `json:"location"`
																	Variable    string `json:"variable,omitempty"`
																	ForcedValue string `json:"forced_value,omitempty"`
																}{
																	Type:        sqlType,
																	SQL:         basicLit.Value,
																	Location:    fmt.Sprintf("%s:%d:%d", upgradeGoPath, fset.Position(basicLit.Pos()).Line, fset.Position(basicLit.Pos()).Column),
																	Variable:    variable,
																	ForcedValue: forcedValue,
																}
																upgrade.Changes = append(upgrade.Changes, change)
															}
														}
													}
												}
											}
										}
									}
								}
							}
						} else if sel, ok := stmt.Fun.(*ast.SelectorExpr); ok {
							if ident, ok := sel.X.(*ast.Ident); ok {
								// Check if it's calling a function on 's' (session) or if it's a direct call
								if (ident.Name == "s" || ident.Name == "sqlexec") && (sel.Sel.Name == "mustExecute" || sel.Sel.Name == "executeSQL") {
									// Extract SQL from arguments
									for _, arg := range stmt.Args {
										if basicLit, ok := arg.(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
											sqlValue := strings.Trim(basicLit.Value, "\"`")
											if sqlRegex.MatchString(sqlValue) {
												sqlType, _, variable, forcedValue := parseSQLQuery(sqlValue)
												if sqlType != "" {
													change := struct {
														Type        string `json:"type"`
														SQL         string `json:"sql,omitempty"`
														Location    string `json:"location"`
														Variable    string `json:"variable,omitempty"`
														ForcedValue string `json:"forced_value,omitempty"`
													}{
														Type:        sqlType,
														SQL:         basicLit.Value,
														Location:    fmt.Sprintf("%s:%d:%d", upgradeGoPath, fset.Position(basicLit.Pos()).Line, fset.Position(basicLit.Pos()).Column),
														Variable:    variable,
														ForcedValue: forcedValue,
													}
													upgrade.Changes = append(upgrade.Changes, change)
												}
											}
										}
										// Handle fmt.Sprintf calls
										if call, ok := arg.(*ast.CallExpr); ok {
											if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
												if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "fmt" && sel.Sel.Name == "Sprintf" {
													// Extract the format string
													if len(call.Args) > 0 {
														if basicLit, ok := call.Args[0].(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
															sqlValue := strings.Trim(basicLit.Value, "\"`")
															if sqlRegex.MatchString(sqlValue) {
																sqlType, _, variable, forcedValue := parseSQLQuery(sqlValue)
																if sqlType != "" {
																	change := struct {
																		Type        string `json:"type"`
																		SQL         string `json:"sql,omitempty"`
																		Location    string `json:"location"`
																		Variable    string `json:"variable,omitempty"`
																		ForcedValue string `json:"forced_value,omitempty"`
																	}{
																		Type:        sqlType,
																		SQL:         basicLit.Value,
																		Location:    fmt.Sprintf("%s:%d:%d", upgradeGoPath, fset.Position(basicLit.Pos()).Line, fset.Position(basicLit.Pos()).Column),
																		Variable:    variable,
																		ForcedValue: forcedValue,
																	}
																	upgrade.Changes = append(upgrade.Changes, change)
																}
															}
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
					return true
				})

				// Only add upgrades that have changes related to mysql.global_variables
				if len(upgrade.Changes) > 0 {
					upgrades = append(upgrades, upgrade)
				}
			}
		}
		return true
	})

	// Sort upgrades by version
	for i := 0; i < len(upgrades)-1; i++ {
		for j := i + 1; j < len(upgrades); j++ {
			if upgrades[i].Version > upgrades[j].Version {
				upgrades[i], upgrades[j] = upgrades[j], upgrades[i]
			}
		}
	}

	// Print JSON output
	fmt.Printf("[\n")
	for i, upgrade := range upgrades {
		if i > 0 {
			fmt.Printf(",\n")
		}
		fmt.Printf("  {\n")
		fmt.Printf("    \"version\": %d,\n", upgrade.Version)
		fmt.Printf("    \"function\": \"%s\",\n", upgrade.Function)
		fmt.Printf("    \"changes\": [\n")
		for j, change := range upgrade.Changes {
			if j > 0 {
				fmt.Printf(",\n")
			}
			fmt.Printf("      {\n")
			fmt.Printf("        \"type\": \"%s\",\n", change.Type)
			if change.SQL != "" {
				fmt.Printf("        \"sql\": %s,\n", change.SQL)
			}
			fmt.Printf("        \"location\": \"%s\"", change.Location)
			if change.Variable != "" {
				fmt.Printf(",\n        \"variable\": \"%s\"", change.Variable)
			}
			if change.ForcedValue != "" {
				fmt.Printf(",\n        \"forced_value\": \"%s\"", change.ForcedValue)
			}
			fmt.Printf("\n      }")
		}
		fmt.Printf("\n    ]\n")
		fmt.Printf("  }")
	}
	fmt.Printf("\n]\n")
}

func parseSQLQuery(query string) (string, string, string, string) {
	// Remove extra spaces and normalize the query
	query = strings.Join(strings.Fields(query), " ")
	
	// Check if this query involves mysql.global_variables table
	if !strings.Contains(query, "mysql.global_variables") && 
	   !strings.Contains(query, "%n.%n") {
		// If it's not related to global_variables, return empty
		return "", "", "", ""
	}
	
	// Extract variable name and forced value if this is an UPDATE statement
	var variable, forcedValue string
	if strings.Contains(strings.ToUpper(query), "UPDATE") {
		// Extract variable name from WHERE clause: VARIABLE_NAME = 'xxx'
		varNameRegex := regexp.MustCompile(`(?i)VARIABLE_NAME\s*=\s*['"]([^'"]+)['"]`)
		varNameMatches := varNameRegex.FindStringSubmatch(query)
		if len(varNameMatches) > 1 {
			variable = varNameMatches[1]
		}
		
		// Extract forced value from SET clause: VARIABLE_VALUE = 'xxx' or VARIABLE_VALUE='%[1]v'
		forcedValueRegex := regexp.MustCompile(`(?i)VARIABLE_VALUE\s*=\s*['"]?([^'"\s,]+)['"]?`)
		forcedValueMatches := forcedValueRegex.FindStringSubmatch(query)
		if len(forcedValueMatches) > 1 {
			forcedValue = forcedValueMatches[1]
		}
	} else if strings.Contains(strings.ToUpper(query), "INSERT") && strings.Contains(query, "mysql.global_variables") {
		// Extract variable name and value from INSERT statement
		// INSERT INTO mysql.global_variables VALUES ('var_name', 'var_value')
		insertRegex := regexp.MustCompile(`(?i)INSERT\s+(?:IGNORE\s+)?INTO\s+mysql\.global_variables.*VALUES\s*[(\s]*['"]([^'"]+)['"]\s*,\s*['"]?([^'"\s\)]+)['"]?`)
		insertMatches := insertRegex.FindStringSubmatch(query)
		if len(insertMatches) > 2 {
			variable = insertMatches[1]
			forcedValue = insertMatches[2]
		}
	} else if strings.Contains(strings.ToUpper(query), "DELETE") && strings.Contains(query, "mysql.global_variables") {
		// Extract variable name from DELETE statement
		varNameRegex := regexp.MustCompile(`(?i)variable_name\s*=\s*['"]([^'"]+)['"]`)
		varNameMatches := varNameRegex.FindStringSubmatch(query)
		if len(varNameMatches) > 1 {
			variable = varNameMatches[1]
		}
		forcedValue = "DELETED" // Mark as deleted
	}
	
	// Handle different types of SQL operations
	switch {
	case strings.HasPrefix(strings.ToUpper(query), "UPDATE"):
		return "UPDATE", query, variable, forcedValue
	case strings.HasPrefix(strings.ToUpper(query), "INSERT"):
		return "INSERT", query, variable, forcedValue
	case strings.HasPrefix(strings.ToUpper(query), "DELETE"):
		return "DELETE", query, variable, forcedValue
	case strings.HasPrefix(strings.ToUpper(query), "ALTER"):
		return "ALTER", query, variable, forcedValue
	default:
		// For other operations that might affect global_variables
		if strings.Contains(query, "mysql.global_variables") || 
		   strings.Contains(query, "%n.%n") {
			return "SQL", query, variable, forcedValue
		}
		return "", "", "", ""
	}
}
