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

import (
	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

// ClusterAnalyzer analyzes TiKV configurations at the cluster level
type ClusterAnalyzer struct {
}

// NewClusterAnalyzer creates a new ClusterAnalyzer
func NewClusterAnalyzer() *ClusterAnalyzer {
	return &ClusterAnalyzer{}
}

// AnalyzeCluster checks for configuration inconsistencies across TiKV instances in a cluster
func (ca *ClusterAnalyzer) AnalyzeCluster(instances []runtime.InstanceState) []*ClusterConfigReport {
	var reports []*ClusterConfigReport

	// Filter to only TiKV instances
	tikvInstances := []runtime.InstanceState{}
	for _, instance := range instances {
		if instance.State.Type == runtime.TiKVComponent {
			tikvInstances = append(tikvInstances, instance)
		}
	}

	// If we have less than 2 TiKV instances, there can't be inconsistencies
	if len(tikvInstances) < 2 {
		return reports
	}

	// Group configurations by parameter name
	configMap := make(map[string][]ConfigValue)
	for _, instance := range tikvInstances {
		for paramName, paramValue := range instance.State.Config {
			configVal := ConfigValue{
				InstanceAddress: instance.Address,
				Value:           paramValue,
			}
			configMap[paramName] = append(configMap[paramName], configVal)
		}
	}

	// Check each parameter for inconsistencies
	for paramName, values := range configMap {
		// If not all instances have this parameter, it's inconsistent
		if len(values) != len(tikvInstances) {
			report := &ClusterConfigReport{
				ParamName:  paramName,
				Severity:   High,
				Message:    "Parameter not configured on all instances",
				Values:     values,
				References: []string{}, // Would link to documentation in a real implementation
			}
			reports = append(reports, report)
			continue
		}

		// Check if all values are the same
		firstValue := values[0].Value
		isConsistent := true
		for _, value := range values[1:] {
			if firstValue != value.Value {
				isConsistent = false
				break
			}
		}

		// If values differ, create a report
		if !isConsistent {
			report := &ClusterConfigReport{
				ParamName:  paramName,
				Severity:   ca.determineSeverity(paramName),
				Message:    "Inconsistent values across instances",
				Values:     values,
				References: []string{}, // Would link to documentation in a real implementation
			}
			reports = append(reports, report)
		}
	}

	return reports
}

// determineSeverity determines the severity level for an inconsistent parameter
func (ca *ClusterAnalyzer) determineSeverity(paramName string) Severity {
	// High severity parameters that must be consistent
	highSeverityParams := map[string]bool{
		"storage.reserve-space":          true,
		"raftstore.raft-entry-max-size":  true,
		"rocksdb.defaultcf.block-size":   true,
		"server.grpc-concurrency":        true,
	}

	if highSeverityParams[paramName] {
		return High
	}

	// Medium severity parameters
	mediumSeverityParams := map[string]bool{
		"rocksdb.defaultcf.block-cache-size": true,
		"readpool.storage.use-unified-pool":  true,
		"log.level":                          true,
	}

	if mediumSeverityParams[paramName] {
		return Medium
	}

	// Low severity for everything else
	return Low
}

// AnalyzeScaleOut evaluates configuration consistency when scaling out a TiKV cluster
func (ca *ClusterAnalyzer) AnalyzeScaleOut(existingInstances []runtime.InstanceState, newInstances []runtime.InstanceState) []*ClusterConfigReport {
	// Combine existing and new instances
	allInstances := append(existingInstances, newInstances...)
	
	// Use the standard cluster analysis
	return ca.AnalyzeCluster(allInstances)
}

// AnalyzeScaleIn evaluates the impact of scaling in a TiKV cluster
func (ca *ClusterAnalyzer) AnalyzeScaleIn(remainingInstances []runtime.InstanceState, removedInstanceIDs []string) []*ClusterConfigReport {
	// For scale-in analysis, we just perform standard cluster analysis
	// A more sophisticated implementation might check if removed instances had unique configs
	return ca.AnalyzeCluster(remainingInstances)
}