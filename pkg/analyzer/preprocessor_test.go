package analyzer

import (
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestPreprocessParameters_FilterPathParameters(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	snapshot := &collector.ClusterSnapshot{
		Components: map[string]collector.ComponentState{
			"tidb": {
				Config: types.ConfigDefaults{
					"data-dir": types.ParameterValue{Value: "/data/tidb", Type: "string"},
					"max-connections": types.ParameterValue{Value: 1000, Type: "int"},
				},
			},
		},
	}

	sourceDefaults := map[string]map[string]interface{}{
		"tidb": {
			"data-dir":        "/data/tidb",
			"max-connections": 1000,
		},
	}

	targetDefaults := map[string]map[string]interface{}{
		"tidb": {
			"data-dir":        "/data/tidb",
			"max-connections": 2000,
		},
	}

	preprocessedResults, cleanedSourceDefaults, cleanedTargetDefaults := analyzer.preprocessParameters(
		snapshot,
		"v7.5.0", "v8.0.0",
		sourceDefaults, targetDefaults,
		nil, nil,
		0, 0,
	)

	// data-dir should be filtered (path parameter)
	assert.NotContains(t, cleanedSourceDefaults["tidb"], "data-dir")
	assert.NotContains(t, cleanedTargetDefaults["tidb"], "data-dir")

	// max-connections should remain (not a path parameter)
	assert.Contains(t, cleanedSourceDefaults["tidb"], "max-connections")
	assert.Contains(t, cleanedTargetDefaults["tidb"], "max-connections")

	// Check that filtered parameter has a CheckResult
	filteredFound := false
	for _, result := range preprocessedResults {
		if result.ParameterName == "data-dir" {
			filteredFound = true
			assert.Equal(t, "filtered", result.Category)
			assert.Equal(t, "PARAMETER_PREPROCESSOR", result.RuleID)
		}
	}
	assert.True(t, filteredFound, "Filtered parameter should have a CheckResult")
}

func TestPreprocessParameters_FilterIdenticalValues(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	snapshot := &collector.ClusterSnapshot{
		Components: map[string]collector.ComponentState{
			"tidb": {
				Config: types.ConfigDefaults{
					"max-connections": types.ParameterValue{Value: 1000, Type: "int"},
				},
			},
		},
	}

	sourceDefaults := map[string]map[string]interface{}{
		"tidb": {
			"max-connections": 1000,
		},
	}

	targetDefaults := map[string]map[string]interface{}{
		"tidb": {
			"max-connections": 1000, // Same as source and current
		},
	}

	preprocessedResults, cleanedSourceDefaults, cleanedTargetDefaults := analyzer.preprocessParameters(
		snapshot,
		"v7.5.0", "v8.0.0",
		sourceDefaults, targetDefaults,
		nil, nil,
		0, 0,
	)

	// max-connections should be filtered (all values identical)
	assert.NotContains(t, cleanedSourceDefaults["tidb"], "max-connections")
	assert.NotContains(t, cleanedTargetDefaults["tidb"], "max-connections")

	// Check that filtered parameter has a CheckResult
	filteredFound := false
	for _, result := range preprocessedResults {
		if result.ParameterName == "max-connections" {
			filteredFound = true
			assert.Equal(t, "filtered", result.Category)
			assert.Contains(t, result.Details, "all values identical")
		}
	}
	assert.True(t, filteredFound, "Identical parameter should have a CheckResult")
}

func TestPreprocessParameters_KeepDifferentValues(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	snapshot := &collector.ClusterSnapshot{
		Components: map[string]collector.ComponentState{
			"tidb": {
				Config: types.ConfigDefaults{
					"max-connections": types.ParameterValue{Value: 1000, Type: "int"},
				},
			},
		},
	}

	sourceDefaults := map[string]map[string]interface{}{
		"tidb": {
			"max-connections": 1000,
		},
	}

	targetDefaults := map[string]map[string]interface{}{
		"tidb": {
			"max-connections": 2000, // Different from source
		},
	}

	preprocessedResults, cleanedSourceDefaults, cleanedTargetDefaults := analyzer.preprocessParameters(
		snapshot,
		"v7.5.0", "v8.0.0",
		sourceDefaults, targetDefaults,
		nil, nil,
		0, 0,
	)

	// max-connections should remain (values differ)
	assert.Contains(t, cleanedSourceDefaults["tidb"], "max-connections")
	assert.Contains(t, cleanedTargetDefaults["tidb"], "max-connections")

	// Should not have a filtered result for this parameter
	for _, result := range preprocessedResults {
		if result.ParameterName == "max-connections" {
			t.Fatal("max-connections should not be filtered")
		}
	}
}

func TestPreprocessParameters_FilterResourceDependent(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	snapshot := &collector.ClusterSnapshot{
		Components: map[string]collector.ComponentState{
			"tikv": {
				Config: types.ConfigDefaults{
					"backup.num-threads": types.ParameterValue{Value: 8, Type: "int"}, // Different from default (auto-tuned)
				},
			},
		},
	}

	sourceDefaults := map[string]map[string]interface{}{
		"tikv": {
			"backup.num-threads": 4, // Source default
		},
	}

	targetDefaults := map[string]map[string]interface{}{
		"tikv": {
			"backup.num-threads": 4, // Same as source (resource-dependent)
		},
	}

	preprocessedResults, cleanedSourceDefaults, cleanedTargetDefaults := analyzer.preprocessParameters(
		snapshot,
		"v7.5.0", "v8.0.0",
		sourceDefaults, targetDefaults,
		nil, nil,
		0, 0,
	)

	// backup.num-threads should be filtered (resource-dependent, source == target, but current differs)
	assert.NotContains(t, cleanedSourceDefaults["tikv"], "backup.num-threads")
	assert.NotContains(t, cleanedTargetDefaults["tikv"], "backup.num-threads")

	// Check that filtered parameter has a CheckResult
	filteredFound := false
	for _, result := range preprocessedResults {
		if result.ParameterName == "backup.num-threads" {
			filteredFound = true
			assert.Equal(t, "filtered", result.Category)
			assert.Contains(t, result.Details, "resource-dependent")
		}
	}
	assert.True(t, filteredFound, "Resource-dependent parameter should have a CheckResult")
}

func TestPreprocessParameters_SystemVariables(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	snapshot := &collector.ClusterSnapshot{
		Components: map[string]collector.ComponentState{
			"tidb": {
				Variables: types.SystemVariables{
					"tidb_max_connections": types.ParameterValue{Value: 1000, Type: "int"},
					"system_time_zone":     types.ParameterValue{Value: "UTC", Type: "string"},
				},
			},
		},
	}

	sourceDefaults := map[string]map[string]interface{}{
		"tidb": {
			"sysvar:tidb_max_connections": 1000,
			"sysvar:system_time_zone":     "UTC",
		},
	}

	targetDefaults := map[string]map[string]interface{}{
		"tidb": {
			"sysvar:tidb_max_connections": 2000,
			"sysvar:system_time_zone":     "UTC",
		},
	}

	preprocessedResults, cleanedSourceDefaults, cleanedTargetDefaults := analyzer.preprocessParameters(
		snapshot,
		"v7.5.0", "v8.0.0",
		sourceDefaults, targetDefaults,
		nil, nil,
		0, 0,
	)

	// system_time_zone should be filtered (deployment-specific)
	assert.NotContains(t, cleanedSourceDefaults["tidb"], "sysvar:system_time_zone")
	assert.NotContains(t, cleanedTargetDefaults["tidb"], "sysvar:system_time_zone")

	// tidb_max_connections should remain (not filtered)
	assert.Contains(t, cleanedSourceDefaults["tidb"], "sysvar:tidb_max_connections")
	assert.Contains(t, cleanedTargetDefaults["tidb"], "sysvar:tidb_max_connections")
}

func TestPreprocessParameters_NewParameters(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	snapshot := &collector.ClusterSnapshot{
		Components: map[string]collector.ComponentState{
			"tidb": {
				Config: types.ConfigDefaults{
					"new-param": types.ParameterValue{Value: 100, Type: "int"},
				},
			},
		},
	}

	sourceDefaults := map[string]map[string]interface{}{
		"tidb": {},
	}

	targetDefaults := map[string]map[string]interface{}{
		"tidb": {
			"new-param": 100, // New parameter, current == target
		},
	}

	preprocessedResults, cleanedSourceDefaults, cleanedTargetDefaults := analyzer.preprocessParameters(
		snapshot,
		"v7.5.0", "v8.0.0",
		sourceDefaults, targetDefaults,
		nil, nil,
		0, 0,
	)

	// new-param should be filtered (new parameter, current == target)
	assert.NotContains(t, cleanedTargetDefaults["tidb"], "new-param")

	// Check that filtered parameter has a CheckResult
	filteredFound := false
	for _, result := range preprocessedResults {
		if result.ParameterName == "new-param" {
			filteredFound = true
			assert.Equal(t, "filtered", result.Category)
			assert.Contains(t, result.Metadata, "is_new_param")
		}
	}
	assert.True(t, filteredFound, "New parameter with current == target should be filtered")
}

func TestPreprocessParameters_ReturnsCheckResults(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	snapshot := &collector.ClusterSnapshot{
		Components: map[string]collector.ComponentState{
			"tidb": {
				Config: types.ConfigDefaults{
					"data-dir": types.ParameterValue{Value: "/data/tidb", Type: "string"},
				},
			},
		},
	}

	sourceDefaults := map[string]map[string]interface{}{
		"tidb": {
			"data-dir": "/data/tidb",
		},
	}

	targetDefaults := map[string]map[string]interface{}{
		"tidb": {
			"data-dir": "/data/tidb",
		},
	}

	preprocessedResults, _, _ := analyzer.preprocessParameters(
		snapshot,
		"v7.5.0", "v8.0.0",
		sourceDefaults, targetDefaults,
		nil, nil,
		0, 0,
	)

	// Should have CheckResults for filtered parameters
	assert.NotEmpty(t, preprocessedResults)

	// Check result structure
	for _, result := range preprocessedResults {
		assert.Equal(t, "PARAMETER_PREPROCESSOR", result.RuleID)
		assert.Equal(t, "filtered", result.Category)
		assert.NotEmpty(t, result.ParameterName)
		assert.NotEmpty(t, result.Component)
		assert.Contains(t, result.Metadata, "filtered")
		assert.Contains(t, result.Metadata, "filter_reason")
	}
}

