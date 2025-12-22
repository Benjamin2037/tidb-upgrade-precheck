// Package high_risk_params provides configuration management for high-risk parameters
package high_risk_params

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
)

// Manager handles high-risk parameters configuration management
type Manager struct{}

// NewManager creates a new configuration manager
func NewManager(configPath string) *Manager {
	// configPath parameter is kept for backward compatibility but not used
	// Knowledge base only maintains a single file: high_risk_params.json
	return &Manager{}
}

// GetKnowledgeBaseConfigPath returns the path to the knowledge base config
// This is the config file (high_risk_params.json) that is copied from pkg directory during KB generation
func GetKnowledgeBaseConfigPath() string {
	// Try to get from environment variable for knowledge base path
	if kbPath := os.Getenv("KNOWLEDGE_BASE_PATH"); kbPath != "" {
		return filepath.Join(kbPath, "high_risk_params", "high_risk_params.json")
	}

	// Default: use knowledge directory relative to executable or current directory
	// Try to find knowledge directory in common locations
	possiblePaths := []string{
		"./knowledge/high_risk_params/high_risk_params.json",
		"../knowledge/high_risk_params/high_risk_params.json",
		"../../knowledge/high_risk_params/high_risk_params.json",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// If not found, return the default relative path
	return "./knowledge/high_risk_params/high_risk_params.json"
}

// LoadConfig loads the high-risk parameters configuration from knowledge base
func (m *Manager) LoadConfig() (*rules.HighRiskParamsConfig, error) {
	config := &rules.HighRiskParamsConfig{}
	kbPath := GetKnowledgeBaseConfigPath()
	if _, err := os.Stat(kbPath); err == nil {
		data, err := os.ReadFile(kbPath)
		if err == nil && len(data) > 0 {
			if err := json.Unmarshal(data, config); err != nil {
				// If knowledge base file is invalid, log but continue with empty config
				fmt.Fprintf(os.Stderr, "Warning: failed to parse knowledge base config at %s: %v\n", kbPath, err)
			}
		}
	}
	return config, nil
}

// FindParameter finds a parameter in the config
func (m *Manager) FindParameter(component, paramType, paramName string) (rules.HighRiskParamConfig, bool) {
	config, err := m.LoadConfig()
	if err != nil {
		return rules.HighRiskParamConfig{}, false
	}

	return FindParameterInConfig(config, component, paramType, paramName)
}

// FindParameterInConfig finds a parameter in the given config
func FindParameterInConfig(config *rules.HighRiskParamsConfig, component, paramType, paramName string) (rules.HighRiskParamConfig, bool) {
	component = strings.ToLower(component)
	paramType = strings.ToLower(paramType)

	switch component {
	case "tidb":
		if paramType == "config" {
			if param, ok := config.TiDB.Config[paramName]; ok {
				return param, true
			}
		} else if paramType == "system_variable" || paramType == "system-variable" || paramType == "sysvar" {
			if param, ok := config.TiDB.SystemVariables[paramName]; ok {
				return param, true
			}
		}
	case "pd":
		if paramType == "config" {
			if param, ok := config.PD.Config[paramName]; ok {
				return param, true
			}
		}
	case "tikv":
		if paramType == "config" {
			if param, ok := config.TiKV.Config[paramName]; ok {
				return param, true
			}
		}
	case "tiflash":
		if paramType == "config" {
			if param, ok := config.TiFlash.Config[paramName]; ok {
				return param, true
			}
		}
	}

	return rules.HighRiskParamConfig{}, false
}
