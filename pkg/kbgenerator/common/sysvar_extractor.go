// Package common provides common utilities for knowledge base generation
package common

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator"
)

// SysVarExtractor extracts system variable defaults from Go source code using AST
type SysVarExtractor struct {
	// VarNameMapper maps variable names (optional, for custom mapping)
	VarNameMapper func(string) string
	// VardefDir is the directory containing vardef constants
	VardefDir string
	// Output stores extracted system variables
	Output kbgenerator.SystemVariables
	// vardefConsts caches parsed vardef constants
	vardefConsts map[string]string
}

// NewSysVarExtractor creates a new system variable extractor
func NewSysVarExtractor(vardefDir string) *SysVarExtractor {
	extractor := &SysVarExtractor{
		VardefDir:    vardefDir,
		Output:       make(kbgenerator.SystemVariables),
		vardefConsts: make(map[string]string),
	}
	// Parse vardef constants
	extractor.parseVardefConstants()
	return extractor
}

// GetVardefConsts returns the parsed vardef constants map
// This allows other packages to reuse the parsed constants without re-parsing
func (e *SysVarExtractor) GetVardefConsts() map[string]string {
	return e.vardefConsts
}

// ExtractFromFile extracts system variables from a Go source file
// This handles sysvar.go files that define system variables
// Also parses tidb_vars.go files to extract system variable name constants
func (e *SysVarExtractor) ExtractFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, data, parser.ParseComments)
	if err != nil {
		// Fallback to regex parsing if AST parsing fails
		return e.extractSysVarsWithRegex(string(data))
	}

	// If this is tidb_vars.go, parse constants first to populate vardefConsts
	if strings.HasSuffix(filePath, "tidb_vars.go") {
		e.parseConstantsFromFile(node)
		// For tidb_vars.go, we only parse constants, don't extract system variables
		return nil
	}

	// If this is sysvar.go, also parse constants from it (starting from line 3513)
	// These constants define system variable names like CharacterSetConnection = "character_set_connection"
	if strings.HasSuffix(filePath, "sysvar.go") {
		e.parseConstantsFromFile(node)
	}

	// Use AST parsing for sysvar.go files
	ast.Walk(e, node)

	// Use regex as fallback, but with strict filtering to avoid incorrect values
	// Only add if not already extracted by AST (to avoid overwriting correct values)
	e.extractSysVarsWithRegex(string(data))

	return nil
}

