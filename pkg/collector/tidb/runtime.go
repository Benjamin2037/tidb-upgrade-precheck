package tidb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// TiDBCollector handles collection of TiDB configuration and system variables
// Connection credentials are provided by external tools (TiUP/TiDB Operator)
type TiDBCollector interface {
	Collect(addr, user, password string) (*types.ComponentState, error)
	// GetConfigByType gets configuration for a specific component type using SHOW CONFIG
	// This can be used to collect PD, TiKV, and TiFlash configs
	GetConfigByType(db *sql.DB, componentType string) (map[string]interface{}, error)
	// GetConfigByTypeAndInstance gets configuration for a specific component type and instance
	// instance should be in format "IP:port" (e.g., "192.168.1.101:20160")
	GetConfigByTypeAndInstance(db *sql.DB, componentType, instance string) (map[string]interface{}, error)
}

type tidbCollector struct {
	httpClient *http.Client
}

// NewTiDBCollector creates a new TiDB collector
func NewTiDBCollector() TiDBCollector {
	return &tidbCollector{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Collect gathers configuration and variables from a TiDB instance
// addr: MySQL protocol endpoint (host:port)
// user: MySQL username (provided by TiUP/Operator)
// password: MySQL password (provided by TiUP/Operator)
func (c *tidbCollector) Collect(addr, user, password string) (*types.ComponentState, error) {
	state := &types.ComponentState{
		Type:      types.ComponentTiDB,
		Config:    make(types.ConfigDefaults),
		Variables: make(types.SystemVariables),
		Status:    make(map[string]interface{}),
	}

	// Default to root if user not provided (for backward compatibility)
	if user == "" {
		user = "root"
	}

	// Get version using MySQL protocol
	version, err := c.getVersion(addr, user, password)
	if err != nil {
		return nil, fmt.Errorf("failed to get TiDB version: %w", err)
	}
	state.Version = version

	// Collect configuration using SHOW CONFIG SQL (preferred method)
	// This can collect TiDB, TiKV, and TiFlash configs from a single TiDB connection
	config, err := c.getConfigViaSQL(addr, user, password)
	if err != nil {
		// Log warning but continue - config might not be available
		fmt.Printf("Warning: failed to get config via SHOW CONFIG: %v\n", err)
		// Create empty config map
		config = make(map[string]interface{})
	} else if len(config) == 0 {
		// SHOW CONFIG executed successfully but returned no results
		// This might happen in older versions (e.g., v6.5.0) where SHOW CONFIG is not fully supported
		fmt.Printf("Warning: SHOW CONFIG returned empty results (may not be supported in this version)\n")
	}
	// Convert to pkg/types.ConfigDefaults format
	state.Config = types.ConvertConfigToDefaults(config)

	// Collect system variables using MySQL protocol
	variables, err := c.getVariables(addr, user, password)
	if err != nil {
		return nil, fmt.Errorf("failed to get TiDB variables: %w", err)
	}
	// Convert to pkg/types.SystemVariables format
	state.Variables = types.ConvertVariablesToSystemVariables(variables)

	return state, nil
}

// getVersion gets TiDB version using MySQL protocol
func (c *tidbCollector) getVersion(addr, user, password string) (string, error) {
	dsn := c.buildDSN(addr, user, password, "")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return "", fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	// Set connection timeout
	db.SetConnMaxLifetime(10 * time.Second)

	var version string
	err = db.QueryRow("SELECT VERSION()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to query version: %w", err)
	}

	return version, nil
}

// getConfigViaSQL gets TiDB configuration using SHOW CONFIG SQL statement
// This can collect TiDB, TiKV, and TiFlash configs from a single TiDB connection
// Example: SHOW CONFIG WHERE type='tidb'
func (c *tidbCollector) getConfigViaSQL(addr, user, password string) (map[string]interface{}, error) {
	dsn := c.buildDSN(addr, user, password, "")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	// Set connection timeout
	db.SetConnMaxLifetime(10 * time.Second)

	// Collect TiDB config
	config := make(map[string]interface{})

	// Get TiDB config
	tidbConfig, err := c.getConfigByType(db, "tidb")
	if err != nil {
		return nil, fmt.Errorf("failed to get TiDB config: %w", err)
	}
	if len(tidbConfig) > 0 {
		config["tidb"] = tidbConfig
	}

	// If we only have TiDB config, return it directly (for backward compatibility)
	if len(config) == 1 && config["tidb"] != nil {
		if tidbMap, ok := config["tidb"].(map[string]interface{}); ok {
			return tidbMap, nil
		}
	}

	return config, nil
}

// GetConfigByType gets configuration for a specific component type using SHOW CONFIG
// This is a public method that can be used to collect TiKV, and TiFlash configs
func (c *tidbCollector) GetConfigByType(db *sql.DB, componentType string) (map[string]interface{}, error) {
	return c.getConfigByType(db, componentType)
}

// GetConfigByTypeAndInstance gets configuration for a specific component type and instance using SHOW CONFIG
// instance should be in format "IP:port" (e.g., "192.168.1.101:20160")
func (c *tidbCollector) GetConfigByTypeAndInstance(db *sql.DB, componentType, instance string) (map[string]interface{}, error) {
	query := fmt.Sprintf("SHOW CONFIG WHERE type='%s' AND instance='%s'", componentType, instance)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query config for type %s and instance %s: %w", componentType, instance, err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	config := make(map[string]interface{})
	rowCount := 0
	for rows.Next() {
		rowCount++
		// Create a slice to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan config row: %w", err)
		}

		// Find the 'name' and 'value' columns (case-insensitive)
		// SHOW CONFIG returns columns: Type, Instance, Name, Value
		var name, value string
		for i, col := range columns {
			colLower := strings.ToLower(col)
			if colLower == "name" {
				if v, ok := values[i].([]byte); ok {
					name = string(v)
				} else if v, ok := values[i].(string); ok {
					name = v
				}
			} else if colLower == "value" {
				if v, ok := values[i].([]byte); ok {
					value = string(v)
				} else if v, ok := values[i].(string); ok {
					value = v
				} else {
					// Try to convert to string
					value = fmt.Sprintf("%v", values[i])
				}
			}
		}

		if name != "" {
			// Try to parse value as JSON first, then as number, then as boolean, finally as string
			var parsedValue interface{} = value

			// Try to parse as JSON
			var jsonValue interface{}
			if err := json.Unmarshal([]byte(value), &jsonValue); err == nil {
				parsedValue = jsonValue
			} else {
				// Try to parse as number
				if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
					parsedValue = intVal
				} else if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
					parsedValue = floatVal
				} else if boolVal, err := strconv.ParseBool(value); err == nil {
					parsedValue = boolVal
				}
			}

			config[name] = parsedValue
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating config rows: %w", err)
	}

	// Debug: log row count
	if rowCount == 0 {
		fmt.Printf("Warning: SHOW CONFIG WHERE type='%s' AND instance='%s' returned 0 rows\n", componentType, instance)
	} else {
		fmt.Printf("SHOW CONFIG WHERE type='%s' AND instance='%s' returned %d rows, extracted %d config parameters\n", componentType, instance, rowCount, len(config))
	}

	return config, nil
}

