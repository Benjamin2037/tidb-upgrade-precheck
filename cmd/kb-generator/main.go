package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator"
)

var (
	tidbRepoRoot = flag.String("tidb-repo", "", "Path to TiDB repository root")
	pdRepoRoot   = flag.String("pd-repo", "", "Path to PD repository root")
	tikvRepoRoot = flag.String("tikv-repo", "", "Path to TiKV repository root")
	all          = flag.Bool("all", false, "Generate knowledge base for all components")
	genHistory   = flag.Bool("gen-history", false, "Generate parameter history for all components")
)

func main() {
	flag.Parse()

	if *all {
		if *tidbRepoRoot != "" {
			if err := generateTiDBKB(); err != nil {
				log.Fatalf("Failed to generate TiDB knowledge base: %v", err)
			}
		}

		if *pdRepoRoot != "" {
			if err := generatePDKB(); err != nil {
				log.Fatalf("Failed to generate PD knowledge base: %v", err)
			}
		}

		if *tikvRepoRoot != "" {
			if err := generateTiKVKB(); err != nil {
				log.Fatalf("Failed to generate TiKV knowledge base: %v", err)
			}
		}
	} else if *genHistory {
		if *tidbRepoRoot != "" {
			if err := generateTiDBParameterHistory(); err != nil {
				log.Fatalf("Failed to generate TiDB parameter history: %v", err)
			}
		}

		if *pdRepoRoot != "" {
			if err := generatePDParameterHistory(); err != nil {
				log.Fatalf("Failed to generate PD parameter history: %v", err)
			}
		}

		// TODO: Implement TiKV parameter history generation
		if *tikvRepoRoot != "" {
			if err := generateTiKVParameterHistory(); err != nil {
				log.Fatalf("Failed to generate TiKV parameter history: %v", err)
			}
		}
	} else {
		fmt.Println("Usage:")
		fmt.Println("  kb-generator --all --tidb-repo=/path/to/tidb --pd-repo=/path/to/pd --tikv-repo=/path/to/tikv")
		fmt.Println("  kb-generator --gen-history --tidb-repo=/path/to/tidb --pd-repo=/path/to/pd --tikv-repo=/path/to/tikv")
	}
}

func generateTiDBKB() error {
	fmt.Println("Generating TiDB knowledge base...")

	versions := []string{"v6.5.0", "v7.1.0", "v7.5.0", "v8.1.0", "v8.5.0"}

	for _, version := range versions {
		fmt.Printf("Collecting TiDB knowledge for version %s...\n", version)

		snapshot, err := kbgenerator.CollectFromTidbSource(*tidbRepoRoot, version)
		if err != nil {
			return fmt.Errorf("failed to collect TiDB knowledge for version %s: %v", version, err)
		}

		outputPath := filepath.Join("knowledge", "tidb", version, "defaults.json")
		if err := kbgenerator.SaveTiDBSnapshot(snapshot, outputPath); err != nil {
			return fmt.Errorf("failed to save TiDB knowledge for version %s: %v", version, err)
		}

		fmt.Printf("Saved TiDB knowledge for version %s to %s\n", version, outputPath)
	}

	// Collect upgrade logic
	fmt.Println("Collecting TiDB upgrade logic...")
	upgradeLogic, err := kbgenerator.CollectUpgradeLogicFromSource(filepath.Join(*tidbRepoRoot, "pkg", "session", "bootstrap.go"))
	if err != nil {
		return fmt.Errorf("failed to collect TiDB upgrade logic: %v", err)
	}

	upgradeLogicPath := filepath.Join("knowledge", "tidb", "upgrade_logic.json")
	if err := kbgenerator.SaveUpgradeLogic(upgradeLogic, upgradeLogicPath); err != nil {
		return fmt.Errorf("failed to save TiDB upgrade logic: %v", err)
	}

	fmt.Printf("Saved TiDB upgrade logic to %s\n", upgradeLogicPath)

	// Generate upgrade script
	script := "#!/bin/bash\n\n# TiDB Upgrade Script\n# Auto-generated - review and modify as needed\n\necho \"Starting TiDB upgrade...\"\n\n# TODO: Add upgrade steps based on the upgrade logic\necho \"Upgrade script completed. Review the changes and adjust as needed.\"\n"
	scriptPath := filepath.Join("knowledge", "tidb", "upgrade_script.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("failed to save TiDB upgrade script: %v", err)
	}

	fmt.Printf("Saved TiDB upgrade script to %s\n", scriptPath)

	return nil
}

