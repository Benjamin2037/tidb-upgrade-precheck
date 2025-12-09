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

// Factory is responsible for creating TiKV components
type Factory struct {
	sourcePath string
}

// NewFactory creates a new TiKV factory
func NewFactory(sourcePath string) *Factory {
	return &Factory{
		sourcePath: sourcePath,
	}
}

// CreateCollector creates a new TiKV collector
func (f *Factory) CreateCollector() *Collector {
	return NewCollector(f.sourcePath)
}

// CreateComparator creates a new TiKV comparator
func (f *Factory) CreateComparator() *Comparator {
	return NewComparator(f.sourcePath)
}

// CreateAnalyzer creates a new TiKV analyzer
func (f *Factory) CreateAnalyzer() *Analyzer {
	return NewAnalyzer(f.sourcePath)
}