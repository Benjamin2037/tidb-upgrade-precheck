package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newHighRiskParamsViewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "View the current configuration",
		Long: `View the current high-risk parameters configuration in JSON format.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadHighRiskParamsConfig()
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(config, "", "  ")
			if err != nil {
				return err
			}

			fmt.Println(string(data))
			return nil
		},
	}

	return cmd
}

