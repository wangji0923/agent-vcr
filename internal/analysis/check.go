package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/config"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

const maxArtifactSecretScanBytes int64 = 1024 * 1024

type runFacts struct {
	ChangedFiles       map[string]string
	FirstChangeEventID string
	Commands           []commandResult
	EventTexts         []scannableText
}

type scannableText struct {
	Text    string
	EventID string
	Path    string
	Source  string
}

func CheckRun(run RunData, cfg config.Config) CheckResult {
	facts := collectRunFacts(run)
	var items []Violation

	items = append(items, checkForbiddenPaths(facts, cfg)...)
	items = append(items, checkMaxChangedFiles(facts, cfg)...)
	items = append(items, checkRequiredCommands(facts, cfg)...)
	items = append(items, checkTestsAfterSourceChange(facts, cfg)...)
	items = append(items, checkDangerousCommands(facts)...)
	items = append(items, checkSecrets(run, facts, cfg)...)

	sort.SliceStable(items, func(i, j int) bool {
		if severityRank(items[i].Severity) != severityRank(items[j].Severity) {
			return severityRank(items[i].Severity) > severityRank(items[j].Severity)
		}
		if items[i].RuleID != items[j].RuleID {
			return items[i].RuleID < items[j].RuleID
		}
		return items[i].Message < items[j].Message
	})

	result := CheckResult{
		Passed:    isPassing(items),
		RiskScore: riskScore(items),
	}
	for _, item := range items {
		if Severity(item.Severity) == SeverityWarning || Severity(item.Severity) == SeverityInfo {
			result.Warnings = append(result.Warnings, item)
		} else {
			result.Violations = append(result.Violations, item)
		}
	}
	return result
}

func RenderCheckText(result CheckResult, ci bool) string {
	var b strings.Builder
	if ci {
		if result.Passed {
			fmt.Fprintf(&b, "Agent VCR check passed (risk score %d)\n", result.RiskScore)
			return b.String()
		}
		b.WriteString("Agent VCR check failed:\n")
		for _, item := range append([]Violation{}, append(result.Violations, result.Warnings...)...) {
			fmt.Fprintf(&b, "- [%s] %s: %s", item.Severity, item.RuleID, item.Message)
			if item.EventID != "" {
				fmt.Fprintf(&b, " (event %s)", item.EventID)
			}
			b.WriteByte('\n')
		}
		return b.String()
	}

	status := "pass"
	if !result.Passed {
		status = "fail"
	}
	fmt.Fprintf(&b, "Status: %s\n", status)
	fmt.Fprintf(&b, "Risk score: %d\n", result.RiskScore)
	writeViolations(&b, "Violations", result.Violations)
	writeViolations(&b, "Warnings", result.Warnings)
	return b.String()
}

func writeViolations(b *strings.Builder, title string, items []Violation) {
	if len(items) == 0 {
		fmt.Fprintf(b, "%s: none\n", title)
		return
	}
	fmt.Fprintf(b, "%s:\n", title)
	for _, item := range items {
		fmt.Fprintf(b, "- [%s] %s: %s", item.Severity, item.RuleID, item.Message)
		if item.EventID != "" {
			fmt.Fprintf(b, " (event %s)", item.EventID)
		}
		if item.Path != "" {
			fmt.Fprintf(b, " path=%s", item.Path)
		}
		b.WriteByte('\n')
	}
}

func collectRunFacts(run RunData) runFacts {
	changed, firstEvent := changedFilesByEvent(run)
	facts := runFacts{
		ChangedFiles:       changed,
		FirstChangeEventID: firstEvent,
		Commands:           commandResults(run.Events),
	}
	for _, event := range run.Events {
		facts.EventTexts = append(facts.EventTexts, scannableText{
			Text:    payloadText(event.Payload),
			EventID: event.EventID,
			Source:  string(event.Type),
		})
	}
	return facts
}

