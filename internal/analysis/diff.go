package analysis

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

type RunData struct {
	RunID    string         `json:"run_id"`
	RunDir   string         `json:"run_dir,omitempty"`
	Metadata trace.Metadata `json:"metadata"`
	Events   []trace.Event  `json:"events"`
}

type DiffResult struct {
	RunA                string                      `json:"run_a"`
	RunB                string                      `json:"run_b"`
	FirstDivergence     *Divergence                 `json:"first_divergence,omitempty"`
	Summary             DiffSummary                 `json:"summary"`
	ToolSequenceDiff    []SequenceDifference        `json:"tool_sequence_diff,omitempty"`
	ChangedFilesDiff    ChangedFilesDifference      `json:"changed_files_diff"`
	CommandExitCodeDiff []CommandExitCodeDifference `json:"command_exit_code_diff,omitempty"`
}

type Divergence struct {
	Index  int          `json:"index"`
	EventA *trace.Event `json:"event_a,omitempty"`
	EventB *trace.Event `json:"event_b,omitempty"`
	Reason string       `json:"reason"`
}

type DiffSummary struct {
	RunAEvents               int `json:"run_a_events"`
	RunBEvents               int `json:"run_b_events"`
	RunAToolCalls            int `json:"run_a_tool_calls"`
	RunBToolCalls            int `json:"run_b_tool_calls"`
	RunAChangedFiles         int `json:"run_a_changed_files"`
	RunBChangedFiles         int `json:"run_b_changed_files"`
	ToolSequenceDifferences  int `json:"tool_sequence_differences"`
	ChangedFileDifferences   int `json:"changed_file_differences"`
	CommandExitCodeDiffCount int `json:"command_exit_code_differences"`
}

type SequenceDifference struct {
	Index int    `json:"index"`
	A     string `json:"a,omitempty"`
	B     string `json:"b,omitempty"`
}

type ChangedFilesDifference struct {
	OnlyInA []string `json:"only_in_a,omitempty"`
	OnlyInB []string `json:"only_in_b,omitempty"`
}

type CommandExitCodeDifference struct {
	Command  string `json:"command"`
	EventIDA string `json:"event_id_a,omitempty"`
	EventIDB string `json:"event_id_b,omitempty"`
	ExitA    *int   `json:"exit_a,omitempty"`
	ExitB    *int   `json:"exit_b,omitempty"`
}

type commandResult struct {
	Command  string
	EventID  string
	ExitCode *int
}

func LoadRunData(projectDir string, ref string) (RunData, error) {
	runID, err := trace.ResolveRunID(projectDir, ref)
	if err != nil {
		return RunData{}, err
	}
	store, err := trace.OpenRun(projectDir, runID)
	if err != nil {
		return RunData{}, err
	}
	meta, err := store.ReadMetadata()
	if err != nil {
		return RunData{}, err
	}
	events, err := ReadTraceFile(store.Path("trace.ndjson"))
	if err != nil {
		return RunData{}, err
	}
	return RunData{
		RunID:    runID,
		RunDir:   store.RunDir,
		Metadata: meta,
		Events:   events,
	}, nil
}

func ReadTraceFile(path string) ([]trace.Event, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []trace.Event{}, nil
		}
		return nil, err
	}
	defer file.Close()
	return readTrace(file)
}