// parseConstantsFromFile parses constant declarations from an AST file
// This extracts system variable name constants like TiDBDDLReorgWorkerCount = "tidb_ddl_reorg_worker_cnt"
// Handles both single const declarations and const blocks
func (e *SysVarExtractor) parseConstantsFromFile(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok {
			if genDecl.Tok == token.CONST {
				for _, spec := range genDecl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						// Handle const blocks where multiple constants are declared
						// Each constant may have its own value, or inherit from previous
						for i, name := range valueSpec.Names {
							constName := name.Name
							// Determine which value to use
							var value ast.Expr
							if len(valueSpec.Values) > i {
								// This constant has its own value
								value = valueSpec.Values[i]
							} else if len(valueSpec.Values) > 0 {
								// Use the last value (for const blocks like: const (A = "a"; B))
								value = valueSpec.Values[len(valueSpec.Values)-1]
							}

							if value != nil {
								// Extract values from constants
								if basicLit, ok := value.(*ast.BasicLit); ok {
									if basicLit.Kind == token.STRING {
										// String literal: TiDBDDLReorgWorkerCount = "tidb_ddl_reorg_worker_cnt"
										e.vardefConsts[constName] = strings.Trim(basicLit.Value, `"`)
									} else if basicLit.Kind == token.INT || basicLit.Kind == token.FLOAT {
										// Numeric literal: DefTiDBDDLReorgBatchSize = 256
										e.vardefConsts[constName] = basicLit.Value
									}
								} else if val, _ := e.extractValue(value); val != nil {
									// Extract other values (e.g., expressions, identifiers)
									e.vardefConsts[constName] = fmt.Sprintf("%v", val)
								}
							}
						}
					}
				}
			}
			// Also extract VAR declarations for default values (e.g., DefTiDBDDLReorgBatchSize = 256)
			if genDecl.Tok == token.VAR {
				for _, spec := range genDecl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for i, name := range valueSpec.Names {
							varName := name.Name
							var value ast.Expr
							if len(valueSpec.Values) > i {
								value = valueSpec.Values[i]
							} else if len(valueSpec.Values) > 0 {
								value = valueSpec.Values[len(valueSpec.Values)-1]
							}
							if value != nil {
								// Try to extract value using extractValue first
								if val, _ := e.extractValue(value); val != nil {
									e.vardefConsts[varName] = fmt.Sprintf("%v", val)
								} else if basicLit, ok := value.(*ast.BasicLit); ok {
									// Handle basic literals directly (INT, FLOAT, STRING)
									if basicLit.Kind == token.INT || basicLit.Kind == token.FLOAT {
										e.vardefConsts[varName] = basicLit.Value
									} else if basicLit.Kind == token.STRING {
										e.vardefConsts[varName] = strings.Trim(basicLit.Value, `"`)
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
}

// Visit implements ast.Visitor
func (e *SysVarExtractor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	// Look for composite literals that might be SysVar definitions
	// Pattern: &SysVar{Name: "var_name", Value: "value", ...}
	if compLit, ok := node.(*ast.CompositeLit); ok {
		if sel, ok := compLit.Type.(*ast.SelectorExpr); ok {
			// Check if it's a SysVar type
			if sel.Sel.Name == "SysVar" || sel.Sel.Name == "SysVarType" {
				varName := ""
				varValue := ""
				var varType string = "string"
				var hasGlobalScope bool = false

				// First pass: extract Scope to check if it has global scope
				for _, elt := range compLit.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if ident, ok := kv.Key.(*ast.Ident); ok {
							if ident.Name == "Scope" {
								hasGlobalScope = e.checkGlobalScope(kv.Value)
								break
							}
						}
					}
				}

				// Only extract other fields if it has global scope
				if !hasGlobalScope {
					return e
				}

				// Second pass: extract Name and Value
				for _, elt := range compLit.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if ident, ok := kv.Key.(*ast.Ident); ok {
							switch ident.Name {
							case "Name":
								// Name can be either a string literal or an identifier (constant)
								// If it's an identifier, look it up in vardef constants
								if basicLit, ok := kv.Value.(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
									// String literal: Name: "tidb_ddl_reorg_worker_cnt"
									varName = strings.Trim(basicLit.Value, `"`)
								} else if ident, ok := kv.Value.(*ast.Ident); ok {
									// It's an identifier like TiDBDDLReorgWorkerCount
									// Look up the actual string value in vardef constants
									if vardefVal, ok := e.vardefConsts[ident.Name]; ok {
										varName = vardefVal
									} else {
										// If not found in vardef, skip this system variable
										// Code is the source of truth - we should only extract user-visible names from vardef
										continue
									}
								} else if sel, ok := kv.Value.(*ast.SelectorExpr); ok {
									// Handle cases like variable.TiDBDDLReorgWorkerCount or vardef.TiDBDDLReorgWorkerCount
									if ident, ok := sel.X.(*ast.Ident); ok {
										key := sel.Sel.Name
										// Try direct lookup first
										if vardefVal, ok := e.vardefConsts[key]; ok {
											varName = vardefVal
										} else if ident.Name == "variable" || ident.Name == "vardef" {
											// Try looking up in vardefConsts with the selector name
											if vardefVal, ok := e.vardefConsts[key]; ok {
												varName = vardefVal
											} else {
												// If not found in vardef, skip this system variable
												continue
											}
										} else {
											// Unknown selector, skip
											continue
										}
									}
								} else {
									// Unknown type, skip
									continue
								}
							case "Value":
								val, paramType := e.extractValue(kv.Value)
								if val != nil {
									// Convert value to string, but preserve type information
									varValue = fmt.Sprintf("%v", val)
									varType = paramType
								} else {
									// If extraction failed, we can't extract the value
									// Skip this system variable - we need a valid value
									// Don't use regex fallback as it might extract incorrect values
									return e
								}
							}
						}
					}
				}

				if varName != "" && varValue != "" {
					// Only add if we have both name and value
					// Handle vardef references
					if strings.HasPrefix(varValue, "vardef.") {
						key := strings.TrimPrefix(varValue, "vardef.")
						if v, ok := e.vardefConsts[key]; ok {
							varValue = v
						} else {
							// Can't resolve vardef reference, skip this system variable
							return e
						}
					}

					// Final check: skip if value is still an identifier or function call
					if !strings.HasPrefix(varValue, "\"") && !strings.HasSuffix(varValue, "\"") {
						// Check if it's an identifier (starts with capital letter)
						identRe := regexp.MustCompile(`^[A-Z][a-zA-Z0-9_]*$`)
						if identRe.MatchString(varValue) {
							// It's an identifier, check if it's in vardefConsts
							if _, ok := e.vardefConsts[varValue]; !ok {
								// Not a known constant, skip it
								return e
							}
						}
						// Check if it contains function call or selector expression
						if strings.Contains(varValue, "(") || (strings.Contains(varValue, ".") && !strings.HasPrefix(varValue, "\"")) {
							return e
						}
					}

					e.Output[varName] = kbgenerator.ParameterValue{
						Value: varValue,
						Type:  varType,
					}
				}
			}
		}
	}

	// Look for variable declarations like: var defaultSysVars = []*SysVar{...}
	// This is the main source of system variable definitions
	if genDecl, ok := node.(*ast.GenDecl); ok {
		if genDecl.Tok == token.VAR {
			for _, spec := range genDecl.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					// Check if this is defaultSysVars variable
					if len(valueSpec.Names) > 0 && valueSpec.Names[0].Name == "defaultSysVars" {
						// Check if it's a slice of *SysVar
						if arrType, ok := valueSpec.Type.(*ast.ArrayType); ok {
							if ptrType, ok := arrType.Elt.(*ast.StarExpr); ok {
								if sel, ok := ptrType.X.(*ast.SelectorExpr); ok {
									if sel.Sel.Name == "SysVar" {
										// Extract from array literal
										if len(valueSpec.Values) > 0 {
											if compLit, ok := valueSpec.Values[0].(*ast.CompositeLit); ok {
												for _, elt := range compLit.Elts {
													if compVar, ok := elt.(*ast.CompositeLit); ok {
														e.extractSysVarFromComposite(compVar)
													}
												}
											}
										}
									}
								}
							}
						}
					}
					// Also handle other SysVar arrays for backward compatibility
					if arrType, ok := valueSpec.Type.(*ast.ArrayType); ok {
						if ptrType, ok := arrType.Elt.(*ast.StarExpr); ok {
							if sel, ok := ptrType.X.(*ast.SelectorExpr); ok {
								if sel.Sel.Name == "SysVar" {
									// Extract from array literal
									if len(valueSpec.Values) > 0 {
										if compLit, ok := valueSpec.Values[0].(*ast.CompositeLit); ok {
											for _, elt := range compLit.Elts {
												if compVar, ok := elt.(*ast.CompositeLit); ok {
													e.extractSysVarFromComposite(compVar)
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

	return e
}

// extractSysVarFromComposite extracts a system variable from a composite literal
func (e *SysVarExtractor) extractSysVarFromComposite(compLit *ast.CompositeLit) {
	varName := ""
	varValue := ""
	var varType string = "string"
	var hasGlobalScope bool = false

	// First pass: extract Scope to check if it has global scope
	for _, elt := range compLit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if ident, ok := kv.Key.(*ast.Ident); ok {
				if ident.Name == "Scope" {
					hasGlobalScope = e.checkGlobalScope(kv.Value)
					break
				}
			}
		}
	}

	// Only extract other fields if it has global scope
	if !hasGlobalScope {
		return
	}

	// Second pass: extract Name and Value
	for _, elt := range compLit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if ident, ok := kv.Key.(*ast.Ident); ok {
				switch ident.Name {
				case "Name":
					// Name can be either a string literal or an identifier (constant)
					// If it's an identifier, look it up in vardef constants
					if val, ok := extractStringValue(kv.Value); ok {
						// Check if it's a string literal (not just an identifier name)
						if basicLit, ok := kv.Value.(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
							varName = strings.Trim(val, `"`)
						} else {
							// It's an identifier name, look it up in vardef
							if vardefVal, ok := e.vardefConsts[val]; ok {
								varName = vardefVal
							} else {
								// If not found in vardef, skip this system variable
								// Code is the source of truth - we should only extract user-visible names from vardef
								return
							}
						}
					} else if ident, ok := kv.Value.(*ast.Ident); ok {
						// It's an identifier like TiDBDDLReorgWorkerCount
						// Look up the actual string value in vardef constants
						if vardefVal, ok := e.vardefConsts[ident.Name]; ok {
							varName = vardefVal
						} else {
							// If not found in vardef, skip this system variable
							// Code is the source of truth - we should only extract user-visible names from vardef
							continue
						}
					} else if sel, ok := kv.Value.(*ast.SelectorExpr); ok {
						// Handle cases like vardef.TiDBDDLReorgWorkerCount
						if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "vardef" {
							key := sel.Sel.Name
							if vardefVal, ok := e.vardefConsts[key]; ok {
								varName = vardefVal
							} else {
								// If not found in vardef, skip this system variable
								// Code is the source of truth - we should only extract user-visible names from vardef
								continue
							}
						}
					}
				case "Value":
					val, paramType := e.extractValue(kv.Value)
					if val != nil {
						varValue = fmt.Sprintf("%v", val)
						varType = paramType
					} else {
						// If extraction failed, skip this system variable
						// This prevents extracting identifiers like DefHostname
						return
					}
				}
			}
		}
	}

	if varName != "" && varValue != "" {
		// Handle vardef references
		if strings.HasPrefix(varValue, "vardef.") {
			key := strings.TrimPrefix(varValue, "vardef.")
			if v, ok := e.vardefConsts[key]; ok {
				varValue = v
			} else {
				// Can't resolve vardef reference, skip this system variable
				return
			}
		}

		// Final check: skip if value is still an identifier or function call
		if !strings.HasPrefix(varValue, "\"") && !strings.HasSuffix(varValue, "\"") {
			// Check if it's an identifier (starts with capital letter)
			identRe := regexp.MustCompile(`^[A-Z][a-zA-Z0-9_]*$`)
			if identRe.MatchString(varValue) {
				// It's an identifier, check if it's in vardefConsts
				if _, ok := e.vardefConsts[varValue]; !ok {
					// Not a known constant, skip it
					return
				}
			}
			// Check if it contains function call or selector expression
			if strings.Contains(varValue, "(") || (strings.Contains(varValue, ".") && !strings.HasPrefix(varValue, "\"")) {
				return
			}
		}

		e.Output[varName] = kbgenerator.ParameterValue{
			Value: varValue,
			Type:  varType,
		}
	}
}

// extractValue extracts a value and its type from an AST expression
func (e *SysVarExtractor) extractValue(expr ast.Expr) (interface{}, string) {
	switch v := expr.(type) {
	case *ast.BasicLit:
		switch v.Kind {
		case token.INT:
			if val, err := strconv.ParseInt(v.Value, 0, 64); err == nil {
				return float64(val), "int"
			}
		case token.FLOAT:
			if val, err := strconv.ParseFloat(v.Value, 64); err == nil {
				return val, "float"
			}
		case token.STRING:
			return strings.Trim(v.Value, `"`), "string"
		}
	case *ast.Ident:
		if v.Name == "true" {
			return true, "bool"
		}
		if v.Name == "false" {
			return false, "bool"
		}
		if v.Name == "nil" {
			return nil, "unknown"
		}
		// Check if it's a vardef constant (e.g., DefTiDBTTLDeleteWorkerCount)
		if e.vardefConsts != nil {
			if val, ok := e.vardefConsts[v.Name]; ok {
				// Try to parse as number if it's a numeric constant
				if num, err := strconv.ParseInt(val, 10, 64); err == nil {
					return float64(num), "int"
				}
				if num, err := strconv.ParseFloat(val, 64); err == nil {
					return num, "float"
				}
				return val, "string"
			}
		}
		// If it's an identifier that's not in vardefConsts, return nil to skip it
		// This prevents extracting identifiers like DefHostname, mysql.DefaultCharset, etc.
		return nil, "unknown"
	case *ast.SelectorExpr:
		// Handle vardef.ConstantName
		if ident, ok := v.X.(*ast.Ident); ok && ident.Name == "vardef" {
			key := v.Sel.Name
			if val, ok := e.vardefConsts[key]; ok {
				// Try to parse as number if it's a numeric constant
				if num, err := strconv.ParseInt(val, 10, 64); err == nil {
					return float64(num), "int"
				}
				if num, err := strconv.ParseFloat(val, 64); err == nil {
					return num, "float"
				}
				return val, "string"
			}
		}
		// For other selector expressions (e.g., mysql.DefaultCharset), return nil to skip
		return nil, "unknown"
	case *ast.CallExpr:
		// Handle function calls like strconv.Itoa(DefTiDBTTLDeleteWorkerCount)
		// Extract the actual argument value instead of the function name
		if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
			// Handle strconv.Itoa(value), strconv.FormatInt(value, base), etc.
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "strconv" {
				if sel.Sel.Name == "Itoa" || sel.Sel.Name == "FormatInt" || sel.Sel.Name == "FormatUint" {
					if len(v.Args) > 0 {
						// Extract the first argument (the number to format)
						return e.extractValue(v.Args[0])
					}
				}
				if sel.Sel.Name == "FormatFloat" {
					if len(v.Args) > 0 {
						// Extract the first argument (the float to format)
						return e.extractValue(v.Args[0])
					}
				}
				if sel.Sel.Name == "FormatBool" {
					if len(v.Args) > 0 {
						// Extract the first argument (the bool to format)
						return e.extractValue(v.Args[0])
					}
				}
			}
		}
		// For other function calls, try to extract arguments if possible
		if len(v.Args) > 0 {
			// Try to extract the first argument as a fallback
			value, paramType := e.extractValue(v.Args[0])
			if value != nil {
				return value, paramType
			}
		}
		// If we can't extract a value from function call, return nil
		return nil, "unknown"
	}
	return nil, "unknown"
}

// extractStringValue extracts a string value from an AST expression
// This function is used for extracting system variable names, not values
// For values, use extractValue which properly handles constants
func extractStringValue(expr ast.Expr) (string, bool) {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			return strings.Trim(v.Value, `"`), true
		}
	case *ast.Ident:
		// Only return identifier name for system variable names, not values
		// Values should be resolved through vardefConsts
		return v.Name, true
	}
	return "", false
}

// parseVardefConstants parses constants from vardef directory
func (e *SysVarExtractor) parseVardefConstants() {
	if e.VardefDir == "" {
		return
	}

	filepath.WalkDir(e.VardefDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Parse constants using AST
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, data, parser.ParseComments)
		if err != nil {
			// Fallback to regex
			e.parseVardefConstantsWithRegex(string(data))
			return nil
		}

		// Use AST to extract constants
		ast.Inspect(node, func(n ast.Node) bool {
			if genDecl, ok := n.(*ast.GenDecl); ok {
				if genDecl.Tok == token.CONST {
					for _, spec := range genDecl.Specs {
						if valueSpec, ok := spec.(*ast.ValueSpec); ok {
							if len(valueSpec.Names) > 0 && len(valueSpec.Values) > 0 {
								constName := valueSpec.Names[0].Name
								// Try to extract value (can be string, number, or other types)
								val, _ := e.extractValue(valueSpec.Values[0])
								if val != nil {
									// Convert to string for storage
									e.vardefConsts[constName] = fmt.Sprintf("%v", val)
								} else if val, ok := extractStringValue(valueSpec.Values[0]); ok {
									e.vardefConsts[constName] = val
								}
							}
						}
					}
				}
				// Also extract VAR declarations for default values (e.g., DefTiDBTTLDeleteWorkerCount = 4)
				if genDecl.Tok == token.VAR {
					for _, spec := range genDecl.Specs {
						if valueSpec, ok := spec.(*ast.ValueSpec); ok {
							if len(valueSpec.Names) > 0 && len(valueSpec.Values) > 0 {
								varName := valueSpec.Names[0].Name
								// Extract numeric or string values
								val, _ := e.extractValue(valueSpec.Values[0])
								if val != nil {
									e.vardefConsts[varName] = fmt.Sprintf("%v", val)
								}
							}
						}
					}
				}
			}
			return true
		})

		return nil
	})
}

// parseVardefConstantsWithRegex parses constants using regex as fallback
func (e *SysVarExtractor) parseVardefConstantsWithRegex(content string) {
	// Pattern for const declarations: const Name = "value" or const Name = value
	constStrRe := regexp.MustCompile(`const\s+([A-Za-z0-9_]+)\s*=\s*"([^"]+)"`)
	matches := constStrRe.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) == 3 {
			e.vardefConsts[m[1]] = m[2]
		}
	}
	// Pattern for numeric const: const Name = 123
	constNumRe := regexp.MustCompile(`const\s+([A-Za-z0-9_]+)\s*=\s*(\d+)`)
	matches = constNumRe.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) == 3 {
			e.vardefConsts[m[1]] = m[2]
		}
	}
	// Pattern for var declarations: var Name = value (for default values like DefTiDBTTLDeleteWorkerCount = 4)
	varRe := regexp.MustCompile(`var\s+([A-Za-z0-9_]+)\s*=\s*(\d+)`)
	varMatches := varRe.FindAllStringSubmatch(content, -1)
	for _, m := range varMatches {
		if len(m) == 3 {
			e.vardefConsts[m[1]] = m[2]
		}
	}
}

