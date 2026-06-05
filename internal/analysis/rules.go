package analysis

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

type CheckResult struct {
	Passed     bool        `json:"passed"`
	RiskScore  int         `json:"risk_score"`
	Violations []Violation `json:"violations,omitempty"`
	Warnings   []Violation `json:"warnings,omitempty"`
}

type Violation struct {
	RuleID   string `json:"rule_id"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	EventID  string `json:"event_id,omitempty"`
	Path     string `json:"path,omitempty"`
}

func riskScore(items []Violation) int {
	score := 0
	for _, item := range items {
		switch Severity(item.Severity) {
		case SeverityCritical:
			score += 40
		case SeverityError:
			score += 25
		case SeverityWarning:
			score += 10
		case SeverityInfo:
			score += 2
		}
	}
	if score > 100 {
		return 100
	}
	return score
}

func isPassing(items []Violation) bool {
	for _, item := range items {
		switch Severity(item.Severity) {
		case SeverityCritical, SeverityError:
			return false
		}
	}
	return true
}
