package visualize

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStepKeyAndSimilarity(t *testing.T) {
	left := alignStep("run-a", 1, VisualStepReadFile, "read source")
	left.Files = []string{`.\src\session.go`}
	right := alignStep("run-b", 1, VisualStepReadFile, "read source")
	right.Files = []string{"src/session.go"}

	if StepKey(left) != StepKey(right) {
		t.Fatalf("StepKey should normalize paths:\nleft=%q\nright=%q", StepKey(left), StepKey(right))
	}
	if score := StepSimilarity(left, right); score < defaultAlignmentMinSimilarity {
		t.Fatalf("similar read steps score = %d, want >= %d", score, defaultAlignmentMinSimilarity)
	}

	edit := alignStep("run-a", 2, VisualStepEditFile, "edit source")
	test := alignStep("run-b", 2, VisualStepRunTest, "run tests")
	if score := StepSimilarity(edit, test); score >= 0 {
		t.Fatalf("edit/test score = %d, want strong difference", score)
	}
}

func TestAlignSingleRunLane(t *testing.T) {
	lane := BehaviorLane{
		RunID: "run-a",
		Label: "single",
		Steps: []VisualStep{
			alignStep("run-a", 1, VisualStepSearch, "search"),
			alignStep("run-a", 2, VisualStepReadFile, "read"),
		},
	}

	rows := AlignLanes([]BehaviorLane{lane}, AlignOptions{})
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].RowIndex != 1 || rows[0].Cells["run-a"].Step == nil || rows[0].Cells["run-a"].Gap {
		t.Fatalf("first row = %#v", rows[0])
	}
}

func TestAlignPairKeepsGapBeforeLaterMatch(t *testing.T) {
	left := BehaviorLane{RunID: "run-a", Steps: []VisualStep{
		alignSearch("run-a", 1, "session"),
		alignFileStep("run-a", 2, VisualStepInspectTest, "tests/session_test.go"),
		alignFileStep("run-a", 3, VisualStepReadFile, "src/session.go"),
		alignFileStep("run-a", 4, VisualStepEditFile, "src/session.go"),
		alignCommandStep("run-a", 5, VisualStepRunTest, "go test ./..."),
	}}
	right := BehaviorLane{RunID: "run-b", Steps: []VisualStep{
		alignSearch("run-b", 1, "session"),
		alignFileStep("run-b", 2, VisualStepReadFile, "src/session.go"),
		alignFileStep("run-b", 3, VisualStepEditFile, "src/session.go"),
		alignCommandStep("run-b", 4, VisualStepRunTest, "go test ./..."),
	}}

	rows := AlignLanes([]BehaviorLane{left, right}, AlignOptions{})
	if len(rows) != 5 {
		t.Fatalf("rows = %d, want 5: %#v", len(rows), rows)
	}
	if cell := rows[1].Cells["run-b"]; !cell.Gap || cell.Step != nil {
		t.Fatalf("row 2 run-b cell = %#v, want gap", cell)
	}
	if rows[2].Cells["run-a"].Step == nil || rows[2].Cells["run-b"].Step == nil {
		t.Fatalf("row 3 should align later source read: %#v", rows[2])
	}
	if StepKey(*rows[2].Cells["run-a"].Step) != StepKey(*rows[2].Cells["run-b"].Step) {
		t.Fatalf("row 3 keys differ")
	}
}

func TestAlignPairMarksFirstDivergence(t *testing.T) {
	left := BehaviorLane{RunID: "run-a", Steps: []VisualStep{
		alignSearch("run-a", 1, "session"),
		alignFileStep("run-a", 2, VisualStepInspectTest, "tests/session_test.go"),
		alignFileStep("run-a", 3, VisualStepReadFile, "src/session.go"),
		alignFileStep("run-a", 4, VisualStepEditFile, "src/session.go"),
		alignCommandStep("run-a", 5, VisualStepRunTest, "go test ./..."),
	}}
	right := BehaviorLane{RunID: "run-b", Steps: []VisualStep{
		alignSearch("run-b", 1, "session"),
		alignFileStep("run-b", 2, VisualStepReadFile, "src/legacy_cookie.go"),
		alignFileStep("run-b", 3, VisualStepEditFile, "src/legacy_cookie.go"),
	}}

	rows := AlignLanes([]BehaviorLane{left, right}, AlignOptions{MarkFirstDivergence: true})
	if len(rows) < 2 {
		t.Fatalf("rows = %d, want at least 2", len(rows))
	}
	row := rows[1]
	if !row.IsDivergent || !strings.HasPrefix(row.Reason, "first_divergence") {
		t.Fatalf("row 2 divergence = %#v", row)
	}
	if row.Cells["run-a"].Step == nil || row.Cells["run-b"].Step == nil {
		t.Fatalf("row 2 should contain both divergent steps: %#v", row)
	}
	if !row.Cells["run-a"].Step.Divergent || !row.Cells["run-b"].Step.Divergent {
		t.Fatalf("row 2 steps not marked divergent: %#v", row)
	}
}

