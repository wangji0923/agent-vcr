package behavior

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

type EventExtractor struct {
	CommandClassifier CommandClassifier
	PathClassifier    PathClassifier
}

func NewEventExtractor(commandClassifier CommandClassifier, pathClassifier PathClassifier) EventExtractor {
	return EventExtractor{
		CommandClassifier: commandClassifier,
		PathClassifier:    pathClassifier,
	}
}

func (e EventExtractor) Extract(ctx context.Context, input ExtractInput) (ExtractResult, error) {
	events := orderedEvents(input.Events)
	runID := input.RunID
	if runID == "" && len(events) > 0 {
		runID = events[0].event.RunID
	}

	state := extractState{
		runID:     runID,
		byID:      map[string]int{},
		extractor: e,
	}
	for _, item := range events {
		if err := ctx.Err(); err != nil {
			return ExtractResult{}, err
		}
		state.applyEvent(item.event)
	}
	state.insertRecoverySteps()
	state.finalize()

	timeline := Timeline{
		SchemaVersion: SchemaVersion,
		RunID:         runID,
		Steps:         state.steps,
		Warnings:      state.warnings,
	}
	return ExtractResult{
		RunID:    runID,
		Timeline: timeline,
		Warnings: state.warnings,
	}, nil
}

type orderedEvent struct {
	event trace.Event
	pos   int
}

func orderedEvents(events []trace.Event) []orderedEvent {
	out := make([]orderedEvent, len(events))
	for i, event := range events {
		out[i] = orderedEvent{event: event, pos: i}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a := out[i].event.EventIndex
		b := out[j].event.EventIndex
		if a > 0 && b > 0 && a != b {
			return a < b
		}
		return out[i].pos < out[j].pos
	})
	return out
}

type extractState struct {
	runID     string
	steps     []Step
	byID      map[string]int
	warnings  []string
	extractor EventExtractor
}

func (s *extractState) applyEvent(event trace.Event) {
	switch event.Type {
	case trace.EventRunStart:
		s.addStep(s.baseStep(event, StepProcessStart, "process start"))
	case trace.EventRunStop:
		step := s.baseStep(event, StepProcessResult, "process result")
		step.Result = resultFromPayload(event.Payload, event.Type)
		s.addStep(step)
	case trace.EventProcessStart:
		step := s.commandStep(event, StepProcessStart)
		step.Summary = summaryForStep(step)
		s.addStep(step)
	case trace.EventProcessResult:
		step := s.commandStep(event, StepProcessResult)
		step.Result = resultFromPayload(event.Payload, event.Type)
		step.Summary = summaryForStep(step)
		s.addStep(step)
		s.addProcessChangedFilesStep(event)
	case trace.EventShellCommand:
		s.applyShellCommand(event)
	case trace.EventShellResult:
		s.applyShellResult(event)
	case trace.EventToolCall:
		s.applyToolCall(event)
	case trace.EventToolResult, trace.EventToolError:
		s.applyToolResult(event)
	case trace.EventFileRead:
		step := s.fileStep(event, StepReadFile)
		step.Kind = s.promoteReadKind(step.Files, step.Target)
		step.Summary = summaryForStep(step)
		s.addStep(step)
	case trace.EventFileWrite:
		step := s.fileStep(event, StepEditFile)
		step.Summary = summaryForStep(step)
		s.addStep(step)
	case trace.EventFilePatch:
		s.applyFilePatch(event)
	case trace.EventPermissionRequest:
		step := s.baseStep(event, StepPermissionRequest, "permission request")
		step.ToolName = payloadString(event.Payload, "tool_name", "name")
		step.Command = commandFromPayload(event.Payload)
		step.Action = firstExtractNonEmpty(step.Command, step.ToolName, "permission request")
		step.Summary = summaryForStep(step)
		s.addStep(step)
	case trace.EventContextCompact:
		s.addStep(s.baseStep(event, StepContextCompact, "context compact"))
	case trace.EventRaw:
		step := s.baseStep(event, StepRawBehavior, "raw behavior")
		step.Significant = false
		step.Confidence = 0.5
		s.addStep(step)
	case trace.EventUserPrompt, trace.EventModelCall, trace.EventModelResult,
		trace.EventSubagentStart, trace.EventSubagentStop:
		return
	default:
		step := s.baseStep(event, StepUnknown, "unknown event")
		step.Significant = false
		step.Confidence = 0.25
		s.addStep(step)
	}
}

