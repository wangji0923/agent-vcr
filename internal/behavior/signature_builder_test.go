package behavior

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestBuildSignatureFromTimelineFiltersNoiseByDefault(t *testing.T) {
	timeline := Timeline{
		SchemaVersion: SchemaVersion,
		RunID:         "run_sig",
		Steps: []Step{
			{Kind: StepProcessStart, Summary: "process start"},
			{
				Kind:           StepSearch,
				Query:          "session",
				Result:         ResultSuccess,
				SourceEventIDs: []string{"evt_1", "evt_1"},
				SourceRefs:     []StepRef{{EventID: "evt_1", EventIndex: 10, EventType: "shell_command"}},
			},
			{
				Kind:    StepRunTest,
				Command: `go test C:\Users\alice\repo\pkg`,
				Files:   []string{`C:\Users\alice\repo\pkg\session_test.go`},
				Result:  ResultSuccess,
			},
			{Kind: StepRawBehavior, Summary: "raw"},
		},
	}

	signature := buildSignatureFromTimelineAt(timeline, SignatureOptions{}, fixedSignatureTime(), "")

	if len(signature.Steps) != 2 {
		t.Fatalf("default signature should filter process/raw noise: got %d steps", len(signature.Steps))
	}
	if signature.Steps[0].StepID != "behavior_step_0001" || signature.Steps[1].StepID != "behavior_step_0002" {
		t.Fatalf("step ids should be deterministic: %#v", []string{signature.Steps[0].StepID, signature.Steps[1].StepID})
	}
	if !signature.Steps[0].Significant || !signature.Steps[1].Significant {
		t.Fatalf("included signature steps should be marked significant")
	}
	if len(signature.Steps[0].SourceRefs) != 0 {
		t.Fatalf("source refs should be omitted by default to avoid event_index noise")
	}
	if !reflect.DeepEqual(signature.Steps[0].SourceEventIDs, []string{"evt_1"}) {
		t.Fatalf("source event ids should be preserved and deduplicated: %#v", signature.Steps[0].SourceEventIDs)
	}
	if signature.Steps[0].Summary == "" || signature.Steps[1].Summary == "" {
		t.Fatalf("signature steps should have readable summaries: %#v", signature.Steps)
	}
	if signature.Metrics.ToolEfficiency.TotalSteps != 2 {
		t.Fatalf("summary metrics should include step count, got %d", signature.Metrics.ToolEfficiency.TotalSteps)
	}
	if signature.Metrics.ToolEfficiency.SearchSteps != 1 {
		t.Fatalf("summary metrics should include search count, got %d", signature.Metrics.ToolEfficiency.SearchSteps)
	}
}

func TestBuildSignatureCanIncludeRawBehaviorAndSourceRefs(t *testing.T) {
	timeline := Timeline{
		SchemaVersion: SchemaVersion,
		RunID:         "run_sig",
		Steps: []Step{
			{Kind: StepRawBehavior, Summary: "raw event", SourceRefs: []StepRef{{EventID: "evt_raw", EventIndex: 4}}},
		},
	}

	signature := buildSignatureFromTimelineAt(timeline, SignatureOptions{
		IncludeRawBehavior: true,
		IncludeSourceRefs:  true,
	}, fixedSignatureTime(), "")

	if len(signature.Steps) != 1 {
		t.Fatalf("raw behavior should be included when requested")
	}
	if got := signature.Steps[0].SourceRefs; len(got) != 1 || got[0].EventIndex != 4 {
		t.Fatalf("source refs should be preserved when requested: %#v", got)
	}
}

func TestStepFingerprintIgnoresIDsIndexesUserPathsAndBlobPaths(t *testing.T) {
	a := Step{
		StepID:         "random_a",
		RunID:          "run_a",
		Index:          1,
		Kind:           StepReadFile,
		Target:         `C:\Users\alice\repo\.agent-vcr\runs\run-a\blobs\stdout.txt`,
		Result:         ResultSuccess,
		SourceEventIDs: []string{"evt_a"},
		SourceRefs:     []StepRef{{EventID: "evt_a", EventIndex: 12, EventType: "file_read"}},
	}
	b := Step{
		StepID:         "random_b",
		RunID:          "run_b",
		Index:          99,
		Kind:           StepReadFile,
		Target:         `/home/bob/repo/.agent-vcr/runs/run-b/blobs/stderr.txt`,
		Result:         ResultSuccess,
		SourceEventIDs: []string{"evt_b"},
		SourceRefs:     []StepRef{{EventID: "evt_b", EventIndex: 88, EventType: "file_read"}},
	}

	if got, want := StepFingerprint(a), StepFingerprint(b); got != want {
		t.Fatalf("fingerprints should ignore ids/indexes/user home/blob path differences:\n%s\n%s", got, want)
	}
	if got := StepFingerprint(a); got != "read_file||<blob>|||||success" {
		t.Fatalf("blob path should be normalized in fingerprint, got %q", got)
	}
}

func TestSignatureJSONGolden(t *testing.T) {
	signature := buildSignatureFromTimelineAt(Timeline{
		SchemaVersion: SchemaVersion,
		RunID:         "run_golden",
		Steps: []Step{
			{Kind: StepSearch, Query: "session", Result: ResultSuccess, SourceEventIDs: []string{"evt_search"}},
			{Kind: StepEditFile, Files: []string{"src/session.go"}, Result: ResultSuccess, SourceEventIDs: []string{"evt_patch"}},
		},
	}, SignatureOptions{}, fixedSignatureTime(), "sha256:trace")

	data, err := json.MarshalIndent(signature, "", "  ")
	if err != nil {
		t.Fatalf("marshal signature: %v", err)
	}
	data = append(data, '\n')

	goldenPath := filepath.Join("..", "..", "testdata", "behavior", "signature", "signature.golden.json")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(data) != string(want) {
		t.Fatalf("signature JSON changed\nwant:\n%s\ngot:\n%s", want, data)
	}
}

func fixedSignatureTime() time.Time {
	return time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
}
