package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Topology represents a TiDB cluster topology file structure
// This structure supports both TiUP and TiDB Operator topology formats
type Topology struct {
	// TiDBVersion is the version of the TiDB cluster
	// In TiUP format, this can be at root level or in metadata section
	TiDBVersion string `yaml:"tidb_version,omitempty"`

	// Metadata section (TiUP format)
	Metadata struct {
		TiDBVersion string `yaml:"tidb_version,omitempty"`
	} `yaml:"metadata,omitempty"`

	// ComponentVersions section (TiUP format)
	// In TiUP, component versions can be specified per component
	// Currently all components share the same version, but in the future they may evolve independently
	ComponentVersions struct {
		TiDB         string `yaml:"tidb,omitempty"`
		TiKV         string `yaml:"tikv,omitempty"`
		TiFlash      string `yaml:"tiflash,omitempty"`
		PD           string `yaml:"pd,omitempty"`
		TSO          string `yaml:"tso,omitempty"`
		Scheduling   string `yaml:"scheduling,omitempty"`
		Dashboard    string `yaml:"tidb_dashboard,omitempty"`
		Pump         string `yaml:"pump,omitempty"`
		Drainer      string `yaml:"drainer,omitempty"`
		CDC          string `yaml:"cdc,omitempty"`
		TiKVCDC      string `yaml:"kvcdc,omitempty"`
		TiProxy      string `yaml:"tiproxy,omitempty"`
		Prometheus   string `yaml:"prometheus,omitempty"`
		Grafana      string `yaml:"grafana,omitempty"`
		AlertManager string `yaml:"alertmanager,omitempty"`
	} `yaml:"component_versions,omitempty"`

	GlobalOptions struct {
		User    string `yaml:"user,omitempty"`
		SSHPort int    `yaml:"ssh_port,omitempty"`
	} `yaml:"global,omitempty"`

	TiDBServers []struct {
		Host       string                 `yaml:"host"`
		Port       int                    `yaml:"port"`
		StatusPort int                    `yaml:"status_port,omitempty"` // HTTP API port (usually port + 10000)
		DeployDir  string                 `yaml:"deploy_dir,omitempty"`
		Config     map[string]interface{} `yaml:"config,omitempty"`
	} `yaml:"tidb_servers,omitempty"`

	TiKVServers []struct {
		Host       string                 `yaml:"host"`
		Port       int                    `yaml:"port"`
		StatusPort int                    `yaml:"status_port,omitempty"` // HTTP API port
		DeployDir  string                 `yaml:"deploy_dir,omitempty"`
		DataDir    string                 `yaml:"data_dir,omitempty"` // TiKV data directory (required for reading last_tikv.toml)
		Config     map[string]interface{} `yaml:"config,omitempty"`
	} `yaml:"tikv_servers,omitempty"`

	PDServers []struct {
		Host       string                 `yaml:"host"`
		ClientPort int                    `yaml:"client_port"` // PD HTTP API port
		PeerPort   int                    `yaml:"peer_port,omitempty"`
		DeployDir  string                 `yaml:"deploy_dir,omitempty"`
		Config     map[string]interface{} `yaml:"config,omitempty"`
	} `yaml:"pd_servers,omitempty"`

	TiFlashServers []struct {
		Host       string                 `yaml:"host"`
		Port       int                    `yaml:"port"`
		StatusPort int                    `yaml:"status_port,omitempty"` // HTTP API port
		DeployDir  string                 `yaml:"deploy_dir,omitempty"`
		Config     map[string]interface{} `yaml:"config,omitempty"`
	} `yaml:"tiflash_servers,omitempty"`
}

