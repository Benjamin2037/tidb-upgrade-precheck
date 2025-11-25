package runtime

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// TiDBCollector handles collection of TiDB configuration and variables from a running cluster
type TiDBCollector interface {
	Collect(addr string) (*ComponentState, error)
}

type tidbCollector struct {
	httpClient *http.Client
}

// NewTiDBCollector creates a new TiDB collector for runtime configuration collection
func NewTiDBCollector() TiDBCollector {
	return &tidbCollector{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Collect gathers current configuration and variables from a running TiDB instance
// This is used for runtime inspection of a cluster's current state
func (c *tidbCollector) Collect(addr string) (*ComponentState, error) {
	state := &ComponentState{
		Type:      "tidb",
		Config:    make(map[string]interface{}),
		Variables: make(map[string]string),
		Status:    make(map[string]interface{}),
	}

	// Get version
	version, err := c.getVersion(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get TiDB version: %w", err)
	}
	state.Version = version

	// Collect configuration
	config, err := c.getConfig(addr)
	if err != nil {
		// Log the error but don't fail the entire collection
		// Configuration via HTTP API might not always be available
		fmt.Printf("Warning: failed to get TiDB config via HTTP API: %v\n", err)
	}
	if config != nil {
		state.Config = config
	}

	// Collect variables
	variables, err := c.getVariables(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get TiDB variables: %w", err)
	}
	state.Variables = variables

	return state, nil
}

func (c *tidbCollector) getVersion(addr string) (string, error) {
	db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(%s)/", addr))
	if err != nil {
		return "", err
	}
	defer db.Close()

	var version string
	err = db.QueryRow("SELECT VERSION()").Scan(&version)
	if err != nil {
		return "", err
	}

	return version, nil
}

func (c *tidbCollector) getConfig(addr string) (map[string]interface{}, error) {
	// Try to determine HTTP API port from MySQL port
	mysqlHost, mysqlPortStr, err := splitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MySQL address: %w", err)
	}
	
	// Default TiDB HTTP port is 10080, but if MySQL port is standard (4000), 
	// we assume HTTP port is 10080
	httpPort := "10080"
	mysqlPort, err := strconv.Atoi(mysqlPortStr)
	if err == nil && mysqlPort > 0 {
		// Calculate HTTP port: MySQL port + 6080 (4000 -> 10080)
		httpPort = strconv.Itoa(mysqlPort + 6080)
	}
	
	httpAddr := fmt.Sprintf("%s:%s", mysqlHost, httpPort)
	
	resp, err := c.httpClient.Get(fmt.Sprintf("http://%s/config", httpAddr))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to HTTP API at %s: %w", httpAddr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	var config map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *tidbCollector) getVariables(addr string) (map[string]string, error) {
	db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(%s)/", addr))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SHOW GLOBAL VARIABLES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	variables := make(map[string]string)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		variables[name] = value
	}

	return variables, rows.Err()
}

// splitHostPort splits an address into host and port, handling cases where 
// the address might not have an explicit port
func splitHostPort(addr string) (host, port string, err error) {
	if !strings.Contains(addr, ":") {
		// No port specified, assume default MySQL port
		return addr, "4000", nil
	}
	
	colonCount := strings.Count(addr, ":")
	if colonCount == 1 {
		// IPv4 or hostname with port
		parts := strings.Split(addr, ":")
		return parts[0], parts[1], nil
	} else if colonCount >= 2 {
		// IPv6 address
		// Find the last colon which separates address from port
		lastColonIndex := strings.LastIndex(addr, ":")
		if lastColonIndex > 0 && lastColonIndex < len(addr)-1 {
			hostPart := addr[:lastColonIndex]
			portPart := addr[lastColonIndex+1:]
			// Remove brackets if present
			hostPart = strings.Trim(hostPart, "[]")
			return hostPart, portPart, nil
		}
	}
	
	return "", "", fmt.Errorf("invalid address format: %s", addr)
}