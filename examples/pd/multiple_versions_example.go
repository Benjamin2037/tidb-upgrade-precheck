package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator"
)

func main() {
	// 使用固定路径
	historyFile := "/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge/pd/parameters-history.json"

	// 检查文件是否存在
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		log.Fatalf("History file does not exist: %s", historyFile)
	}

	// 定义多个版本组合
	versionPairs := [][]string{
		{"v6.5.0", "v7.1.0"},
		{"v6.5.0", "v7.5.0"},
		{"v7.1.0", "v7.5.0"},
		{"v7.5.0", "v8.1.0"},
		{"v8.1.0", "v8.5.0"},
	}

	fmt.Println("PD Parameter Changes Across Multiple Version Pairs")
	fmt.Println("=================================================")

	for _, pair := range versionPairs {
		fromVersion := pair[0]
		toVersion := pair[1]

		changes, err := kbgenerator.GetPDParameterChanges(historyFile, fromVersion, toVersion)
		if err != nil {
			log.Printf("Failed to get parameter changes from %s to %s: %v", fromVersion, toVersion, err)
			continue
		}

		fmt.Printf("\nChanges from %s to %s:\n", fromVersion, toVersion)
		fmt.Println("------------------------")

		if len(changes) == 0 {
			fmt.Println("No parameter changes found.")
			continue
		}

		for _, change := range changes {
			fmt.Printf("Parameter: %s (%s)\n", change.Name, change.Type)
			fmt.Printf("  Description: %s\n", change.Description)
			fmt.Printf("  From Value: %v\n", change.FromValue)
			fmt.Printf("  To Value: %v\n", change.ToValue)
			fmt.Println()
		}

		// 将结果保存为JSON文件
		outputFile := fmt.Sprintf("/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge/pd/changes_%s_to_%s.json", fromVersion, toVersion)
		data, err := json.MarshalIndent(changes, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal changes from %s to %s: %v", fromVersion, toVersion, err)
			continue
		}

		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			log.Printf("Failed to write changes to file for %s to %s: %v", fromVersion, toVersion, err)
			continue
		}

		fmt.Printf("Changes saved to: %s\n", outputFile)
	}
}