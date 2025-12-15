package pd

// RequiredFilesForSparseCheckout returns the list of file paths required for PD knowledge base generation
// These files are used for sparse checkout to minimize download time
// Version parameter is kept for API consistency, but PD file paths don't change by version
// Users can modify this list to add or remove files as needed
func RequiredFilesForSparseCheckout(version string) []string {
	return []string{
		// PD config file (same path for all versions)
		"server/config/config.go",
	}
}


