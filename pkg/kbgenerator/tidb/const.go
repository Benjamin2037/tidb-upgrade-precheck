package tidb

import (
	"strconv"
	"strings"
)

// RequiredFilesForSparseCheckout returns the list of file paths required for TiDB knowledge base generation
// These files are used for sparse checkout to minimize download time
// If version is empty or "all", returns all possible file paths (for compatibility)
// If version is specified (e.g., "v7.1.0"), returns version-specific file paths
// Users can modify this list to add or remove files as needed
func RequiredFilesForSparseCheckout(version string) []string {
	// If version is empty or "all", return all possible paths (for initial clone)
	if version == "" || version == "all" {
		return []string{
			// Config files (TiDB 7.1+ uses pkg/ directory, older versions don't)
			"pkg/config/config.go",
			"config/config.go",
			// System variable files
			"pkg/sessionctx/variable/sysvar.go",
			"sessionctx/variable/sysvar.go",
			// Vardef directory (contains system variable definitions)
			"pkg/sessionctx/vardef",
			"sessionctx/vardef",
			// Upgrade logic files
			"pkg/session/upgrade.go",
			"pkg/session/bootstrap.go",
			"session/upgrade.go",
			"session/bootstrap.go",
			// Session files (for bootstrap version extraction)
			"pkg/session/session.go",
			"session/session.go",
		}
	}

	// Parse version to determine which paths to use
	major, minor := parseVersionTag(version)

	// TiDB 7.1+ uses pkg/ directory structure
	if major > 7 || (major == 7 && minor >= 1) {
		return []string{
			// Config files (TiDB 7.1+)
			"pkg/config/config.go",
			// System variable files
			"pkg/sessionctx/variable/sysvar.go",
			// Vardef directory
			"pkg/sessionctx/vardef",
			// Upgrade logic files
			"pkg/session/upgrade.go",
			"pkg/session/bootstrap.go",
			// Session files
			"pkg/session/session.go",
		}
	}

	// TiDB < 7.1 uses sessionctx/ directory (no pkg/)
	return []string{
		// Config files (TiDB < 7.1)
		"config/config.go",
		// System variable files
		"sessionctx/variable/sysvar.go",
		// Vardef directory
		"sessionctx/vardef",
		// Upgrade logic files
		"session/upgrade.go",
		"session/bootstrap.go",
		// Session files
		"session/session.go",
	}
}

// parseVersionTag parses version tag (e.g., "v7.1.0") into major and minor version numbers
func parseVersionTag(version string) (int, int) {
	// Remove "v" prefix if present
	version = strings.TrimPrefix(version, "v")

	// Split by "."
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, 0
	}

	// Parse major version
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0
	}

	// Parse minor version
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0
	}

	return major, minor
}