// extractSysVarsWithRegex extracts system variables using regex patterns as fallback
func (e *SysVarExtractor) extractSysVarsWithRegex(content string) error {
	// Pattern: Scope: ScopeGlobal | ScopeSession, Name: "var_name", ... Value: "value"
	// We need to match the entire SysVar definition to check scope
	// Match pattern: {Scope: ..., Name: ..., Value: ...}
	sysVarRe := regexp.MustCompile(`\{[\s\S]*?Scope:\s*([^,}]+),[\s\S]*?Name:\s*("?[A-Za-z0-9_\.]+"?),[\s\S]*?Value:\s*([^,}]+)`)
	matches := sysVarRe.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) >= 4 {
			scopeStr := strings.TrimSpace(m[1])
			name := strings.Trim(m[2], "\"")
			val := strings.TrimSpace(m[3])
			val = strings.Trim(val, "\"'`")

			// Check if scope includes ScopeGlobal or ScopeInstance
			// ScopeGlobal = 1, ScopeInstance = 4, ScopeGlobal | ScopeSession = 1 | 2 = 3
			// Pattern: ScopeGlobal, ScopeGlobal | ScopeSession, ScopeInstance, etc.
			hasGlobalScope := strings.Contains(scopeStr, "ScopeGlobal") || strings.Contains(scopeStr, "ScopeInstance")
			if !hasGlobalScope {
				// Skip system variables without global scope
				continue
			}

			// Handle function calls like strconv.Itoa(DefTiDBDDLReorgBatchSize)
			if strings.HasPrefix(val, "strconv.Itoa(") {
				// Extract the argument: DefTiDBDDLReorgBatchSize
				argRe := regexp.MustCompile(`strconv\.Itoa\(([A-Za-z0-9_]+)\)`)
				if argMatch := argRe.FindStringSubmatch(val); len(argMatch) > 1 {
					constName := argMatch[1]
					if constVal, ok := e.vardefConsts[constName]; ok {
						val = constVal
					} else {
						// Skip if we can't resolve the constant
						continue
					}
				} else {
					// Skip if we can't parse the function call
					continue
				}
			} else if strings.HasPrefix(val, "strconv.Format") {
				// Handle other strconv functions similarly
				argRe := regexp.MustCompile(`strconv\.Format\w+\(([A-Za-z0-9_]+)`)
				if argMatch := argRe.FindStringSubmatch(val); len(argMatch) > 1 {
					constName := argMatch[1]
					if constVal, ok := e.vardefConsts[constName]; ok {
						val = constVal
					} else {
						continue
					}
				} else {
					continue
				}
			} else if strings.Contains(val, "(") && !strings.HasPrefix(val, "\"") {
				// Skip other function calls like BoolToOnOff(...), config.getString(...), etc.
				// These are not actual default values
				continue
			} else if strings.Contains(val, ".") && !strings.HasPrefix(val, "\"") && !strings.HasSuffix(val, "\"") {
				// Skip identifier references like mysql.DefaultCharset
				// These are not actual default values
				continue
			} else if !strings.HasPrefix(val, "\"") && !strings.HasSuffix(val, "\"") {
				// Check if it's an identifier (starts with capital letter, no quotes)
				// Skip identifiers that are not in vardefConsts
				identRe := regexp.MustCompile(`^[A-Z][a-zA-Z0-9_]*$`)
				if identRe.MatchString(val) {
					// It's an identifier, check if it's in vardefConsts
					if _, ok := e.vardefConsts[val]; !ok {
						// Not a known constant, skip it
						continue
					}
				}
			}

			// Handle vardef references
			if strings.HasPrefix(val, "vardef.") {
				key := strings.TrimPrefix(val, "vardef.")
				if v, ok := e.vardefConsts[key]; ok {
					val = v
				} else {
					continue
				}
			}

			// Only add if not already extracted by AST (to avoid overwriting correct values)
			// Also try to map name using vardefConsts if it's an internal name
			finalName := name
			if vardefVal, ok := e.vardefConsts[name]; ok {
				// Map internal name to user-visible name
				finalName = vardefVal
			}

			// Final check: skip if value is still an identifier or function call
			if !strings.HasPrefix(val, "\"") && !strings.HasSuffix(val, "\"") {
				// Check if it's an identifier (starts with capital letter)
				identRe := regexp.MustCompile(`^[A-Z][a-zA-Z0-9_]*$`)
				if identRe.MatchString(val) {
					// It's an identifier, check if it's in vardefConsts
					if _, ok := e.vardefConsts[val]; !ok {
						// Not a known constant, skip it
						continue
					}
				}
				// Check if it contains function call or selector expression
				if strings.Contains(val, "(") || (strings.Contains(val, ".") && !strings.HasPrefix(val, "\"")) {
					continue
				}
			}

			if _, exists := e.Output[finalName]; !exists {
				paramType := e.determineValueType(val)
				e.Output[finalName] = kbgenerator.ParameterValue{
					Value: val,
					Type:  paramType,
				}
			}
		}
	}
	return nil
}

