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

	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// ConfigExtractor extracts configuration defaults from source code
// Supports both Go (using AST) and Rust (using regex) source files
type ConfigExtractor struct {
	// FieldNameMapper maps struct field names to config keys
	// If nil, for Go code: uses toml tags (real names from code), skips fields without toml tags
	// If nil, for Rust code: uses automatic conversion (snake_case -> kebab-case)
	FieldNameMapper func(string) string
	// ConfigVarName is the variable name used for config struct (e.g., "cfg", "config")
	// Only used for Go source files
	ConfigVarName string
	// DefaultPrefix is the prefix for default constants (e.g., "default")
	// Only used for Go source files
	DefaultPrefix string
	// Output stores extracted defaults
	Output types.ConfigDefaults
	// Current prefix for nested configs (e.g., "storage.", "raftstore.")
	// Only used for Rust source files
	currentPrefix string
	// Current file being parsed (for toml tag extraction)
	currentFile *ast.File
	// tomlTagMap stores field name -> toml tag mapping extracted from struct definitions
	tomlTagMap map[string]string
}

// NewConfigExtractor creates a new config extractor
func NewConfigExtractor(configVarName, defaultPrefix string) *ConfigExtractor {
	return &ConfigExtractor{
		ConfigVarName: configVarName,
		DefaultPrefix: defaultPrefix,
		Output:        make(types.ConfigDefaults),
		tomlTagMap:    make(map[string]string),
	}
}

// ExtractFromFile extracts configuration defaults from a source file
// Automatically detects file type (Go, Rust, or C++) based on file extension
func (e *ConfigExtractor) ExtractFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Detect file type by extension
	ext := filepath.Ext(filePath)
	switch ext {
	case ".rs":
		// Rust source file (TiKV)
		return e.extractFromRustCode(string(data))
	case ".cpp", ".cc", ".h", ".hpp":
		// C++ source file (TiFlash)
		// TODO: Implement C++ extraction logic
		return e.extractFromCppCode(string(data))
	default:
		// Go source file (default, for TiDB and PD)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filePath, data, parser.ParseComments)
		if err != nil {
			// Fallback to regex parsing if AST parsing fails
			return e.extractWithRegex(string(data))
		}

		// Store file reference for toml tag extraction
		e.currentFile = file

		// Use AST parsing
		ast.Walk(e, file)

		// Also try regex as fallback for patterns AST might miss
		e.extractWithRegex(string(data))

		return nil
	}
}

// ============================================================================
// Common Go Source Code Extraction (Shared by TiDB and PD)
// ============================================================================
// The following methods handle extraction of configuration defaults from Go
// source code (used by both TiDB and PD). These methods use Go's AST parser for
// accurate extraction and fallback to regex parsing when AST parsing fails.
// ============================================================================

// Visit implements ast.Visitor
func (e *ConfigExtractor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	// Look for struct field assignments like: cfg.FieldName = value
	// Also look for: cfg.Section.FieldName = value (nested config)
	if assign, ok := node.(*ast.AssignStmt); ok {
		if len(assign.Lhs) == 1 && len(assign.Rhs) == 1 {
			if sel, ok := assign.Lhs[0].(*ast.SelectorExpr); ok {
				// Handle nested selectors: cfg.Section.FieldName
				var fieldName string
				var configVarName string

				if innerSel, ok := sel.X.(*ast.SelectorExpr); ok {
					// Nested: cfg.Section.FieldName
					if ident, ok := innerSel.X.(*ast.Ident); ok && ident.Name == e.ConfigVarName {
						// For nested configs, we need to extract toml tags to get the correct key
						// This is handled in extractFromCompositeLiteral, so we skip direct assignments here
						// and rely on composite literal extraction
						return e
					}
				} else if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == e.ConfigVarName {
					// Direct: cfg.FieldName
					fieldName = sel.Sel.Name
				}

				if configVarName == e.ConfigVarName {
					value, _ := e.extractValue(assign.Rhs[0])
					if value != nil {
						configKey := e.mapFieldNameToConfigKey(fieldName)
						// Only add if we got a valid key
						if configKey != "" {
							// Apply unit if value is numeric and doesn't already have a unit
							finalValue := applyParameterUnit(configKey, value)
							e.Output[configKey] = types.ParameterValue{
								Value: finalValue,
								Type:  e.determineValueType(finalValue),
							}
						}
					}
				}
			}
		}
	}

	// Look for default constants like: defaultMaxStoreDownTime = 30 * time.Minute
	// Also look for: var defaultConf = Config{...} or var defaultConf Config = Config{...}
	if genDecl, ok := node.(*ast.GenDecl); ok {
		if genDecl.Tok == token.CONST || genDecl.Tok == token.VAR {
			for _, spec := range genDecl.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					if len(valueSpec.Names) > 0 && len(valueSpec.Values) > 0 {
						constName := valueSpec.Names[0].Name
						// Check if it's a default prefix constant
						if strings.HasPrefix(constName, e.DefaultPrefix) {
							value, _ := e.extractValue(valueSpec.Values[0])
							if value != nil {
								// Remove prefix and map to config key
								fieldName := strings.TrimPrefix(constName, e.DefaultPrefix)
								configKey := e.mapFieldNameToConfigKey(fieldName)
								// Only add if we got a valid key (not empty)
								if configKey != "" {
									// Apply unit if value is numeric and doesn't already have a unit
									finalValue := applyParameterUnit(configKey, value)
									e.Output[configKey] = types.ParameterValue{
										Value: finalValue,
										Type:  e.determineValueType(finalValue),
									}
								}
							}
						}
						// Check if it's a default config variable (e.g., var defaultConf = Config{...})
						if constName == "defaultConf" || strings.HasPrefix(constName, "default") {
							// Try to extract from composite literal
							if compLit, ok := valueSpec.Values[0].(*ast.CompositeLit); ok {
								e.extractFromCompositeLiteral(compLit, "")
							}
						}
					}
				}
			}
		}
	}

	// Also look for assignments to defaultConf: defaultConf = Config{...}
	if assign, ok := node.(*ast.AssignStmt); ok {
		if len(assign.Lhs) == 1 && len(assign.Rhs) == 1 {
			if ident, ok := assign.Lhs[0].(*ast.Ident); ok {
				if ident.Name == "defaultConf" || strings.HasPrefix(ident.Name, "default") {
					// Try to extract from composite literal
					if compLit, ok := assign.Rhs[0].(*ast.CompositeLit); ok {
						e.extractFromCompositeLiteral(compLit, "")
					}
				}
			}
		}
	}

	return e
}

