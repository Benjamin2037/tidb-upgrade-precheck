package scan

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ScanUpgradeLogic analyzes upgrade.go file and extracts upgrade logic
// This function only needs to use the latest code to collect upgrade logic
// The tag parameter is kept for compatibility but not used
func ScanUpgradeLogic(repo, tag string) error {
	fmt.Printf("[ScanUpgradeLogic] repo=%s\n", repo)
	
	// Create output directory
	outDir := filepath.Join("knowledge", "tidb")
	os.MkdirAll(outDir, 0755)
	outFile := filepath.Join(outDir, "upgrade_logic.json")
	
	// Get current directory
	currentDir, _ := os.Getwd()
	srcToolPath := filepath.Join(currentDir, "tools", "upgrade_logic_collector.go")
	
	// Run upgrade logic collection tool directly on the repo (using latest code)
	run := exec.Command("go", "run", srcToolPath, repo)
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()
	run.Stdout = f
	run.Stderr = os.Stderr
	if err := run.Run(); err != nil {
		return fmt.Errorf("go run upgrade_logic_collector.go failed: %v", err)
	}
	
	return nil
}

// GetAllUpgradeChanges scans all versions and extracts upgrade changes
func GetAllUpgradeChanges(repo string) error {
	// This would implement scanning all versions for upgrade changes
	// and generating a global upgrade_logic.json
	fmt.Println("[GetAllUpgradeChanges] Collecting upgrade changes from all versions")
	return nil
}