// LoadTopologyFromFile loads a topology file and converts it to ClusterEndpoints
// Supports TiUP topology YAML format
func LoadTopologyFromFile(topologyPath string) (*ClusterEndpoints, error) {
	data, err := os.ReadFile(topologyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read topology file: %w", err)
	}

	var topo Topology
	if err := yaml.Unmarshal(data, &topo); err != nil {
		return nil, fmt.Errorf("failed to parse topology file: %w", err)
	}

	// Extract version from topology (check multiple locations)
	// Priority: root level > metadata section > component_versions
	version := topo.TiDBVersion
	if version == "" && topo.Metadata.TiDBVersion != "" {
		version = topo.Metadata.TiDBVersion
	}
	if version == "" && topo.ComponentVersions.TiDB != "" {
		version = topo.ComponentVersions.TiDB
	}

	endpoints := &ClusterEndpoints{
		TiKVAddrs:     []string{},
		PDAddrs:       []string{},
		TiFlashAddrs:  []string{},
		TiKVDataDirs:  make(map[string]string),
		SourceVersion: version, // Extract version from topology
	}

	// Extract TiDB connection info
	if len(topo.TiDBServers) > 0 {
		// Use the first TiDB instance
		tidb := topo.TiDBServers[0]
		endpoints.TiDBAddr = fmt.Sprintf("%s:%d", tidb.Host, tidb.Port)

		// Extract user from global options if available
		if topo.GlobalOptions.User != "" {
			endpoints.TiDBUser = topo.GlobalOptions.User
		} else {
			// Default to root if not specified
			endpoints.TiDBUser = "root"
		}
		// Password is typically not in topology file for security reasons
		// It should be provided separately by TiUP/Operator
	}

	// Extract TiKV addresses and data directories
	for _, tikv := range topo.TiKVServers {
		// Use status_port if available, otherwise use port
		port := tikv.Port
		if tikv.StatusPort > 0 {
			port = tikv.StatusPort
		}
		addr := fmt.Sprintf("%s:%d", tikv.Host, port)
		endpoints.TiKVAddrs = append(endpoints.TiKVAddrs, addr)

		// Extract data_dir from topology or config
		dataDir := tikv.DataDir
		if dataDir == "" && tikv.Config != nil {
			// Try to get data_dir from config section
			if storage, ok := tikv.Config["storage"].(map[string]interface{}); ok {
				if dd, ok := storage["data_dir"].(string); ok {
					dataDir = dd
				}
			}
		}
		// If still empty, try to construct from deploy_dir (common pattern)
		if dataDir == "" && tikv.DeployDir != "" {
			// Common pattern: data_dir is deploy_dir/data or deploy_dir/tikv-{port}/data
			dataDir = filepath.Join(tikv.DeployDir, "data")
		}

		if dataDir != "" {
			endpoints.TiKVDataDirs[addr] = dataDir
		}
	}

	// Extract PD addresses
	for _, pd := range topo.PDServers {
		endpoints.PDAddrs = append(endpoints.PDAddrs, fmt.Sprintf("%s:%d", pd.Host, pd.ClientPort))
	}

	// Extract TiFlash addresses
	for _, tiflash := range topo.TiFlashServers {
		// Use status_port if available, otherwise use port
		port := tiflash.Port
		if tiflash.StatusPort > 0 {
			port = tiflash.StatusPort
		}
		endpoints.TiFlashAddrs = append(endpoints.TiFlashAddrs, fmt.Sprintf("%s:%d", tiflash.Host, port))
	}

	return endpoints, nil
}