func (s *extractState) applyShellCommand(event trace.Event) {
	step := s.commandStep(event, StepCallTool)
	step.ToolName = firstExtractNonEmpty(step.ToolName, "shell")
	classification := s.classifyCommand(step.Command)
	step.Kind = s.stepKindForClassification(classification, StepCallTool)
	applyCommandClassification(&step, classification)
	step.Summary = summaryForStep(step)
	s.mergeOrAdd(event, step)
}

func (s *extractState) applyShellResult(event trace.Event) {
	result := resultFromPayload(event.Payload, event.Type)
	if index, ok := s.relatedStepIndex(event); ok {
		s.mergeResult(index, event, result)
		return
	}
	step := s.commandStep(event, StepCallTool)
	step.ToolName = firstExtractNonEmpty(step.ToolName, "shell")
	step.Result = result
	classification := s.classifyCommand(step.Command)
	step.Kind = s.stepKindForClassification(classification, StepCallTool)
	applyCommandClassification(&step, classification)
	step.Summary = summaryForStep(step)
	s.mergeAdjacentShellResult(event, step)
}

func (s *extractState) applyToolCall(event trace.Event) {
	toolName := payloadString(event.Payload, "tool_name", "name", "tool")
	command := commandFromPayload(event.Payload)

	step := s.baseStep(event, StepCallTool, "tool call")
	step.ToolName = toolName
	step.Command = command
	step.Action = firstExtractNonEmpty(command, toolName, "tool call")
	step.Files = filesFromPayload(event.Payload)
	step.Target = firstExtractNonEmpty(payloadString(event.Payload, "target", "path", "file", "filename"), firstFile(step.Files))

	switch {
	case isMCPTool(toolName):
		step.Kind = StepCallMCPTool
	case isShellTool(toolName):
		classification := s.classifyCommand(command)
		step.Kind = s.stepKindForClassification(classification, StepCallTool)
		applyCommandClassification(&step, classification)
	case isReadTool(toolName):
		step.Kind = s.promoteReadKind(step.Files, step.Target)
	case isEditTool(toolName):
		step.Kind = StepEditFile
	default:
		step.Kind = StepCallTool
	}
	step.Summary = summaryForStep(step)
	s.mergeOrAdd(event, step)
}

func (s *extractState) applyToolResult(event trace.Event) {
	result := resultFromPayload(event.Payload, event.Type)
	if index, ok := s.relatedStepIndex(event); ok {
		s.mergeResult(index, event, result)
		if files := filesFromPayload(event.Payload); len(files) > 0 {
			s.steps[index].Files = appendUniqueFiles(s.steps[index].Files, files)
			s.steps[index].Target = firstExtractNonEmpty(s.steps[index].Target, firstFile(files))
		}
		return
	}

	step := s.baseStep(event, StepCallTool, "tool result")
	step.ToolName = payloadString(event.Payload, "tool_name", "name", "tool")
	step.Command = commandFromPayload(event.Payload)
	step.Files = filesFromPayload(event.Payload)
	step.Target = firstExtractNonEmpty(payloadString(event.Payload, "target", "path", "file", "filename"), firstFile(step.Files))
	step.Result = result
	if isMCPTool(step.ToolName) {
		step.Kind = StepCallMCPTool
	} else if isShellTool(step.ToolName) {
		classification := s.classifyCommand(step.Command)
		step.Kind = s.stepKindForClassification(classification, StepCallTool)
		applyCommandClassification(&step, classification)
	}
	step.Summary = summaryForStep(step)
	s.addStep(step)
}

func (s *extractState) addProcessChangedFilesStep(event trace.Event) {
	files := filesFromPayload(trace.Payload{"changed_files": event.Payload["changed_files"]})
	if len(files) == 0 {
		return
	}
	step := s.baseStep(event, StepEditFile, "edit files from process result")
	step.Files = files
	step.Target = firstFile(files)
	step.Summary = "edit files from process result"
	step.Significant = true
	s.addStep(step)
}

func (s *extractState) applyFilePatch(event trace.Event) {
	step := s.fileStep(event, StepEditFile)
	step.Summary = summaryForStep(step)
	if index, ok := s.relatedStepIndex(event); ok && s.steps[index].Kind == StepEditFile {
		s.steps[index].Files = appendUniqueFiles(s.steps[index].Files, step.Files)
		s.steps[index].Target = firstExtractNonEmpty(s.steps[index].Target, step.Target)
		s.addRefs(index, event)
		return
	}
	s.addStep(step)
}

