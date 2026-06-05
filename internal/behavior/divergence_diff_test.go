package behavior

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFirstDivergenceStepChanged(t *testing.T) {
	result := DiffSignatures(
		testSignature("run-a", testBehaviorStep(StepRecoverFromError, "inspect failure", "", "", nil, ResultSuccess, "a1")),
		testSignature("run-b", testBehaviorStep(StepSkipValidation, "skipped validation", "", "", nil, ResultSkipped, "b1")),
		DiffOptions{},
	)

	if result.FirstDivergence == nil {
		t.Fatal("expected divergence")
	}
	if result.FirstDivergence.Kind != DivergenceStepChanged {
		t.Fatalf("kind = %q, want %q", result.FirstDivergence.Kind, DivergenceStepChanged)
	}
	if result.FirstDivergence.Index != 0 {
		t.Fatalf("index = %d, want 0", result.FirstDivergence.Index)
	}
	if !strings.Contains(result.FirstDivergence.Summary, "behavior step changed") {
		t.Fatalf("summary = %q", result.FirstDivergence.Summary)
	}
	if got := strings.Join(result.FirstDivergence.RelatedEventIDs, ","); got != "a1,b1" {
		t.Fatalf("related ids = %q, want a1,b1", got)
	}
}

func TestFirstDivergenceMissingStep(t *testing.T) {
	result := DiffSignatures(
		testSignature("run-a", testBehaviorStep(StepSearch, "rg auth src", "auth", "", nil, ResultSuccess, "a1")),
		testSignature("run-b",
			testBehaviorStep(StepSearch, "rg auth src", "auth", "", nil, ResultSuccess, "b1"),
			testBehaviorStep(StepRunTest, "go test ./...", "", "", nil, ResultSuccess, "b2"),
		),
		DiffOptions{},
	)

	if result.FirstDivergence == nil {
		t.Fatal("expected divergence")
	}
	if result.FirstDivergence.Kind != DivergenceMissingInA {
		t.Fatalf("kind = %q, want %q", result.FirstDivergence.Kind, DivergenceMissingInA)
	}
	if result.FirstDivergence.RunAStep != nil {
		t.Fatalf("run A step should be nil for missing-in-A divergence: %#v", result.FirstDivergence.RunAStep)
	}
}

func TestFirstDivergenceNoDivergence(t *testing.T) {
	result := DiffSignatures(
		testSignature("run-a", testBehaviorStep(StepRunTest, "go test ./...", "", "", nil, ResultSuccess, "a1")),
		testSignature("run-b", testBehaviorStep(StepRunTest, "go test ./...", "", "", nil, ResultSuccess, "b1")),
		DiffOptions{},
	)

	if result.FirstDivergence != nil {
		t.Fatalf("unexpected divergence: %#v", result.FirstDivergence)
	}
	if result.Summary.Diverged {
		t.Fatalf("summary diverged = true")
	}
	if result.Summary.DivergenceKind != DivergenceNone {
		t.Fatalf("summary kind = %q, want %q", result.Summary.DivergenceKind, DivergenceNone)
	}
}

func TestFirstDivergenceIgnoresStepIDsAndNoise(t *testing.T) {
	left := testBehaviorStep(
		StepReadFile,
		`cat C:\Users\alice\repo\.agent-vcr\runs\run-a\blobs\stdout.txt`,
		"",
		`C:\Users\alice\repo\.agent-vcr\runs\run-a\blobs\stdout.txt`,
		[]string{`C:\Users\alice\repo\src\auth\session.ts`},
		ResultSuccess,
		"a1",
	)
	left.StepID = "random-left"
	left.Index = 99
	left.SourceRefs = []StepRef{{EventID: "a1", EventIndex: 123, EventType: "shell_command"}}

	right := testBehaviorStep(
		StepReadFile,
		"cat /home/bob/repo/.agent-vcr/runs/run-b/blobs/stdout.txt",
		"",
		"/home/bob/repo/.agent-vcr/runs/run-b/blobs/stdout.txt",
		[]string{"/home/bob/repo/src/auth/session.ts"},
		ResultSuccess,
		"b1",
	)
	right.StepID = "random-right"
	right.Index = 1
	right.SourceRefs = []StepRef{{EventID: "b1", EventIndex: 456, EventType: "shell_command"}}

	result := DiffTimelines(
		Timeline{SchemaVersion: SchemaVersion, RunID: "run-a", Steps: []Step{left}},
		Timeline{SchemaVersion: SchemaVersion, RunID: "run-b", Steps: []Step{right}},
		DiffOptions{},
	)

	if result.FirstDivergence != nil {
		t.Fatalf("unexpected divergence: %#v", result.FirstDivergence)
	}
}

func TestFirstDivergenceSearchQueryChanged(t *testing.T) {
	result := DiffSignatures(
		testSignature("run-a", testBehaviorStep(StepSearch, "rg session src tests", "session", "", nil, ResultSuccess, "a1")),
		testSignature("run-b", testBehaviorStep(StepSearch, "rg cookie src tests", "cookie", "", nil, ResultSuccess, "b1")),
		DiffOptions{},
	)

	divergence := requireFirstDivergence(t, result)
	if !strings.Contains(divergence.Summary, "search query divergence") {
		t.Fatalf("summary = %q", divergence.Summary)
	}
	if divergence.Explanation != "Runs diverged during search/query selection." {
		t.Fatalf("explanation = %q", divergence.Explanation)
	}
}

