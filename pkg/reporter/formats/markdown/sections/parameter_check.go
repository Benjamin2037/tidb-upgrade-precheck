package sections

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats"
)

// ParameterCheckSection renders parameter check results in markdown format
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

// Render renders the section content in markdown format
func (s *ParameterCheckSection) Render(format formats.Format, result *analyzer.AnalysisResult) (string, error) {
	var content strings.Builder

	// Modified Parameters
	if len(result.ModifiedParams) > 0 {
		content.WriteString("## 1. User Modified Parameters\n\n")
		content.WriteString("Parameters that have been modified from source version defaults.\n\n")

		components := []string{"tidb", "pd", "tikv", "tiflash"}
		for _, compType := range components {
			if params, ok := result.ModifiedParams[compType]; ok && len(params) > 0 {
				content.WriteString(fmt.Sprintf("### %s Component\n\n", strings.ToUpper(compType)))
				content.WriteString("| Parameter | Current Value | Source Default | Type |\n")
				content.WriteString("|-----------|---------------|----------------|------|\n")
				for paramName, info := range params {
					content.WriteString(fmt.Sprintf("| `%s` | %v | %v | %s |\n",
						paramName, info.CurrentValue, info.SourceDefault, info.ParamType))
				}
				content.WriteString("\n")
			}
		}
	}

	// TiKV Inconsistencies
	if len(result.TikvInconsistencies) > 0 {
		content.WriteString("## 2. TiKV Parameter Inconsistencies\n\n")
		content.WriteString("TiKV nodes with inconsistent parameter values.\n\n")
		for paramName, nodes := range result.TikvInconsistencies {
			content.WriteString(fmt.Sprintf("### Parameter: `%s`\n\n", paramName))
			content.WriteString("| Node Address | Value |\n")
			content.WriteString("|--------------|-------|\n")
			for _, node := range nodes {
				content.WriteString(fmt.Sprintf("| %s | %v |\n", node.NodeAddress, node.Value))
			}
			content.WriteString("\n")
		}
	}

	// Upgrade Differences
	if len(result.UpgradeDifferences) > 0 || len(result.ForcedChanges) > 0 {
		content.WriteString("## 3. Upgrade Differences\n\n")
		content.WriteString("Parameters that will differ after upgrade.\n\n")

		// Forced Changes
		if len(result.ForcedChanges) > 0 {
			content.WriteString("### Forced Changes (Critical)\n\n")
			content.WriteString("⚠️  **These parameters will be forcibly changed during upgrade and cannot be prevented.**\n\n")
			content.WriteString("| Component | Parameter | Current | Forced To | Summary |\n")
			content.WriteString("|-----------|-----------|---------|-----------|----------|\n")
			for compType, params := range result.ForcedChanges {
				for paramName, change := range params {
					content.WriteString(fmt.Sprintf("| %s | `%s` | %v | %v | %s |\n",
						compType, paramName, change.CurrentValue, change.ForcedValue, change.Summary))
				}
			}
			content.WriteString("\n")
		}

		// Other Upgrade Differences
		if len(result.UpgradeDifferences) > 0 {
			content.WriteString("### Other Upgrade Differences\n\n")
			components := []string{"tidb", "pd", "tikv", "tiflash"}
			for _, compType := range components {
				if params, ok := result.UpgradeDifferences[compType]; ok && len(params) > 0 {
					content.WriteString(fmt.Sprintf("#### %s Component\n\n", strings.ToUpper(compType)))

					// Separate PD info messages (default changed but value preserved) from warnings
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
						content.WriteString("ℹ️  **Default Value Changes (Info):**\n\n")
						content.WriteString("The following parameters have default value changes in the target version, but your current configuration will be preserved during upgrade.\n\n")
						content.WriteString("| Parameter | Current (Preserved) | Target Default | Source Default | Type |\n")
						content.WriteString("|-----------|---------------------|---------------|---------------|------|\n")
						for _, paramName := range pdInfoParams {
							diff := params[paramName]
							content.WriteString(fmt.Sprintf("| `%s` | %v | %v | %v | %s |\n",
								paramName, diff.CurrentValue, diff.TargetDefault, diff.SourceDefault, diff.ParamType))
						}
						content.WriteString("\n")
					}

					// Display other upgrade differences
					if len(otherParams) > 0 {
						if len(pdInfoParams) > 0 && compType == "pd" {
							content.WriteString("**Other Upgrade Differences:**\n\n")
						}
						content.WriteString("| Parameter | Current | Target Default | Source Default | Type |\n")
						content.WriteString("|-----------|---------|---------------|---------------|------|\n")
						for _, paramName := range otherParams {
							diff := params[paramName]
							content.WriteString(fmt.Sprintf("| `%s` | %v | %v | %v | %s |\n",
								paramName, diff.CurrentValue, diff.TargetDefault, diff.SourceDefault, diff.ParamType))
						}
						content.WriteString("\n")
					}
				}
			}
		}
	}

	// Focus Parameters
	if len(result.FocusParams) > 0 {
		content.WriteString("## Focus Parameters\n\n")
		content.WriteString("| Component | Parameter | Current | Modified | Will Change |\n")
		content.WriteString("|-----------|-----------|---------|----------|-------------|\n")
		for compType, params := range result.FocusParams {
			for paramName, info := range params {
				content.WriteString(fmt.Sprintf("| %s | `%s` | %v | %v | %v |\n",
					compType, paramName, info.CurrentValue, info.IsModified, info.WillChange))
			}
		}
		content.WriteString("\n")
	}

	return content.String(), nil
}