// checkGlobalScope checks if a scope expression includes ScopeGlobal
// Handles patterns like: ScopeGlobal, ScopeGlobal | ScopeSession, ScopeInstance, etc.
func (e *SysVarExtractor) checkGlobalScope(expr ast.Expr) bool {
	switch v := expr.(type) {
	case *ast.Ident:
		// Direct identifier: ScopeGlobal, ScopeSession, ScopeNone, ScopeInstance
		if v.Name == "ScopeGlobal" {
			return true
		}
		// ScopeInstance is similar to global (can be set globally but doesn't propagate)
		if v.Name == "ScopeInstance" {
			return true
		}
		return false
	case *ast.BinaryExpr:
		// Binary expression: ScopeGlobal | ScopeSession, ScopeGlobal | ScopeInstance, etc.
		// Check if either side is ScopeGlobal or ScopeInstance
		if e.checkGlobalScope(v.X) || e.checkGlobalScope(v.Y) {
			return true
		}
		return false
	case *ast.SelectorExpr:
		// Selector expression: variable.ScopeGlobal (shouldn't happen, but handle it)
		if v.Sel.Name == "ScopeGlobal" || v.Sel.Name == "ScopeInstance" {
			return true
		}
		return false
	default:
		return false
	}
}

// determineValueType determines the type of a string value
func (e *SysVarExtractor) determineValueType(value string) string {
	if strings.HasSuffix(value, "B") || strings.HasSuffix(value, "KB") ||
		strings.HasSuffix(value, "MB") || strings.HasSuffix(value, "GB") ||
		strings.HasSuffix(value, "TB") {
		return "size"
	}
	if strings.HasSuffix(value, "s") || strings.HasSuffix(value, "m") ||
		strings.HasSuffix(value, "h") || strings.HasSuffix(value, "ms") ||
		strings.HasSuffix(value, "us") || strings.HasSuffix(value, "ns") {
		return "duration"
	}
	return "string"
}

