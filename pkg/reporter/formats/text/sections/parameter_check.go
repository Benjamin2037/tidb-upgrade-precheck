package sections

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats"
)

// ParameterCheckSection renders parameter check results
type ParameterCheckSection struct{}

// NewParameterCheckSection creates a new parameter check section
func NewParameterCheckSection() *ParameterCheckSection {
	return &ParameterCheckSection{}
}

// Name returns the section name
func (s *ParameterCheckSection) Name() string {
	return "Parameter Check"
}

// HasContent checks if this section has any content to render
func (s *ParameterCheckSection) HasContent(result *analyzer.AnalysisResult) bool {
	return len(result.ModifiedParams) > 0 ||
		len(result.TikvInconsistencies) > 0 ||
		len(result.UpgradeDifferences) > 0 ||
		len(result.ForcedChanges) > 0 ||
		len(result.FocusParams) > 0
}

// Render renders the section content
func (s *ParameterCheckSection) Render(format formats.Format, result *analyzer.AnalysisResult) (string, error) {
	var content strings.Builder

	// Modified Parameters
	if len(result.ModifiedParams) > 0 {
		content.WriteString("Modified Parameters (from source defaults):\n")
		for compType, params := range result.ModifiedParams {
			for paramName, info := range params {
				content.WriteString(fmt.Sprintf("  [%s] %s: %v (default: %v)\n",
					compType, paramName, info.CurrentValue, info.SourceDefault))
			}
		}
		content.WriteString("\n")
	}

	// TiKV Inconsistencies
	if len(result.TikvInconsistencies) > 0 {
		content.WriteString("TiKV Parameter Inconsistencies:\n")
		for paramName, nodes := range result.TikvInconsistencies {
			content.WriteString(fmt.Sprintf("  Parameter: %s\n", paramName))
			for _, node := range nodes {
				content.WriteString(fmt.Sprintf("    Node %s: %v\n", node.NodeAddress, node.Value))
			}
		}
		content.WriteString("\n")
	}

	// Upgrade Differences
	if len(result.UpgradeDifferences) > 0 {
		content.WriteString("Upgrade Differences:\n")
		for compType, params := range result.UpgradeDifferences {
			// Separate PD info messages from warnings
			var pdInfoParams []string
			var otherParams []string
			for paramName := range params {
				// Check if this is a PD info message by looking at CheckResults
				isPDInfo := false
				for _, check := range result.CheckResults {
					if check.Component == compType && check.ParameterName == paramName && check.Severity == "info" && compType == "pd" {
						isPDInfo = true
						break
					}
				}
				if isPDInfo {
					pdInfoParams = append(pdInfoParams, paramName)
				} else {
					otherParams = append(otherParams, paramName)
				}
			}

			// Display PD info messages separately
			if len(pdInfoParams) > 0 && compType == "pd" {
				content.WriteString(fmt.Sprintf("  [%s] Default Value Changes (Info):\n", strings.ToUpper(compType)))
				content.WriteString("    The following parameters have default value changes, but current configuration will be preserved:\n")
				for _, paramName := range pdInfoParams {
					diff := params[paramName]
					content.WriteString(fmt.Sprintf("    [INFO] %s: %v (preserved) -> target default: %v, source default: %v\n",
						paramName, diff.CurrentValue, diff.TargetDefault, diff.SourceDefault))
				}
			}

			// Display other upgrade differences
			for _, paramName := range otherParams {
				diff := params[paramName]
				content.WriteString(fmt.Sprintf("  [%s] %s: %v -> %v (source default: %v)\n",
					compType, paramName, diff.CurrentValue, diff.TargetDefault, diff.SourceDefault))
			}
		}
		content.WriteString("\n")
	}

	// Forced Changes
	if len(result.ForcedChanges) > 0 {
		content.WriteString("Forced Changes During Upgrade:\n")
		for compType, params := range result.ForcedChanges {
			for paramName, change := range params {
				content.WriteString(fmt.Sprintf("  [%s] %s: %v -> %v\n",
					compType, paramName, change.CurrentValue, change.ForcedValue))
				if change.Summary != "" {
					content.WriteString(fmt.Sprintf("    Summary: %s\n", change.Summary))
				}
			}
		}
		content.WriteString("\n")
	}

	// Focus Parameters
	if len(result.FocusParams) > 0 {
		content.WriteString("Focus Parameters:\n")
		for compType, params := range result.FocusParams {
			for paramName, info := range params {
				content.WriteString(fmt.Sprintf("  [%s] %s: %v\n",
					compType, paramName, info.CurrentValue))
				if info.IsModified {
					content.WriteString("    (Modified from source default)\n")
				}
				if info.WillChange {
					content.WriteString("    (Will change after upgrade)\n")
				}
			}
		}
		content.WriteString("\n")
	}

	return content.String(), nil
}
