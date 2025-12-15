package tiflash

// RequiredFilesForSparseCheckout returns the list of file paths required for TiFlash knowledge base generation
// These files are used for sparse checkout to minimize download time
// Version parameter is kept for API consistency, but TiFlash file paths don't change by version
// Users can modify this list to add or remove files as needed
func RequiredFilesForSparseCheckout(version string) []string {
	return []string{
		// TiFlash C++ config files (same paths for all versions)
		"dbms/src/Core/SpillConfig.h",
		"dbms/src/Core/SpillConfig.cpp",
		"dbms/src/Server/StorageConfigParser.h",
		"dbms/src/Server/StorageConfigParser.cpp",
		"dbms/src/Server/RaftConfigParser.h",
		"dbms/src/Server/RaftConfigParser.cpp",
		"dbms/src/Server/UserConfigParser.h",
		"dbms/src/Server/UserConfigParser.cpp",
		"dbms/src/Common/config.h.in",
		"dbms/src/Common/config_build.cpp.in",
	}
}
