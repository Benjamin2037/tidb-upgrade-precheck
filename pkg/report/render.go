package report

import (
	"bytes"
	htmltemplate "html/template"
	texttemplate "text/template"
)

// MarkdownReportTemplate is the template for Markdown report rendering.
const MarkdownReportTemplate = `# TiDB Upgrade Precheck Report
**Cluster:** {{.ClusterName}}
**Upgrade Path:** {{.UpgradePath}}

## 1. Executive Summary
| Risk Level | Count | Description |
| :--- | :---: | :--- |
| 🟥 **HIGH** | {{index .Summary .RiskHighKey}} | **Action Required.** Upgrade may fail or behavior will change drastically. |
| 🟨 **MEDIUM** | {{index .Summary .RiskMediumKey}} | **Recommendation.** Performance may lag or feature not optimal. |
| 🟦 **INFO** | {{index .Summary .RiskInfoKey}} | **Audit Notice.** User-modified configurations (Safe deviations). |

---

## 2. Critical Risks (Action Required)
{{range .Risks}}{{if eq .Level "HIGH"}}
### [HIGH] {{.Impact}}
* **Component:** {{.Component}}
* **Parameter:** ` + "`{{.Parameter}}`" + `
* **Impact:** {{.Impact}}
* **R&D Comments:** {{.RDComment}}
{{end}}{{end}}

---

## 3. Recommendations
{{range .Risks}}{{if eq .Level "MEDIUM"}}
### [MEDIUM] {{.Impact}}
* **Component:** {{.Component}}
* **Parameter:** ` + "`{{.Parameter}}`" + `
* **Current:** {{.Current}} (Old Default) -> **Target Default:** {{.Target}}
* **Suggestion:** {{.Suggestion}}
{{end}}{{end}}

---

## 4. Configuration Audit
> **Notice:** The following parameters have been explicitly modified by you (UserSet) and differ from the Target Version's defaults. Please review them.

| Component | Parameter | Your Value | Target Default | Status |
| :--- | :--- | :--- | :--- | :--- |
{{range .Audits}}| {{.Component}} | ` + "`{{.Parameter}}`" + ` | {{.Current}} | {{.Target}} | {{.Status}} |
{{end}}
`

// HTMLReportTemplate is the template for HTML report rendering.
const HTMLReportTemplate = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>TiDB Upgrade Precheck Report</title>
<style>
body { font-family: Arial, sans-serif; margin: 2em; }
h1, h2, h3 { color: #222; }
.risk-high { background: #d32f2f; color: #fff; padding: 4px 8px; border-radius: 4px; }
.risk-medium { background: #fbc02d; color: #222; padding: 4px 8px; border-radius: 4px; }
.risk-info { background: #1976d2; color: #fff; padding: 4px 8px; border-radius: 4px; }
table { border-collapse: collapse; width: 100%; margin-bottom: 2em; }
th, td { border: 1px solid #ccc; padding: 8px; text-align: left; }
.collapsible { background: #f1f1f1; cursor: pointer; padding: 10px; border: none; text-align: left; outline: none; font-size: 1em; }
.active, .collapsible:hover { background-color: #ccc; }
.content { padding: 0 18px; display: none; overflow: hidden; background-color: #f9f9f9; }
@media print { .collapsible, .content { display: block !important; } }
</style>
</head>
<body>
<h1>TiDB Upgrade Precheck Report</h1>
<p><b>Cluster:</b> {{.ClusterName}}<br>
<b>Upgrade Path:</b> {{.UpgradePath}}</p>
<h2>1. Executive Summary</h2>
<table>
<tr><th>Risk Level</th><th>Count</th><th>Description</th></tr>
<tr><td class="risk-high">HIGH</td><td>{{index .Summary .RiskHighKey}}</td><td>Action Required. Upgrade may fail or behavior will change drastically.</td></tr>
<tr><td class="risk-medium">MEDIUM</td><td>{{index .Summary .RiskMediumKey}}</td><td>Recommendation. Performance may lag or feature not optimal.</td></tr>
<tr><td class="risk-info">INFO</td><td>{{index .Summary .RiskInfoKey}}</td><td>Audit Notice. User-modified configurations (Safe deviations).</td></tr>
</table>
<h2>2. Critical Risks (Action Required)</h2>
{{range .Risks}}{{if eq .Level "HIGH"}}
<div class="risk-high"><b>[HIGH]</b> {{.Impact}}</div>
<ul>
<li><b>Component:</b> {{.Component}}</li>
<li><b>Parameter:</b> <code>{{.Parameter}}</code></li>
<li><b>Impact:</b> {{.Impact}}</li>
<li><b>R&D Comments:</b> {{.RDComment}}</li>
</ul>
{{end}}{{end}}
<h2>3. Recommendations</h2>
{{range .Risks}}{{if eq .Level "MEDIUM"}}
<div class="risk-medium"><b>[MEDIUM]</b> {{.Impact}}</div>
<ul>
<li><b>Component:</b> {{.Component}}</li>
<li><b>Parameter:</b> <code>{{.Parameter}}</code></li>
<li><b>Current:</b> {{.Current}} (Old Default) -> <b>Target Default:</b> {{.Target}}</li>
<li><b>Suggestion:</b> {{.Suggestion}}</li>
</ul>
{{end}}{{end}}
<h2>4. Configuration Audit</h2>
<p><b>Notice:</b> The following parameters have been explicitly modified by you (UserSet) and differ from the Target Version's defaults. Please review them.</p>
<button type="button" class="collapsible">Show/Hide Audit Table</button>
<div class="content">
<table>
<tr><th>Component</th><th>Parameter</th><th>Your Value</th><th>Target Default</th><th>Status</th></tr>
{{range .Audits}}<tr><td>{{.Component}}</td><td><code>{{.Parameter}}</code></td><td>{{.Current}}</td><td>{{.Target}}</td><td>{{.Status}}</td></tr>{{end}}
</table>
</div>
<script>
var coll = document.getElementsByClassName("collapsible");
var i;
for (i = 0; i < coll.length; i++) {
  coll[i].addEventListener("click", function() {
    this.classList.toggle("active");
    var content = this.nextElementSibling;
    if (content.style.display === "block") {
      content.style.display = "none";
    } else {
      content.style.display = "block";
    }
  });
}
</script>
</body>
</html>
`

// RenderMarkdownReport renders the report as Markdown.
type reportTemplateData struct {
	*Report
	RiskHighKey   RiskLevel
	RiskMediumKey RiskLevel
	RiskInfoKey   RiskLevel
}

func newReportTemplateData(r *Report) *reportTemplateData {
	return &reportTemplateData{
		Report:        r,
		RiskHighKey:   RiskHigh,
		RiskMediumKey: RiskMedium,
		RiskInfoKey:   RiskInfo,
	}
}

func RenderMarkdownReport(r *Report) (string, error) {
	tmpl, err := texttemplate.New("md").Parse(MarkdownReportTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, newReportTemplateData(r))
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderHTMLReport renders the report as HTML.
func RenderHTMLReport(r *Report) (string, error) {
	tmpl, err := htmltemplate.New("html").Parse(HTMLReportTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, newReportTemplateData(r))
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