func (s *extractState) commandStep(event trace.Event, fallback StepKind) Step {
	step := s.baseStep(event, fallback, "")
	step.Command = commandFromPayload(event.Payload)
	step.ToolName = payloadString(event.Payload, "tool_name", "name", "tool")
	step.Action = firstExtractNonEmpty(step.Command, step.ToolName, string(fallback))
	step.Files = filesFromPayload(event.Payload)
	step.Target = firstExtractNonEmpty(payloadString(event.Payload, "target", "path", "file", "filename"), firstFile(step.Files))
	return step
}

func (s *extractState) fileStep(event trace.Event, kind StepKind) Step {
	step := s.baseStep(event, kind, "")
	step.Files = filesFromPayload(event.Payload)
	step.Target = firstExtractNonEmpty(payloadString(event.Payload, "target", "path", "file", "filename"), firstFile(step.Files))
	step.Action = firstExtractNonEmpty(step.Target, string(kind))
	return step
}

func (s *extractState) baseStep(event trace.Event, kind StepKind, action string) Step {
	runID := firstExtractNonEmpty(s.runID, event.RunID)
	step := Step{
		RunID:          runID,
		Kind:           kind,
		Action:         action,
		Result:         ResultUnknown,
		Significant:    kind != StepRawBehavior && kind != StepUnknown && kind != StepProcessStart && kind != StepProcessResult,
		SourceRefs:     []StepRef{refFromEvent(event)},
		SourceEventIDs: sourceIDs(event),
		Confidence:     0.8,
		Attributes:     map[string]string{},
	}
	if event.Source.Adapter != "" {
		step.Attributes["source_adapter"] = event.Source.Adapter
	}
	if event.Source.RawEventType != "" {
		step.Attributes["raw_event_type"] = event.Source.RawEventType
	}
	if id := correlationID(event); id != "" {
		step.Attributes["correlation_id"] = id
	}
	if event.Type != "" {
		step.Attributes["source_event_type"] = string(event.Type)
	}
	return step
}

func (s *extractState) mergeOrAdd(event trace.Event, step Step) {
	if index, ok := s.relatedStepIndex(event); ok {
		existing := &s.steps[index]
		if existing.Kind == StepCallTool || existing.Kind == StepUnknown {
			existing.Kind = step.Kind
		}
		existing.Action = firstExtractNonEmpty(existing.Action, step.Action)
		existing.Command = firstExtractNonEmpty(existing.Command, step.Command)
		existing.ToolName = firstExtractNonEmpty(existing.ToolName, step.ToolName)
		existing.Target = firstExtractNonEmpty(existing.Target, step.Target)
		existing.Query = firstExtractNonEmpty(existing.Query, step.Query)
		existing.Files = appendUniqueFiles(existing.Files, step.Files)
		existing.Summary = summaryForStep(*existing)
		s.addRefs(index, event)
		return
	}
	s.addStep(step)
}

func (s *extractState) mergeResult(index int, event trace.Event, result StepResult) {
	if result != ResultUnknown || s.steps[index].Result == "" {
		s.steps[index].Result = result
	}
	if code, ok := exitCodeFromPayload(event.Payload); ok {
		s.ensureAttrs(index)
		s.steps[index].Attributes["exit_code"] = strconv.Itoa(code)
	}
	s.addRefs(index, event)
	s.steps[index].Summary = summaryForStep(s.steps[index])
}

func (s *extractState) mergeAdjacentShellResult(event trace.Event, step Step) {
	if len(s.steps) > 0 {
		last := len(s.steps) - 1
		if s.steps[last].Kind == StepSearch ||
			s.steps[last].Kind == StepReadFile ||
			s.steps[last].Kind == StepInspectTest ||
			s.steps[last].Kind == StepRunTest ||
			s.steps[last].Kind == StepRunBuild ||
			s.steps[last].Kind == StepInstallDependency ||
			s.steps[last].Kind == StepCallTool {
			if step.Command == "" || step.Command == s.steps[last].Command {
				s.mergeResult(last, event, step.Result)
				return
			}
		}
	}
	s.addStep(step)
}

