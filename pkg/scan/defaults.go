package scan

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// SysVars represents system variables
type SysVars struct {
	SysVars map[string]string `json:"sysvars"`
}

// ScanDefaults scans default values from TiDB source code
func ScanDefaults(repo, outputFile string, toolFile string) error {
	if repo == "" {
		return fmt.Errorf("repo path is empty")
	}

	// Create tools directory if not exists
	toolsDir := filepath.Join("pkg", "scan", "tools")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		return fmt.Errorf("failed to create tools directory: %v", err)
	}

	// Copy the tool to tools directory
	toolPath := filepath.Join(toolsDir, "export_defaults.go")
	if err := copyFile(toolFile, toolPath); err != nil {
		return fmt.Errorf("failed to copy tool: %v", err)
	}

	// Run the tool
	cmd := exec.Command("go", "run", toolPath, repo)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run export defaults tool: %v", err)
	}

	// Create output directory if not exists
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Write output to file
	if err := os.WriteFile(outputFile, output, 0644); err != nil {
		return fmt.Errorf("failed to write defaults to file: %v", err)
	}

	fmt.Printf("[ScanDefaults] repo=%s output=%s\n", repo, outputFile)
	return nil
}

// selectToolByVersion selects the appropriate tool based on the TiDB version
func selectToolByVersion(repo string) (string, error) {
	// This is a simplified implementation
	// In a real implementation, this would check the TiDB version and select the appropriate tool
	version, err := getTiDBVersion(repo)
	if err != nil {
		return "", fmt.Errorf("failed to get TiDB version: %v", err)
	}

	// Parse version
	major, minor, _, err := parseVersion(version)
	if err != nil {
		return "", fmt.Errorf("failed to parse version: %v", err)
	}

	// Select tool based on version
	switch {
	case major == 6 && minor >= 5:
		return filepath.Join("tools", "tidb-tools", "export_defaults_v6.go"), nil
	case major == 6 && minor < 5:
		return filepath.Join("tools", "tidb-tools", "export_defaults.go"), nil
	case major == 7 && minor <= 4:
		return filepath.Join("tools", "tidb-tools", "export_defaults_v7.go"), nil
	case major == 7 && minor >= 5:
		return filepath.Join("tools", "tidb-tools", "export_defaults.go"), nil
	case major == 8:
		return filepath.Join("tools", "tidb-tools", "export_defaults.go"), nil
	default:
		// Use the latest tool for unknown versions
		return filepath.Join("tools", "tidb-tools", "export_defaults.go"), nil
	}
}

// getTiDBVersion gets the TiDB version from the repository
func getTiDBVersion(repo string) (string, error) {
	// This is a placeholder implementation
	// In a real implementation, this would extract the version from the repository
	return "v7.5.0", nil
}

// parseVersion parses a version string like "v6.5.0" into major, minor, and patch components
func parseVersion(version string) (int, int, int, error) {
	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	// Split by dots
	parts := strings.Split(version, ".")
	if len(parts) < 3 {
		return 0, 0, 0, fmt.Errorf("invalid version format")
	}

	// Parse major, minor, and patch versions
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version: %v", err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor version: %v", err)
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid patch version: %v", err)
	}

	return major, minor, patch, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	return nil
}