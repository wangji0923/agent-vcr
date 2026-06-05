package list

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/analysis"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

type Summary struct {
	RunID              string     `json:"run_id"`
	Source             string     `json:"source"`
	Status             string     `json:"status"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	EndedAt            *time.Time `json:"ended_at,omitempty"`
	ToolCalls          int        `json:"tool_calls"`
	ChangedFiles       int        `json:"changed_files"`
	TestCommandSummary string     `json:"test_command_summary,omitempty"`
	MetadataMissing    bool       `json:"metadata_missing,omitempty"`
	MetadataError      string     `json:"metadata_error,omitempty"`
}

func Runs(projectDir string) ([]Summary, error) {
	projectDir, err := normalizeProjectDir(projectDir)
	if err != nil {
		return nil, err
	}
	runsDir := filepath.Join(projectDir, ".agent-vcr", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Summary{}, nil
		}
		return nil, err
	}

	summaries := make([]Summary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		summary := summarizeRun(filepath.Join(runsDir, entry.Name()), entry.Name())
		summaries = append(summaries, summary)
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		left := timeOrZero(summaries[i].StartedAt)
		right := timeOrZero(summaries[j].StartedAt)
		if !left.Equal(right) {
			return left.After(right)
		}
		return summaries[i].RunID > summaries[j].RunID
	})
	return summaries, nil
}

func summarizeRun(runDir string, runID string) Summary {
	events, eventsErr := analysis.ReadEvents(filepath.Join(runDir, "trace.ndjson"))
	summary := Summary{
		RunID:  runID,
		Source: "unknown",
		Status: trace.RunStatusUnknown,
	}

	meta, metaErr := readMetadata(filepath.Join(runDir, "metadata.json"))
	if metaErr != nil {
		summary.MetadataMissing = errors.Is(metaErr, os.ErrNotExist)
		summary.MetadataError = metaErr.Error()
	} else {
		applyMetadata(&summary, meta)
	}

	if eventsErr == nil {
		applyEvents(&summary, events, runDir, meta.Summary)
	}
	if summary.Source == "" {
		summary.Source = "unknown"
	}
	if summary.Status == "" {
		summary.Status = trace.RunStatusUnknown
	}
	return summary
}

func applyMetadata(summary *Summary, meta trace.Metadata) {
	if meta.RunID != "" {
		summary.RunID = meta.RunID
	}
	if meta.Source != "" {
		summary.Source = meta.Source
	}
	if meta.Status != "" {
		summary.Status = meta.Status
	}
	if !meta.StartedAt.IsZero() {
		started := meta.StartedAt
		summary.StartedAt = &started
	}
	if meta.EndedAt != nil {
		ended := *meta.EndedAt
		summary.EndedAt = &ended
	}
}

func applyEvents(summary *Summary, events []trace.Event, runDir string, metaSummary trace.Payload) {
	changed := map[string]bool{}
	hasSummaryChangedFiles := hasPayloadKey(metaSummary, "changed_files")
	if hasSummaryChangedFiles {
		addFiles(changed, stringSliceFromAny(metaSummary["changed_files"]))
	}
	for _, key := range []string{"final_diff_blob", "patch_blob"} {
		if path := stringFromAny(metaSummary[key]); path != "" {
			addFiles(changed, analysis.ChangedFilesFromPatch(runDir, path))
		}
	}

	var testSummaries []testCommand
	pendingTests := map[string]int{}
	var patchPaths []string
	for _, event := range events {
		if summary.Source == "unknown" && event.Source.Adapter != "" {
			summary.Source = event.Source.Adapter
		}
		if summary.StartedAt == nil && !event.Timestamp.IsZero() {
			started := event.Timestamp
			summary.StartedAt = &started
		}
		if event.Type == trace.EventRunStop && !event.Timestamp.IsZero() {
			ended := event.Timestamp
			summary.EndedAt = &ended
			if status := stringFromAny(event.Payload["status"]); status != "" {
				summary.Status = status
			}
		}
		if event.Type == trace.EventToolCall {
			summary.ToolCalls++
		}
		if !hasSummaryChangedFiles && eventCarriesRunDelta(event.Type) {
			addFiles(changed, filesFromEvent(event))
		}
		for _, artifact := range event.Artifacts {
			if artifact.Kind == trace.ArtifactPatch {
				patchPaths = append(patchPaths, artifact.Path)
			}
		}
		if command := commandFromEvent(event); command != "" && isLikelyTestCommand(command) {
			key := correlationKey(event)
			testSummaries = append(testSummaries, testCommand{Key: key, Command: command})
			if key != "" {
				pendingTests[key] = len(testSummaries) - 1
			}
			if code, ok := exitCodeFromPayload(event.Payload); ok {
				testSummaries[len(testSummaries)-1].ExitCode = &code
			}
		}
		if code, ok := exitCodeFromPayload(event.Payload); ok {
			if key := correlationKey(event); key != "" {
				if idx, ok := pendingTests[key]; ok {
					testSummaries[idx].ExitCode = &code
				}
			}
		}
	}
	if !hasSummaryChangedFiles && len(changed) == 0 {
		addFiles(changed, changedFilesFromPatchDelta(runDir, patchPaths))
	}
	if !hasSummaryChangedFiles && len(changed) == 0 {
		addFiles(changed, stringSliceFromAny(metaSummary["files"]))
	}
	summary.ChangedFiles = len(changed)
	if len(testSummaries) > 0 {
		summary.TestCommandSummary = testSummaries[len(testSummaries)-1].String()
	}
}

func readMetadata(path string) (trace.Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return trace.Metadata{}, err
	}
	var meta trace.Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return trace.Metadata{}, err
	}
	return meta, nil
}

func normalizeProjectDir(projectDir string) (string, error) {
	if projectDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		projectDir = cwd
	}
	return filepath.Abs(projectDir)
}

type testCommand struct {
	Key      string
	Command  string
	ExitCode *int
}

func (cmd testCommand) String() string {
	if cmd.ExitCode == nil {
		return cmd.Command
	}
	return cmd.Command + " (exit " + intString(*cmd.ExitCode) + ")"
}

func commandFromEvent(event trace.Event) string {
	command := firstString(event.Payload, "command", "cmd")
	args := stringSliceFromAny(event.Payload["args"])
	if command == "" {
		for _, key := range []string{"arguments", "input", "tool_input"} {
			if nested, ok := event.Payload[key].(map[string]any); ok {
				command = firstString(nested, "command", "cmd")
				if len(args) == 0 {
					args = stringSliceFromAny(nested["args"])
				}
			}
		}
	}
	if command == "" {
		return ""
	}
	command = strings.Join(strings.Fields(command), " ")
	if len(args) > 0 {
		command = strings.TrimSpace(command + " " + strings.Join(args, " "))
	}
	return command
}

func isLikelyTestCommand(command string) bool {
	value := strings.ToLower(command)
	for _, marker := range []string{
		"go test",
		"npm test",
		"pnpm test",
		"yarn test",
		"pytest",
		"cargo test",
		"mvn test",
		"gradle test",
		"dotnet test",
	} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return strings.Contains(value, " test ") || strings.HasSuffix(value, " test")
}

func filesFromEvent(event trace.Event) []string {
	var files []string
	for _, key := range []string{"changed_files", "files", "paths"} {
		files = append(files, stringSliceFromAny(event.Payload[key])...)
	}
	for _, key := range []string{"path", "file", "filename"} {
		if value := firstString(event.Payload, key); value != "" {
			files = append(files, value)
		}
	}
	return files
}

func addFiles(target map[string]bool, files []string) {
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file != "" {
			target[file] = true
		}
	}
}

func eventCarriesRunDelta(typ trace.EventType) bool {
	switch typ {
	case trace.EventFilePatch, trace.EventFileWrite, trace.EventProcessResult:
		return true
	default:
		return false
	}
}

func changedFilesFromPatchDelta(runDir string, patchPaths []string) []string {
	if len(patchPaths) == 0 {
		return nil
	}
	first := patchSectionsFromArtifact(runDir, patchPaths[0])
	last := patchSectionsFromArtifact(runDir, patchPaths[len(patchPaths)-1])
	if len(first) == 0 {
		return sortedPatchSectionKeys(last)
	}
	if len(last) == 0 {
		return sortedPatchSectionKeys(first)
	}
	seen := map[string]bool{}
	var files []string
	for file, after := range last {
		if before := first[file]; before != after {
			seen[file] = true
			files = append(files, file)
		}
	}
	for file := range first {
		if _, ok := last[file]; !ok && !seen[file] {
			files = append(files, file)
		}
	}
	sort.Strings(files)
	return files
}

func patchSectionsFromArtifact(runDir string, relPath string) map[string]string {
	if relPath == "" {
		return map[string]string{}
	}
	path := filepath.Join(runDir, filepath.FromSlash(relPath))
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	return patchSections(string(data))
}

func patchSections(data string) map[string]string {
	sections := map[string]string{}
	var file string
	var lines []string
	flush := func() {
		if file != "" {
			sections[file] = strings.Join(trimTrailingEmptyLines(lines), "\n")
		}
	}
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			file = patchFileFromDiffHeader(line)
			lines = []string{line}
			continue
		}
		if file == "" {
			continue
		}
		lines = append(lines, line)
	}
	flush()
	return sections
}

func trimTrailingEmptyLines(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func patchFileFromDiffHeader(line string) string {
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return ""
	}
	path := strings.Trim(parts[3], `"`)
	path = strings.TrimPrefix(path, "b/")
	path = strings.TrimPrefix(path, "a/")
	if path == "/dev/null" {
		return ""
	}
	return path
}

