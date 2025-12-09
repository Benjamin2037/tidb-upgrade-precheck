// Package kbgenerator provides tools for generating knowledge base from TiDB source code
// and collecting runtime configuration from running clusters.
package kbgenerator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ParamSnapshot represents a snapshot of TiDB parameters and system variables
type ParamSnapshot struct {
	Version          string                 `json:"version"`
	ConfigDefaults   map[string]interface{} `json:"config_defaults"`
	SystemVariables  map[string]interface{} `json:"system_variables"`
	BootstrapVersion int64                  `json:"bootstrap_version"`
}

// KBSnapshot represents a knowledge base snapshot collected from TiDB source code
type KBSnapshot struct {
	Version          string                 `json:"version"`
	ConfigDefaults   map[string]interface{} `json:"config_defaults"`
	SystemVariables  map[string]interface{} `json:"system_variables"`
	BootstrapVersion int64                  `json:"bootstrap_version"`
}

// RuntimeSnapshot represents a runtime snapshot collected from a running TiDB cluster
type RuntimeSnapshot struct {
	Version         string            `json:"version"`
	ConfigCurrent   map[string]string `json:"config_current"`
	VariablesCurrent map[string]string `json:"variables_current"`
}

// SaveTiDBSnapshot saves a TiDB KB snapshot to a file
func SaveTiDBSnapshot(snapshot *KBSnapshot, outputPath string) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

// CollectFromTidbSource collects TiDB parameters and system variables from source code
// This is used for knowledge base generation
func CollectFromTidbSource(tidbRoot, tag string) (*KBSnapshot, error) {
	// Checkout to the specific tag
	cmd := exec.Command("git", "checkout", tag)
	cmd.Dir = tidbRoot
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git checkout %s: %w", tag, err)
	}

	// Parse config defaults
	configDefaults := parseConfigDefaults(filepath.Join(tidbRoot, "pkg", "config", "config.go"))

	// Parse system variables
	sysVars := parseSysVars(
		filepath.Join(tidbRoot, "pkg", "sessionctx", "variable", "sysvar.go"),
		filepath.Join(tidbRoot, "pkg", "sessionctx", "vardef"),
	)

	// Get bootstrap version
	bootstrapVersion := parseBootstrapVersion(tidbRoot)

	return &KBSnapshot{
		Version:          tag,
		ConfigDefaults:   configDefaults,
		SystemVariables:  sysVars,
		BootstrapVersion: bootstrapVersion,
	}, nil
}

// CollectFromTidbBinary collects TiDB parameters and system variables by running a tool against TiDB source
// This is used for knowledge base generation
func CollectFromTidbBinary(tidbRoot, tag, toolPath string) (*KBSnapshot, error) {
	// Checkout to the specific tag
	cmd := exec.Command("git", "checkout", tag)
	cmd.Dir = tidbRoot
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git checkout %s: %w", tag, err)
	}

	// Run the tool
	cmd = exec.Command("go", "run", toolPath)
	cmd.Dir = tidbRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run export tool: %w", err)
	}

	// Parse the output
	var result struct {
		Sysvars          map[string]interface{} `json:"sysvars"`
		Config           map[string]interface{} `json:"config"`
		BootstrapVersion int64                  `json:"bootstrap_version"`
	}
	
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tool output: %w", err)
	}

	return &KBSnapshot{
		Version:          tag,
		ConfigDefaults:   result.Config,
		SystemVariables:  result.Sysvars,
		BootstrapVersion: result.BootstrapVersion,
	}, nil
}

func parseVardefConstants(vardefDir string) map[string]string {
	result := make(map[string]string)
	filepath.WalkDir(vardefDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		// Fix the regex - remove extra backslashes
		re := regexp.MustCompile(`const\s+([A-Za-z0-9_]+)\s*=\s*(["'` + "`" + `]?)([^"'` + "`" + `\n]+)\2`)
		matches := re.FindAllStringSubmatch(string(data), -1)
		for _, m := range matches {
			if len(m) == 4 {
				result[m[1]] = m[3]
			}
		}
		return nil
	})
	return result
}

func parseConfigDefaults(configPath string) map[string]interface{} {
	result := make(map[string]interface{})
	data, err := os.ReadFile(configPath)
	if err != nil {
		return result
	}
	// Fix the regex - remove extra backslashes
	re := regexp.MustCompile(`([A-Za-z0-9_]+)\s+([A-Za-z0-9_]+)\s+` + "`" + `.*default:(\S+).*` + "`")
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		m := re.FindStringSubmatch(line)
		if len(m) == 4 {
			result[m[2]] = m[3]
		}
	}
	return result
}

func parseSysVars(sysvarPath, vardefDir string) map[string]interface{} {
	result := make(map[string]interface{})
	vardefConsts := parseVardefConstants(vardefDir)
	data, err := os.ReadFile(sysvarPath)
	if err != nil {
		return result
	}
	// Fix the regex - remove extra backslashes
	re := regexp.MustCompile(`Name:\s*("?[A-Za-z0-9_\.]+"?),[\s\S]*?Value:\s*("?[A-Za-z0-9_\.\"\'` + "`" + `]+)"?`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	for _, m := range matches {
		if len(m) >= 3 {
			// Clean up the name and value strings
			name := strings.Trim(m[1], "\"")
			val := strings.Trim(m[2], "\"'")
			
			if strings.HasPrefix(val, "vardef.") {
				key := strings.TrimPrefix(val, "vardef.")
				if v, ok := vardefConsts[key]; ok {
					result[name] = v
				} else {
					result[name] = val
				}
			} else {
				result[name] = val
			}
		}
	}
	return result
}

func parseBootstrapVersion(tidbRoot string) int64 {
	// Look for currentBootstrapVersion in session/session.go
	sessionPath := filepath.Join(tidbRoot, "pkg", "session", "session.go")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return 0
	}
	
	// Regex to find currentBootstrapVersion = X
	re := regexp.MustCompile(`currentBootstrapVersion\s*=\s*(\d+)`)
	matches := re.FindStringSubmatch(string(data))
	if len(matches) == 2 {
		var version int64
		if _, err := fmt.Sscanf(matches[1], "%d", &version); err == nil {
			return version
		}
	}
	
	return 0
}