func readTrace(reader io.Reader) ([]trace.Event, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var events []trace.Event
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event trace.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("parse trace line %d: %w", lineNo, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func DiffRuns(a, b RunData) DiffResult {
	aEvents := significantEvents(a.Events)
	bEvents := significantEvents(b.Events)
	aSigs := signatures(aEvents)
	bSigs := signatures(bEvents)

	result := DiffResult{
		RunA:             displayRunID(a),
		RunB:             displayRunID(b),
		ChangedFilesDiff: diffChangedFiles(ChangedFiles(a), ChangedFiles(b)),
	}

	limit := len(aSigs)
	if len(bSigs) < limit {
		limit = len(bSigs)
	}
	for i := 0; i < limit; i++ {
		if aSigs[i] != bSigs[i] {
			result.FirstDivergence = &Divergence{
				Index:  i + 1,
				EventA: eventPtr(aEvents[i]),
				EventB: eventPtr(bEvents[i]),
				Reason: divergenceReason(aSigs[i], bSigs[i]),
			}
			break
		}
	}
	if result.FirstDivergence == nil && len(aSigs) != len(bSigs) {
		div := Divergence{Index: limit + 1, Reason: "length_divergence"}
		if len(aEvents) > limit {
			div.EventA = eventPtr(aEvents[limit])
		}
		if len(bEvents) > limit {
			div.EventB = eventPtr(bEvents[limit])
		}
		result.FirstDivergence = &div
	}

	result.ToolSequenceDiff = diffToolSequence(toolSequence(aEvents), toolSequence(bEvents))
	result.CommandExitCodeDiff = diffCommandExitCodes(commandResults(aEvents), commandResults(bEvents))
	result.Summary = DiffSummary{
		RunAEvents:               len(aEvents),
		RunBEvents:               len(bEvents),
		RunAToolCalls:            countType(aEvents, trace.EventToolCall),
		RunBToolCalls:            countType(bEvents, trace.EventToolCall),
		RunAChangedFiles:         len(ChangedFiles(a)),
		RunBChangedFiles:         len(ChangedFiles(b)),
		ToolSequenceDifferences:  len(result.ToolSequenceDiff),
		ChangedFileDifferences:   len(result.ChangedFilesDiff.OnlyInA) + len(result.ChangedFilesDiff.OnlyInB),
		CommandExitCodeDiffCount: len(result.CommandExitCodeDiff),
	}
	return result
}

func RenderDiffText(result DiffResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Run A: %s\n", result.RunA)
	fmt.Fprintf(&b, "Run B: %s\n\n", result.RunB)
	if result.FirstDivergence == nil {
		b.WriteString("No divergence found.\n\n")
	} else {
		fmt.Fprintf(&b, "First divergence at normalized event #%d\n", result.FirstDivergence.Index)
		fmt.Fprintf(&b, "Reason: %s\n\n", result.FirstDivergence.Reason)
		b.WriteString("A:\n")
		fmt.Fprintf(&b, "  %s\n\n", DescribeEvent(result.FirstDivergence.EventA))
		b.WriteString("B:\n")
		fmt.Fprintf(&b, "  %s\n\n", DescribeEvent(result.FirstDivergence.EventB))
	}

	b.WriteString("Summary:\n")
	fmt.Fprintf(&b, "  A events: %d\n", result.Summary.RunAEvents)
	fmt.Fprintf(&b, "  B events: %d\n", result.Summary.RunBEvents)
	fmt.Fprintf(&b, "  A tool calls: %d\n", result.Summary.RunAToolCalls)
	fmt.Fprintf(&b, "  B tool calls: %d\n", result.Summary.RunBToolCalls)
	fmt.Fprintf(&b, "  A changed files: %d\n", result.Summary.RunAChangedFiles)
	fmt.Fprintf(&b, "  B changed files: %d\n", result.Summary.RunBChangedFiles)
	fmt.Fprintf(&b, "  Tool sequence differences: %d\n", result.Summary.ToolSequenceDifferences)
	fmt.Fprintf(&b, "  Changed file differences: %d\n", result.Summary.ChangedFileDifferences)
	fmt.Fprintf(&b, "  Command exit code differences: %d\n", result.Summary.CommandExitCodeDiffCount)

	if len(result.ToolSequenceDiff) > 0 {
		b.WriteString("\nTool sequence differences:\n")
		for _, diff := range result.ToolSequenceDiff {
			fmt.Fprintf(&b, "  #%d A=%s B=%s\n", diff.Index, printableMissing(diff.A), printableMissing(diff.B))
		}
	}
	if len(result.ChangedFilesDiff.OnlyInA) > 0 || len(result.ChangedFilesDiff.OnlyInB) > 0 {
		b.WriteString("\nChanged files differences:\n")
		for _, path := range result.ChangedFilesDiff.OnlyInA {
			fmt.Fprintf(&b, "  only in A: %s\n", path)
		}
		for _, path := range result.ChangedFilesDiff.OnlyInB {
			fmt.Fprintf(&b, "  only in B: %s\n", path)
		}
	}
	if len(result.CommandExitCodeDiff) > 0 {
		b.WriteString("\nCommand exit code differences:\n")
		for _, diff := range result.CommandExitCodeDiff {
			fmt.Fprintf(&b, "  %s: A=%s B=%s\n", diff.Command, printableExit(diff.ExitA), printableExit(diff.ExitB))
		}
	}
	return b.String()
}

func DescribeEvent(event *trace.Event) string {
	if event == nil {
		return "<missing>"
	}
	parts := []string{string(event.Type)}
	if tool := signaturePayloadString(event.Payload, "tool_name", "name"); tool != "" {
		parts = append(parts, tool)
	}
	if command := CommandFromEvent(*event); command != "" {
		parts = append(parts, command)
	}
	if exitCode := exitCodeString(event.Payload); exitCode != "" {
		parts = append(parts, "exit="+exitCode)
	}
	if files := NormalizedChangedFiles(event.Payload); len(files) > 0 {
		parts = append(parts, "files="+strings.Join(files, ","))
	}
	return strings.Join(parts, " ")
}

func ChangedFiles(run RunData) []string {
	paths, _ := changedFilesByEvent(run)
	out := make([]string, 0, len(paths))
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func changedFilesByEvent(run RunData) (map[string]string, string) {
	paths := map[string]string{}
	firstEventID := ""
	add := func(path string, eventID string) {
		path = normalizeComparablePath(path)
		if path == "" {
			return
		}
		if _, ok := paths[path]; !ok {
			paths[path] = eventID
		}
		if firstEventID == "" && eventID != "" {
			firstEventID = eventID
		}
	}
	for _, event := range run.Events {
		switch event.Type {
		case trace.EventProcessResult, trace.EventFilePatch, trace.EventFileWrite:
			for _, file := range NormalizedChangedFiles(event.Payload) {
				add(file, event.EventID)
			}
		}
	}
	if run.Metadata.Summary != nil {
		for _, file := range stringSlice(run.Metadata.Summary["changed_files"]) {
			add(file, "")
		}
	}
	return paths, firstEventID
}

func significantEvents(events []trace.Event) []trace.Event {
	out := make([]trace.Event, 0, len(events))
	for _, event := range events {
		if event.Type == "" {
			continue
		}
		if event.Type == trace.EventRaw && event.RawRef != nil && event.RawRef.Kind == trace.ArtifactSnapshot {
			continue
		}
		out = append(out, event)
	}
	return out
}

func signatures(events []trace.Event) []EventSignature {
	out := make([]EventSignature, 0, len(events))
	for _, event := range events {
		out = append(out, Signature(event))
	}
	return out
}

func divergenceReason(a, b EventSignature) string {
	if a.Type != b.Type {
		return "event_type_mismatch"
	}
	if a.ToolName != b.ToolName || a.Command != b.Command {
		return "tool_sequence_mismatch"
	}
	if a.ExitCode != b.ExitCode {
		return "exit_code_mismatch"
	}
	if a.FilesHash != b.FilesHash {
		return "changed_files_mismatch"
	}
	return string(a.Type) + "_signature_mismatch"
}

func diffToolSequence(a, b []string) []SequenceDifference {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	var diffs []SequenceDifference
	for i := 0; i < limit; i++ {
		if a[i] != b[i] {
			diffs = append(diffs, SequenceDifference{Index: i + 1, A: a[i], B: b[i]})
		}
	}
	for i := limit; i < len(a); i++ {
		diffs = append(diffs, SequenceDifference{Index: i + 1, A: a[i]})
	}
	for i := limit; i < len(b); i++ {
		diffs = append(diffs, SequenceDifference{Index: i + 1, B: b[i]})
	}
	return diffs
}

func toolSequence(events []trace.Event) []string {
	var out []string
	for _, event := range events {
		switch event.Type {
		case trace.EventToolCall:
			label := signaturePayloadString(event.Payload, "tool_name", "name")
			if label == "" {
				label = string(event.Type)
			}
			out = append(out, label)
		case trace.EventShellCommand:
			command := CommandFromEvent(event)
			if command == "" {
				command = "<shell>"
			}
			out = append(out, "shell:"+command)
		}
	}
	return out
}

func diffChangedFiles(a, b []string) ChangedFilesDifference {
	aSet := sliceSet(a)
	bSet := sliceSet(b)
	var diff ChangedFilesDifference
	for path := range aSet {
		if !bSet[path] {
			diff.OnlyInA = append(diff.OnlyInA, path)
		}
	}
	for path := range bSet {
		if !aSet[path] {
			diff.OnlyInB = append(diff.OnlyInB, path)
		}
	}
	sort.Strings(diff.OnlyInA)
	sort.Strings(diff.OnlyInB)
	return diff
}

func commandResults(events []trace.Event) []commandResult {
	commandsByID := map[string]string{}
	var results []commandResult
	for _, event := range events {
		switch event.Type {
		case trace.EventShellCommand:
			if id := eventCorrelationID(event); id != "" {
				if command := CommandFromEvent(event); command != "" {
					commandsByID[id] = command
				}
			}
		case trace.EventShellResult:
			command := CommandFromEvent(event)
			if command == "" {
				command = commandsByID[eventCorrelationID(event)]
			}
			if command == "" {
				continue
			}
			exit, ok := exitCodeInt(event.Payload)
			var exitPtr *int
			if ok {
				exitPtr = &exit
			}
			results = append(results, commandResult{Command: command, EventID: event.EventID, ExitCode: exitPtr})
		case trace.EventProcessResult:
			command := CommandFromEvent(event)
			if command == "" {
				continue
			}
			exit, ok := exitCodeInt(event.Payload)
			var exitPtr *int
			if ok {
				exitPtr = &exit
			}
			results = append(results, commandResult{Command: command, EventID: event.EventID, ExitCode: exitPtr})
		}
	}
	return results
}

func diffCommandExitCodes(a, b []commandResult) []CommandExitCodeDifference {
	aMap := commandResultMap(a)
	bMap := commandResultMap(b)
	keys := make([]string, 0, len(aMap)+len(bMap))
	seen := map[string]bool{}
	for key := range aMap {
		keys = append(keys, key)
		seen[key] = true
	}
	for key := range bMap {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	var diffs []CommandExitCodeDifference
	for _, key := range keys {
		aResult, hasA := aMap[key]
		bResult, hasB := bMap[key]
		if hasA && hasB && exitCodeEqual(aResult.ExitCode, bResult.ExitCode) {
			continue
		}
		command := strings.TrimSuffix(key, "#"+occurrenceSuffix(key))
		if hasA && aResult.Command != "" {
			command = aResult.Command
		} else if hasB {
			command = bResult.Command
		}
		diff := CommandExitCodeDifference{Command: command}
		if hasA {
			diff.EventIDA = aResult.EventID
			diff.ExitA = aResult.ExitCode
		}
		if hasB {
			diff.EventIDB = bResult.EventID
			diff.ExitB = bResult.ExitCode
		}
		diffs = append(diffs, diff)
	}
	return diffs
}

func commandResultMap(results []commandResult) map[string]commandResult {
	out := map[string]commandResult{}
	counts := map[string]int{}
	for _, result := range results {
		base := normalizeCommand(result.Command)
		counts[base]++
		key := fmt.Sprintf("%s#%d", base, counts[base])
		out[key] = result
	}
	return out
}

func occurrenceSuffix(key string) string {
	idx := strings.LastIndex(key, "#")
	if idx < 0 {
		return ""
	}
	return key[idx+1:]
}

func exitCodeEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func eventCorrelationID(event trace.Event) string {
	return signaturePayloadString(event.Payload, "tool_use_id", "call_id", "span_id")
}

func countType(events []trace.Event, typ trace.EventType) int {
	count := 0
	for _, event := range events {
		if event.Type == typ {
			count++
		}
	}
	return count
}

func displayRunID(run RunData) string {
	if run.RunID != "" {
		return run.RunID
	}
	if run.Metadata.RunID != "" {
		return run.Metadata.RunID
	}
	if run.RunDir != "" {
		return filepath.Base(run.RunDir)
	}
	return "<unknown>"
}

func eventPtr(event trace.Event) *trace.Event {
	copy := event
	return &copy
}

func sliceSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func printableMissing(value string) string {
	if value == "" {
		return "<missing>"
	}
	return value
}

func printableExit(value *int) string {
	if value == nil {
		return "<missing>"
	}
	return fmt.Sprintf("%d", *value)
}
