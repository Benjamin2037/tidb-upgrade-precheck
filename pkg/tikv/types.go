// Copyright 2024 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package tikv

// Severity represents the severity level of a configuration issue
type Severity string

const (
	// High represents a high severity issue
	High Severity = "high"
	// Medium represents a medium severity issue
	Medium Severity = "medium"
	// Low represents a low severity issue
	Low Severity = "low"
)

// ClusterConfigReport represents a report of TiKV cluster configuration issues
type ClusterConfigReport struct {
	// ParamName is the name of the parameter with the issue
	ParamName string `json:"param_name"`
	// Severity is the severity level of the issue
	Severity Severity `json:"severity"`
	// Message is a description of the issue
	Message string `json:"message"`
	// Values are the values of the parameter across instances
	Values []ConfigValue `json:"values"`
	// References are references to documentation or other resources
	References []string `json:"references"`
}

// ConfigValue represents a configuration value on a specific instance
type ConfigValue struct {
	// InstanceAddress is the address of the instance
	InstanceAddress string `json:"instance_address"`
	// Value is the value of the parameter
	Value interface{} `json:"value"`
}

// ComponentState represents the state of a TiKV component
type ComponentState struct {
	// Type is the type of the component (tikv)
	Type string `json:"type"`
	
	// Version is the version of the component
	Version string `json:"version"`
	
	// Config is the configuration of the component
	Config map[string]interface{} `json:"config"`
	
	// Status is the status information of the component
	Status map[string]interface{} `json:"status"`
}

// InstanceState represents the state of a TiKV instance
type InstanceState struct {
	// Address is the address of the TiKV instance
	Address string `json:"address"`
	
	// State is the state of the instance
	State ComponentState `json:"state"`
}

// ClusterState represents the state of a TiKV cluster
type ClusterState struct {
	// Instances is the list of TiKV instances
	Instances []InstanceState `json:"instances"`
	
	// InconsistentConfigs are the configurations that are inconsistent across instances
	InconsistentConfigs []InconsistentConfig `json:"inconsistent_configs"`
}

// InconsistentConfig represents a configuration that is inconsistent across instances
type InconsistentConfig struct {
	// ParameterName is the name of the parameter
	ParameterName string `json:"parameter_name"`
	
	// Values are the different values of the parameter across instances
	Values []ParameterValue `json:"values"`
	
	// RiskLevel is the risk level of the inconsistency
	RiskLevel string `json:"risk_level"`
	
	// Description is a description of the inconsistency
	Description string `json:"description"`
}

// ParameterValue represents a parameter value on a specific instance
type ParameterValue struct {
	// InstanceAddress is the address of the instance
	InstanceAddress string `json:"instance_address"`
	
	// Value is the value of the parameter
	Value interface{} `json:"value"`
}