// Package tidb provides tools for generating TiDB knowledge base from playground clusters
// This package collects runtime configuration and system variables directly from tiup playground clusters
package tidb

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	runtimeCollector "github.com/pingcap/tidb-upgrade-precheck/pkg/collector/runtime/tidb"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator"
)

const (
	defaultTiDBHost = "127.0.0.1"
	defaultTiDBPort = 4000
	defaultTiDBUser = "root"
	defaultTiDBPass = ""
	defaultPDHost   = "127.0.0.1"
	defaultPDPort   = 2379 // PD HTTP API port

	clusterStartTimeout = 300 // seconds
	connectionTimeout   = 30  // seconds
)

// Collect collects TiDB knowledge base from an existing tiup playground cluster
// This function assumes the playground cluster is already running and ready.
// Playground lifecycle (start/stop/wait) is managed by the caller (main.go).
// This function only:
// 1. Collects runtime configuration and system variables directly from the cluster via SHOW CONFIG and SHOW GLOBAL VARIABLES
// 2. Extracts bootstrap version from source code (needed for upgrade logic)
func Collect(tidbRoot, version, tag string) (*kbgenerator.KBSnapshot, error) {
	if tag == "" {
		return nil, fmt.Errorf("tag is required: playground cluster must be started by caller")
	}

	// Collect runtime configuration and system variables from cluster
	// Since playground cluster provides complete default config and variables,
	// we directly use runtime collector without code extraction
	fmt.Printf("Collecting runtime configuration and system variables from cluster...\n")
	state, err := collectRuntimeConfig(defaultTiDBPort, defaultTiDBUser, defaultTiDBPass)
	if err != nil {
		return nil, fmt.Errorf("failed to collect runtime configuration: %w", err)
	}

	// Extract bootstrap version from code (still needed for upgrade logic)
	// Note: We need to ensure TiDB repository is checked out to the correct version
	// The extractBootstrapVersion function will read from the repository, so it should
	// be called after the repository is in the correct state (or we need to checkout first)
	bootstrapVersion := extractBootstrapVersion(tidbRoot, version)
	if bootstrapVersion == 0 {
		fmt.Printf("Warning: Failed to extract bootstrap version for %s (returned 0). This may indicate the TiDB repository is not checked out to the correct version.\n", version)
	}

	snapshot := &kbgenerator.KBSnapshot{
		Component:        kbgenerator.ComponentTiDB,
		Version:          version,
		ConfigDefaults:   state.Config,    // Direct assignment - types are compatible
		SystemVariables:  state.Variables, // Direct assignment - types are compatible
		BootstrapVersion: bootstrapVersion,
	}

	return snapshot, nil
}

// StartPlayground starts a tiup playground cluster (exported for use by main.go)
func StartPlayground(version, tag string) error {
	return startPlayground(version, tag)
}

