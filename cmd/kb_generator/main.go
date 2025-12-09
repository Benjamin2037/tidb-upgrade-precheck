package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	kbgenerator "github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator"
	pdkb "github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator/pd"
	tidbkb "github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator/tidb"
	tiflashkb "github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator/tiflash"
	tikvkb "github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator/tikv"
)

// getVersionGroup extracts the version group (first two digits) from a full version string
// Example: v6.5.0 -> v6.5, v7.1.0 -> v7.1
func getVersionGroup(version string) string {
	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	// Split by '.' and take first two parts
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		return "v" + parts[0] + "." + parts[1]
	}
	// Fallback: if version doesn't have expected format, return as is
	return "v" + version
}

var (
	tidbRepoRoot    = flag.String("tidb-repo", "", "Path to TiDB repository root (required for code definition extraction)")
	pdRepoRoot      = flag.String("pd-repo", "", "Path to PD repository root (required for code definition extraction)")
	tikvRepoRoot    = flag.String("tikv-repo", "", "Path to TiKV repository root (required for code definition extraction)")
	tiflashRepoRoot = flag.String("tiflash-repo", "", "Path to TiFlash repository root (required for code definition extraction)")
	version         = flag.String("version", "", "Version tag to generate knowledge base (single version mode)")
	fromTag         = flag.String("from-tag", "", "Source version tag (version range mode)")
	toTag           = flag.String("to-tag", "", "Target version tag (version range mode)")
	components      = flag.String("components", "tidb,pd,tikv,tiflash", "Comma-separated list of components to generate (default: all)")
)

const (
	defaultTiDBPort = 4000
	defaultPDPort   = 2379
)

func main() {
	flag.Parse()

	// Validate mode: either (from-tag + to-tag) or version
	if (*fromTag != "" && *toTag != "") && *version != "" {
		fmt.Fprintf(os.Stderr, "Error: Cannot specify both version range (--from-tag/--to-tag) and single version (--version)\n")
		os.Exit(1)
	}

	if *fromTag == "" && *toTag == "" && *version == "" {
		fmt.Fprintf(os.Stderr, "Error: Must specify either --version (single version) or --from-tag/--to-tag (version range)\n")
		flag.Usage()
		os.Exit(1)
	}

	// Determine mode and versions to process
	var versionsToProcess []string
	if *fromTag != "" && *toTag != "" {
		// Version range mode: process both versions
		versionsToProcess = []string{*fromTag, *toTag}
		fmt.Printf("Version range mode: generating knowledge base for %s and %s\n", *fromTag, *toTag)
	} else {
		// Single version mode
		versionsToProcess = []string{*version}
		fmt.Printf("Single version mode: generating knowledge base for %s\n", *version)
	}

	// Parse components list
	componentList := strings.Split(*components, ",")
	componentMap := make(map[string]bool)
	for _, comp := range componentList {
		comp = strings.TrimSpace(comp)
		if comp != "" {
			componentMap[comp] = true
		}
	}

	// Process each version
	for i, version := range versionsToProcess {
		if i > 0 {
			fmt.Printf("\n")
			fmt.Printf("========================================\n")
			fmt.Printf("Processing next version: %s\n", version)
			fmt.Printf("========================================\n")
			fmt.Printf("\n")
		}

		// Generate unique tag for this run (shared across all components)
		tag := fmt.Sprintf("kb-gen-%s-%d", version, time.Now().Unix())

		// Start playground cluster first (before any component collection)
		// This ensures all components can access the cluster data
		fmt.Printf("Starting tiup playground cluster for version %s (tag: %s)...\n", version, tag)
		if err := tidbkb.StartPlayground(version, tag); err != nil {
			log.Fatalf("Failed to start playground cluster: %v", err)
		}

		// Wait for cluster to be ready
		fmt.Printf("Waiting for cluster to be ready...\n")
		if err := tidbkb.WaitForClusterReady(tag, defaultTiDBPort); err != nil {
			log.Fatalf("Cluster failed to become ready: %v", err)
		}

		// Generate TiDB knowledge base (using existing playground)
		var tidbConfig kbgenerator.ConfigDefaults
		if componentMap["tidb"] && *tidbRepoRoot != "" {
			snapshot, err := tidbkb.Collect(*tidbRepoRoot, version, tag)
			if err != nil {
				log.Fatalf("Failed to generate TiDB knowledge base: %v", err)
			}
			tidbConfig = snapshot.ConfigDefaults

			// Save TiDB knowledge base
			versionGroup := getVersionGroup(version)
			outputPath := filepath.Join("knowledge", versionGroup, version, "tidb", "defaults.json")
			if err := kbgenerator.SaveKBSnapshot(snapshot, outputPath); err != nil {
				log.Fatalf("Failed to save TiDB knowledge base: %v", err)
			}
			fmt.Printf("Saved TiDB knowledge for version %s to %s\n", version, outputPath)
		}

		// Generate PD knowledge base (using the same playground instance)
		if componentMap["pd"] && *pdRepoRoot != "" {
			if err := generateSingleVersionPD(version, tag, tidbConfig); err != nil {
				log.Fatalf("Failed to generate PD knowledge base: %v", err)
			}
		}

		// Generate TiKV knowledge base (using the same playground instance)
		if componentMap["tikv"] && *tikvRepoRoot != "" {
			if err := generateSingleVersionTiKV(version, tag); err != nil {
				log.Printf("Warning: failed to generate TiKV knowledge base: %v\n", err)
				log.Printf("Continuing with other components...\n")
			}
		}

		// Generate TiFlash knowledge base (using the same playground instance)
		if componentMap["tiflash"] && *tiflashRepoRoot != "" {
			if err := generateSingleVersionTiFlash(version, tag); err != nil {
				log.Printf("Warning: failed to generate TiFlash knowledge base: %v\n", err)
				log.Printf("Continuing with other components...\n")
			}
		}

		// Cleanup cluster after each version
		// This ensures cleanup happens synchronously and resources are released immediately
		// For serial generation, this ensures complete cleanup after each version to avoid conflicts
		fmt.Printf("========================================\n")
		fmt.Printf("Forcefully cleaning up playground cluster (tag: %s)...\n", tag)
		fmt.Printf("========================================\n")
		if err := tidbkb.StopPlayground(tag); err != nil {
			log.Printf("Warning: failed to stop playground cluster: %v\n", err)
		}
		// Wait longer to ensure all processes are terminated and resources are released
		// This is especially important for serial generation to avoid conflicts
		time.Sleep(5 * time.Second)
		fmt.Printf("✓ Cleanup completed, ready for next version\n")
		fmt.Printf("========================================\n\n")
	}
}

