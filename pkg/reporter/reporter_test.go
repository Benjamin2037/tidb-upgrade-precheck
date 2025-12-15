package reporter

import (
	"os"
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGenerator(t *testing.T) {
	gen := NewGenerator()
	assert.NotNil(t, gen)
}

func TestGenerator_GenerateFromAnalysisResult(t *testing.T) {
	tests := []struct {
		name    string
		result  *analyzer.AnalysisResult
		options *Options
		wantErr bool
	}{
		{
			name: "text format",
			result: &analyzer.AnalysisResult{
				SourceVersion:       "v7.5.0",
				TargetVersion:       "v8.5.0",
				ModifiedParams:      make(map[string]map[string]analyzer.ModifiedParamInfo),
				TikvInconsistencies: make(map[string][]analyzer.InconsistentNode),
				UpgradeDifferences:  make(map[string]map[string]analyzer.UpgradeDifference),
				ForcedChanges:       make(map[string]map[string]analyzer.ForcedChange),
				CheckResults:        []rules.CheckResult{},
			},
			options: &Options{
				Format:    TextFormat,
				OutputDir: t.TempDir(),
				Filename:  "report", // Extension will be added automatically
			},
			wantErr: false,
		},
		{
			name: "markdown format",
			result: &analyzer.AnalysisResult{
				SourceVersion:       "v7.5.0",
				TargetVersion:       "v8.5.2",
				ModifiedParams:      make(map[string]map[string]analyzer.ModifiedParamInfo),
				TikvInconsistencies: make(map[string][]analyzer.InconsistentNode),
				UpgradeDifferences:  make(map[string]map[string]analyzer.UpgradeDifference),
				ForcedChanges:       make(map[string]map[string]analyzer.ForcedChange),
				CheckResults:        []rules.CheckResult{},
			},
			options: &Options{
				Format:    MarkdownFormat,
				OutputDir: t.TempDir(),
				Filename:  "report", // Extension will be added automatically
			},
			wantErr: false,
		},
		{
			name: "html format",
			result: &analyzer.AnalysisResult{
				SourceVersion:       "v7.5.0",
				TargetVersion:       "v8.5.0",
				ModifiedParams:      make(map[string]map[string]analyzer.ModifiedParamInfo),
				TikvInconsistencies: make(map[string][]analyzer.InconsistentNode),
				UpgradeDifferences:  make(map[string]map[string]analyzer.UpgradeDifference),
				ForcedChanges:       make(map[string]map[string]analyzer.ForcedChange),
				CheckResults:        []rules.CheckResult{},
			},
			options: &Options{
				Format:    HTMLFormat,
				OutputDir: t.TempDir(),
				Filename:  "report", // Extension will be added automatically
			},
			wantErr: false,
		},
		{
			name: "json format",
			result: &analyzer.AnalysisResult{
				SourceVersion:       "v7.5.0",
				TargetVersion:       "v8.5.0",
				ModifiedParams:      make(map[string]map[string]analyzer.ModifiedParamInfo),
				TikvInconsistencies: make(map[string][]analyzer.InconsistentNode),
				UpgradeDifferences:  make(map[string]map[string]analyzer.UpgradeDifference),
				ForcedChanges:       make(map[string]map[string]analyzer.ForcedChange),
				CheckResults:        []rules.CheckResult{},
			},
			options: &Options{
				Format:    JSONFormat,
				OutputDir: t.TempDir(),
				Filename:  "report", // Extension will be added automatically
			},
			wantErr: false,
		},
		{
			name: "invalid format",
			result: &analyzer.AnalysisResult{
				SourceVersion: "v7.5.0",
				TargetVersion: "v8.5.0",
			},
			options: &Options{
				Format:    Format("invalid"),
				OutputDir: t.TempDir(),
				Filename:  "report", // Don't include extension, it will be added automatically
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewGenerator()
			content, err := gen.GenerateFromAnalysisResult(tt.result, tt.options)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, content)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, content)

				// Verify file was created (content is the file path)
				_, statErr := os.Stat(content)
				assert.NoError(t, statErr)
			}
		})
	}
}

func TestGenerator_GenerateFromAnalysisResult_WithData(t *testing.T) {
	result := &analyzer.AnalysisResult{
		SourceVersion: "v7.5.0",
		TargetVersion: "v8.5.0",
		ModifiedParams: map[string]map[string]analyzer.ModifiedParamInfo{
			"tidb": {
				"max-connections": {
					Component:     "tidb",
					ParamName:     "max-connections",
					CurrentValue:  2000,
					SourceDefault: 1000,
					ParamType:     "config",
				},
			},
		},
		TikvInconsistencies: make(map[string][]analyzer.InconsistentNode),
		UpgradeDifferences:  make(map[string]map[string]analyzer.UpgradeDifference),
		ForcedChanges:       make(map[string]map[string]analyzer.ForcedChange),
		CheckResults: []rules.CheckResult{
			{
				RuleID:        "USER_MODIFIED_PARAMS",
				Category:      "user_modified",
				Component:     "tidb",
				ParameterName: "max-connections",
				Severity:      "info",
				Message:       "Parameter max-connections has been modified",
				Details:       "Current value: 2000, Source default: 1000",
				Suggestions:   []string{"Review parameter changes"},
			},
		},
	}

	options := &Options{
		Format:    TextFormat,
		OutputDir: t.TempDir(),
		Filename:  "report", // Extension will be added automatically
	}

	gen := NewGenerator()
	filePath, err := gen.GenerateFromAnalysisResult(result, options)

	require.NoError(t, err)
	assert.NotEmpty(t, filePath)

	// Read the file content
	fileContent, err := os.ReadFile(filePath)
	require.NoError(t, err)
	content := string(fileContent)

	assert.Contains(t, content, "v7.5.0")
	assert.Contains(t, content, "v8.5.0")
	assert.Contains(t, content, "max-connections")
}