func checkForbiddenPaths(facts runFacts, cfg config.Config) []Violation {
	var out []Violation
	for path, eventID := range facts.ChangedFiles {
		for _, pattern := range cfg.Rules.ForbiddenPaths {
			if matchPath(pattern, path) {
				out = append(out, Violation{
					RuleID:   "forbidden_paths",
					Severity: string(SeverityCritical),
					Message:  fmt.Sprintf("Touched forbidden path %s", path),
					EventID:  eventID,
					Path:     path,
				})
				break
			}
		}
	}
	return out
}

func checkMaxChangedFiles(facts runFacts, cfg config.Config) []Violation {
	if cfg.Rules.MaxChangedFiles <= 0 || len(facts.ChangedFiles) <= cfg.Rules.MaxChangedFiles {
		return nil
	}
	return []Violation{{
		RuleID:   "max_changed_files",
		Severity: string(SeverityWarning),
		Message:  fmt.Sprintf("Changed %d files, limit is %d", len(facts.ChangedFiles), cfg.Rules.MaxChangedFiles),
		EventID:  facts.FirstChangeEventID,
	}}
}

func checkRequiredCommands(facts runFacts, cfg config.Config) []Violation {
	if len(cfg.Rules.RequiredCommands) == 0 || !hasSourceChange(facts, cfg) {
		return nil
	}
	var out []Violation
	for _, required := range cfg.Rules.RequiredCommands {
		required = strings.TrimSpace(required)
		if required == "" || commandWasRun(facts.Commands, required) {
			continue
		}
		out = append(out, Violation{
			RuleID:   "required_commands",
			Severity: string(SeverityError),
			Message:  fmt.Sprintf("Modified source but did not run %s", required),
			EventID:  facts.FirstChangeEventID,
		})
	}
	return out
}

func checkTestsAfterSourceChange(facts runFacts, cfg config.Config) []Violation {
	if !cfg.Rules.RequireTestsAfterSourceChange || !hasSourceChange(facts, cfg) {
		return nil
	}
	if hasTestChange(facts, cfg) || anyTestCommand(facts.Commands) {
		return nil
	}
	return []Violation{{
		RuleID:   "require_tests_after_source_change",
		Severity: string(SeverityError),
		Message:  "Modified source without changing tests or running a test command",
		EventID:  facts.FirstChangeEventID,
	}}
}

func checkDangerousCommands(facts runFacts) []Violation {
	var out []Violation
	for _, command := range facts.Commands {
		if command.Command == "" {
			continue
		}
		if reason := dangerousCommandReason(command.Command); reason != "" {
			out = append(out, Violation{
				RuleID:   "dangerous_command",
				Severity: string(SeverityError),
				Message:  fmt.Sprintf("Dangerous command matched %s", reason),
				EventID:  command.EventID,
			})
		}
	}
	return out
}

func checkSecrets(run RunData, facts runFacts, cfg config.Config) []Violation {
	if len(cfg.Redaction.Patterns) == 0 {
		return nil
	}
	patterns := compileSecretPatterns(cfg.Redaction.Patterns)
	if len(patterns) == 0 {
		return nil
	}
	texts := append([]scannableText{}, facts.EventTexts...)
	texts = append(texts, artifactTexts(run)...)

	seen := map[string]bool{}
	var out []Violation
	for _, text := range texts {
		if text.Text == "" {
			continue
		}
		for _, pattern := range patterns {
			if !pattern.regex.MatchString(text.Text) {
				continue
			}
			key := pattern.name + "\x00" + text.EventID + "\x00" + text.Path
			if seen[key] {
				continue
			}
			seen[key] = true
			location := text.Source
			if text.Path != "" {
				location = text.Path
			}
			out = append(out, Violation{
				RuleID:   "secret_pattern",
				Severity: string(SeverityCritical),
				Message:  fmt.Sprintf("Detected secret pattern %s in %s", pattern.name, location),
				EventID:  text.EventID,
				Path:     text.Path,
			})
		}
	}
	return out
}

type compiledSecretPattern struct {
	name  string
	regex *regexp.Regexp
}

func compileSecretPatterns(patterns []config.PatternConfig) []compiledSecretPattern {
	out := make([]compiledSecretPattern, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern.Regex == "" {
			continue
		}
		compiled, err := regexp.Compile(pattern.Regex)
		if err != nil {
			continue
		}
		name := pattern.Name
		if name == "" {
			name = pattern.Regex
		}
		out = append(out, compiledSecretPattern{name: name, regex: compiled})
	}
	return out
}

