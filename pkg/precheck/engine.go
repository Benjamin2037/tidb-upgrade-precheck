package precheck

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Severity 描述预检项的严重程度。
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityBlocker Severity = "blocker"
	SeverityError   Severity = "error"
)

// Snapshot 汇总了当前集群与目标版本的关键信息。
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

// ComponentSnapshot 描述集群中某个组件的现状。
type ComponentSnapshot struct {
	Version    string            `json:"version"`
	Config     map[string]any    `json:"config,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// ReportItem 表示单条检查结论。
type ReportItem struct {
	Rule        string   `json:"rule"`
	Severity    Severity `json:"severity"`
	Message     string   `json:"message"`
	Details     []string `json:"details,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	Metadata    any      `json:"metadata,omitempty"`
}

// Summary 汇总统计信息。
type Summary struct {
	Total      int              `json:"total"`
	BySeverity map[Severity]int `json:"by_severity"`
	Blocking   int              `json:"blocking"`
	Warnings   int              `json:"warnings"`
	Infos      int              `json:"infos"`
}

// Report 是 Engine 运行后的完整结果。
type Report struct {
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	Items      []ReportItem `json:"items"`
	Summary    Summary      `json:"summary"`
	Errors     []string     `json:"errors,omitempty"`
}

// HasBlocking 返回报告中是否存在阻断项。
func (r *Report) HasBlocking() bool {
	return r.Summary.Blocking > 0
}

// Rule 定义了单个检查规则。
type Rule interface {
	Name() string
	Evaluate(context.Context, Snapshot) ([]ReportItem, error)
}

// RuleFunc 是 Rule 的便捷适配器。
type RuleFunc struct {
	name string
	fn   func(context.Context, Snapshot) ([]ReportItem, error)
}

// NewRuleFunc 创建一个基于函数的规则。
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

// Name 返回规则名称。
func (r *RuleFunc) Name() string { return r.name }

// Evaluate 执行规则逻辑。
func (r *RuleFunc) Evaluate(ctx context.Context, snapshot Snapshot) ([]ReportItem, error) {
	return r.fn(ctx, snapshot)
}

// Engine 负责串行运行所有规则并聚合结果。
type Engine struct {
	rules []Rule
}

// NewEngine 构造一个新的 Engine。
func NewEngine(rules ...Rule) *Engine {
	return &Engine{rules: append([]Rule(nil), rules...)}
}

// Register 向 Engine 动态添加规则。
func (e *Engine) Register(rule Rule) {
	if rule == nil {
		panic("rule cannot be nil")
	}
	e.rules = append(e.rules, rule)
}

// Rules 返回当前注册的规则列表副本。
func (e *Engine) Rules() []Rule {
	return append([]Rule(nil), e.rules...)
}

// Run 执行所有规则并返回汇总报告。
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

// ValidateSnapshot 做一些基础合法性检查，供外部使用。
func ValidateSnapshot(snapshot Snapshot) error {
	if strings.TrimSpace(snapshot.TargetVersion) == "" {
		return errors.New("target version is required")
	}
	if snapshot.Components == nil {
		snapshot.Components = make(map[string]ComponentSnapshot)
	}
	return nil
}
