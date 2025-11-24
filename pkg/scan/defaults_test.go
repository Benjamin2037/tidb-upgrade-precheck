package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Create a source file with some content
	srcFile := filepath.Join(tempDir, "source.txt")
	content := "test content for copy file function"
	err := os.WriteFile(srcFile, []byte(content), 0644)
	require.NoError(t, err)
	
	// Create destination file path
	dstFile := filepath.Join(tempDir, "destination.txt")
	
	// Test copyFile function
	err = copyFile(srcFile, dstFile)
	require.NoError(t, err)
	
	// Verify the destination file exists and has the same content
	dstContent, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	require.Equal(t, content, string(dstContent))
	
	// Test copying to a non-existent directory (should fail)
	nonExistentDir := filepath.Join(tempDir, "nonexistent", "file.txt")
	err = copyFile(srcFile, nonExistentDir)
	require.Error(t, err)
}

func TestSelectToolByVersion(t *testing.T) {
	testCases := []struct {
		version  string
		expected string
	}{
		{"v8.5.0", "export_defaults_v75plus.go"},
		{"v7.5.0", "export_defaults_v75plus.go"},
		{"v7.1.0", "export_defaults_v71.go"},
		{"v6.5.0", "export_defaults_v6.go"},
		{"v5.4.0", "export_defaults.go"},
		{"v4.0.0", "export_defaults.go"},
	}
	
	for _, tc := range testCases {
		result := selectToolByVersion(tc.version)
		require.Equal(t, tc.expected, result, "Failed for tag: %s", tc.version)
	}
}