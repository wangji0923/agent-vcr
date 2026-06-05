package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestDiffFirstDivergenceGolden(t *testing.T) {
	runA := RunData{
		RunID: "good",
		Events: []trace.Event{
			testEvent("a1", "good", 1, trace.EventRunStart, trace.Payload{"status": "started"}),
			testEvent("a2", "good", 2, trace.EventToolCall, trace.Payload{
				"tool_name": "Bash",
				"input":     map[string]any{"command": `rg "session" src tests`},
			}),
		},
	}
	runB := RunData{
		RunID: "bad",
		Events: []trace.Event{
			testEvent("b1", "bad", 1, trace.EventRunStart, trace.Payload{"status": "started"}),
			testEvent("b2", "bad", 2, trace.EventToolCall, trace.Payload{
				"tool_name": "Bash",
				"input":     map[string]any{"command": `rg "cookie" src`},
			}),
		},
	}

	got := RenderDiffText(DiffRuns(runA, runB))
	wantPath := filepath.Join("..", "..", "testdata", "golden", "diff", "first_divergence.txt")
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("diff output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestDiffIgnoresTimestampAndVolatileFields(t *testing.T) {
	runA := RunData{RunID: "a", Events: []trace.Event{
		testEvent("evt-a", "run-a", 1, trace.EventToolCall, trace.Payload{
			"tool_name":   "Read",
			"duration_ms": 123,
			"input": map[string]any{
				"path": `C:\Users\alice\project\src\main.go`,
			},
			"stdout_blob": "blobs/a.txt",
		}),
	}}
	runB := RunData{RunID: "b", Events: []trace.Event{
		testEvent("evt-b", "run-b", 99, trace.EventToolCall, trace.Payload{
			"tool_name":   "Read",
			"duration_ms": 987,
			"input": map[string]any{
				"path": `C:\Users\bob\project\src\main.go`,
			},
			"stdout_blob": "blobs/b.txt",
		}),
	}}
	runA.Events[0].Timestamp = time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC)
	runB.Events[0].Timestamp = time.Date(2026, 6, 2, 1, 0, 0, 0, time.UTC)

	result := DiffRuns(runA, runB)
	if result.FirstDivergence != nil {
		t.Fatalf("volatile fields caused divergence: %#v", result.FirstDivergence)
	}
}

func TestDiffDetectsPromptHashMismatch(t *testing.T) {
	runA := RunData{RunID: "a", Events: []trace.Event{
		testEvent("a1", "a", 1, trace.EventUserPrompt, trace.Payload{"prompt_sha256": "prompt-a"}),
	}}
	runB := RunData{RunID: "b", Events: []trace.Event{
		testEvent("b1", "b", 1, trace.EventUserPrompt, trace.Payload{"prompt_sha256": "prompt-b"}),
	}}

	result := DiffRuns(runA, runB)
	if result.FirstDivergence == nil || result.FirstDivergence.Reason != "user_prompt_signature_mismatch" {
		t.Fatalf("first divergence = %#v, want user_prompt_signature_mismatch", result.FirstDivergence)
	}
}

func TestDiffDetectsAssistantOutputHashMismatch(t *testing.T) {
	runA := RunData{RunID: "a", Events: []trace.Event{
		testEvent("a1", "a", 1, trace.EventRunStop, trace.Payload{"last_assistant_message_sha256": "output-a"}),
	}}
	runB := RunData{RunID: "b", Events: []trace.Event{
		testEvent("b1", "b", 1, trace.EventRunStop, trace.Payload{"last_assistant_message_sha256": "output-b"}),
	}}

	result := DiffRuns(runA, runB)
	if result.FirstDivergence == nil || result.FirstDivergence.Reason != "run_stop_signature_mismatch" {
		t.Fatalf("first divergence = %#v, want run_stop_signature_mismatch", result.FirstDivergence)
	}
}

func TestChangedFilesDiff(t *testing.T) {
	runA := RunData{RunID: "a", Events: []trace.Event{
		testEvent("a1", "a", 1, trace.EventProcessResult, trace.Payload{
			"command":       "agent",
			"exit_code":     0,
			"changed_files": []string{"src/a.go", "README.md"},
		}),
	}}
	runB := RunData{RunID: "b", Events: []trace.Event{
		testEvent("b1", "b", 1, trace.EventProcessResult, trace.Payload{
			"command":       "agent",
			"exit_code":     0,
			"changed_files": []string{"src/a.go", "src/b.go"},
		}),
	}}

	result := DiffRuns(runA, runB)
	if result.FirstDivergence == nil || result.FirstDivergence.Reason != "changed_files_mismatch" {
		t.Fatalf("first divergence = %#v, want changed_files_mismatch", result.FirstDivergence)
	}
	if strings.Join(result.ChangedFilesDiff.OnlyInA, ",") != "README.md" {
		t.Fatalf("OnlyInA = %#v", result.ChangedFilesDiff.OnlyInA)
	}
	if strings.Join(result.ChangedFilesDiff.OnlyInB, ",") != "src/b.go" {
		t.Fatalf("OnlyInB = %#v", result.ChangedFilesDiff.OnlyInB)
	}
}

func TestCommandExitCodeDiff(t *testing.T) {
	runA := RunData{RunID: "a", Events: []trace.Event{
		testEvent("a1", "a", 1, trace.EventProcessResult, trace.Payload{"command": "go test ./...", "exit_code": 0}),
	}}
	runB := RunData{RunID: "b", Events: []trace.Event{
		testEvent("b1", "b", 1, trace.EventProcessResult, trace.Payload{"command": "go test ./...", "exit_code": 1}),
	}}

	result := DiffRuns(runA, runB)
	if result.FirstDivergence == nil || result.FirstDivergence.Reason != "exit_code_mismatch" {
		t.Fatalf("first divergence = %#v, want exit_code_mismatch", result.FirstDivergence)
	}
	if len(result.CommandExitCodeDiff) != 1 {
		t.Fatalf("CommandExitCodeDiff len = %d, want 1", len(result.CommandExitCodeDiff))
	}
	if result.CommandExitCodeDiff[0].ExitA == nil || *result.CommandExitCodeDiff[0].ExitA != 0 {
		t.Fatalf("ExitA = %#v", result.CommandExitCodeDiff[0].ExitA)
	}
	if result.CommandExitCodeDiff[0].ExitB == nil || *result.CommandExitCodeDiff[0].ExitB != 1 {
		t.Fatalf("ExitB = %#v", result.CommandExitCodeDiff[0].ExitB)
	}
}

func TestDiffJSONOutputIsParseable(t *testing.T) {
	result := DiffRuns(
		RunData{RunID: "a", Events: []trace.Event{testEvent("a1", "a", 1, trace.EventRunStart, trace.Payload{"status": "started"})}},
		RunData{RunID: "b", Events: []trace.Event{testEvent("b1", "b", 1, trace.EventRunStart, trace.Payload{"status": "started"})}},
	)
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded["run_a"] != "a" || decoded["run_b"] != "b" {
		t.Fatalf("decoded run ids = %#v", decoded)
	}
}

func testEvent(id string, runID string, index int64, typ trace.EventType, payload trace.Payload) trace.Event {
	return trace.Event{
		SchemaVersion: trace.SchemaVersion,
		EventID:       id,
		RunID:         runID,
		EventIndex:    index,
		Type:          typ,
		Source:        trace.Source{Adapter: "fixture"},
		Timestamp:     time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		Payload:       payload,
	}
}