// startPlayground starts a tiup playground cluster
func startPlayground(version, tag string) error {
	// Pre-check: ensure components are installed and complete before starting
	// This helps avoid "no such file or directory" errors
	fmt.Printf("Checking if components are installed for version %s...\n", version)

	// Get tiup home directory
	tiupHome := os.Getenv("TIUP_HOME")
	if tiupHome == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			tiupHome = filepath.Join(homeDir, ".tiup")
		}
	}

	// Check all required components for completeness
	// Define component name to binary name mapping
	components := map[string]string{
		"tidb":    "tidb-server",
		"pd":      "pd-server",
		"tikv":    "tikv-server",
		"tiflash": "tiflash",
	}

	missingComponents := []string{}
	useForce := false

	for compName, binaryName := range components {
		compDir := filepath.Join(tiupHome, "components", compName, version)
		binaryPath := filepath.Join(compDir, binaryName)

		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			missingComponents = append(missingComponents, compName)
			// If directory exists but binary is missing, we need to force re-download
			if _, err := os.Stat(compDir); err == nil {
				useForce = true
				fmt.Printf("Component %s directory exists but binary %s is missing\n", compName, binaryName)
			}
		}
	}

	// If any component is missing, install all components
	if len(missingComponents) > 0 {
		fmt.Printf("Missing components: %v, installing components for version %s...\n", missingComponents, version)

		// Build install command with --force if needed
		installArgs := []string{"install"}
		if useForce {
			installArgs = append(installArgs, "--force")
			fmt.Printf("Using --force to re-download incomplete components...\n")
		}
		installArgs = append(installArgs,
			fmt.Sprintf("tidb:%s", version),
			fmt.Sprintf("pd:%s", version),
			fmt.Sprintf("tikv:%s", version),
			fmt.Sprintf("tiflash:%s", version),
		)

		installCmd := exec.Command("tiup", installArgs...)
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("failed to install components for version %s: %w", version, err)
		}
		fmt.Printf("Components installed successfully\n")

		// Verify all components are installed correctly
		stillMissing := []string{}
		for compName, binaryName := range components {
			binaryPath := filepath.Join(tiupHome, "components", compName, version, binaryName)
			if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
				stillMissing = append(stillMissing, fmt.Sprintf("%s/%s", compName, binaryName))
			}
		}

		if len(stillMissing) > 0 {
			return fmt.Errorf("component installation completed but some binaries are still missing: %v. This may indicate a network issue or insufficient disk space.", stillMissing)
		}

		fmt.Printf("All component binaries verified successfully\n")
	} else {
		fmt.Printf("All components are already installed and complete\n")
	}

	// Clean up any stale temporary storage locks before starting
	// This helps avoid "fslock: lock is held" errors when multiple instances start concurrently
	cleanupTempStorageLocks(tag)

	// Create a unique temporary storage path for this instance to avoid file lock conflicts
	tmpStoragePath := fmt.Sprintf("/tmp/tidb-tmp-storage-%s", tag)
	os.MkdirAll(tmpStoragePath, 0755)

	// Create a temporary TiDB config file with unique tmp-storage-path
	tmpConfigFile := filepath.Join(os.TempDir(), fmt.Sprintf("tidb-config-%s.toml", tag))
	configContent := fmt.Sprintf(`# Temporary TiDB configuration for playground instance %s
# This file is auto-generated to avoid tmp-storage-path conflicts

tmp-storage-path = "%s"
`, tag, tmpStoragePath)

	if err := os.WriteFile(tmpConfigFile, []byte(configContent), 0644); err != nil {
		// If we can't create config file, continue without it (cleanup should help)
		fmt.Printf("Warning: failed to create temp config file: %v\n", err)
	} else {
		// Clean up config file after playground starts (defer won't work here, so we'll clean it in stopPlayground)
		defer func() {
			// Try to clean up after a delay (playground needs time to read it)
			go func() {
				time.Sleep(30 * time.Second)
				os.Remove(tmpConfigFile)
				os.RemoveAll(tmpStoragePath)
			}()
		}()
	}

	cmdArgs := []string{
		"playground", version,
		"--tag", tag,
		"--without-monitor",
		"--db", "1",
		"--kv", "1",
		"--pd", "1",
		"--tiflash", "1",
	}

	// Add config file if we created it successfully
	if _, err := os.Stat(tmpConfigFile); err == nil {
		cmdArgs = append(cmdArgs, "--db.config", tmpConfigFile)
	}

	cmd := exec.Command("tiup", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tiup playground: %w", err)
	}

	// Give it a moment to start
	// Add extra delay to ensure previous instances have released locks
	// This is especially important when starting multiple instances concurrently
	time.Sleep(8 * time.Second)

	return nil
}

// cleanupTempStorageLocks cleans up stale temporary storage locks
// This helps avoid "fslock: lock is held" errors when multiple TiDB instances start concurrently
// TiDB generates tmp-storage-path based on connection addresses, so multiple instances with same
// addresses will try to use the same path, causing lock conflicts.
func cleanupTempStorageLocks(tag string) {
	// Find and remove stale lock files in /var/folders (macOS temporary directory)
	// The lock path pattern is: /var/folders/.../T/501_tidb/{base64_encoded}/tmp-storage
	// TiDB uses base64-encoded connection addresses to generate unique paths, but if multiple
	// instances have the same addresses, they'll generate the same path.

	// Strategy 1: Clean up all tmp-storage directories older than 30 seconds
	// This is more aggressive but safer for concurrent starts
	cmd := exec.Command("find", "/var/folders", "-type", "d", "-name", "tmp-storage", "-mmin", "+0.5", "-exec", "rm", "-rf", "{}", "+")
	_ = cmd.Run() // Ignore errors

	// Strategy 2: Clean up based on parent directory pattern
	// TiDB creates: /var/folders/.../T/501_tidb/{encoded}/tmp-storage
	// We can clean up old encoded directories
	cmd = exec.Command("find", "/var/folders", "-type", "d", "-path", "*/501_tidb/*", "-mmin", "+1", "-exec", "sh", "-c", "rm -rf \"$1/tmp-storage\" 2>/dev/null || true", "_", "{}", "+")
	_ = cmd.Run() // Ignore errors

	// Strategy 3: Clean up /tmp/tidb-* directories (alternative location)
	cmd = exec.Command("find", "/tmp", "-type", "d", "-name", "tidb-*", "-mmin", "+1", "-exec", "rm", "-rf", "{}", "+")
	_ = cmd.Run() // Ignore errors

	// Strategy 4: More aggressive cleanup for very old locks (older than 5 minutes)
	// These are definitely stale
	cmd = exec.Command("find", "/var/folders", "-type", "d", "-name", "tmp-storage", "-mmin", "+5", "-exec", "rm", "-rf", "{}", "+")
	_ = cmd.Run() // Ignore errors
}

