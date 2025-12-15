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
type Manager struct {
	configPath string
}

// NewManager creates a new configuration manager
func NewManager(configPath string) *Manager {
	if configPath == "" {
		configPath = GetDefaultConfigPath()
	}
	return &Manager{
		configPath: configPath,
	}
}

// GetDefaultConfigPath returns the default path for high-risk params config
func GetDefaultConfigPath() string {
	// Try to get from environment variable
	if path := os.Getenv("HIGH_RISK_PARAMS_CONFIG"); path != "" {
		return path
	}

	// Default locations (in order of preference)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		// ~/.tiup/high_risk_params.json (for TiUP integration)
		tiupPath := filepath.Join(homeDir, ".tiup", "high_risk_params.json")
		if _, err := os.Stat(tiupPath); err == nil {
			return tiupPath
		}

		// ~/.tidb-upgrade-precheck/high_risk_params.json
		precheckPath := filepath.Join(homeDir, ".tidb-upgrade-precheck", "high_risk_params.json")
		if _, err := os.Stat(precheckPath); err == nil {
			return precheckPath
		}

		// Return TiUP path as default (will be created if doesn't exist)
		return tiupPath
	}

	// Fallback to current directory
	return "./high_risk_params.json"
}

// LoadConfig loads the high-risk parameters configuration from file
func (m *Manager) LoadConfig() (*rules.HighRiskParamsConfig, error) {
	config := &rules.HighRiskParamsConfig{}

	// Check if file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// File doesn't exist, return empty config
		return config, nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if len(data) == 0 {
		// Empty file, return empty config
		return config, nil
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// SaveConfig saves the high-risk parameters configuration to file
func (m *Manager) SaveConfig(config *rules.HighRiskParamsConfig) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
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

// AddParameter adds a parameter to the config
func (m *Manager) AddParameter(component, paramType, paramName string, paramConfig rules.HighRiskParamConfig) error {
	config, err := m.LoadConfig()
	if err != nil {
		return err
	}

	component = strings.ToLower(component)
	paramType = strings.ToLower(paramType)

	switch component {
	case "tidb":
		if paramType == "config" {
			if config.TiDB.Config == nil {
				config.TiDB.Config = make(map[string]rules.HighRiskParamConfig)
			}
			config.TiDB.Config[paramName] = paramConfig
		} else if paramType == "system_variable" || paramType == "system-variable" || paramType == "sysvar" {
			if config.TiDB.SystemVariables == nil {
				config.TiDB.SystemVariables = make(map[string]rules.HighRiskParamConfig)
			}
			config.TiDB.SystemVariables[paramName] = paramConfig
		} else {
			return fmt.Errorf("invalid type for TiDB: %s (must be 'config' or 'system_variable')", paramType)
		}
	case "pd":
		if paramType == "config" {
			if config.PD.Config == nil {
				config.PD.Config = make(map[string]rules.HighRiskParamConfig)
			}
			config.PD.Config[paramName] = paramConfig
		} else {
			return fmt.Errorf("PD only supports 'config' type")
		}
	case "tikv":
		if paramType == "config" {
			if config.TiKV.Config == nil {
				config.TiKV.Config = make(map[string]rules.HighRiskParamConfig)
			}
			config.TiKV.Config[paramName] = paramConfig
		} else {
			return fmt.Errorf("TiKV only supports 'config' type")
		}
	case "tiflash":
		if paramType == "config" {
			if config.TiFlash.Config == nil {
				config.TiFlash.Config = make(map[string]rules.HighRiskParamConfig)
			}
			config.TiFlash.Config[paramName] = paramConfig
		} else {
			return fmt.Errorf("TiFlash only supports 'config' type")
		}
	default:
		return fmt.Errorf("invalid component: %s (must be tidb, pd, tikv, or tiflash)", component)
	}

	return m.SaveConfig(config)
}

// RemoveParameter removes a parameter from the config
func (m *Manager) RemoveParameter(component, paramType, paramName string) error {
	config, err := m.LoadConfig()
	if err != nil {
		return err
	}

	component = strings.ToLower(component)
	paramType = strings.ToLower(paramType)

	var removed bool

	switch component {
	case "tidb":
		if paramType == "config" {
			if _, exists := config.TiDB.Config[paramName]; exists {
				delete(config.TiDB.Config, paramName)
				removed = true
			}
		} else if paramType == "system_variable" || paramType == "system-variable" || paramType == "sysvar" {
			if _, exists := config.TiDB.SystemVariables[paramName]; exists {
				delete(config.TiDB.SystemVariables, paramName)
				removed = true
			}
		} else {
			return fmt.Errorf("invalid type for TiDB: %s", paramType)
		}
	case "pd":
		if paramType == "config" {
			if _, exists := config.PD.Config[paramName]; exists {
				delete(config.PD.Config, paramName)
				removed = true
			}
		} else {
			return fmt.Errorf("PD only supports 'config' type")
		}
	case "tikv":
		if paramType == "config" {
			if _, exists := config.TiKV.Config[paramName]; exists {
				delete(config.TiKV.Config, paramName)
				removed = true
			}
		} else {
			return fmt.Errorf("TiKV only supports 'config' type")
		}
	case "tiflash":
		if paramType == "config" {
			if _, exists := config.TiFlash.Config[paramName]; exists {
				delete(config.TiFlash.Config, paramName)
				removed = true
			}
		} else {
			return fmt.Errorf("TiFlash only supports 'config' type")
		}
	default:
		return fmt.Errorf("invalid component: %s", component)
	}

	if !removed {
		return fmt.Errorf("parameter %s/%s/%s not found", component, paramType, paramName)
	}

	return m.SaveConfig(config)
}

// CollectAllParameters collects all parameters from config for listing
func (m *Manager) CollectAllParameters() ([]ParameterInfo, error) {
	config, err := m.LoadConfig()
	if err != nil {
		return nil, err
	}

	var params []ParameterInfo

	// TiDB config
	for name := range config.TiDB.Config {
		params = append(params, ParameterInfo{
			Component: "tidb",
			Type:      "config",
			Name:      name,
			Display:   fmt.Sprintf("tidb/config/%s", name),
		})
	}
	// TiDB system variables
	for name := range config.TiDB.SystemVariables {
		params = append(params, ParameterInfo{
			Component: "tidb",
			Type:      "system_variable",
			Name:      name,
			Display:   fmt.Sprintf("tidb/system_variable/%s", name),
		})
	}
	// PD
	for name := range config.PD.Config {
		params = append(params, ParameterInfo{
			Component: "pd",
			Type:      "config",
			Name:      name,
			Display:   fmt.Sprintf("pd/config/%s", name),
		})
	}
	// TiKV
	for name := range config.TiKV.Config {
		params = append(params, ParameterInfo{
			Component: "tikv",
			Type:      "config",
			Name:      name,
			Display:   fmt.Sprintf("tikv/config/%s", name),
		})
	}
	// TiFlash
	for name := range config.TiFlash.Config {
		params = append(params, ParameterInfo{
			Component: "tiflash",
			Type:      "config",
			Name:      name,
			Display:   fmt.Sprintf("tiflash/config/%s", name),
		})
	}

	return params, nil
}

// ParameterInfo represents a parameter for listing
type ParameterInfo struct {
	Component string
	Type      string
	Name      string
	Display   string
}