func TestFirstDivergenceTestInspectionVsLegacyRead(t *testing.T) {
	result := DiffSignatures(
		testSignature("run-a", testBehaviorStep(StepInspectTest, "", "", "tests/auth/session.test.ts", []string{"tests/auth/session.test.ts"}, ResultSuccess, "a2")),
		testSignature("run-b", testBehaviorStep(StepReadFile, "", "", "src/auth/legacy-cookie.ts", []string{"src/auth/legacy-cookie.ts"}, ResultSuccess, "b2")),
		DiffOptions{},
	)

	divergence := requireFirstDivergence(t, result)
	if !strings.Contains(divergence.Summary, "file discovery divergence") {
		t.Fatalf("summary = %q", divergence.Summary)
	}
	if divergence.Explanation != "Run B entered a legacy path before Run A inspected tests." {
		t.Fatalf("explanation = %q", divergence.Explanation)
	}
}

func TestFirstDivergenceOutcomeChanged(t *testing.T) {
	result := DiffSignatures(
		testSignature("run-a", testBehaviorStep(StepRunTest, "go test ./...", "", "", nil, ResultSuccess, "a1")),
		testSignature("run-b", testBehaviorStep(StepRunTest, "go test ./...", "", "", nil, ResultFailure, "b1")),
		DiffOptions{},
	)

	divergence := requireFirstDivergence(t, result)
	if divergence.Kind != DivergenceResultChanged {
		t.Fatalf("kind = %q, want %q", divergence.Kind, DivergenceResultChanged)
	}
	if !strings.Contains(divergence.Summary, "outcome divergence") {
		t.Fatalf("summary = %q", divergence.Summary)
	}
}

func TestFirstDivergenceValidationAndToolCall(t *testing.T) {
	validation := DiffSignatures(
		testSignature("run-a", testBehaviorStep(StepRunTest, "go test ./...", "", "", nil, ResultSuccess, "a1")),
		testSignature("run-b", testBehaviorStep(StepEditFile, "", "", "src/session.go", []string{"src/session.go"}, ResultSuccess, "b1")),
		DiffOptions{},
	)
	if divergence := requireFirstDivergence(t, validation); !strings.Contains(divergence.Summary, "validation behavior divergence") {
		t.Fatalf("validation summary = %q", divergence.Summary)
	}

	tool := DiffSignatures(
		testSignature("run-a", testToolStep("Read", "src/session.go", "a1")),
		testSignature("run-b", testToolStep("mcp__repo__search", "session", "b1")),
		DiffOptions{},
	)
	if divergence := requireFirstDivergence(t, tool); !strings.Contains(divergence.Summary, "tool call divergence") {
		t.Fatalf("tool summary = %q", divergence.Summary)
	}
}

func TestFirstDivergenceIgnoresConfiguredNoise(t *testing.T) {
	result := DiffTimelines(
		Timeline{SchemaVersion: SchemaVersion, RunID: "run-a", Steps: []Step{
			testBehaviorStep(StepProcessStart, "agent start", "", "", nil, ResultSuccess, "a0"),
			testBehaviorStep(StepSearch, "rg auth src", "auth", "", nil, ResultSuccess, "a1"),
		}},
		Timeline{SchemaVersion: SchemaVersion, RunID: "run-b", Steps: []Step{
			testBehaviorStep(StepSearch, "rg auth src", "auth", "", nil, ResultSuccess, "b1"),
		}},
		DiffOptions{IgnoreProcessNoise: true},
	)

	if result.FirstDivergence != nil {
		t.Fatalf("unexpected divergence: %#v", result.FirstDivergence)
	}
}

func TestDiffResultJSONGolden(t *testing.T) {
	result := DiffSignatures(
		testSignature("run-a",
			testBehaviorStep(StepSearch, "rg session src tests", "session", "", nil, ResultSuccess, "a1"),
			testBehaviorStep(StepInspectTest, "", "", "tests/auth/session.test.ts", []string{"tests/auth/session.test.ts"}, ResultSuccess, "a2"),
		),
		testSignature("run-b",
			testBehaviorStep(StepSearch, "rg session src tests", "session", "", nil, ResultSuccess, "b1"),
			testBehaviorStep(StepReadFile, "", "", "src/auth/legacy-cookie.ts", []string{"src/auth/legacy-cookie.ts"}, ResultSuccess, "b2"),
		),
		DiffOptions{},
	)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal diff result: %v", err)
	}
	data = append(data, '\n')

	goldenPath := filepath.Join("..", "..", "testdata", "behavior", "divergence", "diff_result.golden.json")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v\nactual:\n%s", err, data)
	}
	if string(data) != string(want) {
		t.Fatalf("golden mismatch\nwant:\n%s\ngot:\n%s", want, data)
	}
}

func testSignature(runID string, steps ...Step) Signature {
	return Signature{
		SchemaVersion: SchemaVersion,
		RunID:         runID,
		Steps:         steps,
		Metrics:       ComputeMetrics(Timeline{SchemaVersion: SchemaVersion, RunID: runID, Steps: steps}),
	}
}

func testBehaviorStep(kind StepKind, command, query, target string, files []string, result StepResult, eventIDs ...string) Step {
	return Step{
		StepID:         "input-step",
		Kind:           kind,
		Action:         firstSignatureNonEmpty(command, target, query),
		Target:         target,
		Query:          query,
		Command:        command,
		Files:          files,
		Result:         result,
		Significant:    true,
		SourceEventIDs: eventIDs,
	}
}

func testToolStep(toolName, target, eventID string) Step {
	return Step{
		StepID:         "tool-step",
		Kind:           StepCallTool,
		ToolName:       toolName,
		Target:         target,
		Result:         ResultSuccess,
		Significant:    true,
		SourceEventIDs: []string{eventID},
	}
}

func requireFirstDivergence(t *testing.T, result DiffResult) *Divergence {
	t.Helper()
	if result.FirstDivergence == nil {
		t.Fatalf("expected divergence")
	}
	return result.FirstDivergence
}