// StopPlayground stops a tiup playground cluster (exported for use by main.go)
func StopPlayground(tag string) error {
	return stopPlayground(tag)
}

// stopPlayground stops a tiup playground cluster and cleans up its data directory
// tiup playground doesn't have a direct stop command, so we kill the process by tag
// This function kills all related processes including child processes
// For serial generation, this ensures complete cleanup after each version
func stopPlayground(tag string) error {
	fmt.Printf("Forcefully stopping and cleaning up playground cluster (tag: %s)...\n", tag)

	// Get tiup home directory first
	tiupHome := os.Getenv("TIUP_HOME")
	if tiupHome == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			tiupHome = filepath.Join(homeDir, ".tiup")
		}
	}

	// Step 1: Find all PIDs related to this tag and kill them
	// This is more aggressive and ensures we catch all processes
	findCmd := exec.Command("pgrep", "-f", tag)
	output, err := findCmd.Output()
	if err == nil && len(output) > 0 {
		pids := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, pidStr := range pids {
			pidStr = strings.TrimSpace(pidStr)
			if pidStr != "" {
				// Kill the process and its children
				exec.Command("kill", "-TERM", pidStr).Run()
				exec.Command("kill", "-9", pidStr).Run()
			}
		}
	}

	// Step 2: Kill the main tiup playground process with the specific tag
	// Use SIGTERM first for graceful shutdown
	cmd := exec.Command("pkill", "-TERM", "-f", fmt.Sprintf("tiup playground.*%s", tag))
	_ = cmd.Run() // Ignore errors, process might already be stopped

	// Wait a bit for graceful shutdown
	time.Sleep(2 * time.Second)

	// Step 3: Get the main playground process PID and kill its process tree
	findCmd = exec.Command("pgrep", "-f", fmt.Sprintf("tiup playground.*%s", tag))
	output, err = findCmd.Output()
	if err == nil && len(output) > 0 {
		pid := strings.TrimSpace(string(output))
		if pid != "" {
			// Kill the process tree (parent and all children)
			exec.Command("pkill", "-TERM", "-P", pid).Run()
			exec.Command("kill", "-TERM", pid).Run()
		}
	}

	// Step 4: Kill all child processes that might still be running
	// These are the actual server processes started by playground
	childProcesses := []string{
		"tidb-server",
		"tikv-server",
		"pd-server",
		"tiflash",
		"tikv-cdc",
		"tiproxy",
	}

	// Kill processes that have the tag in their command line or working directory
	for _, proc := range childProcesses {
		// Kill by tag in command line
		exec.Command("pkill", "-TERM", "-f", fmt.Sprintf("%s.*%s", proc, tag)).Run()
		// Also try to kill by process name if it's in the data directory
		if tiupHome != "" {
			dataDir := filepath.Join(tiupHome, "data", tag)
			exec.Command("pkill", "-TERM", "-f", fmt.Sprintf("%s.*%s", proc, dataDir)).Run()
		}
	}

	// Step 5: Force kill everything (SIGKILL) - more aggressive cleanup
	time.Sleep(1 * time.Second)
	exec.Command("pkill", "-9", "-f", fmt.Sprintf("tiup playground.*%s", tag)).Run()

	// Force kill all child processes related to this tag
	for _, proc := range childProcesses {
		exec.Command("pkill", "-9", "-f", fmt.Sprintf("%s.*%s", proc, tag)).Run()
		if tiupHome != "" {
			dataDir := filepath.Join(tiupHome, "data", tag)
			exec.Command("pkill", "-9", "-f", fmt.Sprintf("%s.*%s", proc, dataDir)).Run()
		}
	}

	// Step 6: Kill any remaining processes by port (if we can identify them)
	// This is a last resort to ensure ports are freed
	// Note: We don't kill by port directly as it might affect other processes
	// Instead, we rely on process killing above

	// Wait a bit for all processes to terminate
	time.Sleep(3 * time.Second)

	// Step 7: Clean up data directory for this tag
	if tiupHome == "" {
		tiupHome = os.Getenv("TIUP_HOME")
		if tiupHome == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				// If we can't get home dir, continue with other cleanup
				tiupHome = ""
			} else {
				tiupHome = filepath.Join(homeDir, ".tiup")
			}
		}
	}

	if tiupHome != "" {
		dataDir := filepath.Join(tiupHome, "data", tag)
		if _, err := os.Stat(dataDir); err == nil {
			// Try multiple times to remove (in case files are still locked)
			for i := 0; i < 3; i++ {
				if err := os.RemoveAll(dataDir); err != nil {
					if i < 2 {
						// Wait a bit and try again
						time.Sleep(1 * time.Second)
						continue
					}
					fmt.Printf("Warning: failed to remove data directory %s after 3 attempts: %v\n", dataDir, err)
				} else {
					fmt.Printf("✓ Cleaned up data directory: %s\n", dataDir)
					break
				}
			}
		}
	}

	// Step 8: Clean up temporary config file and storage paths
	tmpConfigFile := filepath.Join(os.TempDir(), fmt.Sprintf("tidb-config-%s.toml", tag))
	if _, err := os.Stat(tmpConfigFile); err == nil {
		os.Remove(tmpConfigFile)
		fmt.Printf("✓ Cleaned up temp config file: %s\n", tmpConfigFile)
	}

	tmpStoragePath := fmt.Sprintf("/tmp/tidb-tmp-storage-%s", tag)
	if _, err := os.Stat(tmpStoragePath); err == nil {
		os.RemoveAll(tmpStoragePath)
		fmt.Printf("✓ Cleaned up temp storage path: %s\n", tmpStoragePath)
	}

	// Step 9: Clean up any remaining tmp-storage locks in /var/folders
	// These might be left behind even after process termination
	cleanupTempStorageLocks(tag)

	fmt.Printf("✓ Playground cluster cleanup completed for tag: %s\n", tag)
	return nil
}

