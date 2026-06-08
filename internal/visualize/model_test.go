package visualize

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestVisualReportJSONRoundTrip(t *testing.T) {
	report := sampleVisualReport()

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded VisualReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %q", decoded.SchemaVersion)
	}
	if len(decoded.Runs) != 3 {
		t.Fatalf("runs = %d, want 3", len(decoded.Runs))
	}
	if decoded.Mode != RenderModeCompare || decoded.Summary.Mode != RenderModeCompare {
		t.Fatalf("mode = %#v summary=%#v", decoded.Mode, decoded.Summary.Mode)
	}
	if decoded.Summary.FirstDivergence == nil || !decoded.Summary.FirstDivergence.First {
		t.Fatalf("first divergence missing: %#v", decoded.Summary.FirstDivergence)
	}
	if err := ValidateReport(&decoded); err != nil {
		t.Fatalf("ValidateReport decoded: %v", err)
	}
}

func TestBehaviorLaneJSONRoundTrip(t *testing.T) {
	lane := sampleVisualReport().Lanes[0]
	data, err := json.Marshal(lane)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded BehaviorLane
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.RunID != "run-a" || len(decoded.Steps) != 2 {
		t.Fatalf("decoded lane = %#v", decoded)
	}
}

func TestVisualStepJSONRoundTrip(t *testing.T) {
	step := sampleStep("run-a", 2, VisualStepEditFile, VisualPhaseEditing)
	step.Command = "apply patch"
	step.Files = []string{"src/session.go"}
	step.EventIDs = []string{"evt-edit"}
	step.Significant = true
	step.Divergent = true

	data, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded VisualStep
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.RunID != "run-a" || decoded.Kind != VisualStepEditFile || decoded.Phase != VisualPhaseEditing {
		t.Fatalf("decoded step = %#v", decoded)
	}
	if len(decoded.EventIDs) != 1 || decoded.EventIDs[0] != "evt-edit" {
		t.Fatalf("event ids = %#v", decoded.EventIDs)
	}
}

func TestAlignmentRowJSONRoundTrip(t *testing.T) {
	row := sampleVisualReport().Alignment[1]
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded AlignmentRow
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !decoded.IsDivergent || decoded.Cells["run-b"].Gap {
		t.Fatalf("decoded row = %#v", decoded)
	}
	if decoded.Cells["run-a"].Step == nil || decoded.Cells["run-a"].Step.RunID != "run-a" {
		t.Fatalf("run-a cell = %#v", decoded.Cells["run-a"])
	}
}

func TestFileAccessCompareJSONRoundTrip(t *testing.T) {
	fileAccess := sampleVisualReport().FileAccess
	data, err := json.Marshal(fileAccess)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded FileAccessCompare
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	use := decoded.Rows[0].Runs["run-a"]
	if use.ReadCount != 1 || use.EditCount != 1 || use.FirstAction != "read" || use.LastAction != "edit" {
		t.Fatalf("file use = %#v", use)
	}
}

func TestSearchScopeCompareJSONRoundTrip(t *testing.T) {
	scopes := sampleVisualReport().SearchScopes
	data, err := json.Marshal(scopes)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded SearchScopeCompare
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	use := decoded.Rows[0].Runs["run-a"]
	if use.SearchCount != 1 || use.FirstStep != 1 || len(use.Queries) != 1 {
		t.Fatalf("search scope use = %#v", use)
	}
}

func TestMetricsCardJSONRoundTrip(t *testing.T) {
	card := MetricsCard{Group: "Validation Behavior", Name: "ran tests after edit", Value: "yes", Level: "good"}
	data, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded MetricsCard
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded != card {
		t.Fatalf("decoded card = %#v, want %#v", decoded, card)
	}
}