func (s *extractState) relatedStepIndex(event trace.Event) (int, bool) {
	for _, id := range correlationIDs(event) {
		if index, ok := s.byID[id]; ok {
			return index, true
		}
	}
	return 0, false
}

func (s *extractState) addStep(step Step) {
	index := len(s.steps)
	step.Index = index
	step.StepID = fmt.Sprintf("step_%04d", index+1)
	if step.Summary == "" {
		step.Summary = summaryForStep(step)
	}
	if step.Fingerprint == "" {
		step.Fingerprint = step.StableKey()
	}
	if len(step.Attributes) == 0 {
		step.Attributes = nil
	}
	s.steps = append(s.steps, step)
	for _, id := range correlationIDsFromStep(step) {
		s.byID[id] = index
	}
}

func (s *extractState) addRefs(index int, event trace.Event) {
	s.steps[index].SourceRefs = appendUniqueRefs(s.steps[index].SourceRefs, refFromEvent(event))
	s.steps[index].SourceEventIDs = appendUniqueStrings(s.steps[index].SourceEventIDs, sourceIDs(event))
	for _, id := range correlationIDs(event) {
		s.byID[id] = index
	}
}

func (s *extractState) ensureAttrs(index int) {
	if s.steps[index].Attributes == nil {
		s.steps[index].Attributes = map[string]string{}
	}
}

func (s *extractState) finalize() {
	for i := range s.steps {
		s.steps[i].Index = i
		s.steps[i].StepID = fmt.Sprintf("step_%04d", i+1)
		s.steps[i].Files = SortFiles(s.steps[i].Files)
		if s.steps[i].Target != "" {
			s.steps[i].Target = NormalizePathForKey(s.steps[i].Target)
		}
		s.steps[i].Fingerprint = s.steps[i].StableKey()
		if len(s.steps[i].Attributes) == 0 {
			s.steps[i].Attributes = nil
		}
	}
}

func (s *extractState) insertRecoverySteps() {
	var out []Step
	var failed *Step
	for _, step := range s.steps {
		if failed != nil && isRecoveryEvidence(step) {
			recovery := Step{
				RunID:          step.RunID,
				Kind:           StepRecoverFromError,
				Action:         "recover_after_failure",
				Summary:        "recover after failed " + string(failed.Kind),
				Result:         ResultUnknown,
				Significant:    true,
				SourceRefs:     append([]StepRef{}, step.SourceRefs...),
				SourceEventIDs: append([]string{}, step.SourceEventIDs...),
				Confidence:     0.7,
				Attributes: map[string]string{
					"failed_step_kind": string(failed.Kind),
					"recovery_kind":    string(step.Kind),
				},
			}
			out = append(out, recovery)
			failed = nil
		}
		out = append(out, step)
		if step.IsValidation() && step.Result == ResultFailure {
			copy := step
			failed = &copy
			continue
		}
		if step.Result == ResultSuccess {
			failed = nil
		}
	}
	s.steps = out
	s.byID = map[string]int{}
	for i, step := range s.steps {
		for _, id := range correlationIDsFromStep(step) {
			s.byID[id] = i
		}
	}
}

func (s *extractState) stepKindForClassification(classification CommandClassification, fallback StepKind) StepKind {
	switch classification.Kind {
	case CommandSearch:
		return StepSearch
	case CommandReadFile:
		if s.hasTestFile(classification.Files) {
			return StepInspectTest
		}
		return StepReadFile
	case CommandRunTest:
		return StepRunTest
	case CommandRunBuild:
		return StepRunBuild
	case CommandInstallDependency:
		return StepInstallDependency
	default:
		return fallback
	}
}

func (s *extractState) stepKindForCommand(command string, fallback StepKind) StepKind {
	return s.stepKindForClassification(s.classifyCommand(command), fallback)
}

func applyCommandClassification(step *Step, classification CommandClassification) {
	if classification.Kind == "" || classification.Kind == CommandUnknown {
		return
	}
	if step.Attributes == nil {
		step.Attributes = map[string]string{}
	}
	step.Attributes["command_kind"] = string(classification.Kind)
	for key, value := range classification.Attributes {
		if strings.TrimSpace(value) != "" {
			step.Attributes[key] = value
		}
	}
	if classification.Confidence > 0 {
		step.Confidence = classification.Confidence
	}
	switch step.Kind {
	case StepSearch:
		if classification.Query != "" {
			step.Query = classification.Query
		}
		step.Files = appendUniqueFiles(step.Files, classification.Files)
	case StepReadFile, StepInspectTest:
		step.Files = appendUniqueFiles(step.Files, classification.Files)
		if step.Target == "" {
			step.Target = firstFile(step.Files)
		}
	case StepRunTest, StepRunBuild, StepInstallDependency:
		step.Files = appendUniqueFiles(step.Files, classification.Files)
	}
}

