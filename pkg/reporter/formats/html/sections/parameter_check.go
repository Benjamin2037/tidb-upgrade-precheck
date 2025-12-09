package sections

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats"
)

// ParameterCheckSection renders parameter check results in HTML format
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

// Render renders the section content in HTML format
func (s *ParameterCheckSection) Render(format formats.Format, result *analyzer.AnalysisResult) (string, error) {
	var content strings.Builder

	// Modified Parameters
	if len(result.ModifiedParams) > 0 {
		content.WriteString("<h2>1. User Modified Parameters</h2>\n")
		content.WriteString("<p>Parameters that have been modified from source version defaults.</p>\n")

		components := []string{"tidb", "pd", "tikv", "tiflash"}
		for _, compType := range components {
			if params, ok := result.ModifiedParams[compType]; ok && len(params) > 0 {
				content.WriteString(fmt.Sprintf("<h3>%s Component</h3>\n", strings.ToUpper(compType)))
				content.WriteString("<table>\n")
				content.WriteString("<tr><th>Parameter</th><th>Current Value</th><th>Source Default</th><th>Type</th></tr>\n")
				for paramName, info := range params {
					content.WriteString(fmt.Sprintf("<tr><td><code>%s</code></td><td>%v</td><td>%v</td><td>%s</td></tr>\n",
						paramName, info.CurrentValue, info.SourceDefault, info.ParamType))
				}
				content.WriteString("</table>\n")
			}
		}
	}

	// TiKV Inconsistencies
	if len(result.TikvInconsistencies) > 0 {
		content.WriteString("<h2>2. TiKV Parameter Inconsistencies</h2>\n")
		content.WriteString("<p>TiKV nodes with inconsistent parameter values.</p>\n")
		for paramName, nodes := range result.TikvInconsistencies {
			content.WriteString(fmt.Sprintf("<h3>Parameter: <code>%s</code></h3>\n", paramName))
			content.WriteString("<table>\n")
			content.WriteString("<tr><th>Node Address</th><th>Value</th></tr>\n")
			for _, node := range nodes {
				content.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%v</td></tr>\n", node.NodeAddress, node.Value))
			}
			content.WriteString("</table>\n")
		}
	}

	// Upgrade Differences
	if len(result.UpgradeDifferences) > 0 || len(result.ForcedChanges) > 0 {
		content.WriteString("<h2>3. Upgrade Differences</h2>\n")
		content.WriteString("<p>Parameters that will differ after upgrade.</p>\n")

		// Forced Changes
		if len(result.ForcedChanges) > 0 {
			content.WriteString("<h3>Forced Changes (Critical)</h3>\n")
			content.WriteString("<p>⚠️ <strong>These parameters will be forcibly changed during upgrade and cannot be prevented.</strong></p>\n")
			content.WriteString("<table>\n")
			content.WriteString("<tr><th>Component</th><th>Parameter</th><th>Current</th><th>Forced To</th><th>Summary</th></tr>\n")
			for compType, params := range result.ForcedChanges {
				for paramName, change := range params {
					content.WriteString(fmt.Sprintf("<tr><td>%s</td><td><code>%s</code></td><td>%v</td><td>%v</td><td>%s</td></tr>\n",
						compType, paramName, change.CurrentValue, change.ForcedValue, change.Summary))
				}
			}
			content.WriteString("</table>\n")
		}

		// Other Upgrade Differences
		if len(result.UpgradeDifferences) > 0 {
			content.WriteString("<h3>Other Upgrade Differences</h3>\n")
			components := []string{"tidb", "pd", "tikv", "tiflash"}
			for _, compType := range components {
				if params, ok := result.UpgradeDifferences[compType]; ok && len(params) > 0 {
					content.WriteString(fmt.Sprintf("<h4>%s Component</h4>\n", strings.ToUpper(compType)))

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
						content.WriteString("<p>ℹ️ <strong>Default Value Changes (Info):</strong></p>\n")
						content.WriteString("<p>The following parameters have default value changes in the target version, but your current configuration will be preserved during upgrade.</p>\n")
						content.WriteString("<table>\n")
						content.WriteString("<tr><th>Parameter</th><th>Current (Preserved)</th><th>Target Default</th><th>Source Default</th><th>Type</th></tr>\n")
						for _, paramName := range pdInfoParams {
							diff := params[paramName]
							content.WriteString(fmt.Sprintf("<tr><td><code>%s</code></td><td>%v</td><td>%v</td><td>%v</td><td>%s</td></tr>\n",
								paramName, diff.CurrentValue, diff.TargetDefault, diff.SourceDefault, diff.ParamType))
						}
						content.WriteString("</table>\n")
					}

					// Display other upgrade differences
					if len(otherParams) > 0 {
						if len(pdInfoParams) > 0 && compType == "pd" {
							content.WriteString("<h5>Other Upgrade Differences</h5>\n")
						}
						content.WriteString("<table>\n")
						content.WriteString("<tr><th>Parameter</th><th>Current</th><th>Target Default</th><th>Source Default</th><th>Type</th></tr>\n")
						for _, paramName := range otherParams {
							diff := params[paramName]
							content.WriteString(fmt.Sprintf("<tr><td><code>%s</code></td><td>%v</td><td>%v</td><td>%v</td><td>%s</td></tr>\n",
								paramName, diff.CurrentValue, diff.TargetDefault, diff.SourceDefault, diff.ParamType))
						}
						content.WriteString("</table>\n")
					}
				}
			}
		}
	}

	// Focus Parameters
	if len(result.FocusParams) > 0 {
		content.WriteString("<h2>Focus Parameters</h2>\n")
		content.WriteString("<table>\n")
		content.WriteString("<tr><th>Component</th><th>Parameter</th><th>Current</th><th>Modified</th><th>Will Change</th></tr>\n")
		for compType, params := range result.FocusParams {
			for paramName, info := range params {
				content.WriteString(fmt.Sprintf("<tr><td>%s</td><td><code>%s</code></td><td>%v</td><td>%v</td><td>%v</td></tr>\n",
					compType, paramName, info.CurrentValue, info.IsModified, info.WillChange))
			}
		}
		content.WriteString("</table>\n")
	}

	return content.String(), nil
}
