package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules/high_risk_params"
	"github.com/spf13/cobra"
)

// ParameterInput defines the expected JSON file format for add/edit commands
type ParameterInput struct {
	Component     string        `json:"component"`
	Type          string        `json:"type"`
	Name          string        `json:"name"`
	Severity      string        `json:"severity"`
	Description   string        `json:"description,omitempty"`
	CheckModified bool          `json:"check_modified,omitempty"`
	FromVersion   string        `json:"from_version,omitempty"`
	ToVersion     string        `json:"to_version,omitempty"`
	AllowedValues []interface{} `json:"allowed_values,omitempty"`
}

// loadParameterFromFile loads parameter configuration from a JSON file
func loadParameterFromFile(filePath string) (rules.HighRiskParamConfig, string, string, string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return rules.HighRiskParamConfig{}, "", "", "", fmt.Errorf("failed to read file: %w", err)
	}

	var input ParameterInput
	if err := json.Unmarshal(data, &input); err != nil {
		return rules.HighRiskParamConfig{}, "", "", "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate required fields
	if input.Component == "" || input.Type == "" || input.Name == "" || input.Severity == "" {
		return rules.HighRiskParamConfig{}, "", "", "", fmt.Errorf("missing required fields: component, type, name, or severity")
	}

	// AllowedValues is already []interface{} from JSON, use directly
	config := rules.HighRiskParamConfig{
		Severity:      input.Severity,
		Description:   input.Description,
		CheckModified: input.CheckModified,
		FromVersion:   input.FromVersion,
		ToVersion:     input.ToVersion,
		AllowedValues: input.AllowedValues,
	}

	return config, input.Component, input.Type, input.Name, nil
}

// addOrEditParameter adds or edits a parameter in the configuration
func addOrEditParameter(component, paramType, paramName string, paramConfig rules.HighRiskParamConfig, isAdd bool) error {
	manager := getConfigManager()

	component = strings.ToLower(component)
	paramType = strings.ToLower(paramType)

	// Check if parameter exists
	_, exists := manager.FindParameter(component, paramType, paramName)

	if isAdd {
		// Add: parameter must not exist
		if exists {
			return fmt.Errorf("parameter %s/%s/%s already exists. Use 'edit' command to modify it", component, paramType, paramName)
		}
	} else {
		// Edit: parameter must exist
		if !exists {
			return fmt.Errorf("parameter %s/%s/%s not found. Use 'add' command to create it", component, paramType, paramName)
		}
	}

	// Add or update the parameter
	return manager.AddParameter(component, paramType, paramName, paramConfig)
}

func newHighRiskParamsAddCmd() *cobra.Command {
	var inputFile string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a high-risk parameter",
		Long: `Add a high-risk parameter to the configuration.

You must provide a JSON file using --file with the parameter configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputFile == "" {
				return fmt.Errorf("--file is required for add command")
			}

			paramConfig, component, paramType, paramName, err := loadParameterFromFile(inputFile)
			if err != nil {
				return err
			}

			// Add parameter (will check if exists)
			if err := addOrEditParameter(component, paramType, paramName, paramConfig, true); err != nil {
				return err
			}

			configFile := highRiskParamsConfigFile
			if configFile == "" {
				configFile = high_risk_params.GetDefaultConfigPath()
			}

			fmt.Printf("Successfully added high-risk parameter: %s/%s/%s\n", component, paramType, paramName)
			fmt.Printf("Configuration saved to: %s\n", configFile)

			return nil
		},
	}

	cmd.Flags().StringVar(&inputFile, "file", "", "Load parameter configuration from JSON file (required)")
	cmd.MarkFlagRequired("file")

	return cmd
}

func newHighRiskParamsEditCmd() *cobra.Command {
	var inputFile string

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit a high-risk parameter",
		Long: `Edit an existing high-risk parameter in the configuration.

You must provide a JSON file using --file with the parameter configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputFile == "" {
				return fmt.Errorf("--file is required for edit command")
			}

			paramConfig, component, paramType, paramName, err := loadParameterFromFile(inputFile)
			if err != nil {
				return err
			}

			// Edit parameter (will check if exists)
			if err := addOrEditParameter(component, paramType, paramName, paramConfig, false); err != nil {
				return err
			}

			configFile := highRiskParamsConfigFile
			if configFile == "" {
				configFile = high_risk_params.GetDefaultConfigPath()
			}

			fmt.Printf("Successfully edited high-risk parameter: %s/%s/%s\n", component, paramType, paramName)
			fmt.Printf("Configuration saved to: %s\n", configFile)

			return nil
		},
	}

	cmd.Flags().StringVar(&inputFile, "file", "", "Load parameter configuration from JSON file (required)")
	cmd.MarkFlagRequired("file")

	return cmd
}