// generateSingleVersionPD generates PD knowledge base
func generateSingleVersionPD(version string, tag string, tidbConfig kbgenerator.ConfigDefaults) error {
	fmt.Printf("Generating PD knowledge base for version %s...\n", version)

	// Get PD address from TiDB config (collected from runtime)
	var pdAddr string
	if tidbConfig != nil {
		pdPathVal, ok := tidbConfig["path"]
		if ok {
			if pdPathStr, isString := pdPathVal.Value.(string); isString && pdPathStr != "" {
				// path field contains PD endpoints, e.g., "127.0.0.1:2379" or "127.0.0.1:2379,127.0.0.1:2380"
				// Take the first endpoint
				endpoints := strings.Split(pdPathStr, ",")
				if len(endpoints) > 0 {
					pdAddr = strings.TrimSpace(endpoints[0])
					fmt.Printf("Extracted PD address from TiDB config: %s\n", pdAddr)
				}
			}
		}
	}

	if pdAddr == "" {
		// Fallback to default if not found in TiDB config
		pdAddr = fmt.Sprintf("%s:%d", "127.0.0.1", defaultPDPort)
		log.Printf("Warning: PD address not found in TiDB config, using default: %s\n", pdAddr)
	}

	// Collect from playground (using the same playground instance started by TiDB)
	snapshot, err := pdkb.Collect(*pdRepoRoot, version, pdAddr)
	if err != nil {
		return fmt.Errorf("failed to collect PD knowledge for version %s: %v", version, err)
	}

	versionGroup := getVersionGroup(version)
	outputPath := filepath.Join("knowledge", versionGroup, version, "pd", "defaults.json")
	if err := kbgenerator.SaveKBSnapshot(snapshot, outputPath); err != nil {
		return fmt.Errorf("failed to save PD knowledge for version %s: %v", version, err)
	}

	fmt.Printf("Saved PD knowledge for version %s to %s\n", version, outputPath)

	return nil
}

// generateSingleVersionTiKV generates TiKV knowledge base
func generateSingleVersionTiKV(version string, tag string) error {
	fmt.Printf("Generating TiKV knowledge base for version %s...\n", version)

	// Collect from playground (using the same playground instance started by TiDB)
	snapshot, err := tikvkb.Collect(*tikvRepoRoot, version, defaultTiDBPort, tag)
	if err != nil {
		return fmt.Errorf("failed to collect TiKV knowledge for version %s: %v", version, err)
	}

	versionGroup := getVersionGroup(version)
	outputPath := filepath.Join("knowledge", versionGroup, version, "tikv", "defaults.json")
	if err := kbgenerator.SaveKBSnapshot(snapshot, outputPath); err != nil {
		return fmt.Errorf("failed to save TiKV knowledge for version %s: %v", version, err)
	}

	fmt.Printf("Saved TiKV knowledge for version %s to %s\n", version, outputPath)

	return nil
}

// generateSingleVersionTiFlash generates TiFlash knowledge base
func generateSingleVersionTiFlash(version string, tag string) error {
	fmt.Printf("Generating TiFlash knowledge base for version %s...\n", version)

	// Collect from playground (using the same playground instance started by TiDB)
	snapshot, err := tiflashkb.Collect(*tiflashRepoRoot, version, defaultTiDBPort, tag)
	if err != nil {
		return fmt.Errorf("failed to collect TiFlash knowledge for version %s: %v", version, err)
	}

	versionGroup := getVersionGroup(version)
	outputPath := filepath.Join("knowledge", versionGroup, version, "tiflash", "defaults.json")
	if err := kbgenerator.SaveKBSnapshot(snapshot, outputPath); err != nil {
		return fmt.Errorf("failed to save TiFlash knowledge for version %s: %v", version, err)
	}

	fmt.Printf("Saved TiFlash knowledge for version %s to %s\n", version, outputPath)

	return nil
}