// extractValue extracts a value and its type from an AST expression
func (e *ConfigExtractor) extractValue(expr ast.Expr) (interface{}, string) {
	switch v := expr.(type) {
	case *ast.BasicLit:
		switch v.Kind {
		case token.INT:
			if val, err := strconv.ParseInt(v.Value, 0, 64); err == nil {
				// Return as float64 for consistent handling, unit will be applied later
				return float64(val), "int"
			}
		case token.FLOAT:
			if val, err := strconv.ParseFloat(v.Value, 64); err == nil {
				return val, "float"
			}
		case token.STRING:
			val := strings.Trim(v.Value, `"`)
			paramType := e.determineValueType(val)
			return val, paramType
		case token.CHAR:
			return strings.Trim(v.Value, `'`), "string"
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
	case *ast.BinaryExpr:
		// Handle expressions like: 30 * time.Minute
		if v.Op == token.MUL {
			if lit, ok := v.X.(*ast.BasicLit); ok {
				if sel, ok := v.Y.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "time" {
						unit := sel.Sel.Name
						if num, err := strconv.ParseInt(lit.Value, 0, 64); err == nil {
							formatted := e.formatDuration(int(num), unit)
							return formatted, "duration"
						}
					}
				}
			}
		}
	case *ast.CompositeLit:
		// Handle typeutil.Duration{Duration: 30 * time.Minute}
		if sel, ok := v.Type.(*ast.SelectorExpr); ok {
			if sel.Sel.Name == "Duration" {
				for _, elt := range v.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if ident, ok := kv.Key.(*ast.Ident); ok && ident.Name == "Duration" {
							return e.extractValue(kv.Value)
						}
					}
				}
			}
		}
		// Handle []string{} or []interface{}{}
		if _, ok := v.Type.(*ast.ArrayType); ok {
			if len(v.Elts) == 0 {
				return []interface{}{}, "array"
			}
		}
	case *ast.CallExpr:
		// Handle function calls like strconv.Itoa(4), strconv.FormatInt(1024, 10), etc.
		// Extract the actual argument value instead of the function name
		if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
			// Handle strconv.Itoa(value) -> extract value
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
			// Handle other common conversion functions
			if sel.Sel.Name == "String" {
				// Handle .String() method calls - try to get the underlying value
				if len(v.Args) == 0 {
					// For methods without args, we can't extract the value easily
					// Return a placeholder indicating it's a method call
					return nil, "unknown"
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

// extractTomlTagsFromFile extracts toml tags from struct field definitions in the AST
func (e *ConfigExtractor) extractTomlTagsFromFile(file *ast.File) {
	if file == nil {
		return
	}

	// Initialize tomlTagMap if not already initialized
	if e.tomlTagMap == nil {
		e.tomlTagMap = make(map[string]string)
	}

	// Walk the AST to find struct type definitions
	ast.Inspect(file, func(n ast.Node) bool {
		if structType, ok := n.(*ast.StructType); ok {
			for _, field := range structType.Fields.List {
				// Extract field name
				if len(field.Names) > 0 {
					fieldName := field.Names[0].Name

					// Extract toml tag from field tag
					if field.Tag != nil {
						tagValue := strings.Trim(field.Tag.Value, "`")
						// Parse struct tag to find toml tag
						// Format: `toml:"key-name"` or `toml:"key-name,omitempty"`
						re := regexp.MustCompile(`toml:"([^"]+)"`)
						matches := re.FindStringSubmatch(tagValue)
						if len(matches) > 1 {
							tomlKey := matches[1]
							// Remove options like ",omitempty"
							if idx := strings.Index(tomlKey, ","); idx != -1 {
								tomlKey = tomlKey[:idx]
							}
							e.tomlTagMap[fieldName] = tomlKey
						}
					}
				}
			}
		}
		return true
	})
}

// extractFromCompositeLiteral extracts config defaults from a composite literal
// This handles cases like: defaultConf = Config{FieldName: value, Section: Section{Field: value}}
func (e *ConfigExtractor) extractFromCompositeLiteral(compLit *ast.CompositeLit, prefix string) {
	// Extract toml tags from current file if available and not already extracted
	if e.currentFile != nil && len(e.tomlTagMap) == 0 {
		e.extractTomlTagsFromFile(e.currentFile)
	}

	for _, elt := range compLit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			// Handle key-value pairs in composite literal
			if ident, ok := kv.Key.(*ast.Ident); ok {
				fieldName := ident.Name
				// Try to get toml tag value first (user-visible name from code)
				var configKey string
				if tomlTag, ok := e.tomlTagMap[fieldName]; ok && tomlTag != "" {
					configKey = tomlTag
				} else {
					// Fallback to field name mapping if toml tag not found
					configKey = e.mapFieldNameToConfigKey(fieldName)
				}
				// Avoid duplicate prefix (e.g., security.security.enable-sem -> security.enable-sem)
				if prefix != "" && !strings.HasPrefix(configKey, prefix) {
					configKey = prefix + "." + configKey
				} else if prefix != "" && strings.HasPrefix(configKey, prefix) {
					// If configKey already starts with prefix, use it as-is
					// e.g., prefix="security", configKey="security.enable-sem" -> "security.enable-sem"
				} else if prefix != "" {
					configKey = prefix + "." + configKey
				}

				// Try to extract value
				value, _ := e.extractValue(kv.Value)
				if value != nil {
					// Apply unit if value is numeric and doesn't already have a unit
					finalValue := applyParameterUnit(configKey, value)
					e.Output[configKey] = types.ParameterValue{
						Value: finalValue,
						Type:  e.determineValueType(finalValue),
					}
				} else if nestedCompLit, ok := kv.Value.(*ast.CompositeLit); ok {
					// Nested composite literal (e.g., Section: Section{Field: value})
					e.extractFromCompositeLiteral(nestedCompLit, configKey)
				}
			}
		}
	}
}