func (s *extractState) classifyCommand(command string) CommandClassification {
	if s.extractor.CommandClassifier != nil {
		classification := s.extractor.CommandClassifier.ClassifyCommand(command)
		if classification.Kind != "" && classification.Kind != CommandUnknown {
			return classification
		}
	}
	return fallbackClassifyCommand(command)
}

func (s *extractState) promoteReadKind(files []string, target string) StepKind {
	if s.hasTestFile(append(files, target)) {
		return StepInspectTest
	}
	return StepReadFile
}

func (s *extractState) hasTestFile(files []string) bool {
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		if s.extractor.PathClassifier != nil {
			classification := s.extractor.PathClassifier.ClassifyPath(file)
			if classification.Kind == PathTest {
				return true
			}
		}
		if extractFallbackPathKind(file) == PathTest {
			return true
		}
	}
	return false
}

func fallbackClassifyCommand(command string) CommandClassification {
	normalized := normalizeCommandText(command)
	tokens := splitCommand(normalized)
	classification := CommandClassification{
		Kind:       CommandUnknown,
		Command:    normalized,
		Confidence: 0.4,
	}
	if len(tokens) == 0 {
		return classification
	}
	base := extractCommandBase(tokens[0])
	second := tokenAt(tokens, 1)
	third := tokenAt(tokens, 2)
	lower := strings.ToLower(normalized)

	switch {
	case base == "git" && second == "grep":
		classification.Kind = CommandSearch
		classification.Query = tokenAt(tokens, 2)
		classification.Files = commandPathTokens(tokens[3:])
		classification.Confidence = 0.8
	case base == "rg" || base == "grep" || base == "findstr" || base == "fd" || base == "find":
		classification.Kind = CommandSearch
		classification.Query = firstNonOption(tokens[1:])
		classification.Files = commandPathTokens(tokens[1:])
		classification.Confidence = 0.75
	case base == "go" && second == "test":
		classification.Kind = CommandRunTest
		classification.Confidence = 0.95
	case (base == "npm" || base == "pnpm" || base == "yarn") && (second == "test" || (second == "run" && third == "test")):
		classification.Kind = CommandRunTest
		classification.Confidence = 0.9
	case base == "pytest" || (base == "python" && second == "-m" && third == "pytest") ||
		(base == "cargo" && second == "test") || (base == "mvn" && second == "test") ||
		strings.Contains(lower, "gradle test") || strings.Contains(lower, "gradlew test"):
		classification.Kind = CommandRunTest
		classification.Confidence = 0.9
	case base == "go" && (second == "build" || second == "vet"):
		classification.Kind = CommandRunBuild
		classification.Confidence = 0.9
	case (base == "npm" || base == "pnpm" || base == "yarn") && (second == "build" || second == "lint" || (second == "run" && (third == "build" || third == "lint"))):
		classification.Kind = CommandRunBuild
		classification.Confidence = 0.85
	case base == "cat" || base == "type" || base == "head" || base == "tail" ||
		base == "less" || base == "more" || base == "sed" || base == "get-content":
		classification.Kind = CommandReadFile
		classification.Files = commandPathTokens(tokens[1:])
		classification.Confidence = 0.7
	case (base == "npm" && (second == "install" || second == "i")) ||
		(base == "pnpm" && (second == "add" || second == "install")) ||
		(base == "yarn" && (second == "add" || second == "install")):
		classification.Kind = CommandInstallDependency
		classification.Confidence = 0.8
	}
	return classification
}

