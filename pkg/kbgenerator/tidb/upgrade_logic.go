package tidb

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator/common"
)

// findUpgradeLogicFile finds the actual upgrade logic file path for a given TiDB repository root
// Different TiDB versions may use different file paths:
// - pkg/session/upgrade.go (newer versions, e.g., v7.1+)
// - pkg/session/bootstrap.go (older versions)
// - session/upgrade.go (alternative path for older versions)
// - session/bootstrap.go (alternative path for older versions)
func findUpgradeLogicFile(repoRoot string) (string, error) {
	candidates := []string{
		filepath.Join(repoRoot, "pkg", "session", "upgrade.go"),
		filepath.Join(repoRoot, "pkg", "session", "bootstrap.go"),
		filepath.Join(repoRoot, "session", "upgrade.go"),
		filepath.Join(repoRoot, "session", "bootstrap.go"),
	}

	for _, candidate := range candidates {
		// Check if file exists
		if _, err := os.Stat(candidate); err == nil {
			// Verify it contains upgradeToVerXX functions
			data, err := os.ReadFile(candidate)
			if err == nil {
				// Check if file contains upgradeToVer pattern
				funcRe := regexp.MustCompile(`func upgradeToVer\d+`)
				if funcRe.Match(data) {
					return candidate, nil
				}
			}
		}
	}

	return "", fmt.Errorf("upgrade logic file not found in any of the candidate paths: %v", candidates)
}

