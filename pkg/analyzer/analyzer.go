// Package analyzer provides risk analysis logic for upgrade precheck
// Analyzer performs analysis based on rules, which define what data to collect and how to compare
package analyzer

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
)

// AnalysisOptions contains options for analysis
type AnalysisOptions struct {
	// Rules is the list of rules to apply. If empty, default rules will be used
	Rules []rules.Rule `json:"rules,omitempty"`
}

// Analyzer performs comprehensive risk analysis on cluster snapshots based on rules
type Analyzer struct {
	options *AnalysisOptions
	rules   []rules.Rule
}

// NewAnalyzer creates a new analyzer with the provided rules
// If no rules are provided, default rules will be used
func NewAnalyzer(options *AnalysisOptions) *Analyzer {
	if options == nil {
		options = &AnalysisOptions{}
	}

	// Use provided rules or default rules
	ruleList := options.Rules
	if len(ruleList) == 0 {
		ruleList = getDefaultRules()
	}

	return &Analyzer{
		options: options,
		rules:   ruleList,
	}
}

// GetDataRequirements returns the merged data requirements from all rules
// This is used by the analyzer to determine what data to load from knowledge base
func (a *Analyzer) GetDataRequirements() rules.DataSourceRequirement {
	return a.collectDataRequirements()
}

// GetCollectionRequirements returns the collection requirements for the runtime collector
// This extracts only the SourceClusterRequirements from the full data requirements
// since the collector only needs to know what to collect from the running cluster
// Returns a struct compatible with collector/runtime.CollectDataRequirements
func (a *Analyzer) GetCollectionRequirements() CollectionRequirements {
	dataReqs := a.collectDataRequirements()
	return CollectionRequirements{
		Components:          dataReqs.SourceClusterRequirements.Components,
		NeedConfig:          dataReqs.SourceClusterRequirements.NeedConfig,
		NeedSystemVariables: dataReqs.SourceClusterRequirements.NeedSystemVariables,
		NeedAllTikvNodes:    dataReqs.SourceClusterRequirements.NeedAllTikvNodes,
	}
}

// CollectionRequirements defines what data needs to be collected from the cluster
// This is a simplified version of DataSourceRequirement that only includes cluster collection needs
// The structure matches collector/runtime.CollectDataRequirements for compatibility
type CollectionRequirements struct {
	// Components specifies which components' data is needed
	Components []string `json:"components"`
	// NeedConfig indicates if configuration parameters are needed
	NeedConfig bool `json:"need_config"`
	// NeedSystemVariables indicates if system variables are needed (mainly for TiDB)
	NeedSystemVariables bool `json:"need_system_variables"`
	// NeedAllTikvNodes indicates if all TiKV nodes' data is needed (for consistency checks)
	NeedAllTikvNodes bool `json:"need_all_tikv_nodes"`
}

// getDefaultRules returns the default set of rules
func getDefaultRules() []rules.Rule {
	return []rules.Rule{
		rules.NewUserModifiedParamsRule(),
		rules.NewUpgradeDifferencesRule(),
		rules.NewTikvConsistencyRule(),
	}
}