func extractFallbackPathKind(path string) PathKind {
	normalized := strings.ToLower(NormalizePathForKey(path))
	switch {
	case normalized == "":
		return PathUnknown
	case strings.Contains(normalized, ".env") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "id_rsa") ||
		strings.Contains(normalized, "private_key"):
		return PathSecret
	case strings.Contains(normalized, "legacy") ||
		strings.Contains(normalized, "deprecated"):
		return PathLegacy
	case strings.HasPrefix(normalized, "docs/") ||
		strings.HasPrefix(normalized, "doc/") ||
		strings.HasPrefix(normalized, "documentation/") ||
		strings.HasSuffix(normalized, ".md"):
		return PathDocs
	case strings.HasPrefix(normalized, "test/") ||
		strings.HasPrefix(normalized, "tests/") ||
		strings.Contains(normalized, "/test/") ||
		strings.Contains(normalized, "/tests/") ||
		strings.HasSuffix(normalized, "_test.go") ||
		strings.Contains(normalized, ".test.") ||
		strings.Contains(normalized, ".spec."):
		return PathTest
	case strings.HasSuffix(normalized, ".yml") ||
		strings.HasSuffix(normalized, ".yaml") ||
		strings.HasSuffix(normalized, ".json") ||
		strings.HasSuffix(normalized, ".toml") ||
		strings.HasSuffix(normalized, ".ini") ||
		strings.Contains(normalized, "config"):
		return PathConfig
	default:
		return PathSource
	}
}

func summaryForStep(step Step) string {
	target := firstExtractNonEmpty(step.Command, step.Target, step.ToolName, step.Action)
	if target == "" {
		return string(step.Kind)
	}
	return strings.TrimSpace(string(step.Kind) + " " + singleLine(target))
}

func refFromEvent(event trace.Event) StepRef {
	return StepRef{
		EventID:    event.EventID,
		EventIndex: event.EventIndex,
		EventType:  string(event.Type),
	}
}

func sourceIDs(event trace.Event) []string {
	if event.EventID == "" {
		return nil
	}
	return []string{event.EventID}
}

func correlationID(event trace.Event) string {
	ids := correlationIDs(event)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func correlationIDs(event trace.Event) []string {
	var ids []string
	for _, id := range []string{
		payloadString(event.Payload, "tool_use_id", "call_id", "id"),
		event.SpanID,
		event.ParentID,
	} {
		if strings.TrimSpace(id) != "" {
			ids = appendUniqueStrings(ids, []string{id})
		}
	}
	return ids
}

func correlationIDsFromStep(step Step) []string {
	var ids []string
	if step.Attributes != nil {
		if id := step.Attributes["correlation_id"]; id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func commandFromPayload(payload trace.Payload) string {
	if payload == nil {
		return ""
	}
	if command := payloadString(payload, "command", "cmd"); command != "" {
		return singleLine(command)
	}
	for _, key := range []string{"input", "arguments", "tool_input", "params", "request", "result"} {
		if nested, ok := payload[key].(map[string]any); ok {
			if command := payloadString(nested, "command", "cmd"); command != "" {
				return singleLine(command)
			}
		}
	}
	args := stringSlice(payload["args"])
	if len(args) > 0 {
		if command := payloadString(payload, "program", "executable", "name"); command != "" {
			return singleLine(strings.TrimSpace(command + " " + strings.Join(args, " ")))
		}
	}
	return ""
}

func filesFromPayload(payload trace.Payload) []string {
	if payload == nil {
		return nil
	}
	var files []string
	for _, key := range []string{"files", "changed_files", "paths", "path", "file", "filename", "target"} {
		files = append(files, stringSlice(payload[key])...)
	}
	for _, key := range []string{"input", "arguments", "tool_input", "params", "result"} {
		if nested, ok := payload[key].(map[string]any); ok {
			for _, nestedKey := range []string{"files", "changed_files", "paths", "path", "file", "filename", "target"} {
				files = append(files, stringSlice(nested[nestedKey])...)
			}
		}
	}
	return SortFiles(files)
}

func payloadString(payload map[string]any, keys ...string) string {
	if payload == nil {
		return ""
	}
	for _, key := range keys {
		switch value := payload[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case fmt.Stringer:
			text := strings.TrimSpace(value.String())
			if text != "" {
				return text
			}
		case int:
			return strconv.Itoa(value)
		case int64:
			return strconv.FormatInt(value, 10)
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(value)
		}
	}
	return ""
}

func stringSlice(value any) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{strings.TrimSpace(v)}
	case []string:
		return v
	case []any:
		var out []string
		for _, item := range v {
			out = append(out, stringSlice(item)...)
		}
		return out
	default:
		return nil
	}
}

func resultFromPayload(payload trace.Payload, eventType trace.EventType) StepResult {
	if eventType == trace.EventToolError {
		return ResultFailure
	}
	if code, ok := exitCodeFromPayload(payload); ok {
		if code == 0 {
			return ResultSuccess
		}
		return ResultFailure
	}
	status := strings.ToLower(payloadString(payload, "status", "result", "outcome"))
	switch status {
	case "success", "succeeded", "ok", "passed", "pass":
		return ResultSuccess
	case "failure", "failed", "fail", "error", "errored":
		return ResultFailure
	case "skipped", "skip":
		return ResultSkipped
	default:
		return ResultUnknown
	}
}

func exitCodeFromPayload(payload trace.Payload) (int, bool) {
	if payload == nil {
		return 0, false
	}
	for _, key := range []string{"exit_code", "exitCode", "code"} {
		switch value := payload[key].(type) {
		case int:
			return value, true
		case int64:
			return int(value), true
		case float64:
			return int(value), true
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				return parsed, true
			}
		}
	}
	if nested, ok := payload["result"].(map[string]any); ok {
		return exitCodeFromPayload(nested)
	}
	return 0, false
}

