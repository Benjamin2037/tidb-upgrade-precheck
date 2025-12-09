package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

func newHighRiskParamsEditCmd() *cobra.Command {
	var editor string

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit the configuration file",
		Long: `Edit the configuration file using your default editor.

The editor is determined by:
1. --editor flag
2. $EDITOR environment variable
3. Default editor (vim on Linux/Mac, notepad on Windows)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile := highRiskParamsConfigFile
			if configFile == "" {
				configFile = getDefaultConfigPath()
			}

			// Ensure config file exists
			config, err := loadHighRiskParamsConfig()
			if err != nil {
				return err
			}

			// Save empty config if file doesn't exist
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				if err := saveHighRiskParamsConfig(config); err != nil {
					return err
				}
			}

			// Determine editor
			if editor == "" {
				editor = os.Getenv("EDITOR")
			}

			if editor == "" {
				// Default editor based on OS
				switch runtime.GOOS {
				case "windows":
					editor = "notepad"
				default:
					editor = "vim"
				}
			}

			// Open editor
			editCmd := exec.Command(editor, configFile)
			editCmd.Stdin = os.Stdin
			editCmd.Stdout = os.Stdout
			editCmd.Stderr = os.Stderr

			if err := editCmd.Run(); err != nil {
				return fmt.Errorf("failed to open editor: %w", err)
			}

			fmt.Printf("Configuration file edited: %s\n", configFile)
			return nil
		},
	}

	cmd.Flags().StringVar(&editor, "editor", "", "Editor to use (default: $EDITOR or vim/notepad)")

	return cmd
}

