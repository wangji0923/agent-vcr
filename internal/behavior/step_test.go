package behavior

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestStepJSONRoundTrip(t *testing.T) {
	step := Step{
		StepID:      "step_1",
		RunID:       "run_1",
		Index:       3,
		Kind:        StepRunTest,
		Action:      "go test ./...",
		Summary:     "ran Go tests",
		Target:      "project",
		Command:     "go test ./...",
		Files:       []string{"internal/behavior/step.go"},
		Result:      ResultSuccess,
		Fingerprint: "run_test|go test ./...",
		Significant: true,
		SourceRefs: []StepRef{{
			EventID:    "evt_1",
			EventIndex: 7,
			EventType:  "shell_command",
		}},
		SourceEventIDs: []string{"evt_1", "evt_2"},
		Confidence:     0.95,
		Attributes: map[string]string{
			"path_kind": "source",
		},
	}

	data, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("marshal step: %v", err)
	}
	if !strings.Contains(string(data), `"significant":true`) {
		t.Fatalf("significant field should be explicit in JSON: %s", data)
	}

	var got Step
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal step: %v", err)
	}
	if !reflect.DeepEqual(step, got) {
		t.Fatalf("round trip mismatch\nwant: %#v\n got: %#v", step, got)
	}
}

func TestStepKindStableStrings(t *testing.T) {
	tests := map[StepKind]string{
		StepSearch:            "search",
		StepReadFile:          "read_file",
		StepInspectTest:       "inspect_test",
		StepEditFile:          "edit_file",
		StepRunTest:           "run_test",
		StepRunBuild:          "run_build",
		StepInstallDependency: "install_dependency",
		StepCallTool:          "call_tool",
		StepCallMCPTool:       "call_mcp_tool",
		StepPermissionRequest: "permission_request",
		StepRecoverFromError:  "recover_from_error",
		StepSkipValidation:    "skip_validation",
		StepContextCompact:    "context_compact",
		StepProcessStart:      "process_start",
		StepProcessResult:     "process_result",
		StepRawBehavior:       "raw_behavior",
		StepUnknown:           "unknown",
	}

	for kind, want := range tests {
		if string(kind) != want {
			t.Fatalf("%v string changed: got %q want %q", kind, string(kind), want)
		}
		if !IsKnownStepKind(kind) {
			t.Fatalf("%q should be known", kind)
		}
	}
}

func TestStepStableKeyIgnoresIndexAndIDs(t *testing.T) {
	a := Step{
		StepID:         "step_a",
		RunID:          "run_a",
		Index:          1,
		Kind:           StepReadFile,
		Target:         "src/session.go",
		Files:          []string{"src/session.go"},
		Result:         ResultSuccess,
		SourceEventIDs: []string{"evt_a"},
	}
	b := a
	b.StepID = "step_b"
	b.RunID = "run_b"
	b.Index = 42
	b.SourceEventIDs = []string{"evt_b"}
	b.SourceRefs = []StepRef{{EventID: "evt_b", EventIndex: 99, EventType: "tool_call"}}

	if a.StableKey() != b.StableKey() {
		t.Fatalf("stable key should ignore ids and index:\n%s\n%s", a.StableKey(), b.StableKey())
	}
}

func TestStepStableKeyNormalizesUserPaths(t *testing.T) {
	a := Step{
		Kind:    StepRunTest,
		Command: `go test C:\Users\alice\repo\pkg`,
		Files:   []string{`C:\Users\alice\repo\pkg\session_test.go`},
		Result:  ResultSuccess,
	}
	b := Step{
		Kind:    StepRunTest,
		Command: "/usr/local/bin/go test /home/bob/repo/pkg",
		Files:   []string{"/home/bob/repo/pkg/session_test.go"},
		Result:  ResultSuccess,
	}

	if NormalizePathForKey(`C:\Users\alice\repo\pkg\session_test.go`) != "repo/pkg/session_test.go" {
		t.Fatalf("windows user path was not normalized")
	}
	if NormalizePathForKey("src/users/profile.go") != "src/users/profile.go" {
		t.Fatalf("relative users path should not be treated as a home directory")
	}
	if !strings.Contains(a.StableKey(), "repo/pkg/session_test.go") {
		t.Fatalf("stable key should keep normalized file path: %s", a.StableKey())
	}
	if !strings.Contains(b.StableKey(), "repo/pkg/session_test.go") {
		t.Fatalf("stable key should keep normalized Unix file path: %s", b.StableKey())
	}
}

func TestSortFilesStable(t *testing.T) {
	got := SortFiles([]string{
		`C:\Users\alice\repo\b.go`,
		"repo/a.go",
		"repo/a.go",
		"",
	})
	want := []string{"repo/a.go", "repo/b.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SortFiles() = %#v, want %#v", got, want)
	}
}

func TestStepHelpers(t *testing.T) {
	if !(Step{Kind: StepRunTest}).IsValidation() {
		t.Fatalf("run_test should be validation")
	}
	if !(Step{Kind: StepRunBuild}).IsValidation() {
		t.Fatalf("run_build should be validation")
	}
	if !(Step{Kind: StepEditFile}).IsEdit() {
		t.Fatalf("edit_file should be edit")
	}
	if !(Step{Kind: StepInspectTest}).IsFileRead() {
		t.Fatalf("inspect_test should be file read")
	}
}

func TestAgentSpecificKindGuard(t *testing.T) {
	for _, kind := range []StepKind{
		StepSearch,
		StepReadFile,
		StepInspectTest,
		StepEditFile,
		StepRunTest,
		StepRunBuild,
		StepInstallDependency,
		StepCallTool,
		StepCallMCPTool,
		StepPermissionRequest,
		StepRecoverFromError,
		StepSkipValidation,
		StepContextCompact,
		StepProcessStart,
		StepProcessResult,
		StepRawBehavior,
		StepUnknown,
	} {
		if IsAgentSpecificStepKind(kind) {
			t.Fatalf("core behavior kind must not be agent-specific: %q", kind)
		}
	}

	for _, kind := range []StepKind{"codex_tool_call", "kimi_search", "claude_read_file"} {
		if !IsAgentSpecificStepKind(kind) {
			t.Fatalf("expected %q to be rejected as agent-specific", kind)
		}
	}
}