// Analyze performs comprehensive analysis on a cluster snapshot based on rules
// It:
// 1. Collects data requirements from all rules
// 2. Loads necessary data from knowledge base
// 3. Executes all rules
// 4. Returns analysis results organized by rule category
func (a *Analyzer) Analyze(
	ctx context.Context,
	snapshot *collector.ClusterSnapshot,
	sourceVersion, targetVersion string,
	sourceKB, targetKB map[string]interface{},
) (*AnalysisResult, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("snapshot cannot be nil")
	}

	// Step 1: Collect data requirements from all rules
	// Merge requirements from all rules to determine what data needs to be loaded
	dataReqs := a.collectDataRequirements()

	// Step 2: Load knowledge base data based on merged requirements
	// Load once, all rules can reuse the same data
	sourceDefaults, sourceBootstrapVersions := a.loadSourceKB(sourceKB, dataReqs)
	targetDefaults, targetBootstrapVersions := a.loadTargetKB(targetKB, dataReqs)

	// Step 2.1: Build component mapping and validate one-to-one correspondence
	// Map component types to actual component instances in snapshot
	// This ensures source KB defaults and runtime parameters are properly matched
	componentMapping := a.buildComponentMapping(snapshot, sourceDefaults)

	// Validate and report any mismatches (KB has defaults but runtime doesn't, or vice versa)
	mismatchResults := a.validateComponentMapping(snapshot, sourceDefaults, componentMapping, sourceVersion)

	// Load upgrade logic (only need to load once, contains all historical changes)
	// Upgrade logic is version-agnostic and contains all changes with version tags
	upgradeLogic := a.loadUpgradeLogic(sourceKB, targetKB, dataReqs)
	// Debug: log loaded components
	upgradeLogicKeys := make([]string, 0, len(upgradeLogic))
	for k := range upgradeLogic {
		upgradeLogicKeys = append(upgradeLogicKeys, k)
	}
	fmt.Printf("[DEBUG Analyzer] Loaded upgrade_logic for components: %v\n", upgradeLogicKeys)

	// Step 3: Create shared rule context with loaded data
	// All rules share the same context, but each rule only accesses data it needs
	// Extract bootstrap versions for TiDB (most important for upgrade logic filtering)
	sourceBootstrapVersion := sourceBootstrapVersions["tidb"]
	targetBootstrapVersion := targetBootstrapVersions["tidb"]
	fmt.Printf("[DEBUG Analyzer] Bootstrap versions - Source: %d, Target: %d\n", sourceBootstrapVersion, targetBootstrapVersion)

	ruleCtx := rules.NewRuleContext(
		snapshot,
		sourceVersion,
		targetVersion,
		sourceDefaults,
		targetDefaults,
		upgradeLogic,
		sourceBootstrapVersion,
		targetBootstrapVersion,
	)

	// Step 4: Execute all rules with the shared context
	ruleRunner := rules.NewRuleRunner(a.rules)
	checkResults, err := ruleRunner.Run(ctx, ruleCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to run rules: %w", err)
	}

	// Step 5: Merge mismatch results with rule results
	allCheckResults := append(checkResults, mismatchResults...)

	// Step 6: Organize results by category
	result := a.organizeResults(allCheckResults, sourceVersion, targetVersion)

	return result, nil
}

// collectDataRequirements collects data requirements from all rules
// and merges them to determine what data needs to be loaded
func (a *Analyzer) collectDataRequirements() rules.DataSourceRequirement {
	// Start with empty requirements
	merged := rules.DataSourceRequirement{}

	// Merge requirements from all rules
	for _, rule := range a.rules {
		req := rule.DataRequirements()

		// Merge source cluster requirements
		merged.SourceClusterRequirements.Components = mergeStringSlices(
			merged.SourceClusterRequirements.Components,
			req.SourceClusterRequirements.Components,
		)
		merged.SourceClusterRequirements.NeedConfig = merged.SourceClusterRequirements.NeedConfig || req.SourceClusterRequirements.NeedConfig
		merged.SourceClusterRequirements.NeedSystemVariables = merged.SourceClusterRequirements.NeedSystemVariables || req.SourceClusterRequirements.NeedSystemVariables
		merged.SourceClusterRequirements.NeedAllTikvNodes = merged.SourceClusterRequirements.NeedAllTikvNodes || req.SourceClusterRequirements.NeedAllTikvNodes

		// Merge source KB requirements
		merged.SourceKBRequirements.Components = mergeStringSlices(
			merged.SourceKBRequirements.Components,
			req.SourceKBRequirements.Components,
		)
		merged.SourceKBRequirements.NeedConfigDefaults = merged.SourceKBRequirements.NeedConfigDefaults || req.SourceKBRequirements.NeedConfigDefaults
		merged.SourceKBRequirements.NeedSystemVariables = merged.SourceKBRequirements.NeedSystemVariables || req.SourceKBRequirements.NeedSystemVariables
		merged.SourceKBRequirements.NeedUpgradeLogic = merged.SourceKBRequirements.NeedUpgradeLogic || req.SourceKBRequirements.NeedUpgradeLogic
		// Note: upgrade logic is loaded once (not per version) but we need to know if any rule needs it

		// Merge target KB requirements
		merged.TargetKBRequirements.Components = mergeStringSlices(
			merged.TargetKBRequirements.Components,
			req.TargetKBRequirements.Components,
		)
		merged.TargetKBRequirements.NeedConfigDefaults = merged.TargetKBRequirements.NeedConfigDefaults || req.TargetKBRequirements.NeedConfigDefaults
		merged.TargetKBRequirements.NeedSystemVariables = merged.TargetKBRequirements.NeedSystemVariables || req.TargetKBRequirements.NeedSystemVariables
		merged.TargetKBRequirements.NeedUpgradeLogic = merged.TargetKBRequirements.NeedUpgradeLogic || req.TargetKBRequirements.NeedUpgradeLogic
		// Note: upgrade logic is loaded once (not per version) but we need to know if any rule needs it
	}

	return merged
}