func sortedPatchSectionKeys(sections map[string]string) []string {
	files := make([]string, 0, len(sections))
	for file := range sections {
		if file != "" {
			files = append(files, file)
		}
	}
	sort.Strings(files)
	return files
}

func correlationKey(event trace.Event) string {
	for _, key := range []string{"tool_use_id", "call_id", "id"} {
		if value := firstString(event.Payload, key); value != "" {
			return key + ":" + value
		}
	}
	if event.ParentID != "" {
		return "span:" + event.ParentID
	}
	if event.SpanID != "" {
		return "span:" + event.SpanID
	}
	return ""
}

func exitCodeFromPayload(payload trace.Payload) (int, bool) {
	if code, ok := intFromAny(payload["exit_code"]); ok {
		return code, true
	}
	if nested, ok := payload["result"].(map[string]any); ok {
		return intFromAny(nested["exit_code"])
	}
	return 0, false
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringFromAny(payload[key]); value != "" {
			return value
		}
	}
	return ""
}

func hasPayloadKey(payload trace.Payload, key string) bool {
	if payload == nil {
		return false
	}
	_, ok := payload[key]
	return ok
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func stringSliceFromAny(value any) []string {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if value := stringFromAny(item); value != "" {
				out = append(out, value)
			}
		}
		return out
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	default:
		return nil
	}
}

func intFromAny(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	default:
		return 0, false
	}
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	negative := value < 0
	if negative {
		value = -value
	}
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	if negative {
		digits = append(digits, '-')
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

func timeOrZero(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