// getConfigByType gets configuration for a specific component type using SHOW CONFIG
func (c *tidbCollector) getConfigByType(db *sql.DB, componentType string) (map[string]interface{}, error) {
	query := fmt.Sprintf("SHOW CONFIG WHERE type='%s'", componentType)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query config for type %s: %w", componentType, err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	config := make(map[string]interface{})
	rowCount := 0
	for rows.Next() {
		rowCount++
		// Create a slice to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan config row: %w", err)
		}

		// Find the 'name' and 'value' columns (case-insensitive)
		// SHOW CONFIG returns columns: Type, Instance, Name, Value
		var name, value string
		for i, col := range columns {
			colLower := strings.ToLower(col)
			if colLower == "name" {
				if v, ok := values[i].([]byte); ok {
					name = string(v)
				} else if v, ok := values[i].(string); ok {
					name = v
				}
			} else if colLower == "value" {
				if v, ok := values[i].([]byte); ok {
					value = string(v)
				} else if v, ok := values[i].(string); ok {
					value = v
				} else {
					// Try to convert to string
					value = fmt.Sprintf("%v", values[i])
				}
			}
		}

		if name != "" {
			// Try to parse value as JSON first, then as number, then as boolean, finally as string
			var parsedValue interface{} = value

			// Try to parse as JSON
			var jsonValue interface{}
			if err := json.Unmarshal([]byte(value), &jsonValue); err == nil {
				parsedValue = jsonValue
			} else {
				// Try to parse as number
				if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
					parsedValue = intVal
				} else if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
					parsedValue = floatVal
				} else if boolVal, err := strconv.ParseBool(value); err == nil {
					parsedValue = boolVal
				}
			}

			config[name] = parsedValue
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating config rows: %w", err)
	}

	// Debug: log row count and column names
	if rowCount == 0 {
		fmt.Printf("Warning: SHOW CONFIG WHERE type='%s' returned 0 rows\n", componentType)
	} else {
		fmt.Printf("SHOW CONFIG WHERE type='%s' returned %d rows, extracted %d config parameters (columns: %v)\n", componentType, rowCount, len(config), columns)
		if len(config) == 0 && rowCount > 0 {
			fmt.Printf("Warning: parsed 0 config parameters from %d rows, column names: %v\n", rowCount, columns)
		}
	}

	return config, nil
}

// getVariables gets TiDB system variables using MySQL protocol
func (c *tidbCollector) getVariables(addr, user, password string) (map[string]string, error) {
	dsn := c.buildDSN(addr, user, password, "")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	// Set connection timeout
	db.SetConnMaxLifetime(10 * time.Second)

	rows, err := db.Query("SHOW GLOBAL VARIABLES")
	if err != nil {
		return nil, fmt.Errorf("failed to query variables: %w", err)
	}
	defer rows.Close()

	variables := make(map[string]string)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("failed to scan variable row: %w", err)
		}
		variables[name] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating variables: %w", err)
	}

	return variables, nil
}

// buildDSN builds MySQL DSN string
// Connection credentials are provided by external tools (TiUP/TiDB Operator)
func (c *tidbCollector) buildDSN(addr, user, password, database string) string {
	// Default to root if user not provided (for backward compatibility)
	if user == "" {
		user = "root"
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/", user, password, addr)
	if database != "" {
		dsn = fmt.Sprintf("%s:%s@tcp(%s)/%s", user, password, addr, database)
	}

	return dsn
}