// loadKBFromRequirements is a generic function to load knowledge base data based on requirements
// It extracts config defaults and system variables for specified components
// Always loads bootstrap_version if available (needed for upgrade logic filtering)
// Returns: defaults map and bootstrap version map
func (a *Analyzer) loadKBFromRequirements(
	kb map[string]interface{},
	components []string,
	needConfigDefaults, needSystemVariables bool,
) (map[string]map[string]interface{}, map[string]int64) {
	defaults := make(map[string]map[string]interface{})
	bootstrapVersions := make(map[string]int64)

	// Always load bootstrap_version if available (needed for upgrade logic filtering)
	// Even if we don't need config defaults or system variables
	for _, comp := range components {
		if compKB, ok := kb[comp].(map[string]interface{}); ok {
			// Load bootstrap_version (always load if available)
			if bootstrapVersion, ok := compKB["bootstrap_version"]; ok {
				fmt.Printf("[DEBUG loadKBFromRequirements] Found bootstrap_version for %s: %v\n", comp, bootstrapVersion)
				var version int64
				switch v := bootstrapVersion.(type) {
				case int64:
					version = v
				case float64:
					version = int64(v)
				case int:
					version = int64(v)
				default:
					if str, ok := v.(string); ok {
						if parsed, err := strconv.ParseInt(str, 10, 64); err == nil {
							version = parsed
						}
					}
				}
				if version > 0 {
					bootstrapVersions[comp] = version
					fmt.Printf("[DEBUG loadKBFromRequirements] Loaded bootstrap_version for %s: %d\n", comp, version)
				} else {
					fmt.Printf("[DEBUG loadKBFromRequirements] bootstrap_version for %s is 0 or invalid: %v\n", comp, bootstrapVersion)
				}
			} else {
				fmt.Printf("[DEBUG loadKBFromRequirements] No bootstrap_version found for %s in KB\n", comp)
			}
		} else {
			fmt.Printf("[DEBUG loadKBFromRequirements] Component %s not found in KB\n", comp)
		}
	}

	if !needConfigDefaults && !needSystemVariables {
		return defaults, bootstrapVersions
	}

	// Load data for each required component
	for _, comp := range components {
		defaults[comp] = make(map[string]interface{})

		if compKB, ok := kb[comp].(map[string]interface{}); ok {
			// Load config defaults
			if needConfigDefaults {
				if configDefaults, ok := compKB["config_defaults"].(map[string]interface{}); ok {
					for k, v := range configDefaults {
						defaults[comp][k] = v
					}
				}
			}

			// Load system variables
			if needSystemVariables {
				if systemVars, ok := compKB["system_variables"].(map[string]interface{}); ok {
					for k, v := range systemVars {
						// Prefix with "sysvar:" to distinguish from config params
						defaults[comp]["sysvar:"+k] = v
					}
				}
			}

			// Bootstrap version is already loaded above (always load if available)
		}
	}

	return defaults, bootstrapVersions
}

// loadSourceKB loads source version knowledge base data based on requirements
// Returns: defaults map and bootstrap version map
func (a *Analyzer) loadSourceKB(kb map[string]interface{}, req rules.DataSourceRequirement) (map[string]map[string]interface{}, map[string]int64) {
	return a.loadKBFromRequirements(
		kb,
		req.SourceKBRequirements.Components,
		req.SourceKBRequirements.NeedConfigDefaults,
		req.SourceKBRequirements.NeedSystemVariables,
	)
}

// loadTargetKB loads target version knowledge base data based on requirements
// Returns: defaults map and bootstrap version map
func (a *Analyzer) loadTargetKB(kb map[string]interface{}, req rules.DataSourceRequirement) (map[string]map[string]interface{}, map[string]int64) {
	return a.loadKBFromRequirements(
		kb,
		req.TargetKBRequirements.Components,
		req.TargetKBRequirements.NeedConfigDefaults,
		req.TargetKBRequirements.NeedSystemVariables,
	)
}

