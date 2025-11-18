package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Change represents a single upgrade action extracted from TiDB's upgrade catalog.
type Change struct {
	FromVersion   int64    `json:"from_version"`
	ToVersion     int64    `json:"to_version"`
	Kind          string   `json:"kind"`
	Target        string   `json:"target"`
	DefaultValue  string   `json:"default_value"`
	Force         bool     `json:"force"`
	Summary       string   `json:"summary"`
	Details       string   `json:"details"`
	Scope         string   `json:"scope"`
	RiskLevel     string   `json:"risk_level"`
	OptionalHints []string `json:"optional_hints"`
}

type versionBucket struct {
	Version int64    `json:"version"`
	Changes []Change `json:"changes"`
}

type document struct {
	Versions []versionBucket `json:"versions"`
}

// Catalog holds a denormalised view of the upgrade changes keyed by TiDB bootstrap version.
type Catalog struct {
	versions map[int64][]Change
	order    []int64
}

// LoadCatalog parses an upgrade metadata JSON document generated from TiDB.
func LoadCatalog(path string) (*Catalog, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open upgrade metadata: %w", err)
	}
	defer file.Close()

	return LoadCatalogFromReader(file)
}

// LoadCatalogFromReader parses the upgrade metadata document from an io.Reader.
func LoadCatalogFromReader(r io.Reader) (*Catalog, error) {
	bytes, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read upgrade metadata: %w", err)
	}
	return LoadCatalogFromBytes(bytes)
}

// LoadCatalogFromBytes parses the upgrade metadata document from a byte slice.
func LoadCatalogFromBytes(data []byte) (*Catalog, error) {
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse upgrade metadata: %w", err)
	}

	versions := make(map[int64][]Change, len(doc.Versions))
	order := make([]int64, 0, len(doc.Versions))
	for _, bucket := range doc.Versions {
		if bucket.Version == 0 {
			continue
		}
		versions[bucket.Version] = append([]Change(nil), bucket.Changes...)
		order = append(order, bucket.Version)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	return &Catalog{versions: versions, order: order}, nil
}

// ForcedSysvarChanges returns forced global sysvar changes between two bootstrap versions.
func (c *Catalog) ForcedSysvarChanges(fromVersion, toVersion int64) []Change {
	if c == nil || toVersion <= fromVersion {
		return nil
	}

	var result []Change
	for _, version := range c.order {
		if version <= fromVersion || version > toVersion {
			continue
		}
		for _, change := range c.versions[version] {
			if !change.Force {
				continue
			}
			if change.Kind != "sysvar" {
				continue
			}
			if change.Scope != "" && change.Scope != "global" {
				continue
			}
			result = append(result, normalizeChange(change, version))
		}
	}
	return result
}

// LatestGlobalSysvarValues returns the latest known global sysvar values up to the specified bootstrap version.
func (c *Catalog) LatestGlobalSysvarValues(upToVersion int64) map[string]Change {
	if c == nil || upToVersion <= 0 {
		return nil
	}

	result := make(map[string]Change)
	for _, version := range c.order {
		if version > upToVersion {
			break
		}
		for _, change := range c.versions[version] {
			if change.Kind != "sysvar" {
				continue
			}
			if change.Scope != "" && !strings.EqualFold(change.Scope, "global") {
				continue
			}
			normalized := normalizeChange(change, version)
			key := strings.ToLower(normalized.Target)
			result[key] = normalized
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeChange(change Change, version int64) Change {
	copy := change
	if copy.ToVersion == 0 {
		copy.ToVersion = version
	}
	if copy.FromVersion == 0 {
		copy.FromVersion = version - 1
	}
	return copy
}
