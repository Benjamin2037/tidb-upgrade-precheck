//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pingcap/tidb/config"
	"github.com/pingcap/tidb/sessionctx/variable"
)

type Output struct {
	Sysvars          map[string]interface{} `json:"sysvars"`
	Config           map[string]interface{} `json:"config"`
	BootstrapVersion int64                  `json:"bootstrap_version"`
}

func main() {
	// Collect config default values
	cfg := config.GetGlobalConfig()
	cfgMap := make(map[string]interface{})
	data, _ := json.Marshal(cfg)
	json.Unmarshal(data, &cfgMap)

	// Collect all user-visible sysvars
	sysvars := make(map[string]interface{})
	for _, sv := range variable.GetSysVars() {
		if sv.Hidden || sv.Scope == variable.ScopeNone {
			continue
		}
		sysvars[sv.Name] = sv.Value
	}

	// Collect bootstrap version
	// Note: Using int64 type here because currentBootstrapVersion is int64 type
	var ver int64 = 0 // Default value, should actually be obtained from session package

	out := Output{
		Sysvars:          sysvars,
		Config:           cfgMap,
		BootstrapVersion: ver,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "encode failed: %v\n", err)
		os.Exit(1)
	}
}