// buildComponentMapping creates a map from component type to actual component instance
// This ensures one-to-one correspondence between source KB defaults and runtime components
// Returns: map[componentType]componentName (e.g., map["tidb"]"tidb", map["tikv"]"tikv-192-168-1-100-20160")
func (a *Analyzer) buildComponentMapping(snapshot *collector.ClusterSnapshot, sourceDefaults map[string]map[string]interface{}) map[string]string {
	mapping := make(map[string]string)

	// Build mapping for each component type in source defaults
	for compType := range sourceDefaults {
		// Try to find component by exact type match
		for name, comp := range snapshot.Components {
			if string(comp.Type) == compType {
				mapping[compType] = name
				break
			}
		}

		// If not found, try prefix matching for TiKV/TiFlash
		if _, found := mapping[compType]; !found {
			for name := range snapshot.Components {
				if (compType == "tikv" && strings.HasPrefix(name, "tikv")) ||
					(compType == "tiflash" && strings.HasPrefix(name, "tiflash")) {
					// Use the first instance found
					if mapping[compType] == "" {
						mapping[compType] = name
					}
				}
			}
		}
	}

	return mapping
}

// validateComponentMapping validates one-to-one correspondence between source KB and runtime
// Reports mismatches where KB has defaults but runtime doesn't have the component/parameter
// or vice versa
func (a *Analyzer) validateComponentMapping(
	snapshot *collector.ClusterSnapshot,
	sourceDefaults map[string]map[string]interface{},
	componentMapping map[string]string,
	sourceVersion string,
) []rules.CheckResult {
	var results []rules.CheckResult

	// Check 1: KB has defaults for a component, but runtime doesn't have it
	for compType, defaults := range sourceDefaults {
		if compName, ok := componentMapping[compType]; !ok || compName == "" {
			// KB has defaults but runtime doesn't have this component
			results = append(results, rules.CheckResult{
				RuleID:      "COMPONENT_MISMATCH",
				Category:    "validation",
				Component:   compType,
				Severity:    "warning",
				Message:     fmt.Sprintf("Source KB (v%s) has defaults for %s, but component not found in runtime cluster", sourceVersion, compType),
				Details:     fmt.Sprintf("Source KB contains %d parameters for %s, but no corresponding component found in cluster snapshot", len(defaults), compType),
				Suggestions: []string{"Verify cluster topology configuration", "Check if component is actually deployed"},
			})
			continue
		}

		// Check 2: For each component, validate parameter correspondence
		comp, exists := snapshot.Components[componentMapping[compType]]
		if !exists {
			continue
		}

		// Build runtime parameter map for O(1) lookup (avoid nested loops)
		runtimeConfigMap := make(map[string]bool)
		runtimeVarsMap := make(map[string]bool)

		for paramName := range comp.Config {
			runtimeConfigMap[paramName] = true
		}
		for varName := range comp.Variables {
			runtimeVarsMap[varName] = true
		}

		// Check KB defaults against runtime (single loop, O(1) lookup)
		for paramName := range defaults {
			isSystemVar := strings.HasPrefix(paramName, "sysvar:")
			varName := paramName
			if isSystemVar {
				varName = strings.TrimPrefix(paramName, "sysvar:")
			}

			if isSystemVar {
				if !runtimeVarsMap[varName] {
					// KB has system variable default, but runtime doesn't have it
					results = append(results, rules.CheckResult{
						RuleID:        "PARAMETER_MISMATCH",
						Category:      "validation",
						Component:     compType,
						ParameterName: varName,
						ParamType:     "system_variable",
						Severity:      "warning",
						Message:       fmt.Sprintf("Source KB (v%s) has default for system variable %s in %s, but not found in runtime", sourceVersion, varName, compType),
						Details:       fmt.Sprintf("System variable %s exists in source KB defaults but not in runtime cluster", varName),
						Suggestions: []string{
							"Verify system variable name spelling",
							"Check if variable was removed in this version",
						},
					})
				}
			} else {
				if !runtimeConfigMap[paramName] {
					// KB has config parameter default, but runtime doesn't have it
					results = append(results, rules.CheckResult{
						RuleID:        "PARAMETER_MISMATCH",
						Category:      "validation",
						Component:     compType,
						ParameterName: paramName,
						ParamType:     "config",
						Severity:      "warning",
						Message:       fmt.Sprintf("Source KB (v%s) has default for parameter %s in %s, but not found in runtime", sourceVersion, paramName, compType),
						Details:       fmt.Sprintf("Parameter %s exists in source KB defaults but not in runtime cluster", paramName),
						Suggestions: []string{
							"Verify parameter name spelling",
							"Check if parameter was removed in this version",
						},
					})
				}
			}
		}
	}

	return results
}