// determineValueType determines the type of a value
// Supports both string (for Go) and interface{} (for Rust) values
func (e *ConfigExtractor) determineValueType(value interface{}) string {
	var str string
	switch v := value.(type) {
	case string:
		str = v
	default:
		// For non-string values, check the type directly
		switch v.(type) {
		case bool:
			return "bool"
		case float64:
			return "number"
		default:
			return "unknown"
		}
	}

	// Check if it's a size (ends with B, KB, MB, GB)
	if matched, _ := regexp.MatchString(`^\d+(B|KB|MB|GB|TB)$`, str); matched {
		return "size"
	}
	// Check if it's a duration (ends with s, m, h, ms)
	if matched, _ := regexp.MatchString(`^\d+(s|m|h|ms|us|ns)$`, str); matched {
		return "duration"
	}
	// Check if it's a placeholder for constant (Rust)
	if strings.HasPrefix(str, "${") && strings.HasSuffix(str, "}") {
		return "constant"
	}
	return "string"
}

// formatDuration formats a duration value
func (e *ConfigExtractor) formatDuration(num int, unit string) string {
	switch unit {
	case "Second", "Seconds":
		return fmt.Sprintf("%ds", num)
	case "Minute", "Minutes":
		return fmt.Sprintf("%dm", num)
	case "Hour", "Hours":
		return fmt.Sprintf("%dh", num)
	case "Millisecond", "Milliseconds":
		return fmt.Sprintf("%dms", num)
	case "Microsecond", "Microseconds":
		return fmt.Sprintf("%dus", num)
	case "Nanosecond", "Nanoseconds":
		return fmt.Sprintf("%dns", num)
	default:
		return fmt.Sprintf("%d%s", num, strings.ToLower(unit))
	}
}

// mapFieldNameToConfigKey maps a struct field name to a config key
// For Go code: returns empty string (should use toml tags from extractFromCompositeLiteral instead)
// For Rust code: converts snake_case to kebab-case
// Returns empty string if no mapping is available
func (e *ConfigExtractor) mapFieldNameToConfigKey(fieldName string) string {
	if e.FieldNameMapper != nil {
		return e.FieldNameMapper(fieldName)
	}

	// Check if it's snake_case (Rust) or CamelCase (Go)
	// If contains underscore, treat as Rust snake_case
	if strings.Contains(fieldName, "_") {
		// Convert snake_case to kebab-case
		key := strings.ReplaceAll(fieldName, "_", "-")
		// Add prefix if set (for Rust nested configs)
		if e.currentPrefix != "" {
			return e.currentPrefix + key
		}
		return key
	}

	// For Go code, we should rely on toml tags extracted from struct definitions
	// If no toml tag is available, return empty string to skip this field
	// This ensures we only extract user-visible parameter names from code
	return ""
}

// extractWithRegex extracts config defaults using regex patterns as fallback
func (e *ConfigExtractor) extractWithRegex(content string) error {
	// Generic pattern: cfg.FieldName = value
	// This will be component-specific and can be extended
	patterns := []struct {
		pattern *regexp.Regexp
		keyFunc func([]string) string
		valFunc func([]string) (interface{}, string)
	}{
		// Pattern: cfg.FieldName = number
		// Also handle: cfg.Section.FieldName = number
		{
			pattern: regexp.MustCompile(fmt.Sprintf(`%s\.(\w+)(?:\.(\w+))?\s*=\s*(\d+)`, e.ConfigVarName)),
			keyFunc: func(matches []string) string {
				if len(matches) > 2 && matches[2] != "" {
					// Nested: cfg.Section.FieldName
					// For regex fallback, we can't extract toml tags, so skip nested configs
					// They should be handled by AST parsing with toml tag extraction
					return ""
				} else if len(matches) > 1 {
					// Direct: cfg.FieldName
					// Use mapFieldNameToConfigKey which will return empty if no toml tag is available
					return e.mapFieldNameToConfigKey(matches[1])
				}
				return ""
			},
			valFunc: func(matches []string) (interface{}, string) {
				// Value is in the last match (accounting for nested structure)
				valIdx := len(matches) - 1
				if valIdx > 0 {
					if val, err := strconv.Atoi(matches[valIdx]); err == nil {
						return float64(val), "int"
					}
				}
				return nil, "unknown"
			},
		},
		// Pattern: cfg.FieldName = true/false
		// Also handle: cfg.Section.FieldName = true/false
		{
			pattern: regexp.MustCompile(fmt.Sprintf(`%s\.(\w+)(?:\.(\w+))?\s*=\s*(true|false)`, e.ConfigVarName)),
			keyFunc: func(matches []string) string {
				if len(matches) > 2 && matches[2] != "" {
					// Nested: cfg.Section.FieldName
					// For regex fallback, we can't extract toml tags, so skip nested configs
					// They should be handled by AST parsing with toml tag extraction
					return ""
				} else if len(matches) > 1 {
					// Direct: cfg.FieldName
					// Use mapFieldNameToConfigKey which will return empty if no toml tag is available
					return e.mapFieldNameToConfigKey(matches[1])
				}
				return ""
			},
			valFunc: func(matches []string) (interface{}, string) {
				// Value is in the last match
				valIdx := len(matches) - 1
				if valIdx > 0 {
					return matches[valIdx] == "true", "bool"
				}
				return nil, "unknown"
			},
		},
		// Pattern: cfg.FieldName = "string"
		// Also handle: cfg.Section.FieldName = "string"
		{
			pattern: regexp.MustCompile(fmt.Sprintf(`%s\.(\w+)(?:\.(\w+))?\s*=\s*"([^"]+)"`, e.ConfigVarName)),
			keyFunc: func(matches []string) string {
				if len(matches) > 2 && matches[2] != "" {
					// Nested: cfg.Section.FieldName
					// For regex fallback, we can't extract toml tags, so skip nested configs
					// They should be handled by AST parsing with toml tag extraction
					return ""
				} else if len(matches) > 1 {
					// Direct: cfg.FieldName
					// Use mapFieldNameToConfigKey which will return empty if no toml tag is available
					return e.mapFieldNameToConfigKey(matches[1])
				}
				return ""
			},
			valFunc: func(matches []string) (interface{}, string) {
				// Value is in the last match
				valIdx := len(matches) - 1
				if valIdx > 0 {
					val := matches[valIdx]
					paramType := e.determineValueType(val)
					return val, paramType
				}
				return nil, "unknown"
			},
		},
		// Pattern: defaultFieldName = value
		{
			pattern: regexp.MustCompile(fmt.Sprintf(`%s(\w+)\s*=\s*(\d+)\s*\*\s*time\.(\w+)`, e.DefaultPrefix)),
			keyFunc: func(matches []string) string {
				if len(matches) > 1 {
					return e.mapFieldNameToConfigKey(matches[1])
				}
				return ""
			},
			valFunc: func(matches []string) (interface{}, string) {
				if len(matches) > 3 {
					if num, err := strconv.Atoi(matches[2]); err == nil {
						return e.formatDuration(num, matches[3]), "duration"
					}
				}
				return nil, "unknown"
			},
		},
	}

	for _, p := range patterns {
		matches := p.pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			key := p.keyFunc(match)
			value, paramType := p.valFunc(match)
			if key != "" && value != nil {
				if _, exists := e.Output[key]; !exists {
					e.Output[key] = types.ParameterValue{
						Value: value,
						Type:  paramType,
					}
				}
			}
		}
	}

	return nil
}