// WaitForClusterReady waits for the cluster to be ready (exported for use by main.go)
func WaitForClusterReady(tag string, port int) error {
	return waitForClusterReady(tag, port)
}

// waitForClusterReady waits for the cluster to be ready
func waitForClusterReady(tag string, port int) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", defaultTiDBUser, defaultTiDBPass, defaultTiDBHost, port)

	deadline := time.Now().Add(clusterStartTimeout * time.Second)

	for time.Now().Before(deadline) {
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			db.SetConnMaxLifetime(5 * time.Second)
			err = db.Ping()
			if err == nil {
				db.Close()
				fmt.Printf("Cluster is ready!\n")
				return nil
			}
			db.Close()
		}

		fmt.Printf("Waiting for cluster... (%d seconds remaining)\n", int(time.Until(deadline).Seconds()))
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("cluster did not become ready within %d seconds", clusterStartTimeout)
}

// collectRuntimeConfig collects runtime configuration and system variables from the cluster
// This uses the runtime collector to ensure consistency with real cluster collection
// The runtime collector uses SHOW CONFIG and SHOW GLOBAL VARIABLES
func collectRuntimeConfig(port int, user, password string) (*collector.ComponentState, error) {
	// Use runtime collector directly (same logic as real cluster collection)
	tidbCollector := runtimeCollector.NewTiDBCollector()
	addr := fmt.Sprintf("%s:%d", defaultTiDBHost, port)

	// Collect using runtime collector
	state, err := tidbCollector.Collect(addr, user, password)
	if err != nil {
		return nil, fmt.Errorf("failed to collect runtime configuration: %w", err)
	}

	// Debug: log collection result
	if len(state.Config) == 0 {
		fmt.Printf("Warning: collected 0 config parameters from runtime (SHOW CONFIG may have returned empty)\n")
	} else {
		fmt.Printf("Collected %d config parameters from runtime\n", len(state.Config))
	}

	if len(state.Variables) == 0 {
		fmt.Printf("Warning: collected 0 system variables from runtime\n")
	} else {
		fmt.Printf("Collected %d system variables from runtime\n", len(state.Variables))
	}

	return state, nil
}

// Note: extractCodeDefinitions, mergeConfigs, mergeVariables, and determineValueType have been removed.
// TiDB playground cluster provides complete default configuration and system variables via SHOW CONFIG
// and SHOW GLOBAL VARIABLES, so we no longer need to extract from source code or merge with code definitions.
// This simplifies the collection process significantly, similar to PD's approach.
