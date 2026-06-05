package behavior

import (
	"context"
	"reflect"
	"testing"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestExtractShellSearch(t *testing.T) {
	result := extractForTestWithClassifiers(t, []trace.Event{
		behaviorEvent("evt-1", 1, trace.EventShellCommand, trace.Payload{
			"tool_use_id": "u1",
			"command":     `rg "session" src tests`,
		}),
	})

	steps := result.Timeline.Steps
	if len(steps) != 1 {
		t.Fatalf("steps len = %d, want 1", len(steps))
	}
	step := steps[0]
	if step.Kind != StepSearch {
		t.Fatalf("kind = %q, want search", step.Kind)
	}
	if step.Command != `rg "session" src tests` {
		t.Fatalf("command = %q", step.Command)
	}
	if step.Query != "session" {
		t.Fatalf("query = %q, want session", step.Query)
	}
	if !reflect.DeepEqual(step.Files, []string{"src", "tests"}) {
		t.Fatalf("files = %#v, want src/tests", step.Files)
	}
	if step.Attributes["command_kind"] != string(CommandSearch) || step.Attributes["tool"] != "rg" {
		t.Fatalf("classification attributes = %#v", step.Attributes)
	}
	if step.SourceRefs[0].EventID != "evt-1" || step.SourceRefs[0].EventIndex != 1 {
		t.Fatalf("source refs not preserved: %#v", step.SourceRefs)
	}
}

func TestExtractShellReadTestFromClassifier(t *testing.T) {
	result := extractForTestWithClassifiers(t, []trace.Event{
		behaviorEvent("evt-1", 1, trace.EventShellCommand, trace.Payload{
			"command": "cat tests/auth/session.test.ts",
		}),
	})

	step := onlyStep(t, result)
	if step.Kind != StepInspectTest {
		t.Fatalf("kind = %q, want inspect_test", step.Kind)
	}
	if !reflect.DeepEqual(step.Files, []string{"tests/auth/session.test.ts"}) {
		t.Fatalf("files = %#v", step.Files)
	}
	if step.Target != "tests/auth/session.test.ts" {
		t.Fatalf("target = %q", step.Target)
	}
}

func TestExtractShellRunTestWithResult(t *testing.T) {
	result := extractForTest(t, []trace.Event{
		behaviorEvent("evt-1", 1, trace.EventShellCommand, trace.Payload{
			"tool_use_id": "u1",
			"command":     "go test ./...",
		}),
		behaviorEvent("evt-2", 2, trace.EventShellResult, trace.Payload{
			"tool_use_id": "u1",
			"exit_code":   0,
		}),
	})

	steps := result.Timeline.Steps
	if len(steps) != 1 {
		t.Fatalf("steps len = %d, want merged command/result", len(steps))
	}
	step := steps[0]
	if step.Kind != StepRunTest || step.Result != ResultSuccess {
		t.Fatalf("step = %#v, want run_test success", step)
	}
	if !reflect.DeepEqual(step.SourceEventIDs, []string{"evt-1", "evt-2"}) {
		t.Fatalf("source event ids = %#v", step.SourceEventIDs)
	}
	if step.Attributes["exit_code"] != "0" {
		t.Fatalf("exit_code attr = %#v", step.Attributes)
	}
}

func TestExtractToolReadFile(t *testing.T) {
	result := extractForTest(t, []trace.Event{
		behaviorEvent("evt-1", 1, trace.EventToolCall, trace.Payload{
			"tool_use_id": "u1",
			"tool_name":   "Read",
			"path":        "src/session.go",
		}),
	})

	step := onlyStep(t, result)
	if step.Kind != StepReadFile {
		t.Fatalf("kind = %q, want read_file", step.Kind)
	}
	if step.Target != "src/session.go" {
		t.Fatalf("target = %q", step.Target)
	}
}

func TestExtractToolReadTestFileAsInspectTest(t *testing.T) {
	result := extractForTest(t, []trace.Event{
		behaviorEvent("evt-1", 1, trace.EventToolCall, trace.Payload{
			"tool_name": "Read",
			"path":      "tests/session.test.ts",
		}),
	})

	step := onlyStep(t, result)
	if step.Kind != StepInspectTest {
		t.Fatalf("kind = %q, want inspect_test", step.Kind)
	}
}

func TestExtractToolEditFile(t *testing.T) {
	result := extractForTest(t, []trace.Event{
		behaviorEvent("evt-1", 1, trace.EventToolCall, trace.Payload{
			"tool_name": "Edit",
			"path":      "src/session.go",
		}),
	})

	step := onlyStep(t, result)
	if step.Kind != StepEditFile {
		t.Fatalf("kind = %q, want edit_file", step.Kind)
	}
	if !reflect.DeepEqual(step.Files, []string{"src/session.go"}) {
		t.Fatalf("files = %#v", step.Files)
	}
}

func TestExtractFilePatch(t *testing.T) {
	result := extractForTest(t, []trace.Event{
		behaviorEvent("evt-1", 1, trace.EventFilePatch, trace.Payload{
			"changed_files": []any{"src/session.go", "tests/session_test.go"},
		}),
	})

	step := onlyStep(t, result)
	if step.Kind != StepEditFile {
		t.Fatalf("kind = %q, want edit_file", step.Kind)
	}
	want := []string{"src/session.go", "tests/session_test.go"}
	if !reflect.DeepEqual(step.Files, want) {
		t.Fatalf("files = %#v, want %#v", step.Files, want)
	}
}

func TestExtractMCPTool(t *testing.T) {
	result := extractForTest(t, []trace.Event{
		behaviorEvent("evt-1", 1, trace.EventToolCall, trace.Payload{
			"tool_name": "mcp__node_repl__js",
		}),
	})

	step := onlyStep(t, result)
	if step.Kind != StepCallMCPTool {
		t.Fatalf("kind = %q, want call_mcp_tool", step.Kind)
	}
}

func TestExtractUnknownRawEvent(t *testing.T) {
	result := extractForTest(t, []trace.Event{
		behaviorEvent("evt-raw", 1, trace.EventRaw, trace.Payload{"raw": "x"}),
		behaviorEvent("evt-unknown", 2, trace.EventType("custom_event"), trace.Payload{"value": "x"}),
	})

	steps := result.Timeline.Steps
	if len(steps) != 2 {
		t.Fatalf("steps len = %d, want 2", len(steps))
	}
	if steps[0].Kind != StepRawBehavior || steps[0].Significant {
		t.Fatalf("raw step = %#v", steps[0])
	}
	if steps[1].Kind != StepUnknown || steps[1].Significant {
		t.Fatalf("unknown step = %#v", steps[1])
	}
}

func TestExtractorDoesNotPanicOnMissingFields(t *testing.T) {
	result := extractForTest(t, []trace.Event{
		behaviorEvent("evt-1", 1, trace.EventToolCall, nil),
		behaviorEvent("evt-2", 2, trace.EventShellResult, trace.Payload{}),
		behaviorEvent("evt-3", 3, trace.EventFileRead, trace.Payload{}),
	})

	if len(result.Timeline.Steps) == 0 {
		t.Fatalf("expected degraded steps, got none")
	}
}

func TestExtractorMergesToolCallAndResult(t *testing.T) {
	result := extractForTest(t, []trace.Event{
		behaviorEvent("evt-tool-call", 1, trace.EventToolCall, trace.Payload{
			"tool_use_id": "u1",
			"tool_name":   "Bash",
			"input": map[string]any{
				"command": "go test ./...",
			},
		}),
		behaviorEvent("evt-shell-command", 2, trace.EventShellCommand, trace.Payload{
			"tool_use_id": "u1",
			"command":     "go test ./...",
		}),
		behaviorEvent("evt-tool-result", 3, trace.EventToolResult, trace.Payload{
			"tool_use_id": "u1",
			"result": map[string]any{
				"exit_code": float64(1),
			},
		}),
		behaviorEvent("evt-shell-result", 4, trace.EventShellResult, trace.Payload{
			"tool_use_id": "u1",
			"exit_code":   1,
		}),
	})

	steps := result.Timeline.Steps
	if len(steps) != 1 {
		t.Fatalf("steps len = %d, want one merged step: %#v", len(steps), steps)
	}
	step := steps[0]
	if step.Kind != StepRunTest || step.Result != ResultFailure {
		t.Fatalf("step = %#v, want failed run_test", step)
	}
	wantIDs := []string{"evt-tool-call", "evt-shell-command", "evt-tool-result", "evt-shell-result"}
	if !reflect.DeepEqual(step.SourceEventIDs, wantIDs) {
		t.Fatalf("source event ids = %#v, want %#v", step.SourceEventIDs, wantIDs)
	}
}

func TestExtractorStableOrderByEventIndex(t *testing.T) {
	result := extractForTest(t, []trace.Event{
		behaviorEvent("evt-2", 2, trace.EventToolCall, trace.Payload{
			"tool_name": "Edit",
			"path":      "src/session.go",
		}),
		behaviorEvent("evt-1", 1, trace.EventShellCommand, trace.Payload{
			"command": `rg "session" src`,
		}),
	})

	steps := result.Timeline.Steps
	if len(steps) != 2 {
		t.Fatalf("steps len = %d, want 2", len(steps))
	}
	if steps[0].SourceEventIDs[0] != "evt-1" || steps[1].SourceEventIDs[0] != "evt-2" {
		t.Fatalf("steps not ordered by event_index: %#v", steps)
	}
	if steps[0].Index != 0 || steps[0].StepID != "step_0001" ||
		steps[1].Index != 1 || steps[1].StepID != "step_0002" {
		t.Fatalf("step ids/indexes not stable: %#v", steps)
	}
}

func TestExtractorAddsRecoveryStepAfterFailedValidation(t *testing.T) {
	result := extractForTest(t, []trace.Event{
		behaviorEvent("evt-1", 1, trace.EventShellCommand, trace.Payload{
			"tool_use_id": "u1",
			"command":     "go test ./...",
		}),
		behaviorEvent("evt-2", 2, trace.EventShellResult, trace.Payload{
			"tool_use_id": "u1",
			"exit_code":   1,
		}),
		behaviorEvent("evt-3", 3, trace.EventToolCall, trace.Payload{
			"tool_name": "Edit",
			"path":      "src/session.go",
		}),
	})

	steps := result.Timeline.Steps
	if len(steps) != 3 {
		t.Fatalf("steps len = %d, want run_test/recover/edit: %#v", len(steps), steps)
	}
	if steps[0].Kind != StepRunTest || steps[1].Kind != StepRecoverFromError || steps[2].Kind != StepEditFile {
		t.Fatalf("unexpected recovery sequence: %#v", steps)
	}
	if steps[1].Attributes["failed_step_kind"] != string(StepRunTest) {
		t.Fatalf("recovery attributes = %#v", steps[1].Attributes)
	}
}

func TestExtractProcessResultChangedFilesAddsEditStepAndMetrics(t *testing.T) {
	result := extractForTestWithClassifiers(t, []trace.Event{
		behaviorEvent("evt-1", 1, trace.EventProcessResult, trace.Payload{
			"command":       "some-agent fix session",
			"exit_code":     0,
			"changed_files": []any{"src/auth/session.ts"},
		}),
	})

	steps := result.Timeline.Steps
	if len(steps) != 2 {
		t.Fatalf("steps len = %d, want process_result and edit_file: %#v", len(steps), steps)
	}
	if steps[0].Kind != StepProcessResult || steps[1].Kind != StepEditFile {
		t.Fatalf("unexpected steps: %#v", steps)
	}
	if !reflect.DeepEqual(steps[1].Files, []string{"src/auth/session.ts"}) {
		t.Fatalf("edit files = %#v", steps[1].Files)
	}
	if steps[1].SourceRefs[0].EventID != "evt-1" {
		t.Fatalf("edit source refs = %#v", steps[1].SourceRefs)
	}
	metrics := ComputeMetrics(result.Timeline)
	if metrics.EditScope.FilesEdited != 1 {
		t.Fatalf("files edited = %d, want 1", metrics.EditScope.FilesEdited)
	}
}

func TestBehaviorDiffFromTraceSearchQueryDivergence(t *testing.T) {
	runA := extractForTestWithClassifiers(t, []trace.Event{
		behaviorEvent("evt-a-1", 1, trace.EventShellCommand, trace.Payload{"command": `rg "session" src tests`}),
	})
	runB := extractForTestWithClassifiers(t, []trace.Event{
		behaviorEvent("evt-b-1", 1, trace.EventShellCommand, trace.Payload{"command": `rg "cookie" src`}),
	})

	diff := DiffTimelines(runA.Timeline, runB.Timeline, DiffOptions{IgnoreProcessNoise: true, IgnoreRawBehavior: true})
	if diff.FirstDivergence == nil {
		t.Fatalf("expected divergence")
	}
	if diff.FirstDivergence.Summary != "search query divergence at step 1" {
		t.Fatalf("summary = %q", diff.FirstDivergence.Summary)
	}
}

func TestBehaviorDiffFromTraceTestInspectionVsLegacyRead(t *testing.T) {
	runA := extractForTestWithClassifiers(t, []trace.Event{
		behaviorEvent("evt-a-1", 1, trace.EventShellCommand, trace.Payload{"command": `rg "session" src tests`}),
		behaviorEvent("evt-a-2", 2, trace.EventShellCommand, trace.Payload{"command": "cat tests/auth/session.test.ts"}),
	})
	runB := extractForTestWithClassifiers(t, []trace.Event{
		behaviorEvent("evt-b-1", 1, trace.EventShellCommand, trace.Payload{"command": `rg "session" src tests`}),
		behaviorEvent("evt-b-2", 2, trace.EventShellCommand, trace.Payload{"command": "cat src/auth/legacy-cookie.ts"}),
	})

	diff := DiffTimelines(runA.Timeline, runB.Timeline, DiffOptions{IgnoreProcessNoise: true, IgnoreRawBehavior: true})
	if diff.FirstDivergence == nil {
		t.Fatalf("expected divergence")
	}
	if diff.FirstDivergence.RunAStep == nil || diff.FirstDivergence.RunAStep.Kind != StepInspectTest {
		t.Fatalf("run A step = %#v", diff.FirstDivergence.RunAStep)
	}
	if diff.FirstDivergence.RunBStep == nil || diff.FirstDivergence.RunBStep.Kind != StepReadFile {
		t.Fatalf("run B step = %#v", diff.FirstDivergence.RunBStep)
	}
	if diff.FirstDivergence.Explanation != "Run B entered a legacy path before Run A inspected tests." {
		t.Fatalf("explanation = %q", diff.FirstDivergence.Explanation)
	}
}

func extractForTest(t *testing.T, events []trace.Event) ExtractResult {
	t.Helper()
	extractor := NewEventExtractor(nil, nil)
	result, err := extractor.Extract(context.Background(), ExtractInput{
		RunID:  "run-test",
		Events: events,
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return result
}

func extractForTestWithClassifiers(t *testing.T, events []trace.Event) ExtractResult {
	t.Helper()
	extractor := NewEventExtractor(NewDefaultCommandClassifier(), NewDefaultPathClassifier())
	result, err := extractor.Extract(context.Background(), ExtractInput{
		RunID:  "run-test",
		Events: events,
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return result
}

func onlyStep(t *testing.T, result ExtractResult) Step {
	t.Helper()
	if len(result.Timeline.Steps) != 1 {
		t.Fatalf("steps len = %d, want 1: %#v", len(result.Timeline.Steps), result.Timeline.Steps)
	}
	return result.Timeline.Steps[0]
}

func behaviorEvent(id string, index int64, typ trace.EventType, payload trace.Payload) trace.Event {
	return trace.Event{
		SchemaVersion: trace.SchemaVersion,
		EventID:       id,
		RunID:         "run-test",
		EventIndex:    index,
		Type:          typ,
		Source:        trace.Source{Adapter: "fixture", RawEventType: string(typ)},
		Payload:       payload,
	}
}