// loadUpgradeLogic loads upgrade logic from knowledge base
// Upgrade logic is version-agnostic and contains all historical changes with version tags
// We prefer to load from target KB, but fallback to source KB if target doesn't have it
// Since upgrade logic contains all historical changes, we only need to load it once
func (a *Analyzer) loadUpgradeLogic(sourceKB, targetKB map[string]interface{}, req rules.DataSourceRequirement) map[string]interface{} {
	upgradeLogic := make(map[string]interface{})

	// Check if any rule needs upgrade logic
	needUpgradeLogic := req.SourceKBRequirements.NeedUpgradeLogic || req.TargetKBRequirements.NeedUpgradeLogic
	fmt.Printf("[DEBUG loadUpgradeLogic] needUpgradeLogic: %v (Source: %v, Target: %v)\n", needUpgradeLogic, req.SourceKBRequirements.NeedUpgradeLogic, req.TargetKBRequirements.NeedUpgradeLogic)
	if !needUpgradeLogic {
		fmt.Printf("[DEBUG loadUpgradeLogic] No rule needs upgrade logic, returning empty\n")
		return upgradeLogic
	}

	// Get all components that need upgrade logic
	components := mergeStringSlices(req.SourceKBRequirements.Components, req.TargetKBRequirements.Components)
	fmt.Printf("[DEBUG loadUpgradeLogic] Components to check: %v\n", components)

	// Load upgrade logic for each component
	// Prefer target KB, fallback to source KB
	// Since upgrade logic is the same across versions (contains all historical changes),
	// it doesn't matter which KB we load from, but we prefer target KB
	for _, comp := range components {
		// Try target KB first
		if compKB, ok := targetKB[comp].(map[string]interface{}); ok {
			fmt.Printf("[DEBUG loadUpgradeLogic] Found component %s in target KB\n", comp)
			if upgrade, ok := compKB["upgrade_logic"].(map[string]interface{}); ok {
				upgradeLogic[comp] = upgrade
				fmt.Printf("[DEBUG loadUpgradeLogic] ✅ Loaded upgrade_logic for %s from target KB\n", comp)
				continue
			} else {
				fmt.Printf("[DEBUG loadUpgradeLogic] Component %s in target KB but upgrade_logic type is %T (not map[string]interface{})\n", comp, compKB["upgrade_logic"])
			}
		} else {
			fmt.Printf("[DEBUG loadUpgradeLogic] Component %s not found in target KB\n", comp)
		}

		// Fallback to source KB
		if compKB, ok := sourceKB[comp].(map[string]interface{}); ok {
			fmt.Printf("[DEBUG loadUpgradeLogic] Found component %s in source KB\n", comp)
			if upgrade, ok := compKB["upgrade_logic"].(map[string]interface{}); ok {
				upgradeLogic[comp] = upgrade
				fmt.Printf("[DEBUG loadUpgradeLogic] ✅ Loaded upgrade_logic for %s from source KB\n", comp)
			} else {
				fmt.Printf("[DEBUG loadUpgradeLogic] Component %s in source KB but upgrade_logic type is %T (not map[string]interface{})\n", comp, compKB["upgrade_logic"])
			}
		} else {
			fmt.Printf("[DEBUG loadUpgradeLogic] Component %s not found in source KB\n", comp)
		}
	}

	return upgradeLogic
}