func TestValidateReport(t *testing.T) {
	if err := ValidateReport(sampleVisualReport()); err != nil {
		t.Fatalf("ValidateReport: %v", err)
	}

	cases := []struct {
		name    string
		mutate  func(*VisualReport)
		wantErr string
	}{
		{
			name:    "missing schema",
			mutate:  func(report *VisualReport) { report.SchemaVersion = "" },
			wantErr: "schema_version",
		},
		{
			name:    "missing run id",
			mutate:  func(report *VisualReport) { report.Runs[0].RunID = "" },
			wantErr: "run_id",
		},
		{
			name:    "duplicate run id",
			mutate:  func(report *VisualReport) { report.Runs[1].RunID = report.Runs[0].RunID },
			wantErr: "duplicate",
		},
		{
			name:    "unknown lane run",
			mutate:  func(report *VisualReport) { report.Lanes[0].RunID = "missing" },
			wantErr: "unknown run_id",
		},
		{
			name: "unknown alignment run",
			mutate: func(report *VisualReport) {
				report.Alignment[0].Cells["missing"] = StepCell{RunID: "missing", Gap: true}
			},
			wantErr: "unknown run_id",
		},
		{
			name: "gap with step",
			mutate: func(report *VisualReport) {
				cell := report.Alignment[0].Cells["run-a"]
				cell.Gap = true
				report.Alignment[0].Cells["run-a"] = cell
			},
			wantErr: "gap and step",
		},
		{
			name:    "unknown file access run",
			mutate:  func(report *VisualReport) { report.FileAccess.Rows[0].Runs["missing"] = FileUse{ReadCount: 1} },
			wantErr: "unknown run_id",
		},
		{
			name: "unknown search scope run",
			mutate: func(report *VisualReport) {
				report.SearchScopes.Rows[0].Runs["missing"] = SearchScopeUse{SearchCount: 1}
			},
			wantErr: "unknown run_id",
		},
		{
			name:    "unknown metrics run",
			mutate:  func(report *VisualReport) { report.Metrics[0].RunID = "missing" },
			wantErr: "unknown run_id",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			report := sampleVisualReport()
			tt.mutate(report)
			err := ValidateReport(report)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateReport error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateReportAllowsThreeRunModel(t *testing.T) {
	report := sampleVisualReport()
	if len(report.Runs) != 3 || len(report.Lanes) != 3 {
		t.Fatalf("sample runs=%d lanes=%d, want 3/3", len(report.Runs), len(report.Lanes))
	}
	if err := ValidateReport(report); err != nil {
		t.Fatalf("ValidateReport three-run model: %v", err)
	}
}

func sampleVisualReport() *VisualReport {
	start := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Minute)
	runARead := sampleStep("run-a", 1, VisualStepInspectTest, VisualPhaseInspection)
	runARead.Files = []string{"tests/session_test.go"}
	runARead.EventIDs = []string{"evt-a-read"}
	runAEdit := sampleStep("run-a", 2, VisualStepEditFile, VisualPhaseEditing)
	runAEdit.Files = []string{"src/session.go"}
	runAEdit.EventIDs = []string{"evt-a-edit"}

	runBRead := sampleStep("run-b", 1, VisualStepReadFile, VisualPhaseInspection)
	runBRead.Files = []string{"src/legacy_cookie.go"}
	runBRead.EventIDs = []string{"evt-b-read"}
	runBEdit := sampleStep("run-b", 2, VisualStepEditFile, VisualPhaseEditing)
	runBEdit.Files = []string{"src/legacy_cookie.go"}

	runCRead := sampleStep("run-c", 1, VisualStepReadFile, VisualPhaseInspection)
	runCRead.Files = []string{"src/session.go"}

	divergence := DivergenceMarker{
		BaselineRunID:  "run-a",
		CompareRunID:   "run-b",
		StepIndex:      1,
		AlignmentIndex: 2,
		Kind:           "step_changed",
		Summary:        "Run A inspected tests while Run B entered legacy code",
		First:          true,
		Left:           &runARead,
		Right:          &runBRead,
		EventIDs:       []string{"evt-a-read", "evt-b-read"},
	}

	return &VisualReport{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   start,
		Mode:          RenderModeCompare,
		Options: VisualOptions{
			Mode:          RenderModeCompare,
			BaselineRunID: "run-a",
			MaxRuns:       MaxRecommendedRuns,
			Labels:        map[string]string{"run-a": "test-first", "run-b": "legacy", "run-c": "source-first"},
		},
		Summary: VisualSummary{
			RunCount:             3,
			StepCount:            5,
			SignificantStepCount: 5,
			DivergenceCount:      1,
			FirstDivergence:      &divergence,
			FileCount:            3,
			MetricsCardCount:     2,
			Mode:                 RenderModeCompare,
		},
		Runs: []VisualRun{
			{RunID: "run-a", Label: "test-first", Source: "codex-hooks", Status: "completed", StartedAt: &start, EndedAt: &end, StepCount: 2, Summary: map[string]any{"role": "baseline"}},
			{RunID: "run-b", Label: "legacy", Source: "codex-hooks", Status: "completed", StartedAt: &start, EndedAt: &end, StepCount: 2},
			{RunID: "run-c", Label: "source-first", Source: "generic-cli", Status: "completed", StartedAt: &start, EndedAt: &end, StepCount: 1},
		},
		Lanes: []BehaviorLane{
			{RunID: "run-a", Label: "test-first", Steps: []VisualStep{runARead, runAEdit}},
			{RunID: "run-b", Label: "legacy", Steps: []VisualStep{runBRead, runBEdit}},
			{RunID: "run-c", Label: "source-first", Steps: []VisualStep{runCRead}},
		},
		Alignment: []AlignmentRow{
			{RowIndex: 1, Cells: map[string]StepCell{
				"run-a": {RunID: "run-a", Step: &runARead},
				"run-b": {RunID: "run-b", Step: &runBRead},
				"run-c": {RunID: "run-c", Step: &runCRead},
			}},
			{RowIndex: 2, IsDivergent: true, Reason: "first_divergence", Cells: map[string]StepCell{
				"run-a": {RunID: "run-a", Step: &runAEdit},
				"run-b": {RunID: "run-b", Step: &runBEdit},
				"run-c": {RunID: "run-c", Gap: true},
			}},
		},
		Divergences: []DivergenceMarker{divergence},
		FileAccess: FileAccessCompare{Rows: []FileAccessRow{
			{Path: "src/session.go", Runs: map[string]FileUse{
				"run-a": {ReadCount: 1, EditCount: 1, FirstStep: 1, LastStep: 2, FirstAction: "read", LastAction: "edit"},
				"run-c": {ReadCount: 1, FirstStep: 1, LastStep: 1, FirstAction: "read", LastAction: "read"},
			}},
			{Path: "src/legacy_cookie.go", Runs: map[string]FileUse{
				"run-b": {ReadCount: 1, EditCount: 1, FirstStep: 1, LastStep: 2, FirstAction: "read", LastAction: "edit"},
			}},
		}},
		SearchScopes: SearchScopeCompare{Rows: []SearchScopeRow{
			{Scope: "src", Runs: map[string]SearchScopeUse{
				"run-a": {SearchCount: 1, FirstStep: 1, LastStep: 1, Queries: []string{`rg "session" src tests`}},
				"run-b": {SearchCount: 1, FirstStep: 1, LastStep: 1, Queries: []string{`rg "cookie" src`}},
			}},
		}},
		Metrics: []MetricsCardGroup{
			{RunID: "run-a", Label: "test-first", Cards: []MetricsCard{{Group: "Validation Behavior", Name: "ran tests after edit", Value: "yes", Level: "good"}}},
			{RunID: "run-b", Label: "legacy", Cards: []MetricsCard{{Group: "Validation Behavior", Name: "ran tests after edit", Value: "no", Level: "bad"}}},
		},
		PathGraph: &PathGraph{
			Nodes: []PathNode{{ID: "n1", Label: "inspect", Kind: string(VisualStepInspectTest), RunIDs: []string{"run-a"}}},
			Edges: []PathEdge{{From: "n1", To: "n2", RunIDs: []string{"run-a"}}},
		},
	}
}

func sampleStep(runID string, index int, kind VisualStepKind, phase VisualPhase) VisualStep {
	return VisualStep{
		RunID:       runID,
		StepID:      runID + "-step-" + string(rune('0'+index)),
		Index:       index,
		Kind:        kind,
		Phase:       phase,
		Summary:     string(kind),
		Target:      "target",
		Significant: true,
		Attributes:  map[string]string{"source": "test"},
	}
}
