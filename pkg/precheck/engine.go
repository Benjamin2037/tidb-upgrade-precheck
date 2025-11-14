package precheck

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Severity describes the seriousness of a precheck finding.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityBlocker Severity = "blocker"
	SeverityError   Severity = "error"
)

// Snapshot captures key details about the current cluster and desired target.
type Snapshot struct {
	SourceVersion  string                       `json:"source_version"`
	TargetVersion  string                       `json:"target_version"`
	Components     map[string]ComponentSnapshot `json:"components,omitempty"`
	GlobalSysVars  map[string]string            `json:"global_sysvars,omitempty"`
	Config         map[string]any               `json:"config,omitempty"`
	Metadata       map[string]any               `json:"metadata,omitempty"`
	Tags           map[string]string            `json:"tags,omitempty"`
	AdditionalInfo map[string]map[string]any    `json:"additional_info,omitempty"`
}

// ComponentSnapshot represents the state of a single component.
type ComponentSnapshot struct {
	Version    string            `json:"version"`
	Config     map[string]any    `json:"config,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// ReportItem represents a single rule outcome.
type ReportItem struct {
	Rule        string   `json:"rule"`
	Severity    Severity `json:"severity"`
	Message     string   `json:"message"`
	Details     []string `json:"details,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	Metadata    any      `json:"metadata,omitempty"`
}

// Summary aggregates counts by severity.
type Summary struct {
	Total      int              `json:"total"`
	BySeverity map[Severity]int `json:"by_severity"`
	Blocking   int              `json:"blocking"`
	Warnings   int              `json:"warnings"`
	Infos      int              `json:"infos"`
}

// Report is the aggregated output produced by the Engine run.
type Report struct {
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	Items      []ReportItem `json:"items"`
	Summary    Summary      `json:"summary"`
	Errors     []string     `json:"errors,omitempty"`
}

// HasBlocking reports whether any blocking issues were detected.
func (r *Report) HasBlocking() bool {
	return r.Summary.Blocking > 0
}

// Rule defines a single check.
type Rule interface {
	Name() string
	Evaluate(context.Context, Snapshot) ([]ReportItem, error)
}

// RuleFunc adapts a plain function into a Rule.
type RuleFunc struct {
	name string
	fn   func(context.Context, Snapshot) ([]ReportItem, error)
}

// NewRuleFunc builds a Rule from a function.
func NewRuleFunc(name string, fn func(context.Context, Snapshot) ([]ReportItem, error)) Rule {
	sanitized := strings.TrimSpace(name)
	if sanitized == "" {
		panic("rule name cannot be empty")
	}
	if fn == nil {
		panic("rule func cannot be nil")
	}
	return &RuleFunc{name: sanitized, fn: fn}
}

// Name returns the rule name.
func (r *RuleFunc) Name() string { return r.name }

// Evaluate runs the wrapped function.
func (r *RuleFunc) Evaluate(ctx context.Context, snapshot Snapshot) ([]ReportItem, error) {
	return r.fn(ctx, snapshot)
}

// Engine runs every registered rule and collects the results.
type Engine struct {
	rules []Rule
}

// NewEngine constructs a new Engine instance.
func NewEngine(rules ...Rule) *Engine {
	return &Engine{rules: append([]Rule(nil), rules...)}
}

// Register appends a rule to the engine.
func (e *Engine) Register(rule Rule) {
	if rule == nil {
		panic("rule cannot be nil")
	}
	e.rules = append(e.rules, rule)
}

// Rules returns a copy of the registered rules.
func (e *Engine) Rules() []Rule {
	return append([]Rule(nil), e.rules...)
}

// Run executes the rules and returns the aggregated report.
func (e *Engine) Run(ctx context.Context, snapshot Snapshot) Report {
	start := time.Now()
	items := make([]ReportItem, 0, len(e.rules))
	summary := Summary{BySeverity: make(map[Severity]int)}
	errs := make([]string, 0)

	for _, rule := range e.rules {
		if ctx.Err() != nil {
			errs = append(errs, fmt.Sprintf("precheck cancelled before rule %s", rule.Name()))
			break
		}

		reports, err := rule.Evaluate(ctx, snapshot)
		if err != nil {
			errs = append(errs, fmt.Sprintf("rule %s failed: %v", rule.Name(), err))
			items = append(items, ReportItem{
				Rule:     rule.Name(),
				Severity: SeverityError,
				Message:  "rule execution error",
				Details:  []string{err.Error()},
			})
			incrementSummary(&summary, SeverityError)
			continue
		}

		for _, item := range reports {
			if item.Rule == "" {
				item.Rule = rule.Name()
			}
			items = append(items, item)
			incrementSummary(&summary, item.Severity)
		}
	}

	summary.Total = len(items)

	return Report{
		StartedAt:  start,
		FinishedAt: time.Now(),
		Items:      items,
		Summary:    summary,
		Errors:     errs,
	}
}

func incrementSummary(summary *Summary, sev Severity) {
	summary.BySeverity[sev]++
	switch sev {
	case SeverityBlocker:
		summary.Blocking++
	case SeverityWarning:
		summary.Warnings++
	case SeverityInfo:
		summary.Infos++
	}
}

// ValidateSnapshot performs basic sanity checks on a snapshot.
func ValidateSnapshot(snapshot Snapshot) error {
	if strings.TrimSpace(snapshot.TargetVersion) == "" {
		return errors.New("target version is required")
	}
	if snapshot.Components == nil {
		snapshot.Components = make(map[string]ComponentSnapshot)
	}
	return nil
}
