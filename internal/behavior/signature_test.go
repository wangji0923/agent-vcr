package behavior

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestSignatureJSONRoundTrip(t *testing.T) {
	generatedAt := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	signature := Signature{
		SchemaVersion:   SchemaVersion,
		RunID:           "run_1",
		SourceTraceHash: "sha256:abc123",
		GeneratedAt:     generatedAt,
		Steps: []Step{{
			StepID:      "step_1",
			RunID:       "run_1",
			Index:       0,
			Kind:        StepSearch,
			Query:       "session",
			Significant: true,
			Result:      ResultSuccess,
		}},
		Metrics: Metrics{
			ContextDiscipline: ContextDisciplineMetrics{
				ReadTestsBeforeEdit: true,
				UniqueFilesRead:     2,
			},
			Validation: ValidationMetrics{
				RanAnyTests:       true,
				RanTestsAfterEdit: true,
			},
			ToolEfficiency: ToolEfficiencyMetrics{
				TotalSteps:  1,
				SearchSteps: 1,
			},
		},
		Options: SignatureOptions{
			IncludeRawBehavior:  true,
			IncludeSourceRefs:   true,
			NormalizeUserPaths:  true,
			IncludeProcessNoise: false,
		},
	}

	data, err := json.Marshal(signature)
	if err != nil {
		t.Fatalf("marshal signature: %v", err)
	}

	var got Signature
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal signature: %v", err)
	}
	if !reflect.DeepEqual(signature, got) {
		t.Fatalf("round trip mismatch\nwant: %#v\n got: %#v", signature, got)
	}
}

func TestTimelineJSONRoundTrip(t *testing.T) {
	timeline := Timeline{
		SchemaVersion: SchemaVersion,
		RunID:         "run_1",
		Steps: []Step{{
			StepID:      "step_1",
			RunID:       "run_1",
			Kind:        StepEditFile,
			Files:       []string{"src/session.go"},
			Significant: true,
		}},
		Warnings: []string{"missing result"},
	}

	data, err := json.Marshal(timeline)
	if err != nil {
		t.Fatalf("marshal timeline: %v", err)
	}

	var got Timeline
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal timeline: %v", err)
	}
	if !reflect.DeepEqual(timeline, got) {
		t.Fatalf("round trip mismatch\nwant: %#v\n got: %#v", timeline, got)
	}
}
