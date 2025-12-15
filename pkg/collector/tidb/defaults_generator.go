// Package tidb provides helper functions for extracting bootstrap version from source code
// Knowledge base generation now uses tiup playground to collect runtime configuration directly,
// so code extraction functions have been removed. Only bootstrap version extraction remains.
package tidb

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// extractVersionNumber extracts version number from version tag
func extractVersionNumber(version string) string {
	version = strings.TrimPrefix(version, "v")
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		return parts[0] + parts[1]
	}
	return version
}

// parseVersion parses version number into major and minor
func parseVersion(versionNum string) (int, int) {
	if len(versionNum) >= 2 {
		if major, err := strconv.Atoi(versionNum[0:1]); err == nil {
			if minor, err := strconv.Atoi(versionNum[1:]); err == nil {
				return major, minor
			}
		}
	}
	return 0, 0
}

// ExtractBootstrapVersion extracts bootstrap version from TiDB source code (exported for use by update scripts)
// Handles version differences in file paths
// Supports both direct assignment (currentBootstrapVersion = 123) and constant assignment (currentBootstrapVersion = version109)
func ExtractBootstrapVersion(tidbRoot, version string) int64 {
	return extractBootstrapVersion(tidbRoot, version)
}

// extractBootstrapVersion extracts bootstrap version from TiDB source code
// The currentBootstrapVersion is defined in:
//   - pkg/session/upgrade.go (or session/upgrade.go) for newer versions
//   - pkg/session/bootstrap.go (or session/bootstrap.go) for older versions (e.g., v6.5.0)
// It's defined as: var currentBootstrapVersion int64 = versionXXX
// We need to find this assignment and resolve the versionXXX constant to its numeric value
// IMPORTANT: This function will checkout the TiDB repository to the specified version before extraction
func extractBootstrapVersion(tidbRoot, version string) int64 {
	// First, ensure TiDB repository is checked out to the correct version
	// Save current branch/commit to restore later
	originalRef := getCurrentGitRef(tidbRoot)
	
	// Checkout to the target version
	if err := checkoutGitVersion(tidbRoot, version); err != nil {
		fmt.Printf("Warning: Failed to checkout TiDB repository to %s: %v\n", version, err)
		// Continue anyway, maybe the repository is already at the correct version
	}
	
	// Restore original branch/commit after extraction
	defer func() {
		if originalRef != "" {
			if err := restoreGitRef(tidbRoot, originalRef); err != nil {
				fmt.Printf("Warning: Failed to restore TiDB repository to %s: %v\n", originalRef, err)
			}
		}
	}()
	
	versionNum := extractVersionNumber(version)
	major, minor := parseVersion(versionNum)

	var possiblePaths []string

	// TiDB 7.5+ uses pkg/ directory structure
	// v7.5+ uses pkg/session/upgrade.go
	// Older versions (e.g., v6.5.0) may have currentBootstrapVersion in bootstrap.go instead of upgrade.go
	if major > 7 || (major == 7 && minor >= 5) {
		possiblePaths = []string{
			filepath.Join(tidbRoot, "pkg", "session", "upgrade.go"),
			filepath.Join(tidbRoot, "pkg", "session", "bootstrap.go"), // Fallback for older versions
		}
	} else {
		// TiDB < 7.5 (including v6.5.x)
		possiblePaths = []string{
			filepath.Join(tidbRoot, "session", "upgrade.go"),
			filepath.Join(tidbRoot, "session", "bootstrap.go"), // Fallback for older versions (e.g., v6.5.0)
		}
	}

	for _, path := range possiblePaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		content := string(data)

		// First, try direct assignment: currentBootstrapVersion = 123
		re := regexp.MustCompile(`currentBootstrapVersion\s*=\s*(\d+)`)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			if version, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
				return version
			}
		}

		// Second, try constant assignment: var currentBootstrapVersion int64 = version109
		// Find the assignment first (with optional var and type declaration)
		constRe := regexp.MustCompile(`(?:var\s+)?currentBootstrapVersion(?:\s+\w+)?\s*=\s*version(\d+)`)
		constMatches := constRe.FindStringSubmatch(content)
		if len(constMatches) > 1 {
			// Found assignment like: var currentBootstrapVersion int64 = version253
			// Now find the constant definition: version253 = 253
			constName := "version" + constMatches[1]
			constDefRe := regexp.MustCompile(fmt.Sprintf(`%s\s*=\s*(\d+)`, regexp.QuoteMeta(constName)))
			constDefMatches := constDefRe.FindStringSubmatch(content)
			if len(constDefMatches) > 1 {
				if version, err := strconv.ParseInt(constDefMatches[1], 10, 64); err == nil {
					return version
				}
			}
		}

		// Third, try to find the constant value directly by searching for version constants
		// Look for pattern: versionXXX = YYY where YYY is the bootstrap version
		// We'll find the assignment to currentBootstrapVersion and resolve the constant
		versionConstRe := regexp.MustCompile(`version(\d+)\s*=\s*(\d+)`)
		allMatches := versionConstRe.FindAllStringSubmatch(content, -1)
		if len(allMatches) > 0 {
			// Find the assignment to currentBootstrapVersion (with optional var and type)
			assignRe := regexp.MustCompile(`(?:var\s+)?currentBootstrapVersion(?:\s+\w+)?\s*=\s*version(\d+)`)
			assignMatches := assignRe.FindStringSubmatch(content)
			if len(assignMatches) > 1 {
				targetVersion := assignMatches[1]
				// Find the constant definition for this version
				for _, match := range allMatches {
					if match[1] == targetVersion {
						if version, err := strconv.ParseInt(match[2], 10, 64); err == nil {
							return version
						}
					}
				}
			}
		}
	}

	return 0
}

// getCurrentGitRef gets the current git reference (branch or commit) of the repository
func getCurrentGitRef(repoRoot string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		// If not a branch, try to get commit hash
		cmd = exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = repoRoot
		output, err = cmd.Output()
		if err != nil {
			return ""
		}
	}
	return strings.TrimSpace(string(output))
}

// checkoutGitVersion checks out the repository to the specified version tag
func checkoutGitVersion(repoRoot, version string) error {
	// Check if it's a valid git repository
	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", repoRoot)
	}
	
	// Checkout to the version tag
	cmd := exec.Command("git", "checkout", version)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// restoreGitRef restores the repository to the original reference
func restoreGitRef(repoRoot, ref string) error {
	cmd := exec.Command("git", "checkout", ref)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
