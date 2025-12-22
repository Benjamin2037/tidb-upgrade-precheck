package analyzer

import (
	"strings"
)

// FilterConfig contains all parameters that should be filtered during preprocessing
// This centralizes all filtering logic in one place
type FilterConfig struct {
	// ExactMatchParams are parameters that should be filtered by exact name match
	ExactMatchParams map[string]bool

	// PathKeywords are keywords that indicate path-related parameters (checked via Contains)
	PathKeywords []string

	// HostKeywords are keywords that indicate host/network-related parameters
	HostKeywords []string

	// ResourceDependentKeywords are keywords that indicate resource-dependent parameters
	ResourceDependentKeywords []string

	// Exceptions are parameters that contain path keywords but should NOT be filtered
	Exceptions map[string]bool

	// FilenameOnlyParams are parameters that should be compared by filename only (not filtered, but special comparison)
	FilenameOnlyParams map[string]bool
}

// globalFilterConfig is the centralized filter configuration
var globalFilterConfig = &FilterConfig{
	// Exact match parameters (deployment-specific, platform info, timezone, etc.)
	ExactMatchParams: map[string]bool{
		// Deployment-specific host/network parameters
		"host":     true,
		"hostname": true,
		"port":     true,
		"addr":     true,
		"address":  true,

		// Deployment-specific path parameters (TiDB)
		"path":                 true,
		"socket":               true,
		"temp-dir":             true,
		"tmp-storage-path":     true,
		"log.file.filename":    true,
		"log.slow-query-file":  true,
		"log.file.max-size":    true,
		"log.file.max-days":    true,
		"log.file.max-backups": true,
		"log-file":             true,
		"log-dir":              true,
		"log_dir":              true,

		// Deployment-specific path parameters (TiKV)
		"data-dir":             true,
		"data_dir":             true,
		"deploy-dir":           true,
		"deploy_dir":           true,
		"log-backup.temp-path": true,
		"backup.temp-path":     true,
		"temp-path":            true,
		"temp_path":            true,

		// Deployment-specific path parameters (TiFlash)
		"tmp_path":           true,
		"storage.main.dir":   true,
		"storage.latest.dir": true,
		"storage.raft.dir":   true,

		// Compile-time platform information (not user-configurable)
		"version_compile_machine": true,
		"version_compile_os":      true,

		// Timezone-related parameters (deployment-specific, environment-dependent)
		"system_time_zone": true,
		"time_zone":        true,

		// Deployment-specific network/connection parameters
		"pd.endpoints": true,

		// Other parameters to ignore
		"deprecate-integer-display-length": true,
	},

	// Path keywords (checked via Contains)
	PathKeywords: []string{
		"path", "dir", "file", "log", "data", "deploy", "temp", "tmp",
		"storage", "socket", "home", "root", "cache", "config",
		"filename", "file-name", "file_name",
		"log-file", "log-dir", "log_file", "log_dir",
		"data-dir", "data_dir", "deploy-dir", "deploy_dir",
		"temp-path", "temp_path", "tmp-path", "tmp_path",
	},

	// Host/network keywords
	HostKeywords: []string{
		"host", "hostname", "addr", "address", "port",
	},

	// Resource-dependent keywords (auto-tuned by system)
	ResourceDependentKeywords: []string{
		"auto-tune", "auto_tune",
		"num-threads", "num_threads", "thread-count", "thread_count", "threads", "concurrency",
		"region-max-size", "region-max-keys", "region-split-size", "region-split-keys",
		"sst-max-size",
		"batch-compression-threshold", "blob-file-compression",
	},

	// Exceptions: Parameters that contain path keywords but are NOT path parameters
	Exceptions: map[string]bool{
		// RaftDB info-log configuration parameters
		"raftdb.info-log-keep-log-file-num": true,
		"raftdb.info-log-level":             true,
		"raftdb.info-log-max-size":          true,
		"raftdb.info-log-roll-time":         true,
		// RocksDB info-log configuration parameters
		"rocksdb.info-log-keep-log-file-num": true,
		"rocksdb.info-log-level":             true,
		"rocksdb.info-log-max-size":          true,
		"rocksdb.info-log-roll-time":         true,
		// Raft log GC configuration parameters
		"raftstore.raft-log-gc-count-limit":        true,
		"raftstore.raft-log-gc-size-limit":         true,
		"raftstore.raft-log-gc-threshold":          true,
		"raftstore.raft-log-gc-tick-interval":      true,
		"raftstore.raft-log-compact-sync-interval": true,
		// General log configuration parameters
		"log.level":            true,
		"log.format":           true,
		"log.enable-timestamp": true,
		"log.file.max-backups": true,
		"log.file.max-days":    true,
		"log.file.max-size":    true,
		// Log backup configuration parameters (not paths)
		"log-backup.enable":                            true,
		"log-backup.file-size-limit":                   true,
		"log-backup.initial-scan-concurrency":          true,
		"log-backup.initial-scan-pending-memory-quota": true,
		"log-backup.initial-scan-rate-limit":           true,
		"log-backup.max-flush-interval":                true,
		"log-backup.min-ts-interval":                   true,
		"log-backup.num-threads":                       true,
		// Other log-related configuration parameters
		"raftstore.follower-read-max-log-gap": true,
		"raft-engine.enable-log-recycle":      true,
		"server.end-point-slow-log-threshold": true,
		"slow-log-threshold":                  true,
		"pd.retry-log-every":                  true,
		"security.redact-info-log":            true,
	},

	// Filename-only parameters (special comparison, not filtered)
	FilenameOnlyParams: map[string]bool{
		"log.file.filename":   true,
		"log-file":            true,
		"log.slow-query-file": true,
	},
}

