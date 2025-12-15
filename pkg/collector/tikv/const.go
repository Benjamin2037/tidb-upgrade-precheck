package tikv

// RequiredFilesForSparseCheckout returns the list of file paths required for TiKV knowledge base generation
// These files are used for sparse checkout to minimize download time
// Version parameter is kept for API consistency, but TiKV file paths don't change by version
// Users can modify this list to add or remove files as needed
func RequiredFilesForSparseCheckout(version string) []string {
	return []string{
		// TiKV config files (Rust, same paths for all versions)
		"src/config/config.rs",
		"components/config/src/lib.rs",
	}
}