func TestAlignLanesSupportsThreeRuns(t *testing.T) {
	base := BehaviorLane{RunID: "run-a", Steps: []VisualStep{
		alignSearch("run-a", 1, "session"),
		alignFileStep("run-a", 2, VisualStepReadFile, "src/session.go"),
		alignFileStep("run-a", 3, VisualStepEditFile, "src/session.go"),
	}}
	withGap := BehaviorLane{RunID: "run-b", Steps: []VisualStep{
		alignSearch("run-b", 1, "session"),
		alignFileStep("run-b", 2, VisualStepInspectTest, "tests/session_test.go"),
		alignFileStep("run-b", 3, VisualStepReadFile, "src/session.go"),
		alignFileStep("run-b", 4, VisualStepEditFile, "src/session.go"),
	}}
	legacy := BehaviorLane{RunID: "run-c", Steps: []VisualStep{
		alignSearch("run-c", 1, "session"),
		alignFileStep("run-c", 2, VisualStepReadFile, "src/legacy_cookie.go"),
		alignFileStep("run-c", 3, VisualStepEditFile, "src/legacy_cookie.go"),
	}}

	rows := AlignLanes([]BehaviorLane{base, withGap, legacy}, AlignOptions{MarkFirstDivergence: true})
	if len(rows) < 4 {
		t.Fatalf("rows = %d, want at least 4", len(rows))
	}
	for _, row := range rows {
		for _, runID := range []string{"run-a", "run-b", "run-c"} {
			if _, ok := row.Cells[runID]; !ok {
				t.Fatalf("row %d missing %s cell: %#v", row.RowIndex, runID, row.Cells)
			}
		}
	}
	data1, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent rows: %v", err)
	}
	data2, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent rows again: %v", err)
	}
	if string(data1) != string(data2) {
		t.Fatalf("alignment JSON should be stable")
	}
}

func TestMarkDivergenceUsesProvidedMarker(t *testing.T) {
	left := BehaviorLane{RunID: "run-a", Steps: []VisualStep{
		alignSearch("run-a", 1, "session"),
		alignFileStep("run-a", 2, VisualStepReadFile, "src/session.go"),
	}}
	right := BehaviorLane{RunID: "run-b", Steps: []VisualStep{
		alignSearch("run-b", 1, "session"),
		alignFileStep("run-b", 2, VisualStepReadFile, "src/legacy_cookie.go"),
	}}
	rows := AlignPair(left, right, AlignOptions{})
	marked := MarkDivergence(rows, []VisualDivergence{{
		BaselineRunID:  "run-a",
		CompareRunID:   "run-b",
		StepIndex:      2,
		AlignmentIndex: 2,
		Kind:           "step_changed",
		First:          true,
	}})

	if rows[1].IsDivergent {
		t.Fatalf("MarkDivergence mutated input rows")
	}
	if !marked[1].IsDivergent || marked[1].Reason != "first_divergence:step_changed" {
		t.Fatalf("marked row = %#v", marked[1])
	}
	if !marked[1].Cells["run-a"].Step.Divergent || !marked[1].Cells["run-b"].Step.Divergent {
		t.Fatalf("marked steps = %#v", marked[1].Cells)
	}
}

func TestBuildPathGraphMergesSharedPath(t *testing.T) {
	left := BehaviorLane{RunID: "run-a", Steps: []VisualStep{
		alignSearch("run-a", 1, "session"),
		alignFileStep("run-a", 2, VisualStepReadFile, "src/session.go"),
		alignFileStep("run-a", 3, VisualStepEditFile, "src/session.go"),
	}}
	right := BehaviorLane{RunID: "run-b", Steps: []VisualStep{
		alignSearch("run-b", 1, "session"),
		alignFileStep("run-b", 2, VisualStepReadFile, "src/session.go"),
		alignFileStep("run-b", 3, VisualStepEditFile, "src/legacy_cookie.go"),
	}}

	graph := BuildPathGraph([]BehaviorLane{left, right})
	if graph == nil {
		t.Fatal("BuildPathGraph returned nil")
	}
	if len(graph.Nodes) != 4 {
		t.Fatalf("nodes = %d, want 4: %#v", len(graph.Nodes), graph.Nodes)
	}
	if len(graph.Nodes[0].RunIDs) != 2 || graph.Nodes[0].RunIDs[0] != "run-a" || graph.Nodes[0].RunIDs[1] != "run-b" {
		t.Fatalf("shared search node run ids = %#v", graph.Nodes[0].RunIDs)
	}
	if len(graph.Edges) == 0 {
		t.Fatalf("expected graph edges")
	}
}

func alignSearch(runID string, index int, query string) VisualStep {
	step := alignStep(runID, index, VisualStepSearch, "search "+query)
	step.Query = query
	return step
}

func alignFileStep(runID string, index int, kind VisualStepKind, file string) VisualStep {
	step := alignStep(runID, index, kind, string(kind)+" "+file)
	step.Files = []string{file}
	step.Target = file
	return step
}

func alignCommandStep(runID string, index int, kind VisualStepKind, command string) VisualStep {
	step := alignStep(runID, index, kind, command)
	step.Command = command
	return step
}

func alignStep(runID string, index int, kind VisualStepKind, summary string) VisualStep {
	return VisualStep{
		RunID:       runID,
		StepID:      runID + "-step-" + string(rune('0'+index)),
		Index:       index,
		Kind:        kind,
		Summary:     summary,
		Significant: true,
		EventIDs:    []string{runID + "-evt-" + string(rune('0'+index))},
	}
}
