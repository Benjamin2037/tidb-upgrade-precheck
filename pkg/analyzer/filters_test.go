package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldFilterParameter_ExactMatch(t *testing.T) {
	tests := []struct {
		name           string
		paramName      string
		shouldFilter   bool
		filterReason   string
	}{
		{
			name:         "exact match - host",
			paramName:    "host",
			shouldFilter: true,
			filterReason: "deployment-specific parameter (exact match)",
		},
		{
			name:         "exact match - data-dir",
			paramName:    "data-dir",
			shouldFilter: true,
			filterReason: "deployment-specific parameter (exact match)",
		},
		{
			name:         "exact match - log.file.filename",
			paramName:    "log.file.filename",
			shouldFilter: true,
			filterReason: "deployment-specific parameter (exact match)",
		},
		{
			name:         "not exact match",
			paramName:    "max-connections",
			shouldFilter: false,
			filterReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldFilter, filterReason := ShouldFilterParameter(tt.paramName)
			assert.Equal(t, tt.shouldFilter, shouldFilter)
			if tt.shouldFilter {
				assert.Contains(t, filterReason, tt.filterReason)
			}
		})
	}
}

func TestShouldFilterParameter_PathKeywords(t *testing.T) {
	tests := []struct {
		name         string
		paramName    string
		shouldFilter bool
	}{
		{
			name:         "contains path keyword",
			paramName:    "my-custom-path",
			shouldFilter: true,
		},
		{
			name:         "contains dir keyword",
			paramName:    "custom-dir-setting",
			shouldFilter: true,
		},
		{
			name:         "contains file keyword",
			paramName:    "my-file-config",
			shouldFilter: true,
		},
		{
			name:         "exception - log.level",
			paramName:    "log.level",
			shouldFilter: false, // Exception
		},
		{
			name:         "exception - raftdb.info-log-level",
			paramName:    "raftdb.info-log-level",
			shouldFilter: false, // Exception
		},
		{
			name:         "no path keyword",
			paramName:    "max-connections",
			shouldFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldFilter, _ := ShouldFilterParameter(tt.paramName)
			assert.Equal(t, tt.shouldFilter, shouldFilter)
		})
	}
}

func TestShouldFilterParameter_HostKeywords(t *testing.T) {
	tests := []struct {
		name         string
		paramName    string
		shouldFilter bool
	}{
		{
			name:         "exact host keyword",
			paramName:    "host",
			shouldFilter: true,
		},
		{
			name:         "prefix host keyword",
			paramName:    "hostname",
			shouldFilter: true,
		},
		{
			name:         "suffix host keyword",
			paramName:    "server.host",
			shouldFilter: true,
		},
		{
			name:         "contains port",
			paramName:    "server.port",
			shouldFilter: true,
		},
		{
			name:         "no host keyword",
			paramName:    "max-connections",
			shouldFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldFilter, _ := ShouldFilterParameter(tt.paramName)
			assert.Equal(t, tt.shouldFilter, shouldFilter)
		})
	}
}

func TestIsResourceDependentParameter(t *testing.T) {
	tests := []struct {
		name         string
		paramName    string
		isResource   bool
	}{
		{
			name:       "auto-tune parameter",
			paramName:  "auto-tune-threads",
			isResource: true,
		},
		{
			name:       "num-threads parameter",
			paramName:  "backup.num-threads",
			isResource: true,
		},
		{
			name:       "concurrency parameter",
			paramName:  "import.concurrency",
			isResource: true,
		},
		{
			name:       "region-max-size parameter",
			paramName:  "coprocessor.region-max-size",
			isResource: true,
		},
		{
			name:       "not resource dependent",
			paramName:  "max-connections",
			isResource: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isResource := IsResourceDependentParameter(tt.paramName)
			assert.Equal(t, tt.isResource, isResource)
		})
	}
}

func TestIsFilenameOnlyParameter(t *testing.T) {
	tests := []struct {
		name           string
		paramName      string
		isFilenameOnly bool
	}{
		{
			name:           "log.file.filename",
			paramName:      "log.file.filename",
			isFilenameOnly: true,
		},
		{
			name:           "log-file",
			paramName:      "log-file",
			isFilenameOnly: true,
		},
		{
			name:           "log.slow-query-file",
			paramName:      "log.slow-query-file",
			isFilenameOnly: true,
		},
		{
			name:           "not filename only",
			paramName:      "max-connections",
			isFilenameOnly: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isFilenameOnly := IsFilenameOnlyParameter(tt.paramName)
			assert.Equal(t, tt.isFilenameOnly, isFilenameOnly)
		})
	}
}

func TestGetIgnoredParamsMapForUpgradeDifferences(t *testing.T) {
	ignoredParams := GetIgnoredParamsMapForUpgradeDifferences()

	// Check some known parameters
	assert.True(t, ignoredParams["host"])
	assert.True(t, ignoredParams["data-dir"])
	assert.True(t, ignoredParams["log.file.filename"])
	assert.True(t, ignoredParams["version_compile_machine"])

	// Check non-ignored parameter
	assert.False(t, ignoredParams["max-connections"])
}