// ============================================================================
// TiDB Component-Specific Functions
// ============================================================================

// FindTidbConfigFiles finds TiDB config files in a repository
// This is a convenience function that calls FindConfigFiles with ComponentTiDB
func FindTidbConfigFiles(tidbRoot string) []string {
	return FindConfigFiles(tidbRoot, types.ComponentTiDB)
}

// ============================================================================
// PD Component-Specific Functions
// ============================================================================

// FindPdConfigFiles finds PD config files in a repository
// This is a convenience function that calls FindConfigFiles with ComponentPD
func FindPdConfigFiles(pdRoot string) []string {
	return FindConfigFiles(pdRoot, types.ComponentPD)
}

// ============================================================================
// TiKV Component-Specific Functions (Rust Source Code Extraction)
// ============================================================================
// The following methods handle extraction of configuration defaults from Rust
// source code (used by TiKV). These methods are automatically called when
// ExtractFromFile detects a .rs file extension.
// ============================================================================

// extractFromRustCode extracts configuration defaults from Rust code (TiKV)
// This method is called automatically when ExtractFromFile detects a .rs file
func (e *ConfigExtractor) extractFromRustCode(content string) error {
	// Reset prefix
	e.currentPrefix = ""

	// Find all "impl Default for" blocks
	// Pattern: impl Default for ConfigName {
	defaultImplRe := regexp.MustCompile(`impl\s+Default\s+for\s+(\w+)\s*\{`)
	matches := defaultImplRe.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		configName := match[1]

		// Extract the impl block content
		blockContent := e.extractImplBlock(content, match[0])
		if blockContent == "" {
			continue
		}

		// Determine prefix based on config name
		prefix := e.determineRustPrefix(configName)
		e.currentPrefix = prefix

		// Extract field assignments from the impl block (recursively handles nested configs)
		e.extractRustFieldAssignmentsRecursive(blockContent, content)
	}

	// Also look for fn default() -> Self { ... } patterns
	// Pattern: fn default() -> Self {
	defaultFnRe := regexp.MustCompile(`fn\s+default\(\)\s*->\s*Self\s*\{`)
	fnMatches := defaultFnRe.FindAllStringSubmatch(content, -1)

	for _, match := range fnMatches {
		blockContent := e.extractImplBlock(content, match[0])
		if blockContent != "" {
			// Extract from default() function body (recursively handles nested configs)
			e.extractRustFieldAssignmentsRecursive(blockContent, content)
		}
	}

	// Note: We don't extract from the entire file content here anymore
	// because that would match too many false positives (like function parameters, etc.)
	// All extraction should happen within the impl Default blocks above

	return nil
}

