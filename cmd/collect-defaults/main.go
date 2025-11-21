package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pingcap/tidb/pkg/sessionctx/variable"
)

func main() {
	vars := variable.GetSysVars()
	m := make(map[string]string, len(vars))
	for name, sv := range vars {
		m[name] = sv.Value
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
		os.Exit(1)
	}
}
