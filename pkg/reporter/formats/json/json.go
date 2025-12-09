package json

import (
	"encoding/json"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats"
)

// JSONFormatter handles JSON format rendering
type JSONFormatter struct{}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// Generate generates a complete JSON format report
// JSON format doesn't need header/footer/sections, just serialize the result
func (f *JSONFormatter) Generate(result *analyzer.AnalysisResult, options *formats.Options) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

