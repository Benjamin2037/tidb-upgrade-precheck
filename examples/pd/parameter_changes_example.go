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

	// 示例：获取从v6.5.0到v7.1.0的参数变更
	fromVersion := "v6.5.0"
	toVersion := "v7.1.0"

	changes, err := kbgenerator.GetPDParameterChanges(historyFile, fromVersion, toVersion)
	if err != nil {
		log.Fatalf("Failed to get parameter changes: %v", err)
	}

	fmt.Printf("PD Parameter Changes from %s to %s:\n", fromVersion, toVersion)
	fmt.Println("========================================")

	if len(changes) == 0 {
		fmt.Println("No parameter changes found.")
		return
	}

	for _, change := range changes {
		fmt.Printf("Parameter: %s\n", change.Name)
		fmt.Printf("Type: %s\n", change.Type)
		fmt.Printf("Description: %s\n", change.Description)
		fmt.Printf("From Value: %v\n", change.FromValue)
		fmt.Printf("To Value: %v\n", change.ToValue)
		fmt.Println("---")
	}

	// 将结果保存为JSON文件
	outputFile := "/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge/pd/changes_example.json"
	data, err := json.MarshalIndent(changes, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal changes: %v", err)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		log.Fatalf("Failed to write changes to file: %v", err)
	}

	fmt.Printf("\nChanges saved to: %s\n", outputFile)
}