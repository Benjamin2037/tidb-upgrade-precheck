package scan

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// VersionRecord represents a record of a generated version
type VersionRecord struct {
	Tag         string    `json:"tag"`
	GeneratedAt time.Time `json:"generated_at"`
	CommitHash  string    `json:"commit_hash"`
}

// VersionManager manages generated versions to avoid regenerating
type VersionManager struct {
	recordsFile string
	records     map[string]VersionRecord
}

// NewVersionManager creates a new version manager
func NewVersionManager(knowledgeDir string) (*VersionManager, error) {
	vm := &VersionManager{
		recordsFile: filepath.Join(knowledgeDir, "generated_versions.json"),
		records:     make(map[string]VersionRecord),
	}
	
	// Load existing records if file exists
	if _, err := os.Stat(vm.recordsFile); err == nil {
		data, err := ioutil.ReadFile(vm.recordsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read version records: %v", err)
		}
		
		if err := json.Unmarshal(data, &vm.records); err != nil {
			return nil, fmt.Errorf("failed to parse version records: %v", err)
		}
	}
	
	return vm, nil
}

// IsVersionGenerated checks if a version has already been generated
func (vm *VersionManager) IsVersionGenerated(tag string) bool {
	_, exists := vm.records[tag]
	return exists
}

// RecordVersion records that a version has been generated
func (vm *VersionManager) RecordVersion(tag, commitHash string) error {
	vm.records[tag] = VersionRecord{
		Tag:         tag,
		GeneratedAt: time.Now(),
		CommitHash:  commitHash,
	}
	
	return vm.save()
}

// RemoveVersion removes a version record
func (vm *VersionManager) RemoveVersion(tag string) error {
	delete(vm.records, tag)
	return vm.save()
}

// GetGeneratedVersions returns all generated versions
func (vm *VersionManager) GetGeneratedVersions() []VersionRecord {
	var records []VersionRecord
	for _, record := range vm.records {
		records = append(records, record)
	}
	return records
}

// save writes the records to file
func (vm *VersionManager) save() error {
	data, err := json.MarshalIndent(vm.records, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal version records: %v", err)
	}
	
	return ioutil.WriteFile(vm.recordsFile, data, 0644)
}

// GetVersionCommitHash gets the commit hash for a generated version
func (vm *VersionManager) GetVersionCommitHash(tag string) (string, bool) {
	record, exists := vm.records[tag]
	if !exists {
		return "", false
	}
	return record.CommitHash, true
}