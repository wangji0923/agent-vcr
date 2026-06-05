package behavior

import (
	"path/filepath"
	"sort"
	"strings"
)

type StepKind string

const (
	StepSearch            StepKind = "search"
	StepReadFile          StepKind = "read_file"
	StepInspectTest       StepKind = "inspect_test"
	StepEditFile          StepKind = "edit_file"
	StepRunTest           StepKind = "run_test"
	StepRunBuild          StepKind = "run_build"
	StepInstallDependency StepKind = "install_dependency"
	StepCallTool          StepKind = "call_tool"
	StepCallMCPTool       StepKind = "call_mcp_tool"
	StepPermissionRequest StepKind = "permission_request"
	StepRecoverFromError  StepKind = "recover_from_error"
	StepSkipValidation    StepKind = "skip_validation"
	StepContextCompact    StepKind = "context_compact"
	StepProcessStart      StepKind = "process_start"
	StepProcessResult     StepKind = "process_result"
	StepRawBehavior       StepKind = "raw_behavior"
	StepUnknown           StepKind = "unknown"
)

type StepRef struct {
	EventID    string `json:"event_id,omitempty"`
	EventIndex int64  `json:"event_index,omitempty"`
	EventType  string `json:"event_type,omitempty"`
}

type Step struct {
	StepID         string            `json:"step_id"`
	RunID          string            `json:"run_id"`
	Index          int               `json:"index"`
	Kind           StepKind          `json:"kind"`
	Action         string            `json:"action,omitempty"`
	Summary        string            `json:"summary,omitempty"`
	Target         string            `json:"target,omitempty"`
	Query          string            `json:"query,omitempty"`
	Command        string            `json:"command,omitempty"`
	ToolName       string            `json:"tool_name,omitempty"`
	Files          []string          `json:"files,omitempty"`
	Result         StepResult        `json:"result,omitempty"`
	Fingerprint    string            `json:"fingerprint,omitempty"`
	Significant    bool              `json:"significant"`
	SourceRefs     []StepRef         `json:"source_refs,omitempty"`
	SourceEventIDs []string          `json:"source_event_ids,omitempty"`
	Confidence     float64           `json:"confidence,omitempty"`
	Attributes     map[string]string `json:"attributes,omitempty"`
}

type Timeline struct {
	SchemaVersion string   `json:"schema_version"`
	RunID         string   `json:"run_id"`
	Steps         []Step   `json:"steps"`
	Warnings      []string `json:"warnings,omitempty"`
}

func (s Step) StableKey() string {
	parts := []string{
		string(s.Kind),
		normalizeKeyPart(s.Action),
		NormalizePathForKey(s.Target),
		normalizeKeyPart(s.Query),
		normalizeKeyPart(s.Command),
		normalizeKeyPart(s.ToolName),
		strings.Join(SortFiles(s.Files), ","),
		string(normalizedResult(s.Result)),
	}
	return strings.Join(parts, "|")
}

func (s Step) IsValidation() bool {
	return s.Kind == StepRunTest || s.Kind == StepRunBuild
}

func (s Step) IsEdit() bool {
	return s.Kind == StepEditFile
}

func (s Step) IsFileRead() bool {
	return s.Kind == StepReadFile || s.Kind == StepInspectTest
}

func SortFiles(files []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, file := range files {
		normalized := NormalizePathForKey(file)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}

func NormalizePathForKey(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.ReplaceAll(path, "\\", "/")
	path = filepath.ToSlash(path)
	path = stripWindowsDrive(path)
	path = stripHomePrefix(path)
	path = strings.TrimPrefix(path, "./")
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	return strings.Trim(path, "/")
}

func IsAgentSpecificStepKind(kind StepKind) bool {
	value := strings.ToLower(string(kind))
	return strings.HasPrefix(value, "codex_") ||
		strings.HasPrefix(value, "kimi_") ||
		strings.HasPrefix(value, "claude_")
}

func IsKnownStepKind(kind StepKind) bool {
	switch kind {
	case StepSearch, StepReadFile, StepInspectTest, StepEditFile, StepRunTest,
		StepRunBuild, StepInstallDependency, StepCallTool, StepCallMCPTool,
		StepPermissionRequest, StepRecoverFromError, StepSkipValidation,
		StepContextCompact, StepProcessStart, StepProcessResult,
		StepRawBehavior, StepUnknown:
		return true
	default:
		return false
	}
}

func normalizeKeyPart(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Join(strings.Fields(value), " ")
	return normalizeUserPathsInText(value)
}

func normalizedResult(result StepResult) StepResult {
	if result == "" {
		return ResultUnknown
	}
	return result
}

func stripWindowsDrive(path string) string {
	if len(path) >= 3 && path[1] == ':' && path[2] == '/' {
		return path[3:]
	}
	return path
}

func stripHomePrefix(path string) string {
	lower := strings.ToLower(path)
	markers := []string{"users/", "/users/", "home/", "/home/"}
	for _, marker := range markers {
		if !strings.HasPrefix(lower, marker) {
			continue
		}
		rest := path[len(marker):]
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 2 {
			return parts[1]
		}
		return ""
	}
	return path
}

func normalizeUserPathsInText(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	fields := strings.Fields(value)
	for i, field := range fields {
		prefix, core, suffix := splitTokenPathBoundary(field)
		fields[i] = prefix + NormalizePathForKey(core) + suffix
	}
	return strings.Join(fields, " ")
}

func splitTokenPathBoundary(token string) (string, string, string) {
	start := 0
	end := len(token)
	for start < end && strings.ContainsRune("\"'`([{", rune(token[start])) {
		start++
	}
	for end > start && strings.ContainsRune("\"'`)]},;", rune(token[end-1])) {
		end--
	}
	return token[:start], token[start:end], token[end:]
}