// LoadTopologyFromYAML loads topology from YAML content (for TiDB Operator or other formats)
func LoadTopologyFromYAML(yamlContent string) (*ClusterEndpoints, error) {
	var topo Topology
	if err := yaml.Unmarshal([]byte(yamlContent), &topo); err != nil {
		return nil, fmt.Errorf("failed to parse topology YAML: %w", err)
	}

	// Extract version from topology (check multiple locations)
	// Priority: root level > metadata section > component_versions
	version := topo.TiDBVersion
	if version == "" && topo.Metadata.TiDBVersion != "" {
		version = topo.Metadata.TiDBVersion
	}
	if version == "" && topo.ComponentVersions.TiDB != "" {
		version = topo.ComponentVersions.TiDB
	}

	endpoints := &ClusterEndpoints{
		TiKVAddrs:     []string{},
		PDAddrs:       []string{},
		TiFlashAddrs:  []string{},
		TiKVDataDirs:  make(map[string]string),
		SourceVersion: version, // Extract version from topology
	}

	// Extract TiDB connection info
	if len(topo.TiDBServers) > 0 {
		tidb := topo.TiDBServers[0]
		endpoints.TiDBAddr = fmt.Sprintf("%s:%d", tidb.Host, tidb.Port)
		if topo.GlobalOptions.User != "" {
			endpoints.TiDBUser = topo.GlobalOptions.User
		} else {
			endpoints.TiDBUser = "root"
		}
	}

	// Extract TiKV addresses and data directories
	for _, tikv := range topo.TiKVServers {
		port := tikv.Port
		if tikv.StatusPort > 0 {
			port = tikv.StatusPort
		}
		addr := fmt.Sprintf("%s:%d", tikv.Host, port)
		endpoints.TiKVAddrs = append(endpoints.TiKVAddrs, addr)

		// Extract data_dir from topology or config
		dataDir := tikv.DataDir
		if dataDir == "" && tikv.Config != nil {
			// Try to get data_dir from config section
			if storage, ok := tikv.Config["storage"].(map[string]interface{}); ok {
				if dd, ok := storage["data_dir"].(string); ok {
					dataDir = dd
				}
			}
		}
		// If still empty, try to construct from deploy_dir (common pattern)
		if dataDir == "" && tikv.DeployDir != "" {
			// Common pattern: data_dir is deploy_dir/data or deploy_dir/tikv-{port}/data
			dataDir = filepath.Join(tikv.DeployDir, "data")
		}

		if dataDir != "" {
			endpoints.TiKVDataDirs[addr] = dataDir
		}
	}

	// Extract PD addresses
	for _, pd := range topo.PDServers {
		endpoints.PDAddrs = append(endpoints.PDAddrs, fmt.Sprintf("%s:%d", pd.Host, pd.ClientPort))
	}

	// Extract TiFlash addresses
	for _, tiflash := range topo.TiFlashServers {
		port := tiflash.Port
		if tiflash.StatusPort > 0 {
			port = tiflash.StatusPort
		}
		endpoints.TiFlashAddrs = append(endpoints.TiFlashAddrs, fmt.Sprintf("%s:%d", tiflash.Host, port))
	}

	return endpoints, nil
}

// ParseTopologyEndpointString parses a simple endpoint string format
// Format: "tidb=host:port;tikv=host1:port1,host2:port2;pd=host1:port1,host2:port2"
// This is a fallback format for simple integrations
func ParseTopologyEndpointString(endpointStr string) (*ClusterEndpoints, error) {
	if endpointStr == "" {
		return nil, fmt.Errorf("endpoint string cannot be empty")
	}

	endpoints := &ClusterEndpoints{
		TiKVAddrs:    []string{},
		PDAddrs:      []string{},
		TiFlashAddrs: []string{},
	}

	parts := strings.Split(endpointStr, ";")
	validParts := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		validParts++

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		switch key {
		case "tidb":
			endpoints.TiDBAddr = value
		case "tikv":
			endpoints.TiKVAddrs = strings.Split(value, ",")
			for i := range endpoints.TiKVAddrs {
				endpoints.TiKVAddrs[i] = strings.TrimSpace(endpoints.TiKVAddrs[i])
			}
		case "pd":
			endpoints.PDAddrs = strings.Split(value, ",")
			for i := range endpoints.PDAddrs {
				endpoints.PDAddrs[i] = strings.TrimSpace(endpoints.PDAddrs[i])
			}
		case "tiflash":
			endpoints.TiFlashAddrs = strings.Split(value, ",")
			for i := range endpoints.TiFlashAddrs {
				endpoints.TiFlashAddrs[i] = strings.TrimSpace(endpoints.TiFlashAddrs[i])
			}
		}
	}

	// Validate that we parsed at least one valid endpoint
	if validParts == 0 {
		return nil, fmt.Errorf("no valid endpoints found in string: %s", endpointStr)
	}

	return endpoints, nil
}

// ExtractCredentialsFromTopology attempts to extract credentials from topology
// Note: For security reasons, passwords are typically not stored in topology files
// This function may be extended to read from environment variables or secure storage
func ExtractCredentialsFromTopology(topo *Topology) (user, password string) {
	if topo.GlobalOptions.User != "" {
		user = topo.GlobalOptions.User
	} else {
		user = "root" // Default
	}

	// Password is typically not in topology file
	// It should be provided by TiUP/Operator through secure means
	password = ""

	return user, password
}

// ValidateTopology validates that a topology has minimum required information
func ValidateTopology(topo *Topology) error {
	if len(topo.TiDBServers) == 0 && len(topo.TiKVServers) == 0 && len(topo.PDServers) == 0 && len(topo.TiFlashServers) == 0 {
		return fmt.Errorf("topology file must contain at least one component (TiDB, TiKV, PD, or TiFlash)")
	}

	return nil
}
