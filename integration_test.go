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

package main_test

import (
	"context"
	"testing"
	"time"

	precheckEngine "github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
	precheckReport "github.com/pingcap/tidb-upgrade-precheck/pkg/report"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
	"github.com/stretchr/testify/assert"
)

func TestFullPrecheckFlow(t *testing.T) {
	// 1. Create a mock cluster snapshot
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

	// 2. Create analyzer and perform analysis
	factory := precheckEngine.NewFactory()
	analyzer := factory.CreateDefaultAnalyzer()

	report, err := analyzer.Analyze(context.Background(), snapshot, "v7.0.0")
	assert.NoError(t, err)
	assert.NotNil(t, report)

	// 3. Generate report
	reportGenerator := precheckReport.NewGenerator()
	options := &precheckReport.Options{
		Format:    precheckReport.TextFormat,
		OutputDir: t.TempDir(),
		Filename:  "test-report",
	}

	reportPath, err := reportGenerator.Generate(report, options)
	assert.NoError(t, err)
	assert.NotEmpty(t, reportPath)
}