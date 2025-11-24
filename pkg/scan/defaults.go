package scan

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ScanDefaults imports target version packages at runtime, serializes sysvar/config default values, and extracts CurrentBootstrapVersion
func ScanDefaults(repo, tag string) error {
	fmt.Printf("[ScanDefaults] repo=%s tag=%s\n", repo, tag)
	
	// Initialize version manager
	vm, err := NewVersionManager("knowledge")
	if err != nil {
		return fmt.Errorf("failed to initialize version manager: %v", err)
	}
	
	// Check if version already generated
	if vm.IsVersionGenerated(tag) {
		fmt.Printf("[Skipped] Version %s already generated, skipping parameter collection\n", tag)
		return nil
	}
	
	// Create a temporary directory for cloning
	tempDir, err := ioutil.TempDir("", "tidb_upgrade_precheck")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Clone the repo to temporary directory
	cloneCmd := exec.Command("git", "clone", repo, tempDir)
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repo: %v", err)
	}
	
	// Checkout the specific tag in the cloned repo
	checkoutCmd := exec.Command("git", "checkout", tag)
	checkoutCmd.Dir = tempDir
	if err := checkoutCmd.Run(); err != nil {
		return fmt.Errorf("git checkout %s failed: %v", tag, err)
	}
	
	// Determine which export_defaults file to use based on version
	toolFileName := selectToolByVersion(tag)
	
	// Copy the appropriate export_defaults tool to the cloned repo
	// Now we copy from tidb-upgrade-precheck project instead of tidb project
	srcToolPath := filepath.Join("./tools/upgrade-precheck", toolFileName)
	dstToolPath := filepath.Join(tempDir, "tools", "export_defaults.go")
	
	// Create tools directory if it doesn't exist
	if err := os.MkdirAll(filepath.Join(tempDir, "tools"), 0755); err != nil {
		return fmt.Errorf("failed to create tools directory: %v", err)
	}
	
	// Copy file
	if err := copyFile(srcToolPath, dstToolPath); err != nil {
		return fmt.Errorf("failed to copy export_defaults.go: %v", err)
	}
	
	// Create tag directory
	outDir := filepath.Join("knowledge", tag)
	os.MkdirAll(outDir, 0755)
	outFile := filepath.Join(outDir, "defaults.json")
	
	// Run parameter collection tool on the cloned repo
	run := exec.Command("go", "run", "tools/export_defaults.go")
	run.Dir = tempDir
	// Ensure GO111MODULE is enabled for proper module handling
	run.Env = append(os.Environ(), "GO111MODULE=on")
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()
	run.Stdout = f
	run.Stderr = os.Stderr
	if err := run.Run(); err != nil {
		return fmt.Errorf("go run export_defaults.go failed: %v", err)
	}
	
	// Record this version as generated
	// Get commit hash for this tag
	commitCmd := exec.Command("git", "rev-parse", "HEAD")
	commitCmd.Dir = tempDir
	commitHash, err := commitCmd.Output()
	if err == nil {
		if err := vm.RecordVersion(tag, string(commitHash)); err != nil {
			fmt.Printf("[WARN] failed to record version %s: %v\n", tag, err)
		}
	}
	
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	
	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()
	
	_, err = io.Copy(destinationFile, sourceFile)
	return err
}

// selectToolByVersion determines which export_defaults file to use based on the version tag
func selectToolByVersion(tag string) string {
	// Parse version numbers
	version := strings.TrimPrefix(tag, "v")
	parts := strings.Split(version, ".")
	
	if len(parts) < 2 {
		return "export_defaults.go" // Default to latest
	}
	
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "export_defaults.go" // Default to latest
	}
	
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "export_defaults.go" // Default to latest
	}
	
	// Based on the version, select the appropriate tool file
	switch {
	case major == 6:
		return "export_defaults_v6.go"
	case major == 7:
		if minor < 5 {
			return "export_defaults_v71.go" // v7.0 - v7.4
		} else {
			return "export_defaults_v75plus.go" // v7.5+
		}
	case major >= 8:
		return "export_defaults_v75plus.go" // v8.0+
	default:
		return "export_defaults.go" // Default to latest
	}
}