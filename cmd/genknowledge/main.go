package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

var ltsFamilies = []string{
	"v6.5",
	"v7.1",
	"v7.5",
	"v8.1",
	"v8.5",
}

var bootstrapVersionPattern = regexp.MustCompile(`var\s+currentBootstrapVersion\s+int64\s*=\s*version(\d+)`)

func main() {
	tidbRoot, err := filepath.Abs(filepath.Join("..", "tidb"))
	if err != nil {
		fatal(err)
	}

	outRoot, err := filepath.Abs(filepath.Join("knowledge"))
	if err != nil {
		fatal(err)
	}

	if err := os.MkdirAll(outRoot, 0o755); err != nil {
		fatal(fmt.Errorf("create output root: %w", err))
	}

	versionsByFamily, err := discoverLTSVersions(tidbRoot, ltsFamilies)
	if err != nil {
		fatal(err)
	}

	for _, family := range ltsFamilies {
		versions := versionsByFamily[family]
		if len(versions) == 0 {
			continue
		}

		familyRoot := filepath.Join(outRoot, family)
		if err := os.MkdirAll(familyRoot, 0o755); err != nil {
			fatal(fmt.Errorf("create family directory %s: %w", familyRoot, err))
		}

		for _, version := range versions {
			if err := generateVersion(tidbRoot, outRoot, family, version); err != nil {
				fatal(err)
			}
		}
	}
}

func discoverLTSVersions(tidbRoot string, families []string) (map[string][]string, error) {
	seen := make(map[string]struct{})
	results := make(map[string][]string, len(families))
	for _, family := range families {
		tags, err := listTags(tidbRoot, family)
		if err != nil {
			return nil, fmt.Errorf("list tags for %s: %w", family, err)
		}
		canonical := make([]string, 0, len(tags))
		for _, tag := range tags {
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			canonical = append(canonical, tag)
		}

		if len(canonical) == 0 {
			continue
		}

		sort.Slice(canonical, func(i, j int) bool {
			return semver.Compare(canonical[i], canonical[j]) < 0
		})

		results[family] = canonical
	}

	return results, nil
}

func listTags(tidbRoot, family string) ([]string, error) {
	pattern := fmt.Sprintf("%s.*", family)
	cmd := exec.Command("git", "tag", "--list", pattern)
	cmd.Dir = tidbRoot
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(&stdout)
	var tags []string
	for scanner.Scan() {
		tag := strings.TrimSpace(scanner.Text())
		if tag == "" {
			continue
		}
		if !semver.IsValid(tag) {
			continue
		}
		if semver.Prerelease(tag) != "" {
			continue
		}
		if build := semver.Build(tag); build != "" {
			continue
		}
		tags = append(tags, tag)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(tags) == 0 {
		return nil, fmt.Errorf("no tags matched pattern %s", pattern)
	}
	sort.Slice(tags, func(i, j int) bool {
		return semver.Compare(tags[i], tags[j]) < 0
	})

	return tags, nil
}

func generateVersion(tidbRoot, outRoot, family, version string) error {
	fmt.Printf("Generating knowledge for %s\n", version)
	familyOut := filepath.Join(outRoot, family)
	if err := os.MkdirAll(familyOut, 0o755); err != nil {
		return fmt.Errorf("prepare family directory %s: %w", familyOut, err)
	}

	cmd := exec.Command("go", "run", "./tools/paramguard", "--version", version, "--out-dir", familyOut)
	cmd.Dir = tidbRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	bootstrapVer, err := bootstrapVersionForTag(tidbRoot, version)
	if err != nil {
		return fmt.Errorf("resolve bootstrap version: %w", err)
	}

	versionDir := filepath.Join(familyOut, version)
	if err := annotateMetadata(versionDir, bootstrapVer); err != nil {
		return fmt.Errorf("annotate metadata: %w", err)
	}

	return nil
}

func bootstrapVersionForTag(tidbRoot, tag string) (int64, error) {
	candidates := []string{
		filepath.ToSlash(filepath.Join("pkg", "session", "upgrade.go")),
		filepath.ToSlash(filepath.Join("pkg", "session", "bootstrap.go")),
		filepath.ToSlash(filepath.Join("session", "upgrade.go")),
		filepath.ToSlash(filepath.Join("session", "bootstrap.go")),
	}

	var lastErr error
	for _, rel := range candidates {
		spec := fmt.Sprintf("%s:%s", tag, rel)
		cmd := exec.Command("git", "show", spec)
		cmd.Dir = tidbRoot
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			lastErr = fmt.Errorf("git show %s: %s", spec, strings.TrimSpace(stderr.String()))
			continue
		}

		matches := bootstrapVersionPattern.FindSubmatch(stdout.Bytes())
		if len(matches) != 2 {
			lastErr = fmt.Errorf("bootstrap version pattern not found in %s", spec)
			continue
		}

		value, err := strconv.ParseInt(string(matches[1]), 10, 64)
		if err != nil {
			lastErr = fmt.Errorf("parse bootstrap version from %s: %w", spec, err)
			continue
		}
		return value, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("failed to resolve bootstrap version for tag %s", tag)
	}
	return 0, lastErr
}

func annotateMetadata(dir string, bootstrap int64) error {
	targets := []string{"defaults.json", "upgrade_logic.json"}
	for _, name := range targets {
		if err := annotateFile(filepath.Join(dir, name), bootstrap); err != nil {
			return err
		}
	}
	return nil
}

func annotateFile(path string, bootstrap int64) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("unmarshal %s: %w", path, err)
	}

	meta, _ := doc["metadata"].(map[string]any)
	if meta == nil {
		meta = make(map[string]any)
	}
	meta["bootstrap_version"] = bootstrap
	doc["metadata"] = meta

	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}

	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "genknowledge: %v\n", err)
	os.Exit(1)
}