// extractImplBlock extracts the content of an impl Default block
func (e *ConfigExtractor) extractImplBlock(content, startPattern string) string {
	startIdx := strings.Index(content, startPattern)
	if startIdx == -1 {
		return ""
	}

	// Find the matching closing brace
	braceCount := 0
	inString := false
	escapeNext := false
	startIdx += len(startPattern)

	for i := startIdx; i < len(content); i++ {
		char := content[i]

		if escapeNext {
			escapeNext = false
			continue
		}

		if char == '\\' {
			escapeNext = true
			continue
		}

		if char == '"' && !escapeNext {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if char == '{' {
			braceCount++
		} else if char == '}' {
			braceCount--
			if braceCount == 0 {
				return content[startIdx:i]
			}
		}
	}

	return ""
}

// extractRustFieldAssignmentsRecursive extracts field assignments from Rust code recursively
// This handles nested config structures by looking up their Default implementations
func (e *ConfigExtractor) extractRustFieldAssignmentsRecursive(blockContent, fullContent string) {
	e.extractRustFieldAssignments(blockContent, fullContent)
}

// extractRustFieldAssignments extracts field assignments from Rust code
func (e *ConfigExtractor) extractRustFieldAssignments(blockContent string, fullContent string) {
	// Pattern: field_name: value,
	// Pattern: field_name: ConfigType::default(),
	// Pattern: field_name: ReadableSize::mb(300),
	// Pattern: field_name: ReadableDuration::secs(30),
	fieldRe := regexp.MustCompile(`(\w+):\s*([^,}]+),?`)

	lines := strings.Split(blockContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		matches := fieldRe.FindStringSubmatch(line)
		if len(matches) >= 3 {
			fieldName := matches[1]
			valueStr := strings.TrimSpace(matches[2])

			// Skip invalid field names (single characters, numbers, etc.)
			if len(fieldName) <= 1 || (fieldName[0] >= '0' && fieldName[0] <= '9') {
				continue
			}
			// Skip field names that look like code keywords or types
			skipKeywords := []string{"Arc", "Box", "Vec", "String", "Option", "Result", "Self", "usize", "u64", "u32", "i64", "i32", "f64", "f32", "bool", "char", "str"}
			for _, keyword := range skipKeywords {
				if fieldName == keyword {
					continue
				}
			}
			// Skip if field name starts with uppercase (likely a type name)
			if len(fieldName) > 0 && fieldName[0] >= 'A' && fieldName[0] <= 'Z' && !strings.Contains(fieldName, "-") {
				// Allow some exceptions like "ReadableSize", "ReadableDuration"
				if fieldName != "ReadableSize" && fieldName != "ReadableDuration" {
					continue
				}
			}

			// Handle nested configs: field_name: ConfigName::default()
			// Look up the ConfigName's Default implementation and extract recursively
			if strings.Contains(valueStr, "::default()") && !strings.Contains(valueStr, "ReadableSize") &&
				!strings.Contains(valueStr, "ReadableDuration") {
				// Extract config name from pattern like "StorageConfig::default()"
				configNameRe := regexp.MustCompile(`(\w+)::default\(\)`)
				configMatch := configNameRe.FindStringSubmatch(valueStr)
				if len(configMatch) >= 2 {
					nestedConfigName := configMatch[1]
					// Look up this config's Default implementation in fullContent
					nestedImplPattern := fmt.Sprintf("impl\\s+Default\\s+for\\s+%s\\s*\\{", nestedConfigName)
					nestedImplRe := regexp.MustCompile(nestedImplPattern)
					nestedImplMatch := nestedImplRe.FindStringSubmatch(fullContent)
					if len(nestedImplMatch) > 0 {
						// Extract the nested config's Default implementation
						nestedBlockContent := e.extractImplBlock(fullContent, nestedImplMatch[0])
						if nestedBlockContent != "" {
							// Determine prefix for nested config
							oldPrefix := e.currentPrefix
							nestedPrefix := e.determineRustPrefix(nestedConfigName)
							if nestedPrefix != "" {
								e.currentPrefix = nestedPrefix
							} else {
								// Use field name as prefix if no specific prefix found
								e.currentPrefix = e.currentPrefix + e.mapFieldNameToConfigKey(fieldName) + "."
							}
							// Recursively extract from nested config
							e.extractRustFieldAssignments(nestedBlockContent, fullContent)
							e.currentPrefix = oldPrefix
						}
					}
				}
				continue
			}

			value := e.parseRustValue(valueStr)
			if value != nil {
				// Skip if value is a placeholder or contains code snippets
				if strVal, ok := value.(string); ok {
					// Skip values that look like code snippets (contain newlines, multiple lines, etc.)
					if strings.Contains(strVal, "\n") || strings.Contains(strVal, "usize =") ||
						strings.Contains(strVal, "readpool_config!") || strings.Contains(strVal, "const ") {
						continue
					}
					// Skip placeholder values
					if strings.HasPrefix(strVal, "${") && strings.HasSuffix(strVal, "}") {
						continue
					}
				}
				configKey := e.mapFieldNameToConfigKey(fieldName)
				if e.currentPrefix != "" {
					configKey = e.currentPrefix + configKey
				}
				// Skip if key is empty or looks like a code snippet
				if configKey == "" || strings.Contains(configKey, "::") || strings.Contains(configKey, "(") {
					continue
				}
				e.Output[configKey] = types.ParameterValue{
					Value: value,
					Type:  e.determineValueType(value),
				}
			}
		}
	}

	// Also try to extract from nested config structs recursively
	// Look for nested struct initialization patterns
	nestedRe := regexp.MustCompile(`(\w+):\s*(\w+)Config\s*\{`)
	nestedMatches := nestedRe.FindAllStringSubmatch(blockContent, -1)
	for _, match := range nestedMatches {
		if len(match) >= 3 {
			sectionName := match[1]
			// Extract the nested block
			nestedBlockStart := strings.Index(blockContent, match[0])
			if nestedBlockStart >= 0 {
				// Find the matching closing brace for this nested block
				braceCount := 0
				nestedStart := nestedBlockStart + len(match[0])
				for i := nestedStart; i < len(blockContent); i++ {
					if blockContent[i] == '{' {
						braceCount++
					} else if blockContent[i] == '}' {
						braceCount--
						if braceCount == 0 {
							nestedContent := blockContent[nestedStart:i]
							// Recursively extract from nested config
							oldPrefix := e.currentPrefix
							// Determine prefix for nested config
							nestedPrefix := e.determineRustPrefix(match[2])
							if nestedPrefix != "" {
								e.currentPrefix = nestedPrefix
							} else {
								e.currentPrefix = e.currentPrefix + sectionName + "."
							}
							e.extractRustFieldAssignments(nestedContent, fullContent)
							e.currentPrefix = oldPrefix
							break
						}
					}
				}
			}
		}
	}
}

// parseRustValue parses a Rust value expression into a Go value
func (e *ConfigExtractor) parseRustValue(valueStr string) interface{} {
	valueStr = strings.TrimSpace(valueStr)

	// Handle ReadableSize::gb(2), ReadableSize::mb(300), etc.
	if sizeMatch := regexp.MustCompile(`ReadableSize::(gb|mb|kb|b)\((\d+)\)`).FindStringSubmatch(valueStr); len(sizeMatch) >= 3 {
		unit := sizeMatch[1]
		num, err := strconv.ParseInt(sizeMatch[2], 10, 64)
		if err == nil {
			switch unit {
			case "gb":
				return fmt.Sprintf("%dGB", num)
			case "mb":
				return fmt.Sprintf("%dMB", num)
			case "kb":
				return fmt.Sprintf("%dKB", num)
			case "b":
				return fmt.Sprintf("%dB", num)
			}
		}
	}

	// Handle ReadableDuration::hours(0), ReadableDuration::secs(1), etc.
	if durMatch := regexp.MustCompile(`ReadableDuration::(hours|minutes|secs|ms)\((\d+)\)`).FindStringSubmatch(valueStr); len(durMatch) >= 3 {
		unit := durMatch[1]
		num, err := strconv.ParseInt(durMatch[2], 10, 64)
		if err == nil {
			switch unit {
			case "hours":
				return fmt.Sprintf("%dh", num)
			case "minutes":
				return fmt.Sprintf("%dm", num)
			case "secs":
				return fmt.Sprintf("%ds", num)
			case "ms":
				return fmt.Sprintf("%dms", num)
			}
		}
	}

	// Handle string literals: "value" or 'value'
	if strMatch := regexp.MustCompile(`^["']([^"']*)["']$`).FindStringSubmatch(valueStr); len(strMatch) >= 2 {
		return strMatch[1]
	}

	// Handle boolean: true or false
	if valueStr == "true" {
		return true
	}
	if valueStr == "false" {
		return false
	}

	// Handle numbers
	if num, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
		return float64(num)
	}
	if num, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return num
	}

	// Handle .to_owned() or .to_string()
	if strMatch := regexp.MustCompile(`^["']([^"']*)["']\.to_(owned|string)\(\)$`).FindStringSubmatch(valueStr); len(strMatch) >= 2 {
		return strMatch[1]
	}

	// Handle ReadableSize and ReadableDuration without function call syntax
	// Pattern: ReadableSize(0) or ReadableDuration::secs(1)
	if sizeMatch := regexp.MustCompile(`ReadableSize\((\d+)\)`).FindStringSubmatch(valueStr); len(sizeMatch) >= 2 {
		// Default to bytes if no unit specified
		num, err := strconv.ParseInt(sizeMatch[1], 10, 64)
		if err == nil {
			return fmt.Sprintf("%dB", num)
		}
	}

	// Handle enums: EnumType::Variant
	if enumMatch := regexp.MustCompile(`(\w+)::(\w+)`).FindStringSubmatch(valueStr); len(enumMatch) >= 3 {
		// Return the variant name as string
		return enumMatch[2]
	}

	// Handle function calls that return constants
	// Pattern: DEFAULT_CONSTANT or SysQuota::cpu_cores_quota()
	if constMatch := regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)$`).FindStringSubmatch(valueStr); len(constMatch) >= 2 {
		// This is a constant reference, we can't resolve it here
		// Skip it instead of returning a placeholder
		return nil
	}

	// Skip if value contains function calls or complex expressions
	// These are not actual default values
	if strings.Contains(valueStr, "(") || strings.Contains(valueStr, "::") ||
		strings.Contains(valueStr, ".") && !strings.HasPrefix(valueStr, "\"") {
		// Check if it's a simple method call like .to_string() or .to_owned()
		if !regexp.MustCompile(`\.to_(owned|string)\(\)$`).MatchString(valueStr) {
			return nil
		}
	}

	// Skip values that look like code snippets (function calls, method calls, etc.)
	// Pattern: identifier.identifier() or identifier::identifier()
	if regexp.MustCompile(`^\w+\.\w+\(\)$`).MatchString(valueStr) ||
		regexp.MustCompile(`^\w+::\w+\(\)$`).MatchString(valueStr) ||
		strings.Contains(valueStr, ".to_owned()") ||
		strings.Contains(valueStr, ".to_string()") {
		return nil
	}

	// Skip values that are just identifiers (likely constants or variables)
	// But allow enum variants like "Normal", "High", etc.
	if regexp.MustCompile(`^[A-Z][a-zA-Z0-9_]*$`).MatchString(valueStr) {
		// Allow some common enum-like values
		allowedValues := []string{"None", "Some", "Normal", "High", "Low", "Medium", "Disabled", "Enabled", "True", "False"}
		isAllowed := false
		for _, allowed := range allowedValues {
			if valueStr == allowed {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			return nil
		}
	}

	// Default: return as string only if it looks like a valid value
	// Skip if it contains suspicious patterns
	if strings.Contains(valueStr, "usize") || strings.Contains(valueStr, "u64") ||
		strings.Contains(valueStr, "const ") || strings.Contains(valueStr, "let ") ||
		strings.Contains(valueStr, "fn ") || strings.Contains(valueStr, "struct ") {
		return nil
	}

	return valueStr
}

// determineRustPrefix determines the config prefix based on config struct name
func (e *ConfigExtractor) determineRustPrefix(configName string) string {
	configName = strings.ToLower(configName)

	// Special handling for TikvConfig - it's the root config, no prefix
	if configName == "tikvconfig" {
		return ""
	}

	// Map common TiKV config struct names to prefixes
	prefixMap := map[string]string{
		"tikvconfig":             "",
		"storageconfig":          "storage.",
		"raftstoreconfig":        "raftstore.",
		"raftstore":              "raftstore.",
		"serverconfig":           "server.",
		"rocksdbconfig":          "rocksdb.",
		"dbconfig":               "rocksdb.",
		"logconfig":              "log.",
		"metricconfig":           "metric.",
		"securityconfig":         "security.",
		"pdconfig":               "pd.",
		"copconfig":              "coprocessor.",
		"coprocessorconfig":      "coprocessor.",
		"coprocessorv2config":    "coprocessor-v2.",
		"gcconfig":               "gc.",
		"splitconfig":            "split.",
		"cdcconfig":              "cdc.",
		"importconfig":           "import.",
		"backupconfig":           "backup.",
		"pessimistictxnconfig":   "pessimistic-txn.",
		"resolvedtsconfig":       "resolved-ts.",
		"resourcemeteringconfig": "resource-metering.",
		"backupstreamconfig":     "log-backup.",
		"causaltsconfig":         "causal-ts.",
		"resourcecontrolconfig":  "resource-control.",
		"inmemoryengineconfig":   "in-memory-engine.",
		"memoryconfig":           "memory.",
		"quotaconfig":            "quota.",
		"readpoolconfig":         "readpool.",
		"defaultcfconfig":        "rocksdb.defaultcf.",
		"writecfconfig":          "rocksdb.writecf.",
		"lockcfconfig":           "rocksdb.lockcf.",
		"raftcfconfig":           "rocksdb.raftcf.",
		"titanconfig":            "rocksdb.titan.",
		"raftdbconfig":           "raftdb.",
		"raftengineconfig":       "raft-engine.",
	}

	for key, prefix := range prefixMap {
		if strings.Contains(configName, key) {
			return prefix
		}
	}

	return ""
}

// ============================================================================
// TiFlash Component-Specific Functions (C++ Source Code Extraction)
// ============================================================================
// The following methods handle extraction of configuration defaults from C++
// source code (used by TiFlash). These methods are automatically called when
// ExtractFromFile detects a .cpp/.h/.hpp file extension.
// ============================================================================

// extractFromCppCode extracts configuration defaults from C++ code (TiFlash)
// This method is called automatically when ExtractFromFile detects a .cpp/.h file
func (e *ConfigExtractor) extractFromCppCode(content string) error {
	// Reset prefix
	e.currentPrefix = ""

	// Find all struct/class definitions that might contain config
	// Pattern: struct ConfigName { or class ConfigName {
	structRe := regexp.MustCompile(`(?:struct|class)\s+(\w+Config)\s*\{`)
	matches := structRe.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		configName := match[1]

		// Extract the struct/class block content
		blockContent := e.extractCppBlock(content, match[0])
		if blockContent == "" {
			continue
		}

		// Determine prefix based on config name
		prefix := e.determineCppPrefix(configName)
		e.currentPrefix = prefix

		// Extract field assignments from the struct/class block
		e.extractCppFieldAssignments(blockContent)
	}

	// Also extract from inline default values: type field_name = value;
	// Pattern: bool field_name = false; or String field_name = "value";
	inlineRe := regexp.MustCompile(`(\w+)\s+(\w+)\s*=\s*([^;]+);`)
	inlineMatches := inlineRe.FindAllStringSubmatch(content, -1)

	for _, match := range inlineMatches {
		if len(match) >= 4 {
			fieldType := match[1]
			fieldName := match[2]
			valueStr := strings.TrimSpace(match[3])
			value := e.parseCppValue(valueStr, fieldType)
			if value != nil {
				configKey := e.mapFieldNameToConfigKey(fieldName)
				e.Output[configKey] = types.ParameterValue{
					Value: value,
					Type:  e.determineValueType(value),
				}
			}
		}
	}

	return nil
}

// extractCppBlock extracts the content of a C++ struct/class block
func (e *ConfigExtractor) extractCppBlock(content, startPattern string) string {
	startIdx := strings.Index(content, startPattern)
	if startIdx == -1 {
		return ""
	}

	// Find the matching closing brace
	braceCount := 0
	inString := false
	escapeNext := false
	inComment := false
	startIdx += len(startPattern)

	for i := startIdx; i < len(content); i++ {
		char := content[i]

		if escapeNext {
			escapeNext = false
			continue
		}

		// Handle comments
		if i+1 < len(content) {
			if content[i:i+2] == "//" {
				// Skip to end of line
				for i < len(content) && content[i] != '\n' {
					i++
				}
				continue
			}
			if content[i:i+2] == "/*" {
				inComment = true
				i++
				continue
			}
			if inComment && content[i:i+2] == "*/" {
				inComment = false
				i++
				continue
			}
		}

		if inComment {
			continue
		}

		if char == '\\' {
			escapeNext = true
			continue
		}

		if char == '"' && !escapeNext {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if char == '{' {
			braceCount++
		} else if char == '}' {
			braceCount--
			if braceCount == 0 {
				return content[startIdx:i]
			}
		}
	}

	return ""
}

// extractCppFieldAssignments extracts field assignments from C++ code
func (e *ConfigExtractor) extractCppFieldAssignments(blockContent string) {
	// Pattern 1: type field_name = value; (inline initialization)
	// Pattern 2: type field_name{value}; (brace initialization)
	// Pattern 3: type field_name = "value"; (string initialization)
	// Pattern 4: type field_name; (no default, skip)

	// More precise pattern that captures type, field name, and default value
	// Handles: bool is_enabled = false; String endpoint; UInt64 max_connections = 4096;
	fieldRe := regexp.MustCompile(`(\w+(?:\s*\*)?)\s+(\w+)\s*(?:=\s*([^;]+))?;`)

	lines := strings.Split(blockContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") {
			continue
		}

		// Skip function declarations, constructors, destructors
		if strings.Contains(line, "(") && strings.Contains(line, ")") {
			continue
		}

		// Skip static members, inline static, etc.
		if strings.Contains(line, "static") || strings.Contains(line, "inline") {
			continue
		}

		matches := fieldRe.FindStringSubmatch(line)
		if len(matches) >= 3 {
			fieldType := strings.TrimSpace(matches[1])
			fieldName := matches[2]

			// Skip if no default value provided
			if len(matches) < 4 || matches[3] == "" {
				continue
			}

			valueStr := strings.TrimSpace(matches[3])
			// Remove trailing semicolon or brace if present
			valueStr = strings.TrimSuffix(valueStr, ";")
			valueStr = strings.TrimSuffix(valueStr, "}")

			// Skip if it's a nested struct/class initialization or function call
			if strings.Contains(valueStr, "{") || strings.Contains(valueStr, "(") || strings.Contains(valueStr, "std::") {
				continue
			}

			// Skip if value is a type name (like "std::unique_ptr<ConfigReloader>")
			if strings.Contains(valueStr, "<") || strings.Contains(valueStr, "::") {
				continue
			}

			value := e.parseCppValue(valueStr, fieldType)
			if value != nil {
				configKey := e.mapFieldNameToConfigKey(fieldName)
				if e.currentPrefix != "" {
					configKey = e.currentPrefix + configKey
				}
				e.Output[configKey] = types.ParameterValue{
					Value: value,
					Type:  e.determineValueType(value),
				}
			}
		}
	}
}

// parseCppValue parses a C++ value expression into a Go value
func (e *ConfigExtractor) parseCppValue(valueStr, fieldType string) interface{} {
	valueStr = strings.TrimSpace(valueStr)

	// Skip function calls like config.getString(...)
	if strings.Contains(valueStr, "(") && !strings.HasPrefix(valueStr, "\"") {
		return nil
	}

	// Skip identifier references
	if strings.Contains(valueStr, ".") && !strings.HasPrefix(valueStr, "\"") && !strings.HasSuffix(valueStr, "\"") {
		return nil
	}

	// Handle string literals: "value" or 'value'
	if strMatch := regexp.MustCompile(`^["']([^"']*)["']$`).FindStringSubmatch(valueStr); len(strMatch) >= 2 {
		return strMatch[1]
	}

	// Handle boolean: true or false
	if valueStr == "true" {
		return true
	}
	if valueStr == "false" {
		return false
	}

	// Handle numbers
	if num, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
		return float64(num)
	}
	if num, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return num
	}

	// Handle size literals: 1024 * 1024, etc.
	if sizeMatch := regexp.MustCompile(`(\d+)\s*\*\s*(\d+)`).FindStringSubmatch(valueStr); len(sizeMatch) >= 3 {
		num1, _ := strconv.ParseInt(sizeMatch[1], 10, 64)
		num2, _ := strconv.ParseInt(sizeMatch[2], 10, 64)
		return float64(num1 * num2)
	}

	// Skip constants and complex expressions - these are not actual default values
	if constMatch := regexp.MustCompile(`^([A-Z_][A-Z0-9_]*(?:::[A-Z_][A-Z0-9_]*)*)$`).FindStringSubmatch(valueStr); len(constMatch) >= 2 {
		// Skip constant references - we can't resolve them here
		return nil
	}

	// Skip if value contains function calls or complex expressions
	// These are not actual default values
	if strings.Contains(valueStr, "(") || strings.Contains(valueStr, "::") ||
		strings.Contains(valueStr, ".") && !strings.HasPrefix(valueStr, "\"") {
		// Check if it's a simple method call like .to_string() or .to_owned()
		if !regexp.MustCompile(`\.to_(owned|string)\(\)$`).MatchString(valueStr) {
			return nil
		}
	}

	// Default: return as string
	return valueStr
}

// determineCppPrefix determines the config prefix based on config struct/class name
func (e *ConfigExtractor) determineCppPrefix(configName string) string {
	configName = strings.ToLower(configName)

	// Map common TiFlash config struct/class names to prefixes
	prefixMap := map[string]string{
		"spillconfig":        "spill.",
		"storageconfig":      "storage.",
		"storages3config":    "storage.s3.",
		"raftconfig":         "raft.",
		"serverconfig":       "server.",
		"userconfig":         "users.",
		"logconfig":          "log.",
		"metricconfig":       "metric.",
		"securityconfig":     "security.",
		"profilesconfig":     "profiles.",
		"quotasconfig":       "quotas.",
		"flashconfig":        "flash.",
		"flashproxyconfig":   "flash.proxy.",
		"flashserviceconfig": "flash.service.",
	}

	for key, prefix := range prefixMap {
		if strings.Contains(configName, key) {
			return prefix
		}
	}

	return ""
}

// FindTikvConfigFiles finds TiKV config files in a repository
// This is a convenience function that calls FindConfigFiles with ComponentTiKV
func FindTikvConfigFiles(tikvRoot string) []string {
	return FindConfigFiles(tikvRoot, types.ComponentTiKV)
}

// FindTiflashConfigFiles finds TiFlash config files in a repository
// This is a convenience function that calls FindConfigFiles with ComponentTiFlash
func FindTiflashConfigFiles(tiflashRoot string) []string {
	return FindConfigFiles(tiflashRoot, types.ComponentTiFlash)
}

// ============================================================================
// Common Utilities (Shared by All Components)
// ============================================================================

// FindConfigFiles finds configuration files in a repository
// This is a generic function that dispatches to component-specific file paths
func FindConfigFiles(repoRoot string, component types.ComponentType) []string {
	var files []string
	var searchPaths []string

	switch component {
	case types.ComponentPD:
		searchPaths = []string{
			filepath.Join(repoRoot, "server", "config", "config.go"),
		}
	case types.ComponentTiDB:
		// TiDB 7.5+ uses pkg/ directory, older versions may not
		// Try both paths
		searchPaths = []string{
			filepath.Join(repoRoot, "pkg", "config", "config.go"),
			filepath.Join(repoRoot, "config", "config.go"),
		}
	case types.ComponentTiKV:
		searchPaths = []string{
			// Main config file (most important)
			filepath.Join(repoRoot, "src", "config.rs"),
			// Alternative paths (for different TiKV versions)
			filepath.Join(repoRoot, "src", "config", "mod.rs"),
			filepath.Join(repoRoot, "src", "config", "config.rs"),
			filepath.Join(repoRoot, "src", "config", "configurable.rs"),
			filepath.Join(repoRoot, "components", "config", "src", "lib.rs"),
			// Component-specific config files
			filepath.Join(repoRoot, "components", "raftstore", "src", "store", "config.rs"),
			filepath.Join(repoRoot, "components", "raftstore", "src", "coprocessor", "config.rs"),
			filepath.Join(repoRoot, "components", "server", "src", "config.rs"),
			filepath.Join(repoRoot, "components", "server", "src", "gc_worker", "config.rs"),
			filepath.Join(repoRoot, "components", "server", "src", "lock_manager", "config.rs"),
			filepath.Join(repoRoot, "src", "storage", "config.rs"),
			filepath.Join(repoRoot, "components", "engine_rocks", "src", "config.rs"),
			filepath.Join(repoRoot, "components", "pd_client", "src", "config.rs"),
			filepath.Join(repoRoot, "components", "tikv_util", "src", "config.rs"),
		}
	case types.ComponentTiFlash:
		// TiFlash C++ config file paths
		searchPaths = []string{
			filepath.Join(repoRoot, "dbms", "src", "Core", "SpillConfig.h"),
			filepath.Join(repoRoot, "dbms", "src", "Core", "SpillConfig.cpp"),
			filepath.Join(repoRoot, "dbms", "src", "Server", "StorageConfigParser.h"),
			filepath.Join(repoRoot, "dbms", "src", "Server", "StorageConfigParser.cpp"),
			filepath.Join(repoRoot, "dbms", "src", "Server", "RaftConfigParser.h"),
			filepath.Join(repoRoot, "dbms", "src", "Server", "RaftConfigParser.cpp"),
			filepath.Join(repoRoot, "dbms", "src", "Server", "UserConfigParser.h"),
			filepath.Join(repoRoot, "dbms", "src", "Server", "UserConfigParser.cpp"),
			filepath.Join(repoRoot, "dbms", "src", "Common", "config.h.in"),
		}
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			files = append(files, path)
		}
	}

	return files
}
