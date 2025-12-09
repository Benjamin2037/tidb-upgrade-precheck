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

package precheck

import (
	"context"
	"testing"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
	"github.com/stretchr/testify/assert"
)

func TestSimplePrecheckFlow(t *testing.T) {
	// Create a simple test snapshot
	snapshot := &runtime.ClusterSnapshot{
		Timestamp: time.Now(),
		Components: map[string]runtime.ComponentState{
			"tidb": {
				Type:    "tidb",
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"performance.max-procs": 0,
				},
				Variables: map[string]string{
					"tidb_enable_clustered_index": "INT_ONLY",
				},
				Status: make(map[string]interface{}),
			},
		},
	}

	// Create factory and analyzer
	factory := NewFactory()
	analyzer := factory.CreateDefaultAnalyzer()

	// Perform analysis
	ctx := context.Background()
	report, err := analyzer.Analyze(ctx, snapshot, "v7.0.0")
	
	// Verify no errors
	assert.NoError(t, err)
	
	// Verify report is not nil
	assert.NotNil(t, report)
	
	// Verify summary exists
	assert.NotNil(t, report.Summary)
}