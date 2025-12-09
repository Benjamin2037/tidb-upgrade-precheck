package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	pdconfig "github.com/tikv/pd/server/config"
	scheduleconfig "github.com/tikv/pd/pkg/schedule/config"
)

// PDParameter represents a PD configuration parameter
type PDParameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Value       interface{} `json:"value"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
}

// PDVersionDefaults represents the default values for a PD version
type PDVersionDefaults struct {
	Version    string         `json:"version"`
	Parameters []PDParameter `json:"parameters"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run collect_pd_params.go <version>")
		os.Exit(1)
	}

	version := os.Args[1]
	
	// Collect parameters from PD config structs
	parameters := collectPDParameters()
	
	// Create version defaults
	versionDefaults := PDVersionDefaults{
		Version:    version,
		Parameters: parameters,
	}
	
	// Output as JSON
	data, err := json.MarshalIndent(versionDefaults, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println(string(data))
}

func collectPDParameters() []PDParameter {
	var parameters []PDParameter
	
	// Create a config instance to get default values
	cfg := pdconfig.NewConfig()
	
	// Collect from main Config struct
	collectFromStruct(cfg, "", &parameters)
	
	// Collect from ScheduleConfig
	scheduleCfg := &scheduleconfig.ScheduleConfig{}
	collectFromStruct(scheduleCfg, "schedule", &parameters)
	
	// Collect from ReplicationConfig
	replicationCfg := &scheduleconfig.ReplicationConfig{}
	collectFromStruct(replicationCfg, "replication", &parameters)
	
	return parameters
}

func collectFromStruct(s interface{}, prefix string, parameters *[]PDParameter) {
	v := reflect.ValueOf(s).Elem()
	t := reflect.TypeOf(s).Elem()
	
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		
		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}
		
		// Get JSON tag to determine the parameter name
		jsonTag := fieldType.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		
		// Parse the JSON tag to get the name
		name := strings.Split(jsonTag, ",")[0]
		if name == "" {
			continue
		}
		
		// Add prefix if provided
		if prefix != "" {
			name = prefix + "." + name
		}
		
		// Get TOML tag for additional info
		tomlTag := fieldType.Tag.Get("toml")
		tomlName := strings.Split(tomlTag, ",")[0]
		
		// Determine category based on prefix or field name
		category := "general"
		if prefix != "" {
			category = prefix
		} else if strings.Contains(strings.ToLower(name), "schedule") {
			category = "schedule"
		} else if strings.Contains(strings.ToLower(name), "replic") {
			category = "replication"
		} else if strings.Contains(strings.ToLower(name), "log") {
			category = "log"
		} else if strings.Contains(strings.ToLower(name), "security") {
			category = "security"
		}
		
		// Create parameter
		parameter := PDParameter{
			Name:        name,
			Type:        field.Type().String(),
			Value:       field.Interface(),
			Description: fmt.Sprintf("Field %s from %s (toml: %s)", name, t.Name(), tomlName),
			Category:    category,
		}
		
		*parameters = append(*parameters, parameter)
	}
}