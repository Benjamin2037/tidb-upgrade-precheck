package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator"
)

// ParamSnapshot represents a full parameter snapshot of a version
// Contains default values of config and system variables
// Can be extended as needed
type ParamSnapshot struct {
	Version         string                 `json:"version"`
	ConfigDefaults  map[string]interface{} `json:"config_defaults"`
	SystemVariables map[string]interface{} `json:"system_variables"`
}

// Recursively parse all go files under vardef directory to extract constant names and values
func parseVardefConstants(vardefDir string) map[string]string {
	result := make(map[string]string)
	filepath.WalkDir(vardefDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		// Match const XXX = "..." or const XXX = 123
		re := regexp.MustCompile(`const\s+([A-Za-z0-9_]+)\s*=\s*(["']?)([^"'\n]+)\2`)
		matches := re.FindAllStringSubmatch(string(data), -1)
		for _, m := range matches {
			if len(m) == 4 {
				result[m[1]] = m[3]
			}
		}
		return nil
	})
	return result
}

// Parse config.go static defaults (simplified example, needs improvement in practice)
func parseConfigDefaults(configPath string) map[string]interface{} {
	result := make(map[string]interface{})
	data, err := os.ReadFile(configPath)
	if err != nil {
		return result
	}
	// Simple regex matching Config struct fields and default values
	re := regexp.MustCompile(`([A-Za-z0-9_]+)\s+([A-Za-z0-9_]+)\s+` + "`.*default:(\\S+).*")
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		m := re.FindStringSubmatch(line)
		if len(m) == 4 {
			result[m[2]] = m[3]
		}
	}
	return result
}

// Parse sysvar.go static defaults (simplified example, needs improvement in practice)
func parseSysVars(sysvarPath string) map[string]interface{} {
	result := make(map[string]interface{})
	data, err := os.ReadFile(sysvarPath)
	if err != nil {
		return result
	}
	// Match SysVar{Name: ..., Value: ...}
	re := regexp.MustCompile(`Name:\s*"([A-Za-z0-9_]+)",[\s\S]*?Value:\s*"([^"]*)"`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	for _, m := range matches {
		if len(m) == 3 {
			result[m[1]] = m[2]
		}
	}
	return result
}

func main() {
	tidbRoot := flag.String("tidb", "../tidb", "TiDB source code root directory")
	tag := flag.String("tag", "", "TiDB version tag to collect")
	out := flag.String("out", "", "Output file path")
	flag.Parse()

	if *tag == "" || *out == "" {
		fmt.Println("Both --tag and --out are required")
		os.Exit(1)
	}

	snap, err := kbgenerator.CollectFromTidbSource(*tidbRoot, *tag)
	if err != nil {
		fmt.Println("Collection failed:", err)
		os.Exit(1)
	}
	data, _ := json.MarshalIndent(snap, "", "  ")
	err = os.WriteFile(*out, data, 0644)
	if err != nil {
		fmt.Println("Failed to write output file:", err)
		os.Exit(1)
	}
	fmt.Println("Full parameter snapshot generated:", *out)
}