// organizeResults organizes check results by category for reporter
func (a *Analyzer) organizeResults(checkResults []rules.CheckResult, sourceVersion, targetVersion string) *AnalysisResult {
	result := &AnalysisResult{
		SourceVersion:       sourceVersion,
		TargetVersion:       targetVersion,
		ModifiedParams:      make(map[string]map[string]ModifiedParamInfo),
		TikvInconsistencies: make(map[string][]InconsistentNode),
		UpgradeDifferences:  make(map[string]map[string]UpgradeDifference),
		ForcedChanges:       make(map[string]map[string]ForcedChange),
		CheckResults:        []rules.CheckResult{},
		Statistics:          Statistics{},
	}

	// Filter out statistics CheckResults and extract statistics
	var filteredResults []rules.CheckResult
	for _, check := range checkResults {
		// Check if this is a statistics CheckResult
		if check.ParameterName == "__statistics__" && strings.HasSuffix(check.RuleID, "_STATS") {
			// Extract statistics from Description
			// Format: "Compared X parameters, skipped Y (source == target)"
			var totalCompared, totalSkipped int
			fmt.Sscanf(check.Description, "Compared %d parameters, skipped %d", &totalCompared, &totalSkipped)
			result.Statistics.TotalParametersCompared += totalCompared
			result.Statistics.ParametersSkipped += totalSkipped
			result.Statistics.ParametersWithDifferences = totalCompared - totalSkipped
			continue // Skip this CheckResult
		}
		filteredResults = append(filteredResults, check)
	}
	result.CheckResults = filteredResults

	// Organize results by category
	for _, check := range filteredResults {
		switch check.Category {
		case "user_modified":
			a.addModifiedParam(result, check)
		case "upgrade_difference":
			if check.ForcedValue != nil {
				a.addForcedChange(result, check)
			} else {
				a.addUpgradeDifference(result, check)
			}
		case "consistency":
			a.addTikvInconsistency(result, check)
		}
	}

	return result
}

// Helper functions to add results to appropriate structures

func (a *Analyzer) addModifiedParam(result *AnalysisResult, check rules.CheckResult) {
	if result.ModifiedParams[check.Component] == nil {
		result.ModifiedParams[check.Component] = make(map[string]ModifiedParamInfo)
	}
	result.ModifiedParams[check.Component][check.ParameterName] = ModifiedParamInfo{
		Component:     check.Component,
		ParamName:     check.ParameterName,
		CurrentValue:  check.CurrentValue,
		SourceDefault: check.SourceDefault,
		ParamType:     check.ParamType,
	}
}

func (a *Analyzer) addUpgradeDifference(result *AnalysisResult, check rules.CheckResult) {
	if result.UpgradeDifferences[check.Component] == nil {
		result.UpgradeDifferences[check.Component] = make(map[string]UpgradeDifference)
	}
	result.UpgradeDifferences[check.Component][check.ParameterName] = UpgradeDifference{
		Component:     check.Component,
		ParamName:     check.ParameterName,
		CurrentValue:  check.CurrentValue,
		TargetDefault: check.TargetDefault,
		SourceDefault: check.SourceDefault,
		ParamType:     check.ParamType,
	}
}

func (a *Analyzer) addForcedChange(result *AnalysisResult, check rules.CheckResult) {
	if result.ForcedChanges[check.Component] == nil {
		result.ForcedChanges[check.Component] = make(map[string]ForcedChange)
	}
	result.ForcedChanges[check.Component][check.ParameterName] = ForcedChange{
		Component:     check.Component,
		ParamName:     check.ParameterName,
		CurrentValue:  check.CurrentValue,
		ForcedValue:   check.ForcedValue,
		SourceDefault: check.SourceDefault,
		ParamType:     check.ParamType,
		Summary:       check.Details,
	}
}

func (a *Analyzer) addTikvInconsistency(result *AnalysisResult, check rules.CheckResult) {
	// For TiKV consistency, we need to extract node information from the details
	// The actual node information is stored in the check result's metadata or details
	// For now, we'll store the parameter name and let reporter handle the details
	// In a full implementation, we might need to parse the details string or use metadata

	// Check if we already have this parameter
	if _, exists := result.TikvInconsistencies[check.ParameterName]; !exists {
		// Initialize with empty slice - the actual node details are in check.Details
		result.TikvInconsistencies[check.ParameterName] = []InconsistentNode{}
	}

	// Note: Full node details are available in check.Details and check.Metadata
	// Reporter will display them from the CheckResult
}

// Helper function to merge string slices without duplicates
func mergeStringSlices(slice1, slice2 []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, s := range slice1 {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	for _, s := range slice2 {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}
