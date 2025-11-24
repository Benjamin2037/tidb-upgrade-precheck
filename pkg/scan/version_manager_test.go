package scan

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionManager(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Test with non-existent file - use NewVersionManager which handles loading
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)
	require.Empty(t, vm.records)
	
	// Test adding a version
	tag := "v7.1.0"
	commitHash := "abc123def456"
	err = vm.RecordVersion(tag, commitHash)
	require.NoError(t, err)
	
	// Verify version was added
	info, exists := vm.GetVersionCommitHash(tag)
	require.True(t, exists)
	require.Equal(t, commitHash, info)
	
	// Test checking if version exists
	exists = vm.IsVersionGenerated(tag)
	require.True(t, exists)
	
	// Test with non-existent version
	exists = vm.IsVersionGenerated("v99.99.99")
	require.False(t, exists)
	
	// Test saving and loading by creating a new version manager
	vm2, err := NewVersionManager(tempDir)
	require.NoError(t, err)
	
	// Verify data was loaded correctly
	info, exists = vm2.GetVersionCommitHash(tag)
	require.True(t, exists)
	require.Equal(t, commitHash, info)
}

func TestVersionManagerFileOperations(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Test with directory that doesn't exist
	_, err := NewVersionManager(filepath.Join(tempDir, "nonexistent"))
	require.NoError(t, err)
}

func TestVersionManagerGetGeneratedVersions(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)
	
	// Add multiple versions
	versions := []struct {
		tag        string
		commitHash string
	}{
		{"v7.1.0", "abc123"},
		{"v8.1.0", "def456"},
		{"v8.5.0", "ghi789"},
	}
	
	for _, v := range versions {
		err := vm.RecordVersion(v.tag, v.commitHash)
		require.NoError(t, err)
	}
	
	// Get generated versions
	generated := vm.GetGeneratedVersions()
	require.Len(t, generated, 3)
	
	// Verify all versions are present
	versionMap := make(map[string]string)
	for _, record := range generated {
		versionMap[record.Tag] = record.CommitHash
	}
	
	for _, v := range versions {
		require.Equal(t, v.commitHash, versionMap[v.tag])
	}
}

func TestVersionManagerRemoveVersion(t *testing.T) {
	tempDir := t.TempDir()
	vm, err := NewVersionManager(tempDir)
	require.NoError(t, err)
	
	// Add a version
	tag := "v7.1.0"
	commitHash := "abc123"
	err = vm.RecordVersion(tag, commitHash)
	require.NoError(t, err)
	
	// Verify it exists
	exists := vm.IsVersionGenerated(tag)
	require.True(t, exists)
	
	// Remove the version
	err = vm.RemoveVersion(tag)
	require.NoError(t, err)
	
	// Verify it no longer exists
	exists = vm.IsVersionGenerated(tag)
	require.False(t, exists)
	
	// Try to remove a non-existent version (should not error)
	err = vm.RemoveVersion("v99.99.99")
	require.NoError(t, err)
}