func artifactTexts(run RunData) []scannableText {
	if run.RunDir == "" {
		return nil
	}
	var out []scannableText
	for _, dirName := range []string{"blobs", "patches"} {
		root := filepath.Join(run.RunDir, dirName)
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry == nil || entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil || info.Size() > maxArtifactSecretScanBytes {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(run.RunDir, path)
			out = append(out, scannableText{
				Text:   string(data),
				Path:   filepath.ToSlash(rel),
				Source: dirName,
			})
			return nil
		})
	}
	return out
}

func payloadText(payload trace.Payload) string {
	if payload == nil {
		return ""
	}
	normalized := dropHashOnlyPayload(map[string]any(payload))
	data, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Sprintf("%#v", normalized)
	}
	return string(data)
}

func dropHashOnlyPayload(payload map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range payload {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "sha256") || strings.Contains(lower, "hash") {
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			out[key] = dropHashOnlyPayload(typed)
		default:
			out[key] = value
		}
	}
	return out
}

func hasSourceChange(facts runFacts, cfg config.Config) bool {
	for path := range facts.ChangedFiles {
		if matchesAnyPath(cfg.Rules.SourceGlobs, path) {
			return true
		}
	}
	return false
}

func hasTestChange(facts runFacts, cfg config.Config) bool {
	for path := range facts.ChangedFiles {
		if matchesAnyPath(cfg.Rules.TestGlobs, path) {
			return true
		}
	}
	return false
}

func commandWasRun(commands []commandResult, required string) bool {
	for _, command := range commands {
		if strings.Contains(command.Command, required) {
			return true
		}
	}
	return false
}

func anyTestCommand(commands []commandResult) bool {
	for _, command := range commands {
		normalized := strings.ToLower(" " + command.Command + " ")
		for _, marker := range []string{
			" go test ",
			" npm test",
			" pnpm test",
			" yarn test",
			" pytest",
			" vitest",
			" jest",
			" cargo test",
			" mvn test",
			" gradle test",
		} {
			if strings.Contains(normalized, marker) {
				return true
			}
		}
	}
	return false
}

func matchesAnyPath(patterns []string, path string) bool {
	for _, pattern := range patterns {
		if matchPath(pattern, path) {
			return true
		}
	}
	return false
}

func matchPath(pattern, path string) bool {
	pattern = normalizeComparablePath(pattern)
	path = normalizeComparablePath(path)
	if pattern == "" || path == "" {
		return false
	}
	if pattern == path {
		return true
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		if path == suffix || strings.HasSuffix(path, "/"+suffix) {
			return true
		}
	}
	if ok, _ := filepath.Match(pattern, path); ok {
		return true
	}
	if !strings.Contains(pattern, "/") {
		if ok, _ := filepath.Match(pattern, filepath.Base(path)); ok {
			return true
		}
	}
	return false
}

func dangerousCommandReason(command string) string {
	checks := []struct {
		name string
		re   *regexp.Regexp
	}{
		{"rm -rf /", regexp.MustCompile(`(?i)(^|[;&|]\s*)rm\s+-[^\n;&|]*r[^\n;&|]*f\s+(/|[a-z]:[/\\]?)($|\s|[;&|])`)},
		{"sudo", regexp.MustCompile(`(?i)(^|\s|[;&|])sudo(\s|$)`)},
		{"curl_pipe_shell", regexp.MustCompile(`(?i)\b(curl|wget)\b[^\n|]*\|\s*(sh|bash)\b`)},
		{"chmod 777", regexp.MustCompile(`(?i)(^|\s)chmod\s+777(\s|$)`)},
		{"mkfs", regexp.MustCompile(`(?i)(^|\s|[;&|])mkfs(\.[a-z0-9]+)?(\s|$)`)},
	}
	for _, check := range checks {
		if check.re.MatchString(command) {
			return check.name
		}
	}
	return ""
}

func severityRank(severity string) int {
	switch Severity(severity) {
	case SeverityCritical:
		return 4
	case SeverityError:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}
