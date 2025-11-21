package collectparams

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type ParamSnapshot struct {
	Version         string                 `json:"version"`
	ConfigDefaults  map[string]interface{} `json:"config_defaults"`
	SystemVariables map[string]interface{} `json:"system_variables"`
}

func CollectFromTidbSource(tidbRoot, tag string) (*ParamSnapshot, error) {
	cmd := exec.Command("git", "checkout", tag)
	cmd.Dir = tidbRoot
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git checkout %s: %w", tag, err)
	}
	configDefaults := parseConfigDefaults(filepath.Join(tidbRoot, "pkg", "config", "config.go"))
	sysVars := parseSysVars(
		filepath.Join(tidbRoot, "pkg", "sessionctx", "variable", "sysvar.go"),
		filepath.Join(tidbRoot, "pkg", "sessionctx", "vardef"),
	)
	return &ParamSnapshot{
		Version:         tag,
		ConfigDefaults:  configDefaults,
		SystemVariables: sysVars,
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
		re := regexp.MustCompile(`const\\s+([A-Za-z0-9_]+)\\s*=\\s*(["'` + "`" + `]?)([^"'` + "`" + `\\n]+)\\2`)
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
	re := regexp.MustCompile(`([A-Za-z0-9_]+)\s+([A-Za-z0-9_]+)\s+` + "`.*default:(\\S+).*")
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
	re := regexp.MustCompile(`Name:\s*([A-Za-z0-9_\.]+),[\s\S]*?Value:\s*([A-Za-z0-9_\.\"\']+)`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	for _, m := range matches {
		if len(m) == 3 {
			name := m[1]
			val := m[2]
			if strings.HasPrefix(val, "vardef.") {
				key := strings.TrimPrefix(val, "vardef.")
				if v, ok := vardefConsts[key]; ok {
					result[name] = v
				} else {
					result[name] = val
				}
			} else if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
				result[name] = strings.Trim(val, "\"")
			} else {
				result[name] = val
			}
		}
	}
	return result
}
