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
	"testing"
)

func TestFactory_CreateCollector(t *testing.T) {
	factory := NewFactory("/fake/path")
	
	collector := factory.CreateCollector()
	if collector == nil {
		t.Error("Expected collector to be created")
	}
}

func TestFactory_CreateComparator(t *testing.T) {
	factory := NewFactory("/fake/path")
	
	comparator := factory.CreateComparator()
	if comparator == nil {
		t.Error("Expected comparator to be created")
	}
}

func TestFactory_CreateAnalyzer(t *testing.T) {
	factory := NewFactory("/fake/path")
	
	analyzer := factory.CreateAnalyzer()
	if analyzer == nil {
		t.Error("Expected analyzer to be created")
	}
}