func isMCPTool(toolName string) bool {
	name := strings.ToLower(strings.TrimSpace(toolName))
	return strings.HasPrefix(name, "mcp__") || strings.HasPrefix(name, "mcp.")
}

func isShellTool(toolName string) bool {
	name := strings.ToLower(strings.TrimSpace(toolName))
	return name == "bash" || name == "shell" || name == "shell.exec" || name == "sh" || name == "powershell"
}

func isReadTool(toolName string) bool {
	name := strings.ToLower(strings.TrimSpace(toolName))
	return name == "read" || name == "open" || name == "view" || name == "read_file"
}

func isEditTool(toolName string) bool {
	name := strings.ToLower(strings.TrimSpace(toolName))
	return name == "edit" || name == "write" || name == "apply_patch" || name == "file_write"
}

func isRecoveryEvidence(step Step) bool {
	return step.Kind == StepReadFile ||
		step.Kind == StepInspectTest ||
		step.Kind == StepEditFile ||
		step.Kind == StepSearch ||
		step.Kind == StepRunTest
}

func appendUniqueFiles(existing []string, more []string) []string {
	return SortFiles(append(existing, more...))
}

func appendUniqueStrings(existing []string, more []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range existing {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, value := range more {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func appendUniqueRefs(existing []StepRef, ref StepRef) []StepRef {
	key := func(r StepRef) string {
		return r.EventID + "\x00" + strconv.FormatInt(r.EventIndex, 10) + "\x00" + r.EventType
	}
	seen := map[string]bool{}
	var out []StepRef
	for _, item := range existing {
		k := key(item)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, item)
	}
	k := key(ref)
	if !seen[k] {
		out = append(out, ref)
	}
	return out
}

func firstExtractNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstFile(files []string) string {
	if len(files) == 0 {
		return ""
	}
	return files[0]
}

func normalizeCommandText(command string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(command)), " ")
}

func splitCommand(command string) []string {
	var tokens []string
	var current strings.Builder
	var quote rune
	escaped := false
	for _, r := range command {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			current.WriteRune(r)
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func extractCommandBase(token string) string {
	token = strings.TrimSpace(token)
	token = strings.Trim(token, "\"'")
	token = strings.TrimSuffix(token, ".exe")
	token = filepath.Base(strings.ReplaceAll(token, "\\", "/"))
	return strings.ToLower(token)
}

func tokenAt(tokens []string, index int) string {
	if index < 0 || index >= len(tokens) {
		return ""
	}
	return strings.ToLower(tokens[index])
}

func firstNonOption(tokens []string) string {
	for _, token := range tokens {
		if strings.TrimSpace(token) == "" || strings.HasPrefix(token, "-") {
			continue
		}
		return token
	}
	return ""
}

func commandPathTokens(tokens []string) []string {
	var files []string
	for _, token := range tokens {
		token = strings.Trim(token, "\"'")
		if token == "" || strings.HasPrefix(token, "-") {
			continue
		}
		if strings.Contains(token, "/") ||
			strings.Contains(token, "\\") ||
			strings.Contains(token, ".") {
			files = append(files, token)
		}
	}
	return SortFiles(files)
}

func singleLine(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.Join(strings.Fields(value), " ")
}