// FindSysVarFiles finds system variable files in a TiDB repository
// Handles version differences (before and after v7.1.0)
func FindSysVarFiles(tidbRoot, version string) []string {
	var files []string
	var searchPaths []string

	// Extract major and minor version
	versionNum := extractVersionNumber(version)
	major, minor := parseVersion(versionNum)

	// TiDB 7.1+ uses pkg/ directory structure
	if major > 7 || (major == 7 && minor >= 1) {
		searchPaths = []string{
			filepath.Join(tidbRoot, "pkg", "sessionctx", "variable", "sysvar.go"),
			filepath.Join(tidbRoot, "pkg", "sessionctx", "variable", "tidb_vars.go"),
			filepath.Join(tidbRoot, "pkg", "sessionctx", "vardef"),
		}
	} else {
		// TiDB < 7.1 uses sessionctx/ directory (no pkg/)
		searchPaths = []string{
			filepath.Join(tidbRoot, "sessionctx", "variable", "sysvar.go"),
			filepath.Join(tidbRoot, "sessionctx", "variable", "tidb_vars.go"),
			filepath.Join(tidbRoot, "sessionctx", "vardef"),
		}
	}

	for _, path := range searchPaths {
		if info, err := os.Stat(path); err == nil {
			if info.IsDir() {
				// It's a directory, find all .go files
				filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
					if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
						return nil
					}
					files = append(files, p)
					return nil
				})
			} else {
				// It's a file
				files = append(files, path)
			}
		}
	}

	return files
}

// extractVersionNumber extracts version number from version tag
func extractVersionNumber(version string) string {
	version = strings.TrimPrefix(version, "v")
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		return parts[0] + parts[1]
	}
	return version
}

// parseVersion parses version number into major and minor
func parseVersion(versionNum string) (int, int) {
	if len(versionNum) >= 2 {
		if major, err := strconv.Atoi(versionNum[0:1]); err == nil {
			if minor, err := strconv.Atoi(versionNum[1:]); err == nil {
				return major, minor
			}
		}
	}
	return 0, 0
}
