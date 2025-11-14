package knowledge

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"sync"
)

//go:embed */*/upgrade_logic.json
var upgradeLogicFiles embed.FS

var (
	once       sync.Once
	versionMap map[string]int64
	loadErr    error
)

type upgradeLogicDocument struct {
	Metadata struct {
		BootstrapVersion int64  `json:"bootstrap_version"`
		TargetVersion    string `json:"target_version"`
	} `json:"metadata"`
}

func initVersionMap() {
	versionMap = make(map[string]int64)
	loadErr = fs.WalkDir(upgradeLogicFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "upgrade_logic.json") {
			return nil
		}
		data, err := upgradeLogicFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		var doc upgradeLogicDocument
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		target := strings.ToLower(strings.TrimSpace(doc.Metadata.TargetVersion))
		if target == "" {
			return nil
		}
		versionMap[target] = doc.Metadata.BootstrapVersion
		return nil
	})
}

// BootstrapVersion returns the TiDB bootstrap version associated with a semantic version string.
func BootstrapVersion(version string) (int64, bool, error) {
	once.Do(initVersionMap)
	if loadErr != nil {
		return 0, false, loadErr
	}
	v, ok := versionMap[strings.ToLower(strings.TrimSpace(version))]
	return v, ok, nil
}
