package precheck

import (
	"context"
	"fmt"
	"strings"
	
	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

// Analyzer performs analysis on cluster snapshots to identify potential upgrade issues
type Analyzer struct {
	rules []Rule
}

// NewAnalyzer creates a new analyzer with the provided rules
func NewAnalyzer(rules ...Rule) *Analyzer {
	return &Analyzer{
		rules: rules,
	}
}

// AddRule adds a rule to the analyzer
func (a *Analyzer) AddRule(rule Rule) {
	a.rules = append(a.rules, rule)
}

// Analyze performs the analysis on a cluster snapshot
func (a *Analyzer) Analyze(ctx context.Context, snapshot *runtime.ClusterSnapshot, targetVersion string) (*Report, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("snapshot cannot be nil")
	}
	
	// Convert runtime.ClusterSnapshot to precheck.Snapshot
	precheckSnapshot := a.convertSnapshot(snapshot, targetVersion)
	
	// Validate snapshot
	if err := ValidateSnapshot(precheckSnapshot); err != nil {
		return nil, fmt.Errorf("invalid snapshot: %w", err)
	}
	
	// Create engine and run analysis
	engine := NewEngine(a.rules...)
	report := engine.Run(ctx, precheckSnapshot)
	
	return &report, nil
}

// convertSnapshot converts a runtime.ClusterSnapshot to a precheck.Snapshot
func (a *Analyzer) convertSnapshot(snapshot *runtime.ClusterSnapshot, targetVersion string) Snapshot {
	precheckSnapshot := Snapshot{
		SourceVersion: getClusterVersion(snapshot),
		TargetVersion: targetVersion,
		Components:    make(map[string]ComponentSnapshot),
		GlobalSysVars: make(map[string]string),
		Config:        make(map[string]any),
		Metadata:      make(map[string]any),
		Tags:          make(map[string]string),
	}
	
	// Populate components
	for name, component := range snapshot.Components {
		precheckSnapshot.Components[name] = ComponentSnapshot{
			Version:    component.Version,
			Config:     component.Config,
			Attributes: make(map[string]string), // TODO: populate with relevant attributes
		}
	}
	
	// Extract global variables from TiDB if available
	if tidbComponent, exists := snapshot.Components["tidb"]; exists {
		for k, v := range tidbComponent.Variables {
			precheckSnapshot.GlobalSysVars[k] = v
		}
	}
	
	return precheckSnapshot
}

// getClusterVersion extracts the cluster version from a snapshot
func getClusterVersion(snapshot *runtime.ClusterSnapshot) string {
	// Try to get version from TiDB component first
	if tidbComponent, exists := snapshot.Components["tidb"]; exists {
		if tidbComponent.Version != "" {
			return tidbComponent.Version
		}
	}
	
	// Fallback to any component version
	for _, component := range snapshot.Components {
		if component.Version != "" {
			return component.Version
		}
	}
	
	return "unknown"
}

// HasBlockingIssues checks if the report contains any blocking issues
func HasBlockingIssues(report *Report) bool {
	if report == nil {
		return false
	}
	
	for _, item := range report.Items {
		if item.Severity == SeverityBlocker {
			return true
		}
	}
	
	return false
}

// GetSummaryText generates a human-readable summary of the report
func GetSummaryText(report *Report) string {
	if report == nil {
		return "No report available"
	}
	
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Precheck completed in %v\n", report.FinishedAt.Sub(report.StartedAt)))
	summary.WriteString(fmt.Sprintf("Total issues: %d\n", report.Summary.Total))
	
	if report.Summary.Blocking > 0 {
		summary.WriteString(fmt.Sprintf("Blocking issues: %d\n", report.Summary.Blocking))
	}
	
	if report.Summary.Warnings > 0 {
		summary.WriteString(fmt.Sprintf("Warnings: %d\n", report.Summary.Warnings))
	}
	
	if report.Summary.Infos > 0 {
		summary.WriteString(fmt.Sprintf("Info: %d\n", report.Summary.Infos))
	}
	
	if len(report.Errors) > 0 {
		summary.WriteString(fmt.Sprintf("Errors during checks: %d\n", len(report.Errors)))
	}
	
	return summary.String()
}