func generatePDKB() error {
	fmt.Println("Generating PD knowledge base...")

	versions := []string{"v6.5.0", "v7.1.0", "v7.5.0", "v8.1.0", "v8.5.0"}

	for _, version := range versions {
		fmt.Printf("Collecting PD knowledge for version %s...\n", version)

		snapshot, err := kbgenerator.CollectFromPDSource(*pdRepoRoot, version)
		if err != nil {
			return fmt.Errorf("failed to collect PD knowledge for version %s: %v", version, err)
		}

		outputPath := filepath.Join("knowledge", "pd", version, "pd_defaults.json")
		if err := kbgenerator.SavePDSnapshot(snapshot, outputPath); err != nil {
			return fmt.Errorf("failed to save PD knowledge for version %s: %v", version, err)
		}

		fmt.Printf("Saved PD knowledge for version %s to %s\n", version, outputPath)
	}

	return nil
}

func generateTiKVKB() error {
	fmt.Println("Generating TiKV knowledge base...")

	versions := []string{"v6.5.0", "v7.1.0", "v7.5.0", "v8.1.0", "v8.5.0"}

	for _, version := range versions {
		fmt.Printf("Collecting TiKV knowledge for version %s...\n", version)

		snapshot, err := kbgenerator.CollectFromTikvSource(*tikvRepoRoot, version)
		if err != nil {
			return fmt.Errorf("failed to collect TiKV knowledge for version %s: %v", version, err)
		}

		outputPath := filepath.Join("knowledge", "tikv", version, "tikv_defaults.json")
		if err := kbgenerator.SaveTiKVSnapshot(snapshot, outputPath); err != nil {
			return fmt.Errorf("failed to save TiKV knowledge for version %s: %v", version, err)
		}

		fmt.Printf("Saved TiKV knowledge for version %s to %s\n", version, outputPath)
	}

	// Collect upgrade logic
	fmt.Println("Collecting TiKV upgrade logic...")
	fromVersions := []string{"v6.5.0", "v7.1.0", "v7.5.0", "v8.1.0"}
	toVersions := []string{"v7.1.0", "v7.5.0", "v8.1.0", "v8.5.0"}

	for i := 0; i < len(fromVersions); i++ {
		from := fromVersions[i]
		to := toVersions[i]

		fmt.Printf("Collecting TiKV upgrade logic from %s to %s...\n", from, to)

		changes, err := kbgenerator.CollectTikvUpgradeLogic(*tikvRepoRoot, from, to)
		if err != nil {
			return fmt.Errorf("failed to collect TiKV upgrade logic from %s to %s: %v", from, to, err)
		}

		if len(changes) > 0 {
			upgradeLogicPath := filepath.Join("knowledge", "tikv", fmt.Sprintf("%s_to_%s_upgrade_logic.json", from, to))
			data, err := json.MarshalIndent(changes, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal TiKV upgrade logic from %s to %s: %v", from, to, err)
			}

			if err := os.WriteFile(upgradeLogicPath, data, 0644); err != nil {
				return fmt.Errorf("failed to save TiKV upgrade logic from %s to %s: %v", from, to, err)
			}

			fmt.Printf("Saved TiKV upgrade logic from %s to %s to %s\n", from, to, upgradeLogicPath)

			// Generate upgrade script
			script := kbgenerator.GenerateTikvUpgradeScript(changes)
			scriptPath := filepath.Join("knowledge", "tikv", fmt.Sprintf("%s_to_%s_upgrade_script.sh", from, to))
			if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
				return fmt.Errorf("failed to save TiKV upgrade script from %s to %s: %v", from, to, err)
			}

			fmt.Printf("Saved TiKV upgrade script from %s to %s to %s\n", from, to, scriptPath)
		}
	}

	return nil
}

func generateTiDBParameterHistory() error {
	fmt.Println("Generating TiDB parameter history...")

	// This would collect parameter history across all versions
	// For now, we'll just print a message
	fmt.Println("TiDB parameter history generation not yet implemented")

	return nil
}

func generatePDParameterHistory() error {
	fmt.Println("Generating PD parameter history...")

	// TODO: Implement PD parameter history generation
	fmt.Println("PD parameter history generation not yet implemented")

	return nil
}

func generateTiKVParameterHistory() error {
	fmt.Println("Generating TiKV parameter history...")

	// TODO: Implement TiKV parameter history generation
	fmt.Println("TiKV parameter history generation not yet implemented")

	return nil
}