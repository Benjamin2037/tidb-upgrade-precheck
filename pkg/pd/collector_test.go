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

package pd

import (
	"testing"
	
	"github.com/stretchr/testify/assert"
)

func TestCollector_Collect(t *testing.T) {
	collector := NewCollector("")
	
	params, err := collector.Collect("v7.1.0")
	assert.NoError(t, err)
	assert.NotNil(t, params)
	
	assert.Equal(t, "v7.1.0", params.Version)
	assert.NotEmpty(t, params.Parameters)
}

func TestCollector_GetParameter(t *testing.T) {
	collector := NewCollector("")
	
	param, err := collector.GetParameter("v7.1.0", "schedule.max-store-down-time")
	assert.NoError(t, err)
	assert.NotNil(t, param)
	
	assert.Equal(t, "schedule.max-store-down-time", param.Name)
	assert.Equal(t, "duration", param.Type)
}

func TestCollector_ListSupportedVersions(t *testing.T) {
	collector := NewCollector("")
	
	versions, err := collector.ListSupportedVersions()
	assert.NoError(t, err)
	assert.NotEmpty(t, versions)
	
	// Check that we have some expected versions
	assert.Contains(t, versions, "v6.5.0")
	assert.Contains(t, versions, "v7.1.0")
}

func TestCategorizeParameter(t *testing.T) {
	testCases := []struct {
		paramName string
		expected  string
	}{
		{"schedule.max-store-down-time", "schedule"},
		{"replication.location-labels", "replication"},
		{"security.cacert-path", "security"},
		{"log.level", "log"},
		{"metric.address", "metric"},
		{"lease.duration", "lease"},
		{"unknown.param", "other"},
	}
	
	for _, tc := range testCases {
		category := categorizeParameter(tc.paramName)
		assert.Equal(t, tc.expected, category, "Parameter: %s", tc.paramName)
	}
}