// CollectUpgradeLogicFromSource parses upgrade logic file (upgrade.go or bootstrap.go) to extract all variable forced changes within upgradeToVerXX functions
// This function should be called on master branch (or latest version) to extract all historical upgradeToVerXX functions
// All historical upgrade functions are preserved in the latest TiDB codebase
// This is used for knowledge base generation to identify forced parameter changes during upgrades
// The function extracts upgrade functions with their bootstrap version numbers (e.g., upgradeToVer177 -> version 177)
// These bootstrap versions can be mapped to release versions using currentBootstrapVersion from each version tag
func CollectUpgradeLogicFromSource(repoRoot string) (*kbgenerator.UpgradeLogicSnapshot, error) {
	// Find the actual upgrade logic file path
	// This should be called on master branch to get all historical upgrade functions
	upgradeFilePath, err := findUpgradeLogicFile(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to find upgrade logic file: %w", err)
	}

	// Load vardef constants to map variable.XXX to user-visible names
	// Use master branch (current checkout) to find vardef directory
	// Try to find vardef directory (try modern structure first)
	vardefDir := ""
	possiblePaths := []string{
		filepath.Join(repoRoot, "pkg", "sessionctx", "vardef"),
		filepath.Join(repoRoot, "sessionctx", "vardef"),
	}
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			vardefDir = path
			break
		}
	}

	// Load vardef constants to map variable.XXX to user-visible names
	// Use SysVarExtractor to parse constants (reuse existing logic instead of duplicating)
	var vardefConsts map[string]string
	if vardefDir != "" {
		sysVarExtractor := common.NewSysVarExtractor(vardefDir)
		// Parse sysvar.go and tidb_vars.go to get constants
		// SysVarExtractor already parses vardef directory in NewSysVarExtractor
		sysVarFiles := []string{
			filepath.Join(repoRoot, "pkg", "sessionctx", "variable", "sysvar.go"),
			filepath.Join(repoRoot, "pkg", "sessionctx", "variable", "tidb_vars.go"),
			filepath.Join(repoRoot, "sessionctx", "variable", "sysvar.go"),
			filepath.Join(repoRoot, "sessionctx", "variable", "tidb_vars.go"),
		}
		for _, sysVarFile := range sysVarFiles {
			if _, err := os.Stat(sysVarFile); err == nil {
				sysVarExtractor.ExtractFromFile(sysVarFile)
			}
		}
		// Reuse the parsed constants from SysVarExtractor instead of re-parsing
		vardefConsts = sysVarExtractor.GetVardefConsts()
	} else {
		// If vardefDir is not found, still use SysVarExtractor to parse tidb_vars.go
		// Create extractor with empty vardefDir (it will still parse files passed to ExtractFromFile)
		sysVarExtractor := common.NewSysVarExtractor("")
		tidbVarsFiles := []string{
			filepath.Join(repoRoot, "pkg", "sessionctx", "variable", "tidb_vars.go"),
			filepath.Join(repoRoot, "sessionctx", "variable", "tidb_vars.go"),
		}
		for _, tidbVarsFile := range tidbVarsFiles {
			if _, err := os.Stat(tidbVarsFile); err == nil {
				sysVarExtractor.ExtractFromFile(tidbVarsFile)
				break
			}
		}
		vardefConsts = sysVarExtractor.GetVardefConsts()
	}

	f, err := os.Open(upgradeFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open upgrade logic file %s: %w", upgradeFilePath, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var (
		inUpgradeFunc bool
		curFunc       string
		curVersion    string
		curComment    string
		results       []kbgenerator.UpgradeParamChange
		braceDepth    int // Track brace depth to detect function end
	)

	// Match upgradeToVerXX function definition
	// Pattern: func upgradeToVer65(...) or func upgradeToVer75(...)
	// Extract function name and version number (e.g., "65" from "upgradeToVer65")
	funcRe := regexp.MustCompile(`^func (upgradeToVer(\d+))\b`)

	// Match setGlobalSysVar/variable writing calls
	// Pattern: setGlobalSysVar(variableName, value) or writeGlobalSysVar(variableName, value)
	// For initGlobalVariableIfNotExists: initGlobalVariableIfNotExists(s, variable.XXX, value)
	setVarRe := regexp.MustCompile(`(setGlobalSysVar|writeGlobalSysVar)\s*\(\s*([^,]+)\s*,\s*([^,\)]+)`)
	// Separate regex for initGlobalVariableIfNotExists (has 3 parameters: session, varName, value)
	initGlobalVarRe := regexp.MustCompile(`initGlobalVariableIfNotExists\s*\(\s*[^,]+,\s*([^,]+)\s*,\s*([^,\)]+)`)

	// Note: mustExecute SQL statements are now handled inline to only match system variable changes

	// Match SetGlobalSysVar calls
	// Pattern: SetGlobalSysVar(variableName, value) or GlobalVarsAccessor.SetGlobalSysVar(...)
	// Also match: variable.TiDBEnableAsyncMergeGlobalStats (constant name)
	setGlobalWithVarRe := regexp.MustCompile(`SetGlobalSysVar\([^,]*,\s*(variable\.\w+|[a-zA-Z0-9_"']+)`)

	// Match function comments for documentation
	commentRe := regexp.MustCompile(`^//\s*(.*)`)

	// Read all lines to process the file
	// This extracts all upgradeToVerXX functions from the latest version codebase
	// All historical upgrade functions are preserved in the latest TiDB code
	for scanner.Scan() {
		line := scanner.Text()

		// Detect upgradeToVerXX function start
		if m := funcRe.FindStringSubmatch(line); m != nil {
			inUpgradeFunc = true
			curFunc = m[1]     // e.g., "upgradeToVer65" or "upgradeToVer177"
			versionNum := m[2] // e.g., "65" or "177"
			// Keep bootstrap version number as-is (e.g., "65" or "177")
			// Bootstrap version numbers are internal TiDB version numbers, not release versions
			curVersion = versionNum
			curComment = ""
			braceDepth = 0
			// Count opening brace in function signature
			braceDepth += strings.Count(line, "{")
			braceDepth -= strings.Count(line, "}")
			continue
		}

		if inUpgradeFunc {
			// Track brace depth to detect function end
			braceDepth += strings.Count(line, "{")
			braceDepth -= strings.Count(line, "}")

			// Function ends when brace depth reaches 0
			if braceDepth == 0 {
				inUpgradeFunc = false
				continue
			}

			// Extract system variable changes from various patterns

			// Pattern 1a: initGlobalVariableIfNotExists(s, variable.XXX, value)
			if m := initGlobalVarRe.FindStringSubmatch(line); m != nil {
				varNameRaw := strings.TrimSpace(m[1])
				valueRaw := strings.TrimSpace(m[2])
				// Convert variable.XXX to user-visible name
				varName := convertVarNameToUserVisible(varNameRaw, vardefConsts)
				// Convert value (e.g., variable.Off -> "OFF")
				value := convertVarNameToUserVisible(valueRaw, vardefConsts)
				value = strings.Trim(value, "\" '`")
				// Normalize boolean values
				if strings.ToLower(value) == "off" {
					value = "OFF"
				} else if strings.ToLower(value) == "on" {
					value = "ON"
				}
				results = append(results, kbgenerator.UpgradeParamChange{
					Version:     curVersion,
					FuncName:    curFunc,
					VarName:     varName,
					Name:        varName,
					Value:       value,
					Method:      "initGlobalVariableIfNotExists",
					Comment:     curComment,
					Description: curComment,
					Force:       true,
					Type:        "system_variable",
					Severity:    "medium", // Default value behavior may have changed
				})
				continue
			}

			// Pattern 1b: setGlobalSysVar, writeGlobalSysVar
			if m := setVarRe.FindStringSubmatch(line); m != nil {
				method := m[1]
				varNameRaw := strings.Trim(m[2], "\" '`")
				value := strings.Trim(m[3], "\" '`")
				// Convert variable.XXX to user-visible name
				varName := convertVarNameToUserVisible(varNameRaw, vardefConsts)
				results = append(results, kbgenerator.UpgradeParamChange{
					Version:     curVersion,
					FuncName:    curFunc,
					VarName:     varName,
					Name:        varName,
					Value:       value,
					Method:      method,
					Comment:     curComment,
					Description: curComment,
					Force:       true,
					Type:        "system_variable",
					Severity:    "medium", // Default value behavior may have changed
				})
				continue
			}

			// Pattern 2: mustExecute with SQL statements containing system variable changes
			// Only match:
			// 1. SET @@GLOBAL var_name = value
			// 2. SQL statements that operate on mysql.global_variables table (UPDATE, DELETE, INSERT, REPLACE)
			// 3. SQL statements that use mysql.GlobalVariablesTable constant (via %n.%n or %s.%s formatting)
			if strings.Contains(line, "mustExecute") {
				// Pattern 2a: SET @@GLOBAL var_name = value
				// Example: mustExecute(s, "SET @@GLOBAL tidb_track_aggregate_memory_usage = 1")
				setGlobalRe := regexp.MustCompile(`SET\s+@@GLOBAL\.?\s*([a-zA-Z0-9_]+)\s*=\s*([0-9a-zA-Z_"']+)`)
				if m := setGlobalRe.FindStringSubmatch(line); m != nil {
					varName := m[1]
					value := strings.Trim(m[2], `"'`)
					results = append(results, kbgenerator.UpgradeParamChange{
						Version:     curVersion,
						FuncName:    curFunc,
						VarName:     varName,
						Name:        varName,
						Value:       value,
						Method:      "mustExecute",
						Comment:     curComment,
						Description: curComment,
						Force:       true,
						Type:        "system_variable",
						Severity:    "medium", // Default value behavior may have changed
					})
					continue
				}

				// Pattern 2b: Operations on mysql.global_variables table (string literal)
				// Only match if the SQL operates on mysql.global_variables table
				if strings.Contains(line, "mysql.global_variables") {
					// Pattern 2b-1: INSERT IGNORE INTO mysql.global_variables VALUES ('var_name', value)
					// Example: INSERT IGNORE INTO mysql.global_variables VALUES ('tidb_schema_cache_size', 0)
					// Note: INSERT IGNORE means if variable exists, keep existing value (non-forcing)
					insertValuesRe := regexp.MustCompile(`INSERT\s+(?:IGNORE\s+)?INTO\s+mysql\.global_variables\s+VALUES\s*\(['"]([^'"]+)['"]\s*,\s*([^)]+)\)`)
					if m := insertValuesRe.FindStringSubmatch(line); m != nil {
						varName := m[1]
						value := strings.TrimSpace(m[2])
						value = strings.Trim(value, `"'`)
						// INSERT IGNORE is non-forcing (preserves existing value if variable exists)
						isIgnore := strings.Contains(line, "INSERT") && strings.Contains(line, "IGNORE")
						method := "mustExecute-INSERT-IGNORE"
						if !isIgnore {
							method = "mustExecute-INSERT"
						}
						results = append(results, kbgenerator.UpgradeParamChange{
							Version:     curVersion,
							FuncName:    curFunc,
							VarName:     varName,
							Name:        varName,
							Value:       value,
							Method:      method,
							Comment:     curComment,
							Description: curComment,
							Force:       !isIgnore, // INSERT IGNORE is non-forcing
							Type:        "system_variable",
							Severity:    "medium", // Default value behavior may have changed
						})
						continue
					}

					// Pattern 2b-2: Extract variable name from WHERE clause: VARIABLE_NAME = 'var_name'
					// Handle both UPDATE and DELETE statements
					// For UPDATE statements with WHERE clause, also extract the old value (from_value) for value migration
					varNameRe := regexp.MustCompile(`(?:WHERE|where)\s+.*?VARIABLE_NAME\s*=\s*['"]([^'"]+)['"]`)
					if m := varNameRe.FindStringSubmatch(line); m != nil {
						varName := m[1]
						var value string
						var fromValue interface{} // Old value that will be mapped to new value

						// For DELETE statements, value is empty (variable is being deleted)
						if strings.Contains(line, "DELETE") || strings.Contains(line, "delete") {
							value = "" // DELETE operations don't have a value
						} else {
							// For UPDATE statements, extract both new value and old value (if present in WHERE clause)
							// Pattern: UPDATE ... SET VARIABLE_VALUE='new_value' WHERE ... AND VARIABLE_VALUE = 'old_value'
							// This indicates a value migration (e.g., OFF -> '', ON -> 'table')
							oldValueRe := regexp.MustCompile(`AND\s+VARIABLE_VALUE\s*=\s*['"]([^'"]+)['"]`)
							if oldVM := oldValueRe.FindStringSubmatch(line); oldVM != nil {
								// This is a value migration: old value -> new value
								fromValue = oldVM[1]
							}

							// Extract new value from SET clause
							// Note: value can be empty string, so use [^'"]* instead of [^'"]+
							valueRe := regexp.MustCompile(`SET\s+.*?VARIABLE_VALUE\s*=\s*['"]([^'"]*)['"]`)
							if vm := valueRe.FindStringSubmatch(line); len(vm) >= 2 {
								value = vm[1] // This can be empty string
							} else {
								// Try alternative pattern (without SET)
								setValueRe := regexp.MustCompile(`VARIABLE_VALUE\s*=\s*['"]([^'"]*)['"]`)
								if svm := setValueRe.FindStringSubmatch(line); len(svm) >= 2 {
									value = svm[1]
								}
							}
						}

						// Determine severity based on operation type
						// UPDATE and REPLACE: medium risk (default value behavior may have changed)
						// DELETE: low-medium risk (parameter is deprecated)
						severity := "medium"
						if strings.Contains(line, "DELETE") || strings.Contains(line, "delete") {
							severity = "low-medium"
						}

						change := kbgenerator.UpgradeParamChange{
							Version:     curVersion,
							FuncName:    curFunc,
							VarName:     varName,
							Name:        varName,
							Value:       value,
							Method:      "mustExecute",
							Comment:     curComment,
							Description: curComment,
							Force:       true,
							Type:        "system_variable",
							Severity:    severity,
						}
						// Add from_value if this is a value migration
						if fromValue != nil {
							change.FromValue = fromValue
						}

						results = append(results, change)
						continue
					}
				}

				// Pattern 2c: Operations on mysql.GlobalVariablesTable (constant reference)
				// Match mustExecute calls that use mysql.GlobalVariablesTable as a parameter
				// Examples:
				//   mustExecute(s, "UPDATE HIGH_PRIORITY %n.%n SET ...", mysql.SystemDB, mysql.GlobalVariablesTable, ...)
				//   mustExecute(s, "INSERT HIGH_PRIORITY IGNORE INTO %n.%n VALUES (%?, %?);", mysql.SystemDB, mysql.GlobalVariablesTable, varName, value)
				//   mustExecute(s, "REPLACE HIGH_PRIORITY INTO %n.%n VALUES (%?, %?);", mysql.SystemDB, mysql.GlobalVariablesTable, varName, value)
				//   mustExecute(s, fmt.Sprintf("INSERT IGNORE INTO %s.%s ...", mysql.SystemDB, mysql.GlobalVariablesTable, ...))
				//   mustExecute(s, fmt.Sprintf("UPDATE %s.%s SET ...", mysql.SystemDB, mysql.GlobalVariablesTable, ...))
				//
				// Semantic differences:
				// - INSERT IGNORE: If variable exists, ignore insert and keep existing value (user's setting or previous default)
				// - REPLACE: Force update the variable value even if it exists
				if strings.Contains(line, "GlobalVariablesTable") {
					// Match INSERT/REPLACE statements with VALUES clause
					// Pattern: INSERT/REPLACE ... INTO %n.%n VALUES (%?, %?) or fmt.Sprintf("INSERT ... INTO %s.%s VALUES ...", ..., mysql.GlobalVariablesTable, ...)
					// Also match fmt.Sprintf("INSERT IGNORE INTO %s.%s ...", ...) format
					insertReplaceRe := regexp.MustCompile(`(?:INSERT|REPLACE).*?INTO.*?%[ns]\.%[ns].*?VALUES.*?\(.*?%[?]?.*?,\s*%[?]?.*?\)`)
					// Also match fmt.Sprintf format: fmt.Sprintf("INSERT IGNORE INTO %s.%s ...", ...)
					fmtSprintfInsertRe := regexp.MustCompile(`fmt\.Sprintf.*?INSERT.*?IGNORE.*?INTO.*?%[ns]\.%[ns]`)
					if insertReplaceRe.MatchString(line) || fmtSprintfInsertRe.MatchString(line) {
						// Extract variable name and value from parameters
						// Pattern: mustExecute(s, "...", mysql.SystemDB, mysql.GlobalVariablesTable, variable.VarName, value)
						// Or: mustExecute(s, "...", mysql.SystemDB, mysql.GlobalVariablesTable, "var_name", "value")
						varNameRe := regexp.MustCompile(`mysql\.GlobalVariablesTable\s*,\s*([^,)]+)`)
						valueRe := regexp.MustCompile(`mysql\.GlobalVariablesTable\s*,\s*[^,]+,\s*([^,)]+)`)

						var varNameRaw, valueRaw string
						if m := varNameRe.FindStringSubmatch(line); m != nil {
							varNameRaw = strings.TrimSpace(m[1])
							// Try to extract value
							if vm := valueRe.FindStringSubmatch(line); vm != nil {
								valueRaw = strings.TrimSpace(vm[1])
							}
						}

						if varNameRaw != "" {
							// Convert variable.XXX to user-visible name
							varName := convertVarNameToUserVisible(varNameRaw, vardefConsts)
							value := ""
							if valueRaw != "" {
								value = convertVarNameToUserVisible(valueRaw, vardefConsts)
								value = strings.Trim(value, "\" '`")
								// Normalize boolean values
								if strings.ToLower(value) == "off" {
									value = "OFF"
								} else if strings.ToLower(value) == "on" {
									value = "ON"
								}
							}

							// Distinguish between INSERT IGNORE and REPLACE
							// INSERT IGNORE: If variable exists, keep existing value (non-forcing)
							// REPLACE: Force update the variable value (forcing)
							method := "mustExecute"
							if strings.Contains(line, "REPLACE") {
								method = "mustExecute-REPLACE"
							} else if strings.Contains(line, "INSERT") {
								// Check if it's INSERT IGNORE (non-forcing) or regular INSERT (forcing)
								if strings.Contains(line, "IGNORE") {
									method = "mustExecute-INSERT-IGNORE"
								} else {
									method = "mustExecute-INSERT"
								}
							}

							// INSERT IGNORE is non-forcing (preserves existing value if variable exists)
							// REPLACE and regular INSERT are forcing (always set the value)
							force := true
							if method == "mustExecute-INSERT-IGNORE" {
								force = false
							}

							// Determine severity based on operation type
							// REPLACE: medium risk (default value behavior may have changed)
							// INSERT: medium risk (default value behavior may have changed)
							severity := "medium"
							if strings.Contains(method, "REPLACE") {
								severity = "medium" // REPLACE: medium risk
							}

							results = append(results, kbgenerator.UpgradeParamChange{
								Version:     curVersion,
								FuncName:    curFunc,
								VarName:     varName,
								Name:        varName,
								Value:       value,
								Method:      method,
								Comment:     curComment,
								Description: curComment,
								Force:       force,
								Type:        "system_variable",
								Severity:    severity,
							})
							continue
						}
					}

					// Match UPDATE statements
					// Pattern: UPDATE ... %n.%n SET VARIABLE_VALUE = ... WHERE VARIABLE_NAME = ...
					updateRe := regexp.MustCompile(`UPDATE.*?%[ns]\.%[ns].*?SET.*?VARIABLE_VALUE`)
					if updateRe.MatchString(line) {
						// Extract variable name from WHERE clause or from parameters
						// Pattern 1: WHERE VARIABLE_NAME = %? with variable name as parameter
						// Pattern 2: WHERE VARIABLE_NAME = 'var_name' (string literal)
						varNameRe := regexp.MustCompile(`WHERE.*?VARIABLE_NAME\s*=\s*(?:%[?]|['"]([^'"]+)['"])`)
						varNameRaw := ""
						if m := varNameRe.FindStringSubmatch(line); m != nil {
							if len(m) > 1 && m[1] != "" {
								// String literal
								varNameRaw = m[1]
							} else {
								// Parameter, try to extract from function call
								// Look for pattern: mysql.GlobalVariablesTable, ..., variable.VarName or "var_name"
								paramRe := regexp.MustCompile(`mysql\.GlobalVariablesTable\s*,\s*([^,)]+)`)
								if pm := paramRe.FindStringSubmatch(line); pm != nil {
									varNameRaw = strings.TrimSpace(pm[1])
								}
							}
						}

						// Extract value from SET clause or from parameters
						valueRaw := ""
						// Pattern: SET VARIABLE_VALUE = %? or SET VARIABLE_VALUE = 'value'
						valueRe := regexp.MustCompile(`SET.*?VARIABLE_VALUE\s*=\s*(?:%[?]|['"]([^'"]*)['"])`)
						if m := valueRe.FindStringSubmatch(line); m != nil {
							if len(m) > 1 && m[1] != "" {
								// String literal
								valueRaw = m[1]
							} else {
								// Parameter, try to extract from function call
								// Look for pattern after variable name parameter
								paramValueRe := regexp.MustCompile(`mysql\.GlobalVariablesTable\s*,\s*[^,]+,\s*([^,)]+)`)
								if pm := paramValueRe.FindStringSubmatch(line); pm != nil {
									valueRaw = strings.TrimSpace(pm[1])
								}
							}
						}

						// Extract from_value if present (value migration)
						var fromValue interface{}
						fromValueRe := regexp.MustCompile(`AND\s+VARIABLE_VALUE\s*=\s*(?:%[?]|['"]([^'"]+)['"])`)
						if m := fromValueRe.FindStringSubmatch(line); m != nil {
							if len(m) > 1 && m[1] != "" {
								fromValue = m[1]
							} else {
								// Parameter, try to extract from function call (last parameter)
								allParamsRe := regexp.MustCompile(`mysql\.GlobalVariablesTable\s*,\s*([^,]+,\s*[^,]+,\s*[^,)]+)`)
								if pm := allParamsRe.FindStringSubmatch(line); pm != nil {
									// Last parameter is from_value
									params := strings.Split(pm[1], ",")
									if len(params) >= 3 {
										fromValue = strings.TrimSpace(params[2])
									}
								}
							}
						}

						if varNameRaw != "" {
							varName := convertVarNameToUserVisible(varNameRaw, vardefConsts)
							value := ""
							if valueRaw != "" {
								value = convertVarNameToUserVisible(valueRaw, vardefConsts)
								value = strings.Trim(value, "\" '`")
								// Normalize boolean values
								if strings.ToLower(value) == "off" {
									value = "OFF"
								} else if strings.ToLower(value) == "on" {
									value = "ON"
								}
							}

							change := kbgenerator.UpgradeParamChange{
								Version:     curVersion,
								FuncName:    curFunc,
								VarName:     varName,
								Name:        varName,
								Value:       value,
								Method:      "mustExecute-UPDATE",
								Comment:     curComment,
								Description: curComment,
								Force:       true,
								Type:        "system_variable",
								Severity:    "medium", // UPDATE: medium risk (default value behavior may have changed)
							}
							if fromValue != nil {
								change.FromValue = fromValue
							}

							results = append(results, change)
							continue
						}
					}

					// Match DELETE statements
					// Pattern: DELETE FROM %n.%n WHERE VARIABLE_NAME = ...
					deleteRe := regexp.MustCompile(`DELETE.*?FROM.*?%[ns]\.%[ns].*?WHERE.*?VARIABLE_NAME`)
					if deleteRe.MatchString(line) {
						// Extract variable name
						varNameRe := regexp.MustCompile(`WHERE.*?VARIABLE_NAME\s*=\s*(?:%[?]|['"]([^'"]+)['"])`)
						varNameRaw := ""
						if m := varNameRe.FindStringSubmatch(line); m != nil {
							if len(m) > 1 && m[1] != "" {
								varNameRaw = m[1]
							} else {
								// Parameter
								paramRe := regexp.MustCompile(`mysql\.GlobalVariablesTable\s*,\s*([^,)]+)`)
								if pm := paramRe.FindStringSubmatch(line); pm != nil {
									varNameRaw = strings.TrimSpace(pm[1])
								}
							}
						}

						if varNameRaw != "" {
							varName := convertVarNameToUserVisible(varNameRaw, vardefConsts)
							results = append(results, kbgenerator.UpgradeParamChange{
								Version:     curVersion,
								FuncName:    curFunc,
								VarName:     varName,
								Name:        varName,
								Value:       "", // DELETE operations don't have a value
								Method:      "mustExecute-DELETE",
								Comment:     curComment,
								Description: curComment,
								Force:       true,
								Type:        "system_variable",
								Severity:    "low-medium", // DELETE: low-medium risk (parameter is deprecated)
							})
							continue
						}
					}
				}
			}

			// Pattern 3: SetGlobalSysVar calls (including GlobalVarsAccessor.SetGlobalSysVar)
			// Match: SetGlobalSysVar(context.Background(), variable.TiDBEnableAsyncMergeGlobalStats, variable.Off)
			if strings.Contains(line, "SetGlobalSysVar") {
				// Try to extract variable name and value
				// Pattern 1: SetGlobalSysVar(..., variable.VarName, value)
				if m := setGlobalWithVarRe.FindStringSubmatch(line); m != nil {
					varNameRaw := strings.TrimSpace(m[1])
					// Extract value (could be after the variable name)
					valueMatch := regexp.MustCompile(`SetGlobalSysVar\([^,]*,\s*[^,]+,\s*([^)]+)`)
					var valueRaw string
					if vm := valueMatch.FindStringSubmatch(line); len(vm) >= 2 {
						valueRaw = strings.TrimSpace(vm[1])
						// Remove trailing comma or closing paren if present
						valueRaw = strings.TrimRight(valueRaw, ",)")
					} else {
						// If no explicit value, try to find it in the same line
						valueRaw = "unknown"
					}

					// Convert variable.XXX to user-visible name
					varName := convertVarNameToUserVisible(varNameRaw, vardefConsts)
					// Convert value (e.g., variable.Off -> "OFF")
					value := convertVarNameToUserVisible(valueRaw, vardefConsts)
					value = strings.Trim(value, "\" '`")
					// Normalize boolean values
					if strings.ToLower(value) == "off" {
						value = "OFF"
					} else if strings.ToLower(value) == "on" {
						value = "ON"
					}

					results = append(results, kbgenerator.UpgradeParamChange{
						Version:     curVersion,
						FuncName:    curFunc,
						VarName:     varName,
						Name:        varName,
						Value:       value,
						Method:      "SetGlobalSysVar",
						Comment:     curComment,
						Description: curComment,
						Force:       true,
						Type:        "system_variable",
						Severity:    "medium", // Default value behavior may have changed
					})
					continue
				}
			}

			// Extract function comments for documentation
			if m := commentRe.FindStringSubmatch(line); m != nil {
				curComment = m[1]
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &kbgenerator.UpgradeLogicSnapshot{
		Component: kbgenerator.ComponentTiDB,
		Changes:   results,
	}, nil
}

// convertVarNameToUserVisible converts variable constant names to user-visible names
// Examples:
//   - variable.TiDBOptRangeMaxSize -> tidb_opt_range_max_size (from vardefConsts)
//   - variable.Off -> "OFF" (from vardefConsts or known constants)
//   - "tidb_opt_range_max_size" -> "tidb_opt_range_max_size" (already user-visible)
func convertVarNameToUserVisible(varNameRaw string, vardefConsts map[string]string) string {
	varNameRaw = strings.TrimSpace(varNameRaw)

	// If it's already a user-visible name (starts with lowercase or contains underscore), return as-is
	if strings.HasPrefix(varNameRaw, "\"") && strings.HasSuffix(varNameRaw, "\"") {
		// It's a string literal, return without quotes
		return strings.Trim(varNameRaw, "\"")
	}

	// Handle variable.XXX pattern
	if strings.HasPrefix(varNameRaw, "variable.") {
		constName := strings.TrimPrefix(varNameRaw, "variable.")
		if userVisible, ok := vardefConsts[constName]; ok {
			return userVisible
		}
		// If not found, try to convert camelCase to snake_case as fallback
		return camelToSnake(constName)
	}

	// Handle vardef.XXX pattern
	if strings.HasPrefix(varNameRaw, "vardef.") {
		constName := strings.TrimPrefix(varNameRaw, "vardef.")
		if userVisible, ok := vardefConsts[constName]; ok {
			return userVisible
		}
		return camelToSnake(constName)
	}

	// If it's a constant name (starts with uppercase), look it up
	if len(varNameRaw) > 0 && varNameRaw[0] >= 'A' && varNameRaw[0] <= 'Z' {
		// First try direct lookup
		if userVisible, ok := vardefConsts[varNameRaw]; ok {
			return userVisible
		}
		// Known constants
		if varNameRaw == "Off" || varNameRaw == "variable.Off" || strings.ToLower(varNameRaw) == "off" {
			return "OFF"
		}
		if varNameRaw == "On" || varNameRaw == "variable.On" || strings.ToLower(varNameRaw) == "on" {
			return "ON"
		}
		// If not found and it looks like a TiDB variable name, try to find it
		// This shouldn't happen if vardefConsts is properly populated
		// Fallback: convert camelCase to snake_case (but this is not ideal)
		return camelToSnake(varNameRaw)
	}

	// Return as-is if it looks like a user-visible name or value
	return varNameRaw
}

// camelToSnake converts CamelCase to snake_case
// Example: TiDBOptRangeMaxSize -> tidb_opt_range_max_size
// This is a fallback function - should prefer looking up in vardefConsts first
func camelToSnake(s string) string {
	if len(s) == 0 {
		return s
	}

	var result strings.Builder
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		isUpper := r >= 'A' && r <= 'Z'

		if i > 0 {
			prevR := runes[i-1]
			prevIsUpper := prevR >= 'A' && prevR <= 'Z'
			prevIsLower := prevR >= 'a' && prevR <= 'z'

			// Add underscore before uppercase if:
			// 1. Previous was lowercase (e.g., Opt -> _opt)
			// 2. Previous was uppercase and current is uppercase but next is lowercase (e.g., DB in TiDBCost -> d_b)
			if isUpper && prevIsLower {
				result.WriteByte('_')
			} else if isUpper && prevIsUpper && i < len(runes)-1 {
				// Check if next is lowercase (e.g., TiDBCost -> TiDB_Cost)
				nextR := runes[i+1]
				if nextR >= 'a' && nextR <= 'z' {
					result.WriteByte('_')
				}
			}
		}

		if isUpper {
			result.WriteRune(r + 32) // Convert to lowercase
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}