// ShouldFilterParameter checks if a parameter should be filtered during preprocessing
// Returns (shouldFilter, filterReason)
func ShouldFilterParameter(paramName string) (bool, string) {
	paramNameLower := strings.ToLower(paramName)

	// Check exact match first
	if globalFilterConfig.ExactMatchParams[paramName] {
		return true, "deployment-specific parameter (exact match)"
	}

	// Check if it's an exception (contains path keywords but should not be filtered)
	if globalFilterConfig.Exceptions[paramName] {
		return false, ""
	}

	// Check host/network keywords
	for _, keyword := range globalFilterConfig.HostKeywords {
		if paramNameLower == keyword || strings.HasSuffix(paramNameLower, "."+keyword) || strings.HasPrefix(paramNameLower, keyword+".") {
			return true, "host/network parameter (deployment-specific)"
		}
	}

	// Check path keywords
	for _, keyword := range globalFilterConfig.PathKeywords {
		if strings.Contains(paramNameLower, keyword) {
			return true, "path parameter (deployment-specific)"
		}
	}

	return false, ""
}

// IsResourceDependentParameter checks if a parameter is resource-dependent
// Resource-dependent parameters are auto-tuned by TiKV/TiFlash based on system resources
func IsResourceDependentParameter(paramName string) bool {
	paramNameLower := strings.ToLower(paramName)

	for _, keyword := range globalFilterConfig.ResourceDependentKeywords {
		if strings.Contains(paramNameLower, keyword) {
			return true
		}
	}

	return false
}

// IsFilenameOnlyParameter checks if a parameter should be compared by filename only (ignoring path)
// This is used during rule evaluation for special comparison strategy, not for filtering
func IsFilenameOnlyParameter(paramName string) bool {
	return globalFilterConfig.FilenameOnlyParams[paramName]
}

// GetIgnoredParamsMapForUpgradeDifferences returns a map of ignored parameter names
// for use in CompareOptions.IgnoredParams. This is used during rule evaluation
// to filter nested map fields in CompareMapsDeep.
func GetIgnoredParamsMapForUpgradeDifferences() map[string]bool {
	// Return a copy of ExactMatchParams (they are the same)
	result := make(map[string]bool, len(globalFilterConfig.ExactMatchParams))
	for k, v := range globalFilterConfig.ExactMatchParams {
		result[k] = v
	}